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

	"github.com/go-logr/logr"
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	"github.com/kubeslice/worker-operator/pkg/logger"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	log logr.Logger = logger.NewLogger()
)

// GetSlice returns slice object by slice name
func GetSlice(ctx context.Context, c client.Client, slice string) (*kubeslicev1beta1.Slice, error) {
	s := &kubeslicev1beta1.Slice{}

	err := c.Get(ctx, types.NamespacedName{
		Name:      slice,
		Namespace: ControlPlaneNamespace,
	}, s)
	if err != nil {
		return nil, err
	}

	return s, nil
}

func GetSliceIngressGwPod(ctx context.Context, c client.Client, sliceName string) (*kubeslicev1beta1.AppPod, error) {
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

// SliceAppNamespaceConfigured returns true if the namespace is present in the application namespace list
// configured for the slice
func SliceAppNamespaceConfigured(ctx context.Context, slice string, namespace string) (bool, error) {
	scheme := runtime.NewScheme()
	utilruntime.Must(kubeslicev1beta1.AddToScheme(scheme))
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	c, err := client.New(ctrl.GetConfigOrDie(), client.Options{
		Scheme: scheme,
	})
	if err != nil {
		log.Error(err, "Creating new client for webhook call->SliceAppNamespaceConfigured")
		return false, err
	}

	s, err := GetSlice(ctx, c, slice)
	if err != nil {
		return false, err
	}

	for _, ns := range s.Status.SliceConfig.NamespaceIsolationProfile.ApplicationNamespaces {
		if ns == namespace {
			return true, nil
		}
	}
	return false, nil
}
