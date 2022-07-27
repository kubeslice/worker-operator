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
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	log                        logr.Logger = logger.NewLogger()
	allowedNamespacesByDefault             = []string{"kubeslice-system", "kube-system", "istio-system"}
)

const (
	ApplicationNamespaceSelectorLabelKey = "kubeslice.io/slice"
	AllowedNamespaceSelectorLabelKey     = "kubeslice.io/namespace"
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

// GetSliceIngressGwPod returns a bool to indicate if ingress gateway is enabled for the slice,  a struct of type AppPod that
// contains info on the ingress gw pod and an error var to indicate if an error was encountered while executing the func.
func GetSliceIngressGwPod(ctx context.Context, c client.Client, sliceName string) (bool, *kubeslicev1beta1.AppPod, error) {
	slice, err := GetSlice(ctx, c, sliceName)
	if err != nil {
		return false, nil, err
	}

	if slice.Status.SliceConfig == nil {
		err := fmt.Errorf("sliceconfig is not reconciled from hub")
		return false, nil, err
	}

	if slice.Status.SliceConfig.ExternalGatewayConfig == nil ||
		slice.Status.SliceConfig.ExternalGatewayConfig.Ingress == nil ||
		!slice.Status.SliceConfig.ExternalGatewayConfig.Ingress.Enabled {
		return false, nil, nil
	}

	for i := range slice.Status.AppPods {
		pod := &slice.Status.AppPods[i]
		if strings.Contains(pod.PodName, "ingressgateway") && pod.PodNamespace == ControlPlaneNamespace && pod.NsmIP != "" {
			return true, pod, nil
		}
	}

	return true, nil, nil
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

func ContructNetworkPolicyObject(ctx context.Context, slice *kubeslicev1beta1.Slice, appNs string) *networkingv1.NetworkPolicy {
	netPolicy := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      slice.Name + "-" + appNs,
			Namespace: appNs,
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
				networkingv1.PolicyTypeEgress,
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				networkingv1.NetworkPolicyIngressRule{
					From: []networkingv1.NetworkPolicyPeer{
						networkingv1.NetworkPolicyPeer{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{ApplicationNamespaceSelectorLabelKey: slice.Name},
							},
						},
					},
				},
			},
			Egress: []networkingv1.NetworkPolicyEgressRule{
				networkingv1.NetworkPolicyEgressRule{
					To: []networkingv1.NetworkPolicyPeer{
						networkingv1.NetworkPolicyPeer{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{ApplicationNamespaceSelectorLabelKey: slice.Name},
							},
						},
					},
				},
			},
		},
	}

	var cfgAllowedNsList []string
	cfgAllowedNsList = slice.Status.SliceConfig.NamespaceIsolationProfile.AllowedNamespaces
	// traffic from "kubeslice-system","istio-system","kube-system" namespaces is allowed by default
	for _, v := range allowedNamespacesByDefault {
		if !exists(cfgAllowedNsList, v) {
			cfgAllowedNsList = append(cfgAllowedNsList, v)
		}
	}
	for _, allowedNs := range cfgAllowedNsList {
		ingressRule := networkingv1.NetworkPolicyPeer{
			NamespaceSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{AllowedNamespaceSelectorLabelKey: allowedNs},
			},
		}
		netPolicy.Spec.Ingress[0].From = append(netPolicy.Spec.Ingress[0].From, ingressRule)

		egressRule := networkingv1.NetworkPolicyPeer{
			NamespaceSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{AllowedNamespaceSelectorLabelKey: allowedNs},
			},
		}
		netPolicy.Spec.Egress[0].To = append(netPolicy.Spec.Egress[0].To, egressRule)
	}
	return netPolicy
}

func exists(i []string, o string) bool {
	for _, v := range i {
		if v == o {
			return true
		}
	}
	return false
}
