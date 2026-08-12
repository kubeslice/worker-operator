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
	"errors"
	"testing"
	"time"

	spokev1alpha1 "github.com/kubeslice/apis/pkg/worker/v1alpha1"
	mevents "github.com/kubeslice/kubeslice-monitoring/pkg/events"
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	ossEvents "github.com/kubeslice/worker-operator/events"
	utilmock "github.com/kubeslice/worker-operator/pkg/mocks"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

var testSvcImportName = "test-svcim"
var testSvcImportNamespace = "kubeslice-avesha"
var testServiceName = "test-service"
var testServiceNamespace = "app-namespace"
var testSliceName = "test-slice"

var testWorkerServiceImport = &spokev1alpha1.WorkerServiceImport{
	ObjectMeta: metav1.ObjectMeta{
		Name:      testSvcImportName,
		Namespace: testSvcImportNamespace,
	},
	Spec: spokev1alpha1.WorkerServiceImportSpec{
		ServiceName:      testServiceName,
		ServiceNamespace: testServiceNamespace,
		SliceName:        testSliceName,
		ServiceDiscoveryPorts: []spokev1alpha1.ServiceDiscoveryPort{
			{
				Name:            "http",
				Port:            8080,
				Protocol:        "TCP",
				ServicePort:     80,
				ServiceProtocol: "http",
			},
		},
		ServiceDiscoveryEndpoints: []spokev1alpha1.ServiceDiscoveryEndpoint{
			{
				PodName: "test-pod",
				Cluster: "test-cluster",
				NsmIp:   "10.0.0.1",
				DnsName: "test-pod.app-namespace.svc.cluster.local",
				Port:    8080,
			},
		},
		Aliases: []string{"test-alias.example.com"},
	},
}

var testMeshSlice = &kubeslicev1beta1.Slice{
	ObjectMeta: metav1.ObjectMeta{
		Name:      testSliceName,
		Namespace: ControlPlaneNamespace,
	},
	Spec: kubeslicev1beta1.SliceSpec{},
}

func TestServiceImportReconcilerNotFound(t *testing.T) {
	expected := struct {
		ctx    context.Context
		req    reconcile.Request
		res    reconcile.Result
		errStr string
	}{
		context.Background(),
		reconcile.Request{NamespacedName: types.NamespacedName{Name: testSvcImportName, Namespace: testSvcImportNamespace}},
		reconcile.Result{},
		"object not found",
	}

	client := utilmock.NewClient()
	eventRecorder := mevents.NewEventRecorder(client, scheme.Scheme, ossEvents.EventsMap, mevents.EventRecorderOptions{})
	reconciler := &ServiceImportReconciler{
		Client:        client,
		MeshClient:    client,
		EventRecorder: &eventRecorder,
	}

	ctx := context.Background()
	svcImKey := types.NamespacedName{Namespace: testSvcImportNamespace, Name: testSvcImportName}

	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(svcImKey),
		mock.IsType(&spokev1alpha1.WorkerServiceImport{}),
	).Return(errors.New("object not found"))

	result, err := reconciler.Reconcile(expected.ctx, expected.req)
	if expected.res != result {
		t.Error("Expected response :", expected.res, " but got ", result)
	}
	if err == nil || expected.errStr != err.Error() {
		t.Error("Expected error:", expected.errStr, " but got ", err)
	}
}

