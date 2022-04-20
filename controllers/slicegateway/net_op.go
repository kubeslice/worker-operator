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

package slicegateway

import (
	"context"

	meshv1beta1 "bitbucket.org/realtimeai/kubeslice-operator/api/v1beta1"
	"bitbucket.org/realtimeai/kubeslice-operator/internal/logger"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *SliceGwReconciler) getNetOpPods(ctx context.Context, slicegateway *meshv1beta1.SliceGateway) error {
	log := logger.FromContext(ctx).WithValues("type", "net_op")
	debugLog := log.V(1)

	netOpPods, err := GetNetOpPods(ctx, slicegateway.Namespace, r.List)
	if err != nil {
		log.Error(err, "Failed to list net_op pods")
		return err
	}
	debugLog.Info("got netop pods", "pods", netOpPods)
	r.NetOpPods = netOpPods
	return nil
}

// GetNetOpPods returns the netop pods in the cluster
func GetNetOpPods(ctx context.Context, namespace string,
	listFn func(context.Context, client.ObjectList, ...client.ListOption) error) ([]NetOpPod, error) {
	labels := map[string]string{"app": "app_net_op"}

	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingLabels(labels),
	}
	if err := listFn(ctx, podList, listOpts...); err != nil {
		return nil, err
	}

	pods := []NetOpPod{}
	for _, pod := range podList.Items {
		pods = append(pods, NetOpPod{
			PodName: pod.Name,
			PodIP:   pod.Status.PodIP,
			Node:    pod.Spec.NodeName,
		})
	}
	return pods, nil
}
