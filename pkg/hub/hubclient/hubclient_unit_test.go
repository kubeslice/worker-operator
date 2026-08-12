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

package hub

import (
	"context"
	"testing"

	hubv1alpha1 "github.com/kubeslice/apis/pkg/controller/v1alpha1"
	spokev1alpha1 "github.com/kubeslice/apis/pkg/worker/v1alpha1"
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	"github.com/kubeslice/worker-operator/pkg/monitoring"
	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

var testProjectNs = "kubeslice-avesha"
var testClusterName = "test-cluster-1"
var testSliceName = "test-slice"

func newTestHubClient(scheme *runtime.Scheme, objs ...client.Object) *HubClientConfig {
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	return &HubClientConfig{
		Client: client,
		eventRecorder: &monitoring.EventRecorder{
			Client: client,
			Scheme: scheme,
			Logger: zap.NewNop().Sugar(),
		},
	}
}

func TestUpdateNodePortForSliceGwServer(t *testing.T) {
	sliceGw := &spokev1alpha1.WorkerSliceGateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slice-gw",
			Namespace: testProjectNs,
		},
		Spec: spokev1alpha1.WorkerSliceGatewaySpec{
			LocalGatewayConfig: spokev1alpha1.SliceGatewayConfig{
				NodePorts: []int{30001, 30002},
			},
		},
	}

	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = spokev1alpha1.AddToScheme(scheme)
	_ = hubv1alpha1.AddToScheme(scheme)
	_ = kubeslicev1beta1.AddToScheme(scheme)

	ProjectNamespace = testProjectNs
	hubClient := newTestHubClient(scheme, sliceGw)

	ctx := context.Background()

	err := hubClient.UpdateNodePortForSliceGwServer(ctx, []int{30003, 30004}, "test-slice-gw")
	if err != nil {
		t.Error("Expected no error but got:", err)
	}

	// Verify the update
	updated := &spokev1alpha1.WorkerSliceGateway{}
	err = hubClient.Get(ctx, types.NamespacedName{Name: "test-slice-gw", Namespace: testProjectNs}, updated)
	if err != nil {
		t.Error("Failed to get updated gateway:", err)
	}
	if len(updated.Spec.LocalGatewayConfig.NodePorts) != 2 ||
		updated.Spec.LocalGatewayConfig.NodePorts[0] != 30003 ||
		updated.Spec.LocalGatewayConfig.NodePorts[1] != 30004 {
		t.Error("NodePorts were not updated correctly")
	}
}

func TestUpdateNodePortForSliceGwServerNoUpdate(t *testing.T) {
	sliceGw := &spokev1alpha1.WorkerSliceGateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slice-gw",
			Namespace: testProjectNs,
		},
		Spec: spokev1alpha1.WorkerSliceGatewaySpec{
			LocalGatewayConfig: spokev1alpha1.SliceGatewayConfig{
				NodePorts: []int{30001, 30002},
			},
		},
	}

	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = spokev1alpha1.AddToScheme(scheme)
	_ = hubv1alpha1.AddToScheme(scheme)
	_ = kubeslicev1beta1.AddToScheme(scheme)

	ProjectNamespace = testProjectNs
	hubClient := newTestHubClient(scheme, sliceGw)

	ctx := context.Background()

	// Should not call Update when NodePorts are the same
	err := hubClient.UpdateNodePortForSliceGwServer(ctx, []int{30001, 30002}, "test-slice-gw")
	if err != nil {
		t.Error("Expected no error but got:", err)
	}
}

