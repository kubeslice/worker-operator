package serviceexport

import (
	"context"

	kubeslicev1beta1 "github.com/kubeslice/operator/api/v1beta1"
	"github.com/kubeslice/operator/controllers"
	"github.com/kubeslice/operator/internal/logger"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ReconcileAppPod reconciles serviceexport app pods
func (r *Reconciler) ReconcileAppPod(
	ctx context.Context, serviceexport *kubeslicev1beta1.ServiceExport) (ctrl.Result, error, bool) {
	log := logger.FromContext(ctx).WithValues("type", "app pod")
	debugLog := log.V(1)

	appPods, err := r.getAppPods(ctx, serviceexport)
	if err != nil {
		log.Error(err, "Failed to fetch app pods for serviceexport")
		return ctrl.Result{}, err, true
	}

	if !isServiceAppPodChanged(appPods, serviceexport.Status.Pods) {
		// No change in service pods, don't do anything and return
		return ctrl.Result{}, nil, false
	}
	serviceexport.Status.Pods = appPods
	serviceexport.Status.LastSync = 0                                        // Force sync to hub in next loop
	serviceexport.Status.ExportStatus = kubeslicev1beta1.ExportStatusPending // Set status to pending
	serviceexport.Status.AvailableEndpoints = len(appPods)

	log.Info("updating service app pods")
	debugLog.Info("updating service app pods", "app pods", appPods)
	err = r.Status().Update(ctx, serviceexport)
	if err != nil {
		log.Error(err, "Failed to update ServiceExport status for app pods")
		return ctrl.Result{}, err, true
	}
	log.Info("Service App pod status updated")
	return ctrl.Result{Requeue: true}, nil, true
}

func (r *Reconciler) getAppPods(ctx context.Context, serviceexport *kubeslicev1beta1.ServiceExport) ([]kubeslicev1beta1.ServicePod, error) {
	log := logger.FromContext(ctx).WithValues("type", "app pod")
	debugLog := log.V(1)

	podList := &corev1.PodList{}

	listOpts := []client.ListOption{
		client.MatchingLabels(serviceexport.Spec.Selector.MatchLabels),
		client.InNamespace(serviceexport.Namespace),
	}
	if err := r.List(ctx, podList, listOpts...); err != nil {
		log.Error(err, "Failed to list pods")
		return nil, err
	}

	debugLog.Info("pods matching labels", "count", len(podList.Items))

	appPods := []kubeslicev1beta1.ServicePod{}
	appPodsInSlice, err := getAppPodsInSlice(ctx, r.Client, serviceexport.Spec.Slice)
	if err != nil {
		log.Error(err, "Unable to fetch app pods in slice")
		return nil, err
	}

	debugLog.Info("app pods in slice", "pods", appPodsInSlice)
	for _, pod := range podList.Items {
		if pod.Status.Phase == corev1.PodRunning {
			dnsName := pod.Name + "." + getClusterName() + "." + serviceexport.Name + "." + serviceexport.Namespace + ".svc.slice.local"
			ip := getNsmIP(&pod, appPodsInSlice)
			appPods = append(appPods, kubeslicev1beta1.ServicePod{
				Name:    pod.Name,
				NsmIP:   ip,
				PodIp:   pod.Status.PodIP,
				DNSName: dnsName,
			})
		}
	}
	debugLog.Info("valid app pods in slice", "pods", appPods)
	return appPods, nil
}

func (r *Reconciler) DeleteServiceExportResources(ctx context.Context, serviceexport *kubeslicev1beta1.ServiceExport) error {
	log := logger.FromContext(ctx)
	slice, err := controllers.GetSlice(ctx, r.Client, serviceexport.Spec.Slice)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		log.Error(err, "Unable to fetch slice for serviceexport cleanup")
		return err
	}

	if slice.Status.SliceConfig == nil {
		return nil
	}

	return r.DeleteIstioResources(ctx, serviceexport, slice)
}