func TestServiceImportReconcilerWithFinalizer(t *testing.T) {
	svcim := testWorkerServiceImport.DeepCopy()
	svcim.DeletionTimestamp = nil

	expected := struct {
		ctx context.Context
		req reconcile.Request
		res reconcile.Result
		err error
	}{
		context.Background(),
		reconcile.Request{NamespacedName: types.NamespacedName{Name: testSvcImportName, Namespace: testSvcImportNamespace}},
		reconcile.Result{},
		nil,
	}

	client := utilmock.NewClient()
	meshClient := utilmock.NewClient()
	eventRecorder := mevents.NewEventRecorder(client, scheme.Scheme, ossEvents.EventsMap, mevents.EventRecorderOptions{})
	reconciler := &ServiceImportReconciler{
		Client:        client,
		MeshClient:    meshClient,
		EventRecorder: &eventRecorder,
	}

	ctx := context.Background()
	svcImKey := types.NamespacedName{Namespace: testSvcImportNamespace, Name: testSvcImportName}

	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(svcImKey),
		mock.IsType(&spokev1alpha1.WorkerServiceImport{}),
	).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*spokev1alpha1.WorkerServiceImport)
		*arg = *svcim
	})

	client.On("Update",
		mock.IsType(ctx),
		mock.IsType(&spokev1alpha1.WorkerServiceImport{}),
		mock.IsType([]k8sclient.UpdateOption(nil)),
	).Return(nil)

	meshClient.On("Get",
		mock.IsType(ctx),
		mock.IsType(types.NamespacedName{Name: testSliceName, Namespace: ControlPlaneNamespace}),
		mock.IsType(&kubeslicev1beta1.Slice{}),
	).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*kubeslicev1beta1.Slice)
		*arg = *testMeshSlice
	})

	meshClient.On("Get",
		mock.IsType(ctx),
		mock.IsType(types.NamespacedName{Name: testServiceName, Namespace: testServiceNamespace}),
		mock.IsType(&kubeslicev1beta1.ServiceImport{}),
	).Return(apierrors.NewNotFound(schema.GroupResource{Resource: "serviceimport"}, testServiceName))

	meshClient.On("Create",
		mock.IsType(ctx),
		mock.IsType(&kubeslicev1beta1.ServiceImport{}),
		mock.IsType([]k8sclient.CreateOption(nil)),
	).Return(nil)

	meshClient.StatusMock.On("Update",
		mock.IsType(ctx),
		mock.IsType(&kubeslicev1beta1.ServiceImport{}),
		mock.IsType([]k8sclient.UpdateOption(nil)),
	).Return(nil)

	meshClient.StatusMock.On("Update",
		mock.IsType(ctx),
		mock.IsType(&kubeslicev1beta1.ServiceImport{}),
		mock.IsType([]k8sclient.SubResourceUpdateOption(nil)),
	).Return(nil)

	client.On("Create",
		mock.IsType(ctx),
		mock.IsType(&corev1.Event{}),
		mock.IsType([]k8sclient.CreateOption(nil)),
	).Return(nil)

	result, err := reconciler.Reconcile(expected.ctx, expected.req)
	if expected.res != result {
		t.Error("Expected response :", expected.res, " but got ", result)
	}
	if expected.err != err {
		t.Error("Expected error:", expected.err, " but got ", err)
	}
}

func TestServiceImportReconcilerUpdateExisting(t *testing.T) {
	svcim := testWorkerServiceImport.DeepCopy()

	existingMeshSvcIm := &kubeslicev1beta1.ServiceImport{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testServiceName,
			Namespace: testServiceNamespace,
		},
		Spec: kubeslicev1beta1.ServiceImportSpec{
			Slice:   testSliceName,
			DNSName: testServiceName + "." + testServiceNamespace + ".svc.slice.local",
		},
	}

	expected := struct {
		ctx context.Context
		req reconcile.Request
		res reconcile.Result
		err error
	}{
		context.Background(),
		reconcile.Request{NamespacedName: types.NamespacedName{Name: testSvcImportName, Namespace: testSvcImportNamespace}},
		reconcile.Result{},
		nil,
	}

	client := utilmock.NewClient()
	meshClient := utilmock.NewClient()
	eventRecorder := mevents.NewEventRecorder(client, scheme.Scheme, ossEvents.EventsMap, mevents.EventRecorderOptions{})
	reconciler := &ServiceImportReconciler{
		Client:        client,
		MeshClient:    meshClient,
		EventRecorder: &eventRecorder,
	}

	ctx := context.Background()
	svcImKey := types.NamespacedName{Namespace: testSvcImportNamespace, Name: testSvcImportName}

	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(svcImKey),
		mock.IsType(&spokev1alpha1.WorkerServiceImport{}),
	).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*spokev1alpha1.WorkerServiceImport)
		*arg = *svcim
	})

	client.On("Update",
		mock.IsType(ctx),
		mock.IsType(&spokev1alpha1.WorkerServiceImport{}),
		mock.IsType([]k8sclient.UpdateOption(nil)),
	).Return(nil)

	meshClient.On("Get",
		mock.IsType(ctx),
		mock.IsType(types.NamespacedName{Name: testSliceName, Namespace: ControlPlaneNamespace}),
		mock.IsType(&kubeslicev1beta1.Slice{}),
	).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*kubeslicev1beta1.Slice)
		*arg = *testMeshSlice
	})

	meshClient.On("Get",
		mock.IsType(ctx),
		mock.IsType(types.NamespacedName{Name: testServiceName, Namespace: testServiceNamespace}),
		mock.IsType(&kubeslicev1beta1.ServiceImport{}),
	).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*kubeslicev1beta1.ServiceImport)
		*arg = *existingMeshSvcIm
	})

	meshClient.On("Update",
		mock.IsType(ctx),
		mock.IsType(&kubeslicev1beta1.ServiceImport{}),
		mock.IsType([]k8sclient.UpdateOption(nil)),
	).Return(nil)

	meshClient.StatusMock.On("Update",
		mock.IsType(ctx),
		mock.IsType(&kubeslicev1beta1.ServiceImport{}),
		mock.IsType([]k8sclient.UpdateOption(nil)),
	).Return(nil)

	meshClient.StatusMock.On("Update",
		mock.IsType(ctx),
		mock.IsType(&kubeslicev1beta1.ServiceImport{}),
		mock.IsType([]k8sclient.SubResourceUpdateOption(nil)),
	).Return(nil)

	client.On("Create",
		mock.IsType(ctx),
		mock.IsType(&corev1.Event{}),
		mock.IsType([]k8sclient.CreateOption(nil)),
	).Return(nil)

	result, err := reconciler.Reconcile(expected.ctx, expected.req)
	if expected.res != result {
		t.Error("Expected response :", expected.res, " but got ", result)
	}
	if expected.err != err {
		t.Error("Expected error:", expected.err, " but got ", err)
	}
}