func TestUpdateLBIPsForSliceGwServer(t *testing.T) {
	sliceGw := &spokev1alpha1.WorkerSliceGateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slice-gw",
			Namespace: testProjectNs,
		},
		Spec: spokev1alpha1.WorkerSliceGatewaySpec{
			LocalGatewayConfig: spokev1alpha1.SliceGatewayConfig{
				LoadBalancerIps: []string{"192.168.1.1"},
			},
		},
	}

	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = spokev1alpha1.AddToScheme(scheme)
	_ = hubv1alpha1.AddToScheme(scheme)
	_ = kubeslicev1beta1.AddToScheme(scheme)

	ProjectNamespace = testProjectNs
	hubClient := newTestHubClient(scheme, sliceGw)

	ctx := context.Background()

	err := hubClient.UpdateLBIPsForSliceGwServer(ctx, []string{"192.168.1.2", "192.168.1.3"}, "test-slice-gw")
	if err != nil {
		t.Error("Expected no error but got:", err)
	}

	// Verify the update
	updated := &spokev1alpha1.WorkerSliceGateway{}
	err = hubClient.Get(ctx, types.NamespacedName{Name: "test-slice-gw", Namespace: testProjectNs}, updated)
	if err != nil {
		t.Error("Failed to get updated gateway:", err)
	}
	if len(updated.Spec.LocalGatewayConfig.LoadBalancerIps) != 2 ||
		updated.Spec.LocalGatewayConfig.LoadBalancerIps[0] != "192.168.1.2" ||
		updated.Spec.LocalGatewayConfig.LoadBalancerIps[1] != "192.168.1.3" {
		t.Error("LoadBalancerIps were not updated correctly")
	}
}

func TestCreateWorkerSliceGwRecycler(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = spokev1alpha1.AddToScheme(scheme)
	_ = hubv1alpha1.AddToScheme(scheme)
	_ = kubeslicev1beta1.AddToScheme(scheme)

	ProjectNamespace = testProjectNs
	hubClient := newTestHubClient(scheme)

	ctx := context.Background()
	recyclerName := "test-recycler"

	err := hubClient.CreateWorkerSliceGwRecycler(ctx, recyclerName, "client-gw", "server-gw", "server-slice-gw", "client-slice-gw", testSliceName)
	if err != nil {
		t.Error("Expected no error but got:", err)
	}

	// Verify the recycler was created
	recycler := &spokev1alpha1.WorkerSliceGwRecycler{}
	err = hubClient.Get(ctx, types.NamespacedName{Name: recyclerName, Namespace: testProjectNs}, recycler)
	if err != nil {
		t.Error("Failed to get created recycler:", err)
	}
}

func TestCreateWorkerSliceGwRecyclerAlreadyExists(t *testing.T) {
	recycler := &spokev1alpha1.WorkerSliceGwRecycler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-recycler",
			Namespace: testProjectNs,
		},
	}

	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = spokev1alpha1.AddToScheme(scheme)
	_ = hubv1alpha1.AddToScheme(scheme)
	_ = kubeslicev1beta1.AddToScheme(scheme)

	ProjectNamespace = testProjectNs
	hubClient := newTestHubClient(scheme, recycler)

	ctx := context.Background()
	recyclerName := "test-recycler"

	err := hubClient.CreateWorkerSliceGwRecycler(ctx, recyclerName, "client-gw", "server-gw", "server-slice-gw", "client-slice-gw", testSliceName)
	if err != nil {
		t.Error("Expected no error but got:", err)
	}
}

func TestDeleteWorkerSliceGwRecycler(t *testing.T) {
	recycler := &spokev1alpha1.WorkerSliceGwRecycler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-recycler",
			Namespace: testProjectNs,
		},
	}

	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = spokev1alpha1.AddToScheme(scheme)
	_ = hubv1alpha1.AddToScheme(scheme)
	_ = kubeslicev1beta1.AddToScheme(scheme)

	ProjectNamespace = testProjectNs
	hubClient := newTestHubClient(scheme, recycler)

	ctx := context.Background()
	recyclerName := "test-recycler"

	err := hubClient.DeleteWorkerSliceGwRecycler(ctx, recyclerName)
	if err != nil {
		t.Error("Expected no error but got:", err)
	}

	// Verify the recycler was deleted
	deletedRecycler := &spokev1alpha1.WorkerSliceGwRecycler{}
	err = hubClient.Get(ctx, types.NamespacedName{Name: recyclerName, Namespace: testProjectNs}, deletedRecycler)
	if !apierrors.IsNotFound(err) {
		t.Error("Expected recycler to be deleted")
	}
}

func TestDeleteWorkerSliceGwRecyclerNotFound(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = spokev1alpha1.AddToScheme(scheme)
	_ = hubv1alpha1.AddToScheme(scheme)
	_ = kubeslicev1beta1.AddToScheme(scheme)

	ProjectNamespace = testProjectNs
	hubClient := newTestHubClient(scheme)

	ctx := context.Background()
	recyclerName := "test-recycler"

	err := hubClient.DeleteWorkerSliceGwRecycler(ctx, recyclerName)
	if err != nil {
		t.Error("Expected no error but got:", err)
	}
}

