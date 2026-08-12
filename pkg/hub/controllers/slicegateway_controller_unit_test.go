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

	hubv1alpha1 "github.com/kubeslice/apis/pkg/controller/v1alpha1"
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
)

var testSliceGwName = "test-slice-gw-server"
var testSliceGwNamespace = "kubeslice-avesha"
var testClusterName = "test-cluster-1"
var testRemoteClusterName = "test-cluster-2"

var testWorkerSliceGateway = &spokev1alpha1.WorkerSliceGateway{
	ObjectMeta: metav1.ObjectMeta{
		Name:      testSliceGwName,
		Namespace: testSliceGwNamespace,
	},
	Spec: spokev1alpha1.WorkerSliceGatewaySpec{
		SliceName:       testSliceName,
		GatewayHostType: "Server",
		GatewayNumber:   1,
		GatewayType:     "OpenVPN",
		GatewayConnectivityType: "DIRECT",
		GatewayProtocol: "UDP",
		LocalGatewayConfig: spokev1alpha1.SliceGatewayConfig{
			ClusterName:   testClusterName,
			GatewayName:   testSliceGwName,
			GatewaySubnet: "10.1.0.0/16",
			VpnIp:         "10.1.0.1",
			NodePorts:     []int{30001, 30002},
		},
		RemoteGatewayConfig: spokev1alpha1.SliceGatewayConfig{
			ClusterName:   testRemoteClusterName,
			GatewayName:   "test-slice-gw-client",
			GatewaySubnet: "10.2.0.0/16",
			VpnIp:         "10.2.0.1",
			NodeIps:       []string{"192.168.1.1", "192.168.1.2"},
			NodePorts:     []int{30003, 30004},
		},
	},
}

var testVpnKeyRotation = &hubv1alpha1.VpnKeyRotation{
	ObjectMeta: metav1.ObjectMeta{
		Name:      testSliceName,
		Namespace: testSliceGwNamespace,
	},
	Spec: hubv1alpha1.VpnKeyRotationSpec{
		RotationCount: 0,
	},
}

var testSliceGwSecret = &corev1.Secret{
	ObjectMeta: metav1.ObjectMeta{
		Name:      testSliceGwName,
		Namespace: testSliceGwNamespace,
	},
	Data: map[string][]byte{
		"ca.crt":  []byte("test-ca"),
		"tls.crt": []byte("test-cert"),
		"tls.key": []byte("test-key"),
	},
}

func TestSliceGwReconcilerNotFound(t *testing.T) {
	expected := struct {
		ctx context.Context
		req reconcile.Request
		res reconcile.Result
		err error
	}{
		context.Background(),
		reconcile.Request{NamespacedName: types.NamespacedName{Name: testSliceGwName, Namespace: testSliceGwNamespace}},
		reconcile.Result{},
		nil,
	}

	client := utilmock.NewClient()
	eventRecorder := mevents.NewEventRecorder(client, scheme.Scheme, ossEvents.EventsMap, mevents.EventRecorderOptions{})
	reconciler := &SliceGwReconciler{
		Client:        client,
		MeshClient:    client,
		EventRecorder: &eventRecorder,
		ClusterName:   testClusterName,
	}

	ctx := context.Background()
	sliceGwKey := types.NamespacedName{Namespace: testSliceGwNamespace, Name: testSliceGwName}

	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(sliceGwKey),
		mock.IsType(&spokev1alpha1.WorkerSliceGateway{}),
	).Return(apierrors.NewNotFound(schema.GroupResource{Resource: "workerslicegateway"}, testSliceGwName))

	result, err := reconciler.Reconcile(expected.ctx, expected.req)
	if expected.res != result {
		t.Error("Expected response :", expected.res, " but got ", result)
	}
	if expected.err != err {
		t.Error("Expected error:", expected.err, " but got ", err)
	}
}

