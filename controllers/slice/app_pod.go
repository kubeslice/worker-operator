/*
 *  Copyright (c) 2022 Avesha, Inc. All rights reserved.
 *
 *  SPDX-License-Identifier: Apache-2.0
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */

package slice

import (
	"context"
	"time"

	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	"github.com/kubeslice/worker-operator/controllers"
	"github.com/kubeslice/worker-operator/pkg/logger"
	webhook "github.com/kubeslice/worker-operator/pkg/webhook/pod"

	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func isAppPodStatusChanged(current []kubeslicev1beta1.AppPod, old []kubeslicev1beta1.AppPod) bool {
	if len(current) != len(old) {
		return true
	}

	oldPodMap := make(map[string]string)

	for _, op := range old {
		oldPodMap[op.PodName] = op.PodIP
	}

	for _, cp := range current {
		if _, ok := oldPodMap[cp.PodName]; !ok {
			return true
		}
	}

	return false
}

func (r *SliceReconciler) getAppPods(ctx context.Context, slice *kubeslicev1beta1.Slice) ([]kubeslicev1beta1.AppPod, error) {
	log := logger.FromContext(ctx).WithValues("type", "app_pod")
	debugLog := log.V(1)

	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.MatchingLabels(labelsForAppPods(slice.Name)),
	}
	if err := r.List(ctx, podList, listOpts...); err != nil {
		log.Error(err, "Failed to list pods")
		return nil, err
	}

	appPods := []kubeslicev1beta1.AppPod{}

	for _, pod := range podList.Items {
		a := pod.Annotations
		if !isAppPodConnectedToSliceRouter(a, "vl3-service-"+slice.Name) {
			// Could get noisy. Review needed.
			debugLog.Info("App pod is not part of the slice", "pod", pod.Name, "slice", slice.Name)
			continue
		}

		if pod.Status.Phase == corev1.PodRunning {
			appPods = append(appPods, kubeslicev1beta1.AppPod{
				PodName:      pod.Name,
				PodNamespace: pod.Namespace,
				PodIP:        pod.Status.PodIP,
			})
		}
	}

	return appPods, nil
}

// labelsForAppPods returns the labels for App pods
func labelsForAppPods(sliceName string) map[string]string {
	return map[string]string{webhook.PodInjectLabelKey: "app", ApplicationNamespaceSelectorLabelKey: sliceName}
}

func isAppPodConnectedToSliceRouter(annotations map[string]string, sliceRouter string) bool {
	return annotations["ns.networkservicemesh.io"] == sliceRouter
}

// ReconcileAppPod reconciles app pods
func (r *SliceReconciler) ReconcileAppPod(ctx context.Context, slice *kubeslicev1beta1.Slice) (ctrl.Result, error, bool) {
	log := logger.FromContext(ctx).WithValues("type", "app_pod")
	debugLog := log.V(1)

	sliceName := slice.Name

	// Get the list of clients currently connected to the slice router. The list would include
	// both app pods and slice GW pods. It will be compared against the list of app pods obtained
	// from the k8s api using the labels used on app pods. This way the slice GW pods get filtered out.
	podsConnectedToSlice, err := r.getSliceRouterConnectedPods(ctx, sliceName)
	if err != nil {
		log.Error(err, "Failed to get pods connected to slice")
		return ctrl.Result{}, err, true
	}
	debugLog.Info("Got pods connected to slice", "result", podsConnectedToSlice)

	appPods, err := r.getAppPods(ctx, slice)
	debugLog.Info("app pods", "slice", sliceName, "pods", appPods, "err", err)
	if err != nil {
		return ctrl.Result{}, err, true
	}

	updateNeeded := false

	// First, reconcile the current set of pods in the slice namespaces with the pods stored
	// in the slice status.
	if isAppPodStatusChanged(appPods, slice.Status.AppPods) {
		log.Info("Latest app pods do not match the pods in slice status. Update needed", "slice", sliceName)
		updateNeeded = true
		slice.Status.AppPods = appPods
	}

	for i := range slice.Status.AppPods {
		pod := &slice.Status.AppPods[i]
		debugLog.Info("getting app pod connectivity status", "podIp", pod.PodIP, "podName", pod.PodName)
		appPodConnectedToSlice := findAppPodConnectedToSlice(pod.PodName, podsConnectedToSlice)
		// Presence of an nsm interface is good enough for now to consider the app pod as healthy with
		// respect to its connectivity to the slice.
		if appPodConnectedToSlice == nil {
			debugLog.Info("App pod not connected to slice", "podName", pod.PodName)
			if pod.NsmIP != "" {
				pod.NsmIP = ""
				updateNeeded = true
			}
			continue
		}

		if pod.NsmIP != appPodConnectedToSlice.NsmIP {
			debugLog.Info("app pod status changed", "old nsmIp", pod.NsmIP, "new nsmIp", appPodConnectedToSlice.NsmIP)
			pod.NsmIP = appPodConnectedToSlice.NsmIP
			updateNeeded = true
		}
	}

	if updateNeeded {
		slice.Status.AppPodsUpdatedOn = time.Now().Unix()
		err := r.Status().Update(ctx, slice)
		if err != nil {
			log.Error(err, "Failed to update Slice status for app pods")
			return ctrl.Result{}, err, true
		}
	}

	// Reconcile nsm IP label on app pods
	err = r.labelAppPodWithNsmIp(ctx, slice)
	if err != nil {
		log.Error(err, "Failed to label app pods")
		return ctrl.Result{}, err, true
	}

	return ctrl.Result{}, nil, false
}