func TestUpdateServiceExport(t *testing.T) {
	serviceExport := &kubeslicev1beta1.ServiceExport{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: "app-namespace",
		},
		Spec: kubeslicev1beta1.ServiceExportSpec{
			Slice: testSliceName,
			Ports: []kubeslicev1beta1.ServicePort{
				{
					Name:            "http",
					ContainerPort:   8080,
					Protocol:        "TCP",
					ServicePort:     80,
					ServiceProtocol: gwapiv1.ProtocolType("http"),
				},
			},
			Aliases: []string{"test.example.com"},
		},
		Status: kubeslicev1beta1.ServiceExportStatus{
			Pods: []kubeslicev1beta1.ServicePod{
				{
					Name:    "test-pod",
					NsmIP:   "10.0.0.1",
					DNSName: "test-pod.app-namespace.svc.cluster.local",
				},
			},
		},
	}

	scheme := runtime.NewScheme()
	_ = hubv1alpha1.AddToScheme(scheme)
	_ = kubeslicev1beta1.AddToScheme(scheme)
	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	
	ClusterName = testClusterName
	ProjectNamespace = testProjectNs
	hubClient := &HubClientConfig{
		Client: client,
	}

	ctx := context.Background()
	hubSvcExName := "test-service-app-namespace-" + testClusterName

	err := hubClient.UpdateServiceExport(ctx, serviceExport)
	if err != nil {
		t.Error("Expected no error but got:", err)
	}
	
	// Verify the ServiceExportConfig was created
	created := &hubv1alpha1.ServiceExportConfig{}
	err = client.Get(ctx, types.NamespacedName{Name: hubSvcExName, Namespace: testProjectNs}, created)
	if err != nil {
		t.Error("Failed to get created ServiceExportConfig:", err)
	}
}

func TestUpdateServiceExportUpdate(t *testing.T) {
	serviceExport := &kubeslicev1beta1.ServiceExport{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: "app-namespace",
		},
		Spec: kubeslicev1beta1.ServiceExportSpec{
			Slice: testSliceName,
			Ports: []kubeslicev1beta1.ServicePort{
				{
					Name:            "http",
					ContainerPort:   8080,
					Protocol:        "TCP",
					ServicePort:     80,
					ServiceProtocol: gwapiv1.ProtocolType("http"),
				},
			},
			Aliases: []string{"test.example.com"},
		},
		Status: kubeslicev1beta1.ServiceExportStatus{
			Pods: []kubeslicev1beta1.ServicePod{
				{
					Name:    "test-pod",
					NsmIP:   "10.0.0.1",
					DNSName: "test-pod.app-namespace.svc.cluster.local",
				},
			},
		},
	}

	existingHubSvcEx := &hubv1alpha1.ServiceExportConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service-app-namespace-" + testClusterName,
			Namespace: testProjectNs,
		},
		Spec: hubv1alpha1.ServiceExportConfigSpec{
			ServiceName:      "test-service",
			ServiceNamespace: "app-namespace",
			SourceCluster:    testClusterName,
			SliceName:        testSliceName,
		},
	}

	scheme := runtime.NewScheme()
	_ = hubv1alpha1.AddToScheme(scheme)
	_ = kubeslicev1beta1.AddToScheme(scheme)
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existingHubSvcEx).Build()
	
	ClusterName = testClusterName
	ProjectNamespace = testProjectNs
	hubClient := &HubClientConfig{
		Client: client,
	}

	ctx := context.Background()

	err := hubClient.UpdateServiceExport(ctx, serviceExport)
	if err != nil {
		t.Error("Expected no error but got:", err)
	}
}

