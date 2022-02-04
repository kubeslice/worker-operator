package controllers

import (
	"context"

	spokev1alpha1 "bitbucket.org/realtimeai/mesh-hub/apis/mesh/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type SliceReconciler struct {
	client.Client
}

func (a *SliceReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	logger := log.FromContext(ctx)

	slice := &spokev1alpha1.Slice{}
	err := a.Get(ctx, req.NamespacedName, slice)
	if err != nil {
		return reconcile.Result{}, err
	}

	logger.Info("got slice from hub", "slice", slice.Name)

	return reconcile.Result{}, nil
}

func (a *SliceReconciler) InjectClient(c client.Client) error {
	a.Client = c
	return nil
}
