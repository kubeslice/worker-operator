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
 *
 *  updateSliceHealth / getComponentStatus coverage. The hub controllers package
 *  is exercised in Linux CI (make test / GitHub Actions); local test runs on
 *  Windows may not compile this package due to vendored netns/iptables deps.
 */

package controllers

import (
	"context"
	"testing"

	controllerv1alpha1 "github.com/kubeslice/apis/pkg/controller/v1alpha1"
	workerv1alpha1 "github.com/kubeslice/apis/pkg/worker/v1alpha1"
	mevents "github.com/kubeslice/kubeslice-monitoring/pkg/events"
	"github.com/kubeslice/kubeslice-monitoring/pkg/metrics"
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	ossEvents "github.com/kubeslice/worker-operator/events"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func healthTestSlice(sliceName string) *workerv1alpha1.WorkerSliceConfig {
	return &workerv1alpha1.WorkerSliceConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sliceName + "-worker",
			Namespace: "project-namespace",
		},
		Spec: workerv1alpha1.WorkerSliceConfigSpec{
			SliceName:                    sliceName,
			SliceType:                    "Application",
			SliceSubnet:                  "10.0.0.0/16",
			SliceIpamType:                "Local",
			IpamClusterOctet:             1,
			OverlayNetworkDeploymentMode: controllerv1alpha1.NetworkType("single-network"),
		},
	}
}

func TestUpdateSliceHealth_skipsWhenNoNetworkDeployment(t *testing.T) {
	client := NewClient()
	mf, err := metrics.NewMetricsFactory(prometheus.NewRegistry(), metrics.MetricsFactoryOptions{})
	assert.NoError(t, err)
	eventRecorder := mevents.NewEventRecorder(client, &runtime.Scheme{}, ossEvents.EventsMap, mevents.EventRecorderOptions{
		Cluster: clusterName,
	})
	r := NewSliceReconciler(client, client, &eventRecorder, mf)
	assert.NoError(t, r.InjectClient(client))

	slice := healthTestSlice("s1")
	slice.Spec.OverlayNetworkDeploymentMode = controllerv1alpha1.NONET
	slice.Status.SliceHealth = &workerv1alpha1.SliceHealth{
		SliceHealthStatus: workerv1alpha1.SliceHealthStatusWarning,
		ComponentStatuses: []workerv1alpha1.ComponentStatus{
			{Component: "stale", ComponentHealthStatus: workerv1alpha1.ComponentHealthStatusError},
		},
	}

	assert.NoError(t, r.updateSliceHealth(context.Background(), slice))

	assert.Equal(t, workerv1alpha1.SliceHealthStatusNormal, slice.Status.SliceHealth.SliceHealthStatus)
	assert.Empty(t, slice.Status.SliceHealth.ComponentStatuses)
	client.AssertNotCalled(t, "List", mock.Anything, mock.Anything, mock.Anything)
}

func TestUpdateSliceHealth_reportsWarningWhenMandatoryComponentsMissingPods(t *testing.T) {
	client := NewClient()
	mf, err := metrics.NewMetricsFactory(prometheus.NewRegistry(), metrics.MetricsFactoryOptions{})
	assert.NoError(t, err)
	eventRecorder := mevents.NewEventRecorder(client, &runtime.Scheme{}, ossEvents.EventsMap, mevents.EventRecorderOptions{
		Cluster: clusterName,
	})
	r := NewSliceReconciler(client, client, &eventRecorder, mf)
	assert.NoError(t, r.InjectClient(client))

	var listCalls int
	client.On("List", mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		listCalls++
		switch obj := args.Get(1).(type) {
		case *corev1.PodList:
			obj.Items = nil
		case *kubeslicev1beta1.SliceGatewayList:
			obj.Items = nil
		}
	})

	slice := healthTestSlice("s2")
	slice.Status.SliceHealth = &workerv1alpha1.SliceHealth{}

	assert.NoError(t, r.updateSliceHealth(context.Background(), slice))

	assert.Equal(t, workerv1alpha1.SliceHealthStatusWarning, slice.Status.SliceHealth.SliceHealthStatus)
	assert.Len(t, slice.Status.SliceHealth.ComponentStatuses, 2, "dns and slice-router should report error when their pods are missing")
	assert.Equal(t, "dns", slice.Status.SliceHealth.ComponentStatuses[0].Component)
	assert.Equal(t, workerv1alpha1.ComponentHealthStatusError, slice.Status.SliceHealth.ComponentStatuses[0].ComponentHealthStatus)
	assert.Equal(t, "slice-router", slice.Status.SliceHealth.ComponentStatuses[1].Component)
	assert.Equal(t, workerv1alpha1.ComponentHealthStatusError, slice.Status.SliceHealth.ComponentStatuses[1].ComponentHealthStatus)
	assert.Equal(t, 6, listCalls, "dns pods + slice-gw list + router pods + istio egress + istio ingress + tunnel gw list")
}

func TestGetComponentStatus_dns_healthyRunningPods(t *testing.T) {
	client := NewClient()
	mf, err := metrics.NewMetricsFactory(prometheus.NewRegistry(), metrics.MetricsFactoryOptions{})
	assert.NoError(t, err)
	eventRecorder := mevents.NewEventRecorder(client, &runtime.Scheme{}, ossEvents.EventsMap, mevents.EventRecorderOptions{
		Cluster: clusterName,
	})
	r := NewSliceReconciler(client, client, &eventRecorder, mf)
	assert.NoError(t, r.InjectClient(client))

	client.On("List", mock.Anything, mock.IsType(&corev1.PodList{}), mock.Anything).Return(nil).Once().Run(func(args mock.Arguments) {
		pl := args.Get(1).(*corev1.PodList)
		pl.Items = []corev1.Pod{{
			ObjectMeta: metav1.ObjectMeta{Name: "kubeslice-dns-xx", Namespace: ControlPlaneNamespace},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
				ContainerStatuses: []corev1.ContainerStatus{{
					Name: "dns",
				}},
			},
		}}
	})

	cs, err := r.getComponentStatus(context.Background(), &components[0], "any-slice")
	assert.NoError(t, err)
	assert.NotNil(t, cs)
	assert.Equal(t, "dns", cs.Component)
	assert.Equal(t, workerv1alpha1.ComponentHealthStatusNormal, cs.ComponentHealthStatus)
}

func TestGetComponentStatus_egress_missingPodsIgnored(t *testing.T) {
	client := NewClient()
	mf, err := metrics.NewMetricsFactory(prometheus.NewRegistry(), metrics.MetricsFactoryOptions{})
	assert.NoError(t, err)
	eventRecorder := mevents.NewEventRecorder(client, &runtime.Scheme{}, ossEvents.EventsMap, mevents.EventRecorderOptions{
		Cluster: clusterName,
	})
	r := NewSliceReconciler(client, client, &eventRecorder, mf)
	assert.NoError(t, r.InjectClient(client))

	client.On("List", mock.Anything, mock.IsType(&corev1.PodList{}), mock.Anything).Return(nil).Once().Run(func(args mock.Arguments) {
		args.Get(1).(*corev1.PodList).Items = nil
	})

	cs, err := r.getComponentStatus(context.Background(), &components[3], "any-slice")
	assert.NoError(t, err)
	assert.Nil(t, cs, "egress uses ignoreMissing: no pods should not surface a component status")
}