func TestSliceGwReconcilerWrongCluster(t *testing.T) {
	sliceGw := testWorkerSliceGateway.DeepCopy()
	sliceGw.Spec.LocalGatewayConfig.ClusterName = "different-cluster"

	expected := struct {
		ctx context.Context
		req reconcile.Request
		res reconcile.Result
		err error
	}{
		context.Background(),
		reconcile.Request{NamespacedName: types.NamespacedName{Name: testSliceGwName, Namespace: testSliceGwNamespace}},
		reconcile.Result{},
		nil,
	}

	client := utilmock.NewClient()
	eventRecorder := mevents.NewEventRecorder(client, scheme.Scheme, ossEvents.EventsMap, mevents.EventRecorderOptions{})
	reconciler := &SliceGwReconciler{
		Client:        client,
		MeshClient:    client,
		EventRecorder: &eventRecorder,
		ClusterName:   testClusterName,
	}

	ctx := context.Background()
	sliceGwKey := types.NamespacedName{Namespace: testSliceGwNamespace, Name: testSliceGwName}

	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(sliceGwKey),
		mock.IsType(&spokev1alpha1.WorkerSliceGateway{}),
	).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*spokev1alpha1.WorkerSliceGateway)
		*arg = *sliceGw
	})

	result, err := reconciler.Reconcile(expected.ctx, expected.req)
	if expected.res != result {
		t.Error("Expected response :", expected.res, " but got ", result)
	}
	if expected.err != err {
		t.Error("Expected error:", expected.err, " but got ", err)
	}
}

