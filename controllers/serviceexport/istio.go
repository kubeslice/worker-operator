package serviceexport

import (
	"context"
	"fmt"

	meshv1beta1 "bitbucket.org/realtimeai/kubeslice-operator/api/v1beta1"
	"bitbucket.org/realtimeai/kubeslice-operator/controllers"
	"bitbucket.org/realtimeai/kubeslice-operator/internal/logger"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *Reconciler) ReconcileIstio(ctx context.Context, serviceexport *meshv1beta1.ServiceExport) (ctrl.Result, error, bool) {
	log := logger.FromContext(ctx).WithValues("type", "Istio")
	debugLog := log.V(1)

	slice, err := controllers.GetSlice(ctx, r.Client, serviceexport.Spec.Slice)
	if err != nil {
		log.Error(err, "Unable to fetch slice for serviceexport")
		return ctrl.Result{}, err, true
	}

	if slice.Status.SliceConfig == nil {
		err := fmt.Errorf("sliceconfig not reconciled from hub")
		return ctrl.Result{}, err, true
	}

	if slice.Status.SliceConfig.ExternalGatewayConfig == nil ||
		slice.Status.SliceConfig.ExternalGatewayConfig.Ingress == nil ||
		slice.Status.SliceConfig.ExternalGatewayConfig.Ingress.Enabled == false {
		debugLog.Info("istio ingress not enabled for slice, skipping reconcilation")
		return ctrl.Result{}, nil, false
	}

	debugLog.Info("reconciling istio")

	res, err, requeue := r.ReconcileServiceEntries(ctx, serviceexport)
	if requeue {
		return res, err, requeue
	}

	res, err, requeue = r.ReconcileVirtualService(ctx, serviceexport)
	if requeue {
		return res, err, requeue
	}

	return ctrl.Result{}, nil, false
}

func (r *Reconciler) DeleteIstioResources(ctx context.Context, serviceexport *meshv1beta1.ServiceExport, slice *meshv1beta1.Slice) error {
	// We should only clean up resources that were created in the control plane namespace. Setting the service export object
	// in the app namespace as the owner reference does not clean up resources in other namespaces.
	// Resources in application namespaces are garbage collected because the owner reference for them is set to be the
	// service export object, so we do not have to delete them explicitly here.
	if slice.Status.SliceConfig.ExternalGatewayConfig == nil ||
		slice.Status.SliceConfig.ExternalGatewayConfig.Ingress == nil ||
		slice.Status.SliceConfig.ExternalGatewayConfig.Ingress.Enabled == false {
		return nil
	}

	err := r.DeleteIstioServiceEntries(ctx, serviceexport)
	if err != nil {
		return err
	}

	err = r.DeleteIstioVirtualServices(ctx, serviceexport)
	if err != nil {
		return err
	}

	return nil
}