func TestServiceImportReconcilerSliceNotFound(t *testing.T) {
	svcim := testWorkerServiceImport.DeepCopy()

	expected := struct {
		ctx context.Context
		req reconcile.Request
		res reconcile.Result
		err error
	}{
		context.Background(),
		reconcile.Request{NamespacedName: types.NamespacedName{Name: testSvcImportName, Namespace: testSvcImportNamespace}},
		reconcile.Result{RequeueAfter: 30 * time.Second},
		nil,
	}

	client := utilmock.NewClient()
	meshClient := utilmock.NewClient()
	eventRecorder := mevents.NewEventRecorder(client, scheme.Scheme, ossEvents.EventsMap, mevents.EventRecorderOptions{})
	reconciler := &ServiceImportReconciler{
		Client:        client,
		MeshClient:    meshClient,
		EventRecorder: &eventRecorder,
	}

	ctx := context.Background()
	svcImKey := types.NamespacedName{Namespace: testSvcImportNamespace, Name: testSvcImportName}

	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(svcImKey),
		mock.IsType(&spokev1alpha1.WorkerServiceImport{}),
	).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*spokev1alpha1.WorkerServiceImport)
		*arg = *svcim
	})

	client.On("Update",
		mock.IsType(ctx),
		mock.IsType(&spokev1alpha1.WorkerServiceImport{}),
		mock.IsType([]k8sclient.UpdateOption(nil)),
	).Return(nil)

	meshClient.On("Get",
		mock.IsType(ctx),
		mock.IsType(types.NamespacedName{Name: testSliceName, Namespace: ControlPlaneNamespace}),
		mock.IsType(&kubeslicev1beta1.Slice{}),
	).Return(apierrors.NewNotFound(schema.GroupResource{Resource: "slice"}, testSliceName))

	result, err := reconciler.Reconcile(expected.ctx, expected.req)
	if expected.res != result {
		t.Error("Expected response :", expected.res, " but got ", result)
	}
	if expected.err != err {
		t.Error("Expected error:", expected.err, " but got ", err)
	}
}

