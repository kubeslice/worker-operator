package controllers

import (
	"context"

	meshv1beta1 "bitbucket.org/realtimeai/kubeslice-operator/api/v1beta1"
	"bitbucket.org/realtimeai/kubeslice-operator/internal/logger"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *SliceReconciler) getAppPods(ctx context.Context, slice *meshv1beta1.Slice) ([]meshv1beta1.AppPod, error) {
	log := logger.FromContext(ctx).WithValues("type", "app_pod")
	debugLog := log.V(1)

	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.MatchingLabels(labelsForAppPods()),
	}
	if err := r.List(ctx, podList, listOpts...); err != nil {
		log.Error(err, "Failed to list pods")
		return nil, err
	}
	appPods := []meshv1beta1.AppPod{}
	for _, pod := range podList.Items {

		a := pod.Annotations

		if !isAppPodConnectedToSliceRouter(a, "vl3-service-"+slice.Name) {
			// Could get noisy. Review needed.
			debugLog.Info("App pod is not part of the slice", "pod", pod.Name, "slice", slice.Name)
			continue
		}

		if pod.Status.Phase == corev1.PodRunning {
			appPods = append(appPods, meshv1beta1.AppPod{
				PodName:      pod.Name,
				PodNamespace: pod.Namespace,
				PodIP:        pod.Status.PodIP,
			})
		}
	}
	return appPods, nil
}

// labelsForAppPods returns the labels for App pods
func labelsForAppPods() map[string]string {
	return map[string]string{"avesha.io/pod-type": "app"}
}

func isAppPodConnectedToSliceRouter(annotations map[string]string, sliceRouter string) bool {
	return annotations["ns.networkservicemesh.io"] == sliceRouter
}
