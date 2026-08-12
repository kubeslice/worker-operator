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

package namespace

import (
	"context"
	"os"
	"testing"

	hubv1alpha1 "github.com/kubeslice/apis/pkg/controller/v1alpha1"
	mevents "github.com/kubeslice/kubeslice-monitoring/pkg/events"
	"github.com/kubeslice/worker-operator/controllers"
	ossEvents "github.com/kubeslice/worker-operator/events"
	hub "github.com/kubeslice/worker-operator/pkg/hub/hubclient"
	utilmock "github.com/kubeslice/worker-operator/pkg/mocks"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

var testNamespaceName = "test-app-namespace"
var testSliceName = "test-slice"
var testClusterName = "test-cluster"
var testProjectNs = "kubeslice-avesha"

func TestReconcileNamespaceCreated(t *testing.T) {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: testNamespaceName,
			Labels: map[string]string{
				controllers.ApplicationNamespaceSelectorLabelKey: testSliceName,
			},
		},
	}

	hubCluster := &hubv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testClusterName,
			Namespace: testProjectNs,
		},
		Status: hubv1alpha1.ClusterStatus{
			Namespaces: []hubv1alpha1.NamespacesConfig{},
		},
	}

	expected := struct {
		ctx context.Context
		req ctrl.Request
		res ctrl.Result
		err error
	}{
		context.Background(),
		ctrl.Request{NamespacedName: types.NamespacedName{Name: testNamespaceName}},
		ctrl.Result{},
		nil,
	}

	os.Setenv("CLUSTER_NAME", testClusterName)
	os.Setenv("HUB_PROJECT_NAMESPACE", testProjectNs)

	client := utilmock.NewClient()
	hubClient := utilmock.NewClient()
	eventRecorder := mevents.NewEventRecorder(client, scheme.Scheme, ossEvents.EventsMap, mevents.EventRecorderOptions{})
	reconciler := &Reconciler{
		Client:        client,
		EventRecorder: &eventRecorder,
		Hubclient: &hub.HubClientConfig{
			Client: hubClient,
		},
	}

	nsKey := types.NamespacedName{Name: testNamespaceName}

	client.On("Get",
		mock.Anything,
		mock.IsType(nsKey),
		mock.IsType(&corev1.Namespace{}),
	).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*corev1.Namespace)
		*arg = *namespace
	})

	hubClient.On("Get",
		mock.Anything,
		mock.IsType(types.NamespacedName{Name: testClusterName, Namespace: testProjectNs}),
		mock.IsType(&hubv1alpha1.Cluster{}),
	).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*hubv1alpha1.Cluster)
		*arg = *hubCluster
	})

	hubClient.StatusMock.On("Update",
		mock.Anything,
		mock.IsType(&hubv1alpha1.Cluster{}),
		mock.IsType([]k8sclient.UpdateOption(nil)),
	).Return(nil)

	hubClient.StatusMock.On("Update",
		mock.Anything,
		mock.IsType(&hubv1alpha1.Cluster{}),
		mock.IsType([]k8sclient.SubResourceUpdateOption(nil)),
	).Return(nil)

	client.On("Create",
		mock.Anything,
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

func TestReconcileNamespaceDeleted(t *testing.T) {
	hubCluster := &hubv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testClusterName,
			Namespace: testProjectNs,
		},
		Status: hubv1alpha1.ClusterStatus{
			Namespaces: []hubv1alpha1.NamespacesConfig{
				{
					Name:      testNamespaceName,
					SliceName: testSliceName,
				},
			},
		},
	}

	expected := struct {
		ctx context.Context
		req ctrl.Request
		res ctrl.Result
		err error
	}{
		context.Background(),
		ctrl.Request{NamespacedName: types.NamespacedName{Name: testNamespaceName}},
		ctrl.Result{},
		nil,
	}

	os.Setenv("CLUSTER_NAME", testClusterName)
	os.Setenv("HUB_PROJECT_NAMESPACE", testProjectNs)

	client := utilmock.NewClient()
	hubClient := utilmock.NewClient()
	eventRecorder := mevents.NewEventRecorder(client, scheme.Scheme, ossEvents.EventsMap, mevents.EventRecorderOptions{})
	reconciler := &Reconciler{
		Client:        client,
		EventRecorder: &eventRecorder,
		Hubclient: &hub.HubClientConfig{
			Client: hubClient,
		},
	}

	nsKey := types.NamespacedName{Name: testNamespaceName}

	client.On("Get",
		mock.Anything,
		mock.IsType(nsKey),
		mock.IsType(&corev1.Namespace{}),
	).Return(apierrors.NewNotFound(schema.GroupResource{Resource: "namespace"}, testNamespaceName))

	hubClient.On("Get",
		mock.Anything,
		mock.IsType(types.NamespacedName{Name: testClusterName, Namespace: testProjectNs}),
		mock.IsType(&hubv1alpha1.Cluster{}),
	).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*hubv1alpha1.Cluster)
		*arg = *hubCluster
	})

	hubClient.StatusMock.On("Update",
		mock.Anything,
		mock.IsType(&hubv1alpha1.Cluster{}),
		mock.IsType([]k8sclient.UpdateOption(nil)),
	).Return(nil)

	hubClient.StatusMock.On("Update",
		mock.Anything,
		mock.IsType(&hubv1alpha1.Cluster{}),
		mock.IsType([]k8sclient.SubResourceUpdateOption(nil)),
	).Return(nil)

	client.On("Create",
		mock.Anything,
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

func TestReconcileNamespaceExcluded(t *testing.T) {
	expected := struct {
		ctx context.Context
		req ctrl.Request
		res ctrl.Result
		err error
	}{
		context.Background(),
		ctrl.Request{NamespacedName: types.NamespacedName{Name: "kube-system"}},
		ctrl.Result{},
		nil,
	}

	os.Setenv("EXCLUDED_NS", "kube-system,kube-public")

	client := utilmock.NewClient()
	eventRecorder := mevents.NewEventRecorder(client, scheme.Scheme, ossEvents.EventsMap, mevents.EventRecorderOptions{})
	reconciler := &Reconciler{
		Client:        client,
		EventRecorder: &eventRecorder,
	}

	result, err := reconciler.Reconcile(expected.ctx, expected.req)
	if expected.res != result {
		t.Error("Expected response :", expected.res, " but got ", result)
	}
	if expected.err != err {
		t.Error("Expected error:", expected.err, " but got ", err)
	}
}

func TestReconcileNamespaceUpdateExisting(t *testing.T) {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: testNamespaceName,
			Labels: map[string]string{
				controllers.ApplicationNamespaceSelectorLabelKey: "new-slice",
			},
		},
	}

	hubCluster := &hubv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testClusterName,
			Namespace: testProjectNs,
		},
		Status: hubv1alpha1.ClusterStatus{
			Namespaces: []hubv1alpha1.NamespacesConfig{
				{
					Name:      testNamespaceName,
					SliceName: testSliceName,
				},
			},
		},
	}

	expected := struct {
		ctx context.Context
		req ctrl.Request
		res ctrl.Result
		err error
	}{
		context.Background(),
		ctrl.Request{NamespacedName: types.NamespacedName{Name: testNamespaceName}},
		ctrl.Result{},
		nil,
	}

	os.Setenv("CLUSTER_NAME", testClusterName)
	os.Setenv("HUB_PROJECT_NAMESPACE", testProjectNs)

	client := utilmock.NewClient()
	hubClient := utilmock.NewClient()
	eventRecorder := mevents.NewEventRecorder(client, scheme.Scheme, ossEvents.EventsMap, mevents.EventRecorderOptions{})
	reconciler := &Reconciler{
		Client:        client,
		EventRecorder: &eventRecorder,
		Hubclient: &hub.HubClientConfig{
			Client: hubClient,
		},
	}

	nsKey := types.NamespacedName{Name: testNamespaceName}

	client.On("Get",
		mock.Anything,
		mock.IsType(nsKey),
		mock.IsType(&corev1.Namespace{}),
	).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*corev1.Namespace)
		*arg = *namespace
	})

	hubClient.On("Get",
		mock.Anything,
		mock.IsType(types.NamespacedName{Name: testClusterName, Namespace: testProjectNs}),
		mock.IsType(&hubv1alpha1.Cluster{}),
	).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*hubv1alpha1.Cluster)
		*arg = *hubCluster
	})

	hubClient.StatusMock.On("Update",
		mock.Anything,
		mock.IsType(&hubv1alpha1.Cluster{}),
		mock.IsType([]k8sclient.UpdateOption(nil)),
	).Return(nil)

	hubClient.StatusMock.On("Update",
		mock.Anything,
		mock.IsType(&hubv1alpha1.Cluster{}),
		mock.IsType([]k8sclient.SubResourceUpdateOption(nil)),
	).Return(nil)

	client.On("Create",
		mock.Anything,
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

func TestReconcileNamespaceNoLabel(t *testing.T) {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   testNamespaceName,
			Labels: map[string]string{},
		},
	}

	hubCluster := &hubv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testClusterName,
			Namespace: testProjectNs,
		},
		Status: hubv1alpha1.ClusterStatus{
			Namespaces: []hubv1alpha1.NamespacesConfig{},
		},
	}

	expected := struct {
		ctx context.Context
		req ctrl.Request
		res ctrl.Result
		err error
	}{
		context.Background(),
		ctrl.Request{NamespacedName: types.NamespacedName{Name: testNamespaceName}},
		ctrl.Result{},
		nil,
	}

	os.Setenv("CLUSTER_NAME", testClusterName)
	os.Setenv("HUB_PROJECT_NAMESPACE", testProjectNs)

	client := utilmock.NewClient()
	hubClient := utilmock.NewClient()
	eventRecorder := mevents.NewEventRecorder(client, scheme.Scheme, ossEvents.EventsMap, mevents.EventRecorderOptions{})
	reconciler := &Reconciler{
		Client:        client,
		EventRecorder: &eventRecorder,
		Hubclient: &hub.HubClientConfig{
			Client: hubClient,
		},
	}

	nsKey := types.NamespacedName{Name: testNamespaceName}

	client.On("Get",
		mock.Anything,
		mock.IsType(nsKey),
		mock.IsType(&corev1.Namespace{}),
	).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*corev1.Namespace)
		*arg = *namespace
	})

	hubClient.On("Get",
		mock.Anything,
		mock.IsType(types.NamespacedName{Name: testClusterName, Namespace: testProjectNs}),
		mock.IsType(&hubv1alpha1.Cluster{}),
	).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*hubv1alpha1.Cluster)
		*arg = *hubCluster
	})

	hubClient.StatusMock.On("Update",
		mock.Anything,
		mock.IsType(&hubv1alpha1.Cluster{}),
		mock.IsType([]k8sclient.UpdateOption(nil)),
	).Return(nil)

	hubClient.StatusMock.On("Update",
		mock.Anything,
		mock.IsType(&hubv1alpha1.Cluster{}),
		mock.IsType([]k8sclient.SubResourceUpdateOption(nil)),
	).Return(nil)

	client.On("Create",
		mock.Anything,
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

func TestGetSliceNameFromNs(t *testing.T) {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: testNamespaceName,
			Labels: map[string]string{
				controllers.ApplicationNamespaceSelectorLabelKey: testSliceName,
			},
		},
	}

	client := utilmock.NewClient()
	reconciler := &Reconciler{
		Client: client,
	}

	nsKey := types.NamespacedName{Name: testNamespaceName}

	client.On("Get",
		mock.Anything,
		mock.IsType(nsKey),
		mock.IsType(&corev1.Namespace{}),
	).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*corev1.Namespace)
		*arg = *namespace
	})

	sliceName, err := reconciler.getSliceNameFromNs(testNamespaceName)
	if err != nil {
		t.Error("Expected no error but got:", err)
	}
	if sliceName != testSliceName {
		t.Errorf("Expected slice name %s but got %s", testSliceName, sliceName)
	}
}

func TestGetSliceNameFromNsNoLabels(t *testing.T) {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: testNamespaceName,
		},
	}

	client := utilmock.NewClient()
	reconciler := &Reconciler{
		Client: client,
	}

	nsKey := types.NamespacedName{Name: testNamespaceName}

	client.On("Get",
		mock.Anything,
		mock.IsType(nsKey),
		mock.IsType(&corev1.Namespace{}),
	).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*corev1.Namespace)
		*arg = *namespace
	})

	sliceName, err := reconciler.getSliceNameFromNs(testNamespaceName)
	if err != nil {
		t.Error("Expected no error but got:", err)
	}
	if sliceName != "" {
		t.Errorf("Expected empty slice name but got %s", sliceName)
	}
}
