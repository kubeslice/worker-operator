package controllers

import (
	"context"
	"time"

	meshv1beta1 "bitbucket.org/realtimeai/kubeslice-operator/api/v1beta1"
	"bitbucket.org/realtimeai/kubeslice-operator/internal/logger"
	spokev1alpha1 "bitbucket.org/realtimeai/mesh-apis/pkg/spoke/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type SliceGwReconciler struct {
	client.Client
	MeshClient client.Client
}

func (r *SliceGwReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := logger.FromContext(ctx)

	sliceGw := &spokev1alpha1.SpokeSliceGateway{}
	err := r.Get(ctx, req.NamespacedName, sliceGw)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Return and don't requeue
			log.Info("SliceGw resource not found in hub. Ignoring since object must be deleted")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	log.Info("got sliceGw from hub", "sliceGw", sliceGw.Name)

	// Return if the slice gw resource does not belong to our cluster
	if sliceGw.Spec.SliceClusterId != clusterName {
		return reconcile.Result{}, nil
	}

	meshSliceGwCerts := &corev1.Secret{}
	err = r.MeshClient.Get(ctx, types.NamespacedName{
		Name:      sliceGw.Name,
		Namespace: ControlPlaneNamespace,
	}, meshSliceGwCerts)
	if err != nil {
		if errors.IsNotFound(err) {
			sliceGwCerts := &corev1.Secret{}
			err := r.Get(ctx, req.NamespacedName, sliceGwCerts)
			if err != nil {
				log.Error(err, "unable to fetch slicegw certs from the hub", "sliceGw", sliceGw.Name)
				return reconcile.Result{
					RequeueAfter: 10 * time.Second,
				}, nil
			}
			meshSliceGwCerts := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      sliceGwCerts.ObjectMeta.Name,
					Namespace: ControlPlaneNamespace,
				},
				Data: sliceGwCerts.Data,
			}
			err = r.MeshClient.Create(ctx, meshSliceGwCerts)
			if err != nil {
				log.Error(err, "unable to create secret to store slicegw certs in spoke cluster", "sliceGw", sliceGw.Name)
				return reconcile.Result{}, err
			}
			log.Info("sliceGw secret created in spoke cluster")
		} else {
			log.Error(err, "unable to fetch slicegw certs from the spoke", "sliceGw", sliceGw.Name)
			return reconcile.Result{}, err
		}
	}

	sliceGwName := sliceGw.Name
	sliceName := sliceGw.Spec.Slice

	meshSliceGw := &meshv1beta1.SliceGateway{}

	sliceGwRef := client.ObjectKey{
		Name:      sliceGwName,
		Namespace: ControlPlaneNamespace,
	}

	err = r.MeshClient.Get(ctx, sliceGwRef, meshSliceGw)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, create it in the spoke cluster
			log.Info("SliceGw resource not found in spoke cluster, creating")
			meshSliceGw = &meshv1beta1.SliceGateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      sliceGwName,
					Namespace: ControlPlaneNamespace,
				},
				Spec: meshv1beta1.SliceGatewaySpec{
					SliceName: sliceName,
				},
			}

			err = r.MeshClient.Create(ctx, meshSliceGw)
			if err != nil {
				log.Error(err, "unable to create sliceGw in spoke cluster", "sliceGw", sliceGwName)
				return reconcile.Result{}, err
			}
			log.Info("sliceGw created in spoke cluster", "sliceGw", sliceGwName)
		} else {
			log.Error(err, "unable to fetch sliceGw in spoke cluster", "sliceGw", sliceGwName)
			return reconcile.Result{}, err
		}
	}

	meshSliceGw.Status.Config = meshv1beta1.SliceGatewayConfig{
		SliceGatewaySubnet:   sliceGw.Spec.SpokeSliceGatewaySubnet,
		SliceGatewayHostType: sliceGw.Spec.SpokeSliceGatewayHostType,
		SliceGatewayStatus:   sliceGw.Spec.SpokeSliceGatewayStatus,
	}
	err = r.MeshClient.Status().Update(ctx, meshSliceGw)
	if err != nil {
		log.Error(err, "unable to update sliceGw status in spoke cluster", "sliceGw", sliceGwName)
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, err
}

func (r *SliceGwReconciler) InjectClient(c client.Client) error {
	r.Client = c
	return nil
}
