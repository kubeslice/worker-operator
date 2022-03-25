package controllers

import (
	"context"

	meshv1beta1 "bitbucket.org/realtimeai/kubeslice-operator/api/v1beta1"
	"bitbucket.org/realtimeai/kubeslice-operator/internal/logger"
	spokev1alpha1 "bitbucket.org/realtimeai/mesh-apis/pkg/spoke/v1alpha1"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type SliceReconciler struct {
	client.Client
	Log        logr.Logger
	MeshClient client.Client
}

var sliceFinalizer = "hub.kubeslice.io/hubSpokeSlice-finalizer"

func (r *SliceReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := r.Log.WithValues("sliceconfig", req.NamespacedName)
	ctx = logger.WithLogger(ctx, log)
	debuglog := log.V(1)
	slice := &spokev1alpha1.SpokeSliceConfig{}
	err := r.Get(ctx, req.NamespacedName, slice)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Return and don't requeue
			log.Info("Slice resource not found in hub. Ignoring since object must be deleted")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	log.Info("got slice from hub", "slice", slice.Name)
	debuglog.Info("got slice from hub", "slice", slice)
	// examine DeletionTimestamp to determine if object is under deletion
	if slice.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !controllerutil.ContainsFinalizer(slice, sliceFinalizer) {
			controllerutil.AddFinalizer(slice, sliceFinalizer)
			if err := r.Update(ctx, slice); err != nil {
				return reconcile.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(slice, sliceFinalizer) {
			// our finalizer is present, so lets handle any external dependency
			if err := r.deleteSliceResourceOnSpoke(ctx, slice); err != nil {
				// if fail to delete the external dependency here, return with error
				// so that it can be retried
				return reconcile.Result{}, err
			}
			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(slice, sliceFinalizer)
			if err := r.Update(ctx, slice); err != nil {
				return reconcile.Result{}, err
			}
		}
		// Stop reconciliation as the item is being deleted
		return reconcile.Result{}, nil
	}

	sliceName := slice.Spec.SliceName
	meshSlice := &meshv1beta1.Slice{}
	sliceRef := client.ObjectKey{
		Name:      sliceName,
		Namespace: ControlPlaneNamespace,
	}

	err = r.MeshClient.Get(ctx, sliceRef, meshSlice)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, create it in the spoke cluster
			log.Info("Slice resource not found in spoke cluster, creating")
			s := &meshv1beta1.Slice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      sliceName,
					Namespace: ControlPlaneNamespace,
				},
				Spec: meshv1beta1.SliceSpec{},
			}

			err = r.MeshClient.Create(ctx, s)
			if err != nil {
				log.Error(err, "unable to create slice in spoke cluster", "slice", s)
				return reconcile.Result{}, err
			}
			log.Info("slice created in spoke cluster")

			err = r.updateSliceConfig(ctx, s, slice)
			if err != nil {
				log.Error(err, "unable to update slice status in spoke cluster", "slice", s)
				return reconcile.Result{}, err
			}
			log.Info("slice status updated in spoke cluster")

			return reconcile.Result{}, nil
		}

		return reconcile.Result{}, err
	}

	err = r.updateSliceConfig(ctx, meshSlice, slice)
	if err != nil {
		log.Error(err, "unable to update slice status in spoke cluster", "slice", meshSlice)
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *SliceReconciler) updateSliceConfig(ctx context.Context, meshSlice *meshv1beta1.Slice, spokeSlice *spokev1alpha1.SpokeSliceConfig) error {
	if meshSlice.Status.SliceConfig == nil {
		meshSlice.Status.SliceConfig = &meshv1beta1.SliceConfig{
			SliceDisplayName: spokeSlice.Spec.SliceName,
			SliceSubnet:      spokeSlice.Spec.SliceSubnet,
			SliceIpam: meshv1beta1.SliceIpamConfig{
				SliceIpamType:    spokeSlice.Spec.SliceIpamType,
				IpamClusterOctet: spokeSlice.Spec.IpamClusterOctet,
			},
			SliceType: spokeSlice.Spec.SliceType,
		}
	}

	if meshSlice.Status.SliceConfig.SliceSubnet == "" {
		meshSlice.Status.SliceConfig.SliceSubnet = spokeSlice.Spec.SliceSubnet
	}

	if meshSlice.Status.SliceConfig.SliceIpam.IpamClusterOctet == 0 {
		meshSlice.Status.SliceConfig.SliceIpam.IpamClusterOctet = spokeSlice.Spec.IpamClusterOctet
	}

	meshSlice.Status.SliceConfig.QosProfileDetails = meshv1beta1.QosProfileDetails{
		QueueType:               spokeSlice.Spec.QosProfileDetails.QueueType,
		BandwidthCeilingKbps:    spokeSlice.Spec.QosProfileDetails.BandwidthCeilingKbps,
		BandwidthGuaranteedKbps: spokeSlice.Spec.QosProfileDetails.BandwidthGuaranteedKbps,
		DscpClass:               spokeSlice.Spec.QosProfileDetails.DscpClass,
		TcType:                  spokeSlice.Spec.QosProfileDetails.TcType,
		Priority:                spokeSlice.Spec.QosProfileDetails.Priority,
	}

	extGwCfg := spokeSlice.Spec.ExternalGatewayConfig
	meshSlice.Status.SliceConfig.ExternalGatewayConfig = &meshv1beta1.ExternalGatewayConfig{
		GatewayType: extGwCfg.GatewayType,
		Egress: &meshv1beta1.ExternalGatewayConfigOptions{
			Enabled: extGwCfg.Egress.Enabled,
		},
		Ingress: &meshv1beta1.ExternalGatewayConfigOptions{
			Enabled: extGwCfg.Ingress.Enabled,
		},
		NsIngress: &meshv1beta1.ExternalGatewayConfigOptions{
			Enabled: extGwCfg.NsIngress.Enabled,
		},
	}

	return r.MeshClient.Status().Update(ctx, meshSlice)
}

func (a *SliceReconciler) InjectClient(c client.Client) error {
	a.Client = c
	return nil
}

func (r *SliceReconciler) deleteSliceResourceOnSpoke(ctx context.Context, slice *spokev1alpha1.SpokeSliceConfig) error {
	log := logger.FromContext(ctx)
	sliceOnSpoke := &meshv1beta1.Slice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      slice.Spec.SliceName,
			Namespace: ControlPlaneNamespace,
		},
	}
	if err := r.MeshClient.Delete(ctx, sliceOnSpoke); err != nil {
		return err
	}
	log.Info("Deleted Slice CR on spoke cluster", "slice", sliceOnSpoke.Name)
	return nil
}