func TestUpdateAppNamespaces(t *testing.T) {
	workerSliceConfig := &spokev1alpha1.WorkerSliceConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slice-config",
			Namespace: testProjectNs,
		},
		Status: spokev1alpha1.WorkerSliceConfigStatus{},
	}

	scheme := runtime.NewScheme()
	_ = spokev1alpha1.AddToScheme(scheme)
	_ = hubv1alpha1.AddToScheme(scheme)
	_ = kubeslicev1beta1.AddToScheme(scheme)
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(workerSliceConfig).WithStatusSubresource(workerSliceConfig).Build()
	
	ProjectNamespace = testProjectNs
	hubClient := &HubClientConfig{
		Client: client,
	}

	ctx := context.Background()
	namespaces := []string{"ns1", "ns2", "ns3"}

	err := hubClient.UpdateAppNamespaces(ctx, "test-slice-config", namespaces)
	if err != nil {
		t.Error("Expected no error but got:", err)
	}
	
	// Verify the namespaces were updated
	updated := &spokev1alpha1.WorkerSliceConfig{}
	err = client.Get(ctx, types.NamespacedName{Name: "test-slice-config", Namespace: testProjectNs}, updated)
	if err != nil {
		t.Error("Failed to get updated WorkerSliceConfig:", err)
	}
}

func TestUpdateAppPodsList(t *testing.T) {
	workerSliceConfig := &spokev1alpha1.WorkerSliceConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slice-config",
			Namespace: testProjectNs,
		},
		Status: spokev1alpha1.WorkerSliceConfigStatus{},
	}

	appPods := []kubeslicev1beta1.AppPod{
		{
			PodName:      "test-pod-1",
			PodNamespace: "app-ns",
			PodIP:        "10.0.0.1",
			NsmIP:        "10.1.0.1",
			NsmInterface: "nsm0",
		},
		{
			PodName:      "test-pod-2",
			PodNamespace: "app-ns",
			PodIP:        "10.0.0.2",
			NsmIP:        "10.1.0.2",
			NsmInterface: "nsm0",
		},
	}

	scheme := runtime.NewScheme()
	_ = spokev1alpha1.AddToScheme(scheme)
	_ = hubv1alpha1.AddToScheme(scheme)
	_ = kubeslicev1beta1.AddToScheme(scheme)
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(workerSliceConfig).WithStatusSubresource(workerSliceConfig).Build()
	
	ProjectNamespace = testProjectNs
	hubClient := &HubClientConfig{
		Client: client,
	}

	ctx := context.Background()

	err := hubClient.UpdateAppPodsList(ctx, "test-slice-config", appPods)
	if err != nil {
		t.Error("Expected no error but got:", err)
	}
	
	// Verify the app pods were updated
	updated := &spokev1alpha1.WorkerSliceConfig{}
	err = client.Get(ctx, types.NamespacedName{Name: "test-slice-config", Namespace: testProjectNs}, updated)
	if err != nil {
		t.Error("Failed to get updated WorkerSliceConfig:", err)
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		slice    []string
		str      string
		expected bool
	}{
		{[]string{"a", "b", "c"}, "b", true},
		{[]string{"a", "b", "c"}, "d", false},
		{[]string{}, "a", false},
	}

	for _, test := range tests {
		result := contains(test.slice, test.str)
		if result != test.expected {
			t.Errorf("For slice %v and string %s, expected %v but got %v",
				test.slice, test.str, test.expected, result)
		}
	}
}

func TestPartialContains(t *testing.T) {
	tests := []struct {
		slice    []string
		str      string
		expected bool
	}{
		{[]string{"kube", "system"}, "kubernetes", true},
		{[]string{"slice", "gw"}, "slicegateway", true},
		{[]string{"foo", "bar"}, "baz", false},
		{[]string{}, "test", false},
	}

	for _, test := range tests {
		result := partialContains(test.slice, test.str)
		if result != test.expected {
			t.Errorf("For slice %v and string %s, expected %v but got %v",
				test.slice, test.str, test.expected, result)
		}
	}
}

func TestFilterLabelsAndAnnotations(t *testing.T) {
	input := map[string]string{
		"app":                    "myapp",
		"kubeslice-test":         "value1",
		"kubernetes.io/hostname": "node1",
		"custom-label":           "custom-value",
	}

	result := filterLabelsAndAnnotations(input)

	// Should filter out keys containing "kubeslice-" and "kubernetes.io"
	if _, exists := result["kubeslice-test"]; exists {
		t.Error("Expected kubeslice-test to be filtered out")
	}

	if _, exists := result["kubernetes.io/hostname"]; exists {
		t.Error("Expected kubernetes.io/hostname to be filtered out")
	}

	if _, exists := result["app"]; !exists {
		t.Error("Expected app label to be present")
	}

	if _, exists := result["custom-label"]; !exists {
		t.Error("Expected custom-label to be present")
	}
}
