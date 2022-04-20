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

	meshv1beta1 "bitbucket.org/realtimeai/kubeslice-operator/api/v1beta1"
	"bitbucket.org/realtimeai/kubeslice-operator/controllers"
	"bitbucket.org/realtimeai/kubeslice-operator/internal/logger"
	"bitbucket.org/realtimeai/kubeslice-operator/internal/router"

	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
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

// ReconcileAppPod reconciles app pods
func (r *SliceReconciler) ReconcileAppPod(ctx context.Context, slice *meshv1beta1.Slice) (ctrl.Result, error, bool) {
	log := logger.FromContext(ctx).WithValues("type", "app_pod")
	debugLog := log.V(1)

	sliceName := slice.Name

	// Get the list of clients currently connected to the slice router. The list would include
	// both app pods and slice GW pods. It will be compared against the list of app pods obtained
	// from the k8s api using the labels used on app pods. This way the slice GW pods get filtered out.
	podsConnectedToSlice, err := getSliceRouterConnectedPods(ctx, sliceName)
	if err != nil {
		log.Error(err, "Failed to get pods connected to slice")
		return ctrl.Result{}, err, true
	}
	debugLog.Info("Got pods connected to slice", "result", podsConnectedToSlice)
	corePodList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.MatchingLabels(labelsForAppPods()),
	}
	if err := r.List(ctx, corePodList, listOpts...); err != nil {
		log.Error(err, "Failed to list pods")
		return ctrl.Result{}, err, true
	}
	for i := range slice.Status.AppPods {
		pod := &slice.Status.AppPods[i]
		debugLog.Info("getting app pod connectivity status", "podIp", pod.PodIP, "podName", pod.PodName)
		appPodConnectedToSlice := findAppPodConnectedToSlice(pod.PodName, podsConnectedToSlice)
		// Presence of an nsm interface is good enough for now to consider the app pod as healthy with
		// respect to its connectivity to the slice.
		if appPodConnectedToSlice == nil {
			debugLog.Info("App pod unhealthy: Not connected to slice", "podName", pod.PodName)

			if pod.NsmIP != "" || pod.NsmPeerIP != "" {
				pod.NsmIP = ""
				pod.NsmPeerIP = ""
				slice.Status.AppPodsUpdatedOn = time.Now().Unix()
				debugLog.Info("Setting app pod nsm and peer Ip to null")
				err = r.Status().Update(ctx, slice)
				if err != nil {
					log.Error(err, "Failed to update Slice status for app pods which sets nsmip and peerip to null")
					return ctrl.Result{}, err, true
				}
				debugLog.Info("App pod status updated and nsmip peerip set to null")
				return ctrl.Result{}, nil, true
			}
			debugLog.Info("App pod unhealthy, skipping reconciliation")
			continue
		}

		if pod.NsmIP != appPodConnectedToSlice.NsmIP {
			pod.NsmIP, pod.NsmPeerIP, pod.NsmInterface =
				appPodConnectedToSlice.NsmIP, appPodConnectedToSlice.NsmPeerIP, appPodConnectedToSlice.NsmInterface
			slice.Status.AppPodsUpdatedOn = time.Now().Unix()
			log.Info("app pod status changed", "nsmIp", pod.NsmIP, "peerIp", pod.NsmPeerIP)

			//Label pod with NSM IP
			podIndex := findPodInPodList(pod.PodName, corePodList)
			if podIndex == -1 {
				debugLog.Info("Could not find pod in podList, skipping nsmIP labelling")
			} else {
				corePod := corePodList.Items[podIndex]
				labels := corePod.GetLabels()
				labels[controllers.NSMIPLabelSelectorKey] = pod.NsmIP
				corePod.SetLabels(labels)

				err := r.Update(ctx, &corePod)
				if err != nil {
					log.Error(err, "Failed to update NSM IP label for app pod")
					return ctrl.Result{}, err, true
				}
				debugLog.Info("App pod label added/updated", "nsmIP", pod.NsmIP)
			}
			err = r.Status().Update(ctx, slice)
			if err != nil {
				log.Error(err, "Failed to update Slice status for app pods")
				return ctrl.Result{}, err, true
			}
			log.Info("App pod status updated")
			return ctrl.Result{}, nil, true
		}
	}
	return ctrl.Result{}, nil, false
}

func getSliceRouterConnectedPods(ctx context.Context, sliceName string) ([]meshv1beta1.AppPod, error) {
	sidecarGrpcAddress := sliceRouterDeploymentNamePrefix + sliceName + ":5000"
	return router.GetClientConnectionInfo(ctx, sidecarGrpcAddress)
}

func findAppPodConnectedToSlice(podName string, connectedPods []meshv1beta1.AppPod) *meshv1beta1.AppPod {
	for _, v := range connectedPods {
		if v.PodName == podName {
			return &v
		}
	}
	return nil
}

// findPodInPodList returns the index of the pod in the podList that matches
// the input pod to be found
func findPodInPodList(podName string, podList *corev1.PodList) int {
	for i := range podList.Items {
		pod := podList.Items[i]
		if podName == pod.Name {
			return i
		}
	}
	return -1
}
