/*
 *  Copyright (c) 2022 Avesha, Inc. All rights reserved.
 *
 *	SPDX-License-Identifier: Apache-2.0
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

	workerv1alpha1 "github.com/kubeslice/apis/pkg/worker/v1alpha1"
	mevents "github.com/kubeslice/kubeslice-monitoring/pkg/events"
	"github.com/kubeslice/kubeslice-monitoring/pkg/metrics"
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	ossEvents "github.com/kubeslice/worker-operator/events"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/mock"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var hundred = 100

var controllerSlice = &workerv1alpha1.WorkerSliceConfig{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "test-slice-cluster-1",
		Namespace: "project-namespace",
		Labels: map[string]string{
			"worker-cluster":      "cluster-1",
			"original-slice-name": "test-slice",
		},
		DeletionTimestamp: &metav1.Time{Time: time.Now()},
	},
	Spec: workerv1alpha1.WorkerSliceConfigSpec{
		SliceName:        "test-slice-cluster-1",
		SliceType:        "Application",
		SliceSubnet:      "10.0.0.1/16",
		SliceIpamType:    "Local",
		IpamClusterOctet: hundred,
	},
}

var workerslice = &kubeslicev1beta1.Slice{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "test-slice",
		Namespace: "kubeslice-system",
	},
	Spec: kubeslicev1beta1.SliceSpec{},
}

func TestReconcileToReturnErrorWhileFetchingControllerSlice(t *testing.T) {

	expected := struct {
		ctx    context.Context
		req    reconcile.Request
		res    reconcile.Result
		errStr string
	}{

		context.WithValue(context.Background(), types.NamespacedName{Namespace: "kube-slice", Name: "kube-slice"}, controllerSlice),
		reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-slice", Namespace: "kubeslice-system"}},
		reconcile.Result{},
		"object not found",
	}
	client := NewClient()

	ctx := context.WithValue(context.Background(), types.NamespacedName{Name: "test-slice", Namespace: "kubeslice-system"}, controllerSlice)
	sliceKey := types.NamespacedName{Namespace: "kubeslice-system", Name: "test-slice"}

	//Expectation
	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(sliceKey),
		mock.IsType(&workerv1alpha1.WorkerSliceConfig{}),
	).Return(errors.New("object not found"))

	mf, _ := metrics.NewMetricsFactory(prometheus.NewRegistry(), metrics.MetricsFactoryOptions{})
	eventRecorder := mevents.NewEventRecorder(client, nil, ossEvents.EventsMap, mevents.EventRecorderOptions{
		Cluster: clusterName,
	})
	reconciler := NewSliceReconciler(client, client, &eventRecorder, mf)
	reconciler.InjectClient(client)
	result, err := reconciler.Reconcile(expected.ctx, expected.req)
	if expected.res != result {
		t.Error("Expected response :", expected.res, " but got ", result)
	}
	if expected.errStr != err.Error() {
		t.Error("Expected error:", expected.errStr, " but got ", err)
	}
}

func TestReconcileToReturnErrorWhileFetchingWorkerSlice(t *testing.T) {
	expected := struct {
		ctx    context.Context
		req    reconcile.Request
		res    reconcile.Result
		errStr string
	}{
		context.WithValue(context.Background(), types.NamespacedName{Namespace: "kube-slice", Name: "kube-slice"}, controllerSlice),
		reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-slice", Namespace: "kubeslice-system"}},
		reconcile.Result{},
		"object not found",
	}

	client := NewClient()

	ctx := context.WithValue(context.Background(), types.NamespacedName{Name: "test-slice", Namespace: "kubeslice-system"}, controllerSlice)
	sliceKey := types.NamespacedName{Namespace: "kubeslice-system", Name: "test-slice"}

	//Expectations
	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(sliceKey),
		mock.IsType(&workerv1alpha1.WorkerSliceConfig{}),
	).Return(nil)
	client.On("Update",
		mock.IsType(ctx),
		mock.IsType(controllerSlice),
		mock.IsType([]k8sclient.UpdateOption(nil)),
	).Return(nil)
	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(sliceKey),
		mock.IsType(&kubeslicev1beta1.Slice{}),
	).Return(errors.New("object not found"))

	mf, _ := metrics.NewMetricsFactory(prometheus.NewRegistry(), metrics.MetricsFactoryOptions{})
	eventRecorder := mevents.NewEventRecorder(client, &runtime.Scheme{}, ossEvents.EventsMap, mevents.EventRecorderOptions{
		Cluster: clusterName,
	})
	reconciler := NewSliceReconciler(client, client, &eventRecorder, mf)
	reconciler.InjectClient(client)
	result, err := reconciler.Reconcile(expected.ctx, expected.req)
	if expected.res != result {
		t.Error("Expected response :", expected.res, " but got ", result)
	}
	if expected.errStr != err.Error() {
		t.Error("Expected error:", expected.errStr, " but got ", err)
	}
}

func TestReconcileToUpdateWorkerSlice(t *testing.T) {
	expected := struct {
		ctx    context.Context
		req    reconcile.Request
		res    reconcile.Result
		errStr error
	}{
		context.WithValue(context.Background(), types.NamespacedName{Namespace: "kube-slice", Name: "kube-slice"}, controllerSlice),
		reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-slice", Namespace: "kubeslice-system"}},
		reconcile.Result{RequeueAfter: time.Second * 120},
		nil,
	}
	client := NewClient()
	ctx := context.WithValue(context.Background(), types.NamespacedName{Name: "test-slice", Namespace: "kubeslice-system"}, controllerSlice)
	sliceKey := types.NamespacedName{Namespace: "kubeslice-system", Name: "test-slice"}

	//Expectations
	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(sliceKey),
		mock.IsType(&workerv1alpha1.WorkerSliceConfig{}),
	).Return(nil)
	client.On("Update",
		mock.IsType(ctx),
		mock.IsType(controllerSlice),
		mock.IsType([]k8sclient.UpdateOption(nil)),
	).Return(nil)
	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(sliceKey),
		mock.IsType(&kubeslicev1beta1.Slice{}),
	).Return(nil)
	client.StatusMock.On("Update",
		mock.IsType(ctx),
		mock.IsType(workerslice),
		mock.IsType([]k8sclient.UpdateOption(nil)),
	).Return(nil)
	client.On("List",
		mock.IsType(ctx),
		mock.IsType(&corev1.PodList{}),
		mock.IsType([]k8sclient.ListOption{}),
	).Return(nil)
	client.On("List",
		mock.IsType(ctx),
		mock.IsType(&appsv1.DeploymentList{}),
		mock.IsType([]k8sclient.ListOption{}),
	).Return(nil)
	client.On("List",
		mock.IsType(ctx),
		mock.IsType(&kubeslicev1beta1.SliceGatewayList{}),
		mock.IsType([]k8sclient.ListOption{}),
	).Return(nil)
	client.StatusMock.On("Update",
		mock.IsType(ctx),
		mock.IsType(&workerv1alpha1.WorkerSliceConfig{}),
		mock.IsType([]k8sclient.UpdateOption(nil)),
	).Return(nil)
	client.StatusMock.On("Update",
		mock.IsType(ctx),
		mock.IsType(&workerv1alpha1.WorkerSliceConfig{}),
		mock.IsType([]k8sclient.SubResourceUpdateOption(nil)),
	).Return(nil)
	client.StatusMock.On("Update",
		mock.IsType(ctx),
		mock.IsType(&kubeslicev1beta1.Slice{}),
		mock.IsType([]k8sclient.SubResourceUpdateOption(nil)),
	).Return(nil)

	mf, _ := metrics.NewMetricsFactory(prometheus.NewRegistry(), metrics.MetricsFactoryOptions{})
	eventRecorder := mevents.NewEventRecorder(client, &runtime.Scheme{}, ossEvents.EventsMap, mevents.EventRecorderOptions{
		Cluster: clusterName,
	})
	reconciler := NewSliceReconciler(client, client, &eventRecorder, mf)
	reconciler.InjectClient(client)
	result, err := reconciler.Reconcile(expected.ctx, expected.req)
	if expected.res != result {
		t.Error("Expected response :", expected.res, " but got ", result)
	}
	if expected.errStr != err {
		t.Error("Expected error:", expected.errStr, " but got ", err)
	}
}

func TestUpdateSliceConfig(t *testing.T) {
	expected := struct {
		ctx context.Context
		req reconcile.Request
		res reconcile.Result
		err error
	}{
		context.WithValue(context.Background(), types.NamespacedName{Namespace: "kubeslice-system", Name: "test-slice"}, controllerSlice),
		reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-slice", Namespace: "kubeslice-system"}},
		reconcile.Result{},
		nil,
	}
	client := NewClient()

	reconciler := &SliceReconciler{
		Client:     client,
		MeshClient: client,
		Log:        ctrl.Log.WithName("controller").WithName("controllers").WithName("SliceConfig"),
	}
	ctx := context.WithValue(context.Background(), types.NamespacedName{Name: "test-slice", Namespace: "kubeslice-system"}, controllerSlice)

	client.StatusMock.On("Update",
		mock.IsType(ctx),
		mock.IsType(&kubeslicev1beta1.Slice{}),
		mock.IsType([]k8sclient.UpdateOption(nil)),
	).Return(nil)
	client.StatusMock.On("Update",
		mock.IsType(ctx),
		mock.IsType(&kubeslicev1beta1.Slice{}),
		mock.IsType([]k8sclient.SubResourceUpdateOption(nil)),
	).Return(nil)
	client.StatusMock.On("Update",
		mock.IsType(ctx),
		mock.IsType(&workerv1alpha1.WorkerSliceConfig{}),
		mock.IsType([]k8sclient.SubResourceUpdateOption(nil)),
	).Return(nil)
	err := reconciler.updateSliceConfig(expected.ctx, workerslice, controllerSlice)
	if expected.err != err {
		t.Error("Expected error:", expected.err, " but got ", err)
	}
}
func TestUpdateSliceHealth(t *testing.T) {
	expected := struct {
		ctx context.Context
		req reconcile.Request
		res reconcile.Result
		err error
	}{
		context.WithValue(context.Background(), types.NamespacedName{Namespace: "kubeslice-system", Name: "test-slice"}, controllerSlice),
		reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-slice", Namespace: "kubeslice-system"}},
		reconcile.Result{RequeueAfter: 10 * time.Second},
		nil,
	}
	client := NewClient()

	mf, _ := metrics.NewMetricsFactory(prometheus.NewRegistry(), metrics.MetricsFactoryOptions{})
	eventRecorder := mevents.NewEventRecorder(client, &runtime.Scheme{}, ossEvents.EventsMap, mevents.EventRecorderOptions{
		Cluster: clusterName,
	})
	reconciler := NewSliceReconciler(client, client, &eventRecorder, mf)
	reconciler.InjectClient(client)
	ctx := context.WithValue(context.Background(), types.NamespacedName{Name: "test-slice", Namespace: "kubeslice-system"}, controllerSlice)

	client.On("List",
		mock.IsType(expected.ctx),
		mock.IsType(&corev1.PodList{}),
		mock.IsType([]k8sclient.ListOption{}),
	).Return(nil)
	client.StatusMock.On("Update",
		mock.IsType(ctx),
		mock.IsType(&kubeslicev1beta1.Slice{}),
		mock.IsType([]k8sclient.UpdateOption(nil)),
	).Return(nil)
	client.StatusMock.On("Update",
		mock.IsType(ctx),
		mock.IsType(&workerv1alpha1.WorkerSliceConfig{}),
	).Return(nil)
	client.On("List",
		mock.IsType(ctx),
		mock.IsType(&kubeslicev1beta1.SliceGatewayList{}),
		mock.IsType([]k8sclient.ListOption{}),
	).Return(nil)
	client.On("List",
		mock.IsType(expected.ctx),
		mock.IsType(&appsv1.DeploymentList{}),
		mock.IsType([]k8sclient.ListOption{}),
	).Return(nil)
	controllerSlice.Status.SliceHealth = &workerv1alpha1.SliceHealth{}
	err := reconciler.updateSliceHealth(expected.ctx, controllerSlice)
	if expected.err != err {
		t.Error("Expected error:", expected.err, " but got ", err)
	}
}
func TestUpdateSliceConfigByModyfingSubnetOfControllerSlice(t *testing.T) {
	expected := struct {
		ctx context.Context
		req reconcile.Request
		res reconcile.Result
		err error
	}{
		context.WithValue(context.Background(), types.NamespacedName{Namespace: "kubeslice-system", Name: "test-slice"}, controllerSlice),
		reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-slice", Namespace: "kubeslice-system"}},
		reconcile.Result{},
		nil,
	}
	client := NewClient()

	mf, _ := metrics.NewMetricsFactory(prometheus.NewRegistry(), metrics.MetricsFactoryOptions{})
	eventRecorder := mevents.NewEventRecorder(client, &runtime.Scheme{}, ossEvents.EventsMap, mevents.EventRecorderOptions{
		Cluster: clusterName,
	})
	reconciler := NewSliceReconciler(client, client, &eventRecorder, mf)
	reconciler.InjectClient(client)
	ctx := context.WithValue(context.Background(), types.NamespacedName{Name: "test-slice", Namespace: "kubeslice-system"}, controllerSlice)

	client.StatusMock.On("Update",
		mock.IsType(ctx),
		mock.IsType(workerslice),
		mock.IsType([]k8sclient.UpdateOption(nil)),
	).Return(nil)
	client.StatusMock.On("Update",
		mock.IsType(ctx),
		mock.IsType(&workerv1alpha1.WorkerSliceConfig{}),
		mock.IsType([]k8sclient.SubResourceUpdateOption(nil)),
	).Return(nil)
	client.StatusMock.On("Update",
		mock.IsType(ctx),
		mock.IsType(&kubeslicev1beta1.Slice{}),
		mock.IsType([]k8sclient.SubResourceUpdateOption(nil)),
	).Return(nil)
	controllerSlice.Spec.SliceSubnet = "10.0.0.2/16"
	workerslice.Status = kubeslicev1beta1.SliceStatus{}
	err := reconciler.updateSliceConfig(expected.ctx, workerslice, controllerSlice)
	if expected.err != err {
		t.Error("Expected error:", expected.err, " but got ", err)
	}
	if workerslice.Status.SliceConfig.SliceSubnet != controllerSlice.Spec.SliceSubnet {
		t.Error("Expected error:", controllerSlice.Spec.SliceSubnet, " but got ", workerslice.Status.SliceConfig.SliceSubnet)
	}
}
func TestDeleteSliceResourceOnWorker(t *testing.T) {
	expected := struct {
		ctx context.Context
		req reconcile.Request
		res reconcile.Result
		err error
	}{
		context.WithValue(context.Background(), types.NamespacedName{Namespace: "kubeslice-system", Name: "test-slice"}, controllerSlice),
		reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-slice", Namespace: "kubeslice-system"}},
		reconcile.Result{},
		nil,
	}

	client := NewClient()

	mf, _ := metrics.NewMetricsFactory(prometheus.NewRegistry(), metrics.MetricsFactoryOptions{})
	eventRecorder := mevents.NewEventRecorder(client, &runtime.Scheme{}, ossEvents.EventsMap, mevents.EventRecorderOptions{
		Cluster: clusterName,
	})
	reconciler := NewSliceReconciler(client, client, &eventRecorder, mf)
	reconciler.InjectClient(client)

	client.On("Delete",
		mock.IsType(expected.ctx),
		mock.IsType(&kubeslicev1beta1.Slice{}),
		mock.IsType([]k8sclient.DeleteOption(nil)),
	).Return(nil)

	err := reconciler.deleteSliceResourceOnSpoke(expected.ctx, controllerSlice)
	if expected.err != err {
		t.Error("Expected error:", expected.err, " but got ", err)
	}
}