func TestSliceGwReconcilerCreateSliceGw(t *testing.T) {
	sliceGw := testWorkerSliceGateway.DeepCopy()

	expected := struct {
		ctx context.Context
		req reconcile.Request
		res reconcile.Result
		err error
	}{
		context.Background(),
		reconcile.Request{NamespacedName: types.NamespacedName{Name: testSliceGwName, Namespace: testSliceGwNamespace}},
		reconcile.Result{},
		nil,
	}

	client := utilmock.NewClient()
	meshClient := utilmock.NewClient()
	_ = kubeslicev1beta1.AddToScheme(scheme.Scheme)
	_ = spokev1alpha1.AddToScheme(scheme.Scheme)
	_ = hubv1alpha1.AddToScheme(scheme.Scheme)
	eventRecorder := mevents.NewEventRecorder(client, scheme.Scheme, ossEvents.EventsMap, mevents.EventRecorderOptions{})
	reconciler := &SliceGwReconciler{
		Client:        client,
		MeshClient:    meshClient,
		EventRecorder: &eventRecorder,
		ClusterName:   testClusterName,
	}

	meshClient.On("Scheme").Return(scheme.Scheme)

	ctx := context.Background()
	sliceGwKey := types.NamespacedName{Namespace: testSliceGwNamespace, Name: testSliceGwName}

	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(sliceGwKey),
		mock.IsType(&spokev1alpha1.WorkerSliceGateway{}),
	).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*spokev1alpha1.WorkerSliceGateway)
		*arg = *sliceGw
	})

	client.On("Update",
		mock.IsType(ctx),
		mock.IsType(&spokev1alpha1.WorkerSliceGateway{}),
		mock.IsType([]k8sclient.UpdateOption(nil)),
	).Return(nil)

	// VPN key rotation get
	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(types.NamespacedName{Name: testSliceName, Namespace: testSliceGwNamespace}),
		mock.IsType(&hubv1alpha1.VpnKeyRotation{}),
	).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*hubv1alpha1.VpnKeyRotation)
		*arg = *testVpnKeyRotation
	})

	// Check for existing secret in mesh
	meshClient.On("Get",
		mock.IsType(ctx),
		mock.IsType(types.NamespacedName{Name: testSliceGwName + "-0", Namespace: ControlPlaneNamespace}),
		mock.IsType(&corev1.Secret{}),
	).Return(apierrors.NewNotFound(corev1.Resource("secret"), testSliceGwName+"-0"))

	// Get secret from hub
	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(sliceGwKey),
		mock.IsType(&corev1.Secret{}),
	).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*corev1.Secret)
		*arg = *testSliceGwSecret
	})

	// Create secret in mesh
	meshClient.On("Create",
		mock.IsType(ctx),
		mock.IsType(&corev1.Secret{}),
		mock.IsType([]k8sclient.CreateOption(nil)),
	).Return(nil)

	// Check if slice gateway exists
	meshClient.On("Get",
		mock.Anything,
		mock.IsType(types.NamespacedName{Name: testSliceGwName, Namespace: ControlPlaneNamespace}),
		mock.IsType(&kubeslicev1beta1.SliceGateway{}),
	).Return(apierrors.NewNotFound(schema.GroupResource{Resource: "slicegateway"}, testSliceGwName)).Once()

	// Get slice to set as owner
	meshClient.On("Get",
		mock.Anything,
		mock.IsType(types.NamespacedName{Name: testSliceName, Namespace: ControlPlaneNamespace}),
		mock.IsType(&kubeslicev1beta1.Slice{}),
	).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*kubeslicev1beta1.Slice)
		*arg = *testMeshSlice
	})

	// Create slice gateway
	meshClient.On("Create",
		mock.Anything,
		mock.IsType(&kubeslicev1beta1.SliceGateway{}),
		mock.IsType([]k8sclient.CreateOption(nil)),
	).Return(nil)

	// Get created slice gateway for status update
	meshClient.On("Get",
		mock.Anything,
		mock.IsType(types.NamespacedName{Name: testSliceGwName, Namespace: ControlPlaneNamespace}),
		mock.IsType(&kubeslicev1beta1.SliceGateway{}),
	).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*kubeslicev1beta1.SliceGateway)
		arg.ObjectMeta = metav1.ObjectMeta{
			Name:      testSliceGwName,
			Namespace: ControlPlaneNamespace,
		}
		arg.Status.Config = kubeslicev1beta1.SliceGatewayConfig{}
	})

	meshClient.StatusMock.On("Update",
		mock.Anything,
		mock.IsType(&kubeslicev1beta1.SliceGateway{}),
		mock.IsType([]k8sclient.UpdateOption(nil)),
	).Return(nil)

	meshClient.StatusMock.On("Update",
		mock.Anything,
		mock.IsType(&kubeslicev1beta1.SliceGateway{}),
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

func TestSliceGwReconcilerUpdateSliceGw(t *testing.T) {
	sliceGw := testWorkerSliceGateway.DeepCopy()
	sliceGw.Spec.RemoteGatewayConfig.NodeIps = []string{"192.168.1.3", "192.168.1.4"}

	existingMeshSliceGw := &kubeslicev1beta1.SliceGateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testSliceGwName,
			Namespace: ControlPlaneNamespace,
		},
		Spec: kubeslicev1beta1.SliceGatewaySpec{
			SliceName: testSliceName,
		},
		Status: kubeslicev1beta1.SliceGatewayStatus{
			Config: kubeslicev1beta1.SliceGatewayConfig{
				SliceGatewayID:              testSliceGwName,
				SliceGatewaySubnet:          "10.1.0.0/16",
				SliceGatewayRemoteSubnet:    "10.2.0.0/16",
				SliceGatewayHostType:        "Server",
				SliceGatewayRemoteNodeIPs:   []string{"192.168.1.1", "192.168.1.2"},
				SliceGatewayRemoteClusterID: testRemoteClusterName,
				SliceGatewayRemoteGatewayID: "test-slice-gw-client",
			},
		},
	}

	expected := struct {
		ctx context.Context
		req reconcile.Request
		res reconcile.Result
		err error
	}{
		context.Background(),
		reconcile.Request{NamespacedName: types.NamespacedName{Name: testSliceGwName, Namespace: testSliceGwNamespace}},
		reconcile.Result{},
		nil,
	}

	client := utilmock.NewClient()
	meshClient := utilmock.NewClient()
	eventRecorder := mevents.NewEventRecorder(client, scheme.Scheme, ossEvents.EventsMap, mevents.EventRecorderOptions{})
	reconciler := &SliceGwReconciler{
		Client:        client,
		MeshClient:    meshClient,
		EventRecorder: &eventRecorder,
		ClusterName:   testClusterName,
	}

	ctx := context.Background()
	sliceGwKey := types.NamespacedName{Namespace: testSliceGwNamespace, Name: testSliceGwName}

	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(sliceGwKey),
		mock.IsType(&spokev1alpha1.WorkerSliceGateway{}),
	).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*spokev1alpha1.WorkerSliceGateway)
		*arg = *sliceGw
	})

	client.On("Update",
		mock.IsType(ctx),
		mock.IsType(&spokev1alpha1.WorkerSliceGateway{}),
		mock.IsType([]k8sclient.UpdateOption(nil)),
	).Return(nil)

	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(types.NamespacedName{Name: testSliceName, Namespace: testSliceGwNamespace}),
		mock.IsType(&hubv1alpha1.VpnKeyRotation{}),
	).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*hubv1alpha1.VpnKeyRotation)
		*arg = *testVpnKeyRotation
	})

	meshClient.On("Get",
		mock.IsType(ctx),
		mock.IsType(types.NamespacedName{Name: testSliceGwName + "-0", Namespace: ControlPlaneNamespace}),
		mock.IsType(&corev1.Secret{}),
	).Return(nil)

	meshClient.On("Get",
		mock.IsType(ctx),
		mock.IsType(types.NamespacedName{Name: testSliceGwName, Namespace: ControlPlaneNamespace}),
		mock.IsType(&kubeslicev1beta1.SliceGateway{}),
	).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*kubeslicev1beta1.SliceGateway)
		*arg = *existingMeshSliceGw
	})

	meshClient.StatusMock.On("Update",
		mock.IsType(ctx),
		mock.IsType(&kubeslicev1beta1.SliceGateway{}),
		mock.IsType([]k8sclient.UpdateOption(nil)),
	).Return(nil)

	meshClient.StatusMock.On("Update",
		mock.IsType(ctx),
		mock.IsType(&kubeslicev1beta1.SliceGateway{}),
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

func TestSliceGwReconcilerDeletion(t *testing.T) {
	sliceGw := testWorkerSliceGateway.DeepCopy()
	now := metav1.Now()
	sliceGw.DeletionTimestamp = &now
	sliceGw.Finalizers = []string{"controller.kubeslice.io/sliceGw-finalizer"}

	expected := struct {
		ctx context.Context
		req reconcile.Request
		res reconcile.Result
		err error
	}{
		context.Background(),
		reconcile.Request{NamespacedName: types.NamespacedName{Name: testSliceGwName, Namespace: testSliceGwNamespace}},
		reconcile.Result{},
		nil,
	}

	client := utilmock.NewClient()
	meshClient := utilmock.NewClient()
	eventRecorder := mevents.NewEventRecorder(client, scheme.Scheme, ossEvents.EventsMap, mevents.EventRecorderOptions{})
	reconciler := &SliceGwReconciler{
		Client:        client,
		MeshClient:    meshClient,
		EventRecorder: &eventRecorder,
		ClusterName:   testClusterName,
	}

	ctx := context.Background()
	sliceGwKey := types.NamespacedName{Namespace: testSliceGwNamespace, Name: testSliceGwName}

	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(sliceGwKey),
		mock.IsType(&spokev1alpha1.WorkerSliceGateway{}),
	).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*spokev1alpha1.WorkerSliceGateway)
		*arg = *sliceGw
	})

	meshClient.On("Delete",
		mock.IsType(ctx),
		mock.IsType(&kubeslicev1beta1.SliceGateway{}),
		mock.IsType([]k8sclient.DeleteOption(nil)),
	).Return(nil)

	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(sliceGwKey),
		mock.IsType(&spokev1alpha1.WorkerSliceGateway{}),
	).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*spokev1alpha1.WorkerSliceGateway)
		arg.Finalizers = []string{"controller.kubeslice.io/sliceGw-finalizer"}
	})

	client.On("Update",
		mock.IsType(ctx),
		mock.IsType(&spokev1alpha1.WorkerSliceGateway{}),
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

func TestSliceGwReconcilerCreateCertsError(t *testing.T) {
	sliceGw := testWorkerSliceGateway.DeepCopy()

	expected := struct {
		ctx context.Context
		req reconcile.Request
	}{
		context.Background(),
		reconcile.Request{NamespacedName: types.NamespacedName{Name: testSliceGwName, Namespace: testSliceGwNamespace}},
	}

	client := utilmock.NewClient()
	meshClient := utilmock.NewClient()
	eventRecorder := mevents.NewEventRecorder(client, scheme.Scheme, ossEvents.EventsMap, mevents.EventRecorderOptions{})
	reconciler := &SliceGwReconciler{
		Client:        client,
		MeshClient:    meshClient,
		EventRecorder: &eventRecorder,
		ClusterName:   testClusterName,
	}

	ctx := context.Background()
	sliceGwKey := types.NamespacedName{Namespace: testSliceGwNamespace, Name: testSliceGwName}

	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(sliceGwKey),
		mock.IsType(&spokev1alpha1.WorkerSliceGateway{}),
	).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*spokev1alpha1.WorkerSliceGateway)
		*arg = *sliceGw
	})

	client.On("Update",
		mock.IsType(ctx),
		mock.IsType(&spokev1alpha1.WorkerSliceGateway{}),
		mock.IsType([]k8sclient.UpdateOption(nil)),
	).Return(nil)

	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(types.NamespacedName{Name: testSliceName, Namespace: testSliceGwNamespace}),
		mock.IsType(&hubv1alpha1.VpnKeyRotation{}),
	).Return(errors.New("vpn key rotation not found"))

	_, err := reconciler.Reconcile(expected.ctx, expected.req)
	if err == nil {
		t.Error("Expected error but got nil")
	}
}
