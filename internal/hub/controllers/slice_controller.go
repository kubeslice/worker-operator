package controllers

import (
	"context"

	meshv1beta1 "bitbucket.org/realtimeai/kubeslice-operator/api/v1beta1"
	"bitbucket.org/realtimeai/kubeslice-operator/internal/logger"
	spokev1alpha1 "bitbucket.org/realtimeai/mesh-apis/pkg/spoke/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type SliceReconciler struct {
	client.Client
	MeshClient client.Client
}

func (r *SliceReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := logger.FromContext(ctx)

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

	log.Info("got slice from hub", "slice", slice)

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

		return r.MeshClient.Status().Update(ctx, meshSlice)
	}

	// TODO: Update when we bring in more fields later on

	return nil
}

func (a *SliceReconciler) InjectClient(c client.Client) error {
	a.Client = c
	return nil
}
