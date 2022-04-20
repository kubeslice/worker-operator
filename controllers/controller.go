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

package controllers

import (
	"context"
	"fmt"
	"strings"

	meshv1beta1 "bitbucket.org/realtimeai/kubeslice-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetSlice returns slice object by slice name
func GetSlice(ctx context.Context, c client.Client, slice string) (*meshv1beta1.Slice, error) {
	s := &meshv1beta1.Slice{}

	err := c.Get(ctx, types.NamespacedName{
		Name:      slice,
		Namespace: ControlPlaneNamespace,
	}, s)
	if err != nil {
		return nil, err
	}

	return s, nil
}

func GetSliceIngressGwPod(ctx context.Context, c client.Client, sliceName string) (*meshv1beta1.AppPod, error) {
	slice, err := GetSlice(ctx, c, sliceName)
	if err != nil {
		return nil, err
	}

	if slice.Status.SliceConfig == nil {
		err := fmt.Errorf("sliceconfig is not reconciled from hub")
		return nil, err
	}

	if slice.Status.SliceConfig.ExternalGatewayConfig == nil ||
		slice.Status.SliceConfig.ExternalGatewayConfig.Ingress == nil ||
		!slice.Status.SliceConfig.ExternalGatewayConfig.Ingress.Enabled {
		return nil, nil
	}

	for i := range slice.Status.AppPods {
		pod := &slice.Status.AppPods[i]
		if strings.Contains(pod.PodName, "ingressgateway") && pod.PodNamespace == ControlPlaneNamespace && pod.NsmIP != "" {
			return pod, nil
		}
	}

	return nil, nil
}

func GetSliceRouterPodNameAndIP(ctx context.Context, c client.Client, sliceName string) (string, string, error) {
	labels := map[string]string{"networkservicemesh.io/impl": "vl3-service-" + sliceName}

	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.MatchingLabels(labels),
	}
	if err := c.List(ctx, podList, listOpts...); err != nil {
		return "", "", err
	}

	for _, pod := range podList.Items {
		if pod.Status.Phase != corev1.PodRunning {
			continue
		}

		return pod.Name, pod.Status.PodIP, nil
	}

	return "", "", nil
}