func (r *SliceReconciler) getSliceRouterConnectedPods(ctx context.Context, sliceName string) ([]kubeslicev1beta1.AppPod, error) {
	sidecarGrpcAddress := sliceRouterDeploymentNamePrefix + sliceName + ":5000"
	return r.WorkerRouterClient.GetClientConnectionInfo(ctx, sidecarGrpcAddress)
}

func findAppPodConnectedToSlice(podName string, connectedPods []kubeslicev1beta1.AppPod) *kubeslicev1beta1.AppPod {
	for _, v := range connectedPods {
		if v.PodName == podName {
			return &v
		}
	}
	return nil
}

func (r *SliceReconciler) labelAppPodWithNsmIp(ctx context.Context, slice *kubeslicev1beta1.Slice) error {
	log := logger.FromContext(ctx).WithValues("type", "app_pod")
	debugLog := log.V(1)

	corePodList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.MatchingLabels(labelsForAppPods(slice.Name)),
	}
	if err := r.List(ctx, corePodList, listOpts...); err != nil {
		log.Error(err, "Label app pods error: Failed to list pods")
		return err
	}

	for _, pod := range corePodList.Items {
		updateNeeded := false
		podInSlice := findAppPodConnectedToSlice(pod.Name, slice.Status.AppPods)
		if podInSlice == nil {
			continue
		}
		labels := pod.GetLabels()
		if labels == nil {
			labels = make(map[string]string)
		}

		annotations := pod.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
		}

		_, ok := labels[controllers.NSMIPLabelSelectorKey]
		if !ok {
			updateNeeded = true
			labels[controllers.NSMIPLabelSelectorKey] = podInSlice.NsmIP
		} else {
			if labels[controllers.NSMIPLabelSelectorKey] != podInSlice.NsmIP {
				updateNeeded = true
				labels[controllers.NSMIPLabelSelectorKey] = podInSlice.NsmIP
			}
		}

		// Add Istio exclusion for NSM communication
		expectedExcludePort := "5000"
		if annotations["traffic.sidecar.istio.io/excludeOutboundPorts"] != expectedExcludePort {
			updateNeeded = true
			annotations["traffic.sidecar.istio.io/excludeOutboundPorts"] = expectedExcludePort
		}

		if updateNeeded {
			pod.SetLabels(labels)
			pod.SetAnnotations(annotations)
			err := r.Update(ctx, &pod)
			if err != nil {
				log.Error(err, "Failed to update NSM IP label and Istio annotations for app pod", "pod", pod.Name)
				return err
			}
			debugLog.Info("App pod label and Istio exclusion added/updated", "pod", pod.Name, "nsmIP", podInSlice.NsmIP)
		}
	}

	return nil
}