func TestServiceImportReconcilerDeletion(t *testing.T) {
	svcim := testWorkerServiceImport.DeepCopy()
	now := metav1.Now()
	svcim.DeletionTimestamp = &now
	svcim.Finalizers = []string{"controller.kubeslice.io/hubWorkerServiceImport-finalizer"}

	expected := struct {
		ctx context.Context
		req reconcile.Request
		res reconcile.Result
		err error
	}{
		context.Background(),
		reconcile.Request{NamespacedName: types.NamespacedName{Name: testSvcImportName, Namespace: testSvcImportNamespace}},
		reconcile.Result{},
		nil,
	}

	client := utilmock.NewClient()
	meshClient := utilmock.NewClient()
	eventRecorder := mevents.NewEventRecorder(client, scheme.Scheme, ossEvents.EventsMap, mevents.EventRecorderOptions{})
	reconciler := &ServiceImportReconciler{
		Client:        client,
		MeshClient:    meshClient,
		EventRecorder: &eventRecorder,
	}

	ctx := context.Background()
	svcImKey := types.NamespacedName{Namespace: testSvcImportNamespace, Name: testSvcImportName}

	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(svcImKey),
		mock.IsType(&spokev1alpha1.WorkerServiceImport{}),
	).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*spokev1alpha1.WorkerServiceImport)
		*arg = *svcim
	})

	meshClient.On("Delete",
		mock.IsType(ctx),
		mock.IsType(&kubeslicev1beta1.ServiceImport{}),
		mock.IsType([]k8sclient.DeleteOption(nil)),
	).Return(nil)

	client.On("Update",
		mock.IsType(ctx),
		mock.IsType(&spokev1alpha1.WorkerServiceImport{}),
		mock.IsType([]k8sclient.UpdateOption(nil)),
	).Return(nil)

	result, err := reconciler.Reconcile(expected.ctx, expected.req)
	if expected.res != result {
		t.Error("Expected response :", expected.res, " but got ", result)
	}
	if expected.err != err {
		t.Error("Expected error:", expected.err, " but got ", err)
	}
}

func TestGetProtocol(t *testing.T) {
	tests := []struct {
		input    string
		expected corev1.Protocol
	}{
		{"TCP", corev1.ProtocolTCP},
		{"UDP", corev1.ProtocolUDP},
		{"SCTP", corev1.ProtocolSCTP},
		{"unknown", ""},
	}

	for _, test := range tests {
		result := getProtocol(test.input)
		if result != test.expected {
			t.Errorf("For input %s, expected %s but got %s", test.input, test.expected, result)
		}
	}
}

func TestGetMeshServiceImportPortList(t *testing.T) {
	svcim := testWorkerServiceImport.DeepCopy()
	portList := getMeshServiceImportPortList(svcim)

	if len(portList) != len(svcim.Spec.ServiceDiscoveryPorts) {
		t.Errorf("Expected %d ports but got %d", len(svcim.Spec.ServiceDiscoveryPorts), len(portList))
	}

	if portList[0].Name != "http" {
		t.Errorf("Expected port name 'http' but got %s", portList[0].Name)
	}

	if portList[0].ContainerPort != 8080 {
		t.Errorf("Expected port 8080 but got %d", portList[0].ContainerPort)
	}

	if portList[0].Protocol != corev1.ProtocolTCP {
		t.Errorf("Expected protocol TCP but got %s", portList[0].Protocol)
	}

	if portList[0].ServiceProtocol != gwapiv1.ProtocolType("http") {
		t.Errorf("Expected service protocol http but got %s", portList[0].ServiceProtocol)
	}
}

func TestGetMeshServiceImportEpList(t *testing.T) {
	svcim := testWorkerServiceImport.DeepCopy()
	epList := getMeshServiceImportEpList(svcim)

	if len(epList) != len(svcim.Spec.ServiceDiscoveryEndpoints) {
		t.Errorf("Expected %d endpoints but got %d", len(svcim.Spec.ServiceDiscoveryEndpoints), len(epList))
	}

	if epList[0].Name != "test-pod" {
		t.Errorf("Expected endpoint name 'test-pod' but got %s", epList[0].Name)
	}

	if epList[0].IP != "10.0.0.1" {
		t.Errorf("Expected IP 10.0.0.1 but got %s", epList[0].IP)
	}

	if epList[0].ClusterID != "test-cluster" {
		t.Errorf("Expected cluster ID 'test-cluster' but got %s", epList[0].ClusterID)
	}
}

func TestGetMeshServiceImportObj(t *testing.T) {
	svcim := testWorkerServiceImport.DeepCopy()
	meshSvcIm := getMeshServiceImportObj(svcim)

	if meshSvcIm.Name != testServiceName {
		t.Errorf("Expected name %s but got %s", testServiceName, meshSvcIm.Name)
	}

	if meshSvcIm.Namespace != testServiceNamespace {
		t.Errorf("Expected namespace %s but got %s", testServiceNamespace, meshSvcIm.Namespace)
	}

	if meshSvcIm.Spec.Slice != testSliceName {
		t.Errorf("Expected slice %s but got %s", testSliceName, meshSvcIm.Spec.Slice)
	}

	expectedDNS := testServiceName + "." + testServiceNamespace + ".svc.slice.local"
	if meshSvcIm.Spec.DNSName != expectedDNS {
		t.Errorf("Expected DNS name %s but got %s", expectedDNS, meshSvcIm.Spec.DNSName)
	}
}
