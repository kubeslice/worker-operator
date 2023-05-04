package cluster

import (
	"context"
	"errors"
	"testing"
	"time"

	hubv1alpha1 "github.com/kubeslice/apis/pkg/controller/v1alpha1"
	mevents "github.com/kubeslice/kubeslice-monitoring/pkg/events"
	"github.com/kubeslice/kubeslice-monitoring/pkg/metrics"
	"github.com/kubeslice/worker-operator/controllers"
	ossEvents "github.com/kubeslice/worker-operator/events"
	utilmock "github.com/kubeslice/worker-operator/pkg/mocks"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/mock"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/scheme"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var testClusterObjWithFinalizer = &hubv1alpha1.Cluster{
	ObjectMeta: metav1.ObjectMeta{
		Name:              "test-cluster",
		Namespace:         "kubeslice-avesha",
		DeletionTimestamp: &metav1.Time{Time: time.Now()},
		Finalizers:        []string{clusterDeregisterFinalizer},
	},
	Spec: hubv1alpha1.ClusterSpec{},
	Status: hubv1alpha1.ClusterStatus{
		IsDeregisterInProgress: false,
	},
}

func TestReconcileToReturnErrorWhileFetchingControllerCluster(t *testing.T) {
	expected := struct {
		ctx    context.Context
		req    reconcile.Request
		res    reconcile.Result
		errStr string
	}{

		context.WithValue(context.Background(), types.NamespacedName{Namespace: "kube-slice", Name: "kube-slice"}, testClusterObj),
		reconcile.Request{NamespacedName: types.NamespacedName{Name: testClusterName, Namespace: testProjectNamespace}},
		reconcile.Result{},
		"object not found",
	}
	client := utilmock.NewClient()

	ctx := context.WithValue(context.Background(), types.NamespacedName{Name: testClusterName, Namespace: testProjectNamespace}, testClusterObj)
	clusterKey := types.NamespacedName{Namespace: testProjectNamespace, Name: testClusterName}

	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(clusterKey),
		mock.IsType(&hubv1alpha1.Cluster{}),
	).Return(errors.New("object not found"))

	mf, _ := metrics.NewMetricsFactory(prometheus.NewRegistry(), metrics.MetricsFactoryOptions{})
	reconciler := NewReconciler(client, nil, mf)
	reconciler.InjectClient(client)
	result, err := reconciler.Reconcile(expected.ctx, expected.req)
	if expected.res != result {
		t.Error("Expected response :", expected.res, " but got ", result)
	}
	if expected.errStr != err.Error() {
		t.Error("Expected error:", expected.errStr, " but got ", err)
	}
}

func TestReconcileToCallHandleClusterDeletion(t *testing.T) {
	expected := struct {
		ctx    context.Context
		req    reconcile.Request
		res    reconcile.Result
		errStr string
	}{

		context.WithValue(context.Background(), types.NamespacedName{Namespace: "kube-slice", Name: "kube-slice"}, testClusterObj),
		reconcile.Request{NamespacedName: types.NamespacedName{Name: testClusterName, Namespace: testProjectNamespace}},
		reconcile.Result{},
		"failed to update cluster CR",
	}
	client := utilmock.NewClient()

	ctx := context.WithValue(context.Background(), types.NamespacedName{Name: testClusterName, Namespace: testProjectNamespace}, testClusterObj)
	clusterKey := types.NamespacedName{Namespace: testProjectNamespace, Name: testClusterName}

	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(clusterKey),
		mock.IsType(&hubv1alpha1.Cluster{}),
	).Return(nil)
	client.On("Update",
		mock.IsType(ctx),
		mock.IsType(&hubv1alpha1.Cluster{}),
		mock.IsType([]k8sclient.UpdateOption(nil)),
	).Return(errors.New("failed to update cluster CR"))

	mf, _ := metrics.NewMetricsFactory(prometheus.NewRegistry(), metrics.MetricsFactoryOptions{})
	reconciler := NewReconciler(client, nil, mf)
	reconciler.InjectClient(client)
	result, err := reconciler.Reconcile(expected.ctx, expected.req)
	if expected.res != result {
		t.Error("Expected response :", expected.res, " but got ", result)
	}
	if expected.errStr != err.Error() {
		t.Error("Expected error:", expected.errStr, " but got ", err)
	}
}

func TestReconcilerHandleClusterDeletion(t *testing.T) {
	expected := struct {
		ctx context.Context
		req reconcile.Request
		res reconcile.Result
		err error
	}{

		context.WithValue(context.Background(), types.NamespacedName{Namespace: "kube-slice", Name: "kube-slice"}, testClusterObj),
		reconcile.Request{NamespacedName: types.NamespacedName{Name: testClusterName, Namespace: testProjectNamespace}},
		reconcile.Result{},
		nil,
	}
	client := utilmock.NewClient()

	mf, _ := metrics.NewMetricsFactory(prometheus.NewRegistry(), metrics.MetricsFactoryOptions{})
	reconciler := NewReconciler(client, nil, mf)
	reconciler.InjectClient(client)
	ctx := context.WithValue(context.Background(), types.NamespacedName{Name: testClusterName, Namespace: testProjectNamespace}, testClusterObj)

	client.On("Update",
		mock.IsType(ctx),
		mock.IsType(&hubv1alpha1.Cluster{}),
		mock.IsType([]k8sclient.UpdateOption(nil)),
	).Return(nil)

	testClusterObj.Status.IsDeregisterInProgress = true
	_, _, err := reconciler.handleClusterDeletion(testClusterObj, ctx, expected.req)
	if expected.err != err {
		t.Error("Expected error:", expected.err, " but got ", err)
	}
}

func TestReconcilerHandleExternalDependency(t *testing.T) {
	expected := struct {
		ctx context.Context
		req reconcile.Request
		res reconcile.Result
		err error
	}{

		context.WithValue(context.Background(), types.NamespacedName{Namespace: "kube-slice", Name: "kube-slice"}, testClusterObjWithFinalizer),
		reconcile.Request{NamespacedName: types.NamespacedName{Name: testClusterName, Namespace: testProjectNamespace}},
		reconcile.Result{},
		nil,
	}
	client := utilmock.NewClient()

	mf, _ := metrics.NewMetricsFactory(prometheus.NewRegistry(), metrics.MetricsFactoryOptions{})
	reconciler := NewReconciler(client, nil, mf)
	reconciler.InjectClient(client)
	ctx := context.WithValue(context.Background(), types.NamespacedName{Name: testClusterName, Namespace: testProjectNamespace}, testClusterObjWithFinalizer)
	clusterKey := types.NamespacedName{Namespace: testProjectNamespace, Name: testClusterName}

	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(clusterKey),
		mock.IsType(&hubv1alpha1.Cluster{}),
	).Return(nil)
	client.StatusMock.On("Update",
		mock.IsType(ctx),
		mock.IsType(&hubv1alpha1.Cluster{}),
		mock.IsType([]k8sclient.UpdateOption(nil)),
	).Return(nil)
	client.On("Create",
		mock.IsType(ctx),
		mock.IsType(&corev1.ServiceAccount{}),
		mock.IsType([]k8sclient.CreateOption(nil)),
	).Return(nil)
	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(types.NamespacedName{Name: operatorClusterRoleName}),
		mock.IsType(&rbacv1.ClusterRole{}),
	).Return(nil)
	client.On("Create",
		mock.IsType(ctx),
		mock.IsType(&rbacv1.ClusterRole{}),
		mock.IsType([]k8sclient.CreateOption(nil)),
	).Return(nil)
	client.On("Create",
		mock.IsType(ctx),
		mock.IsType(&rbacv1.ClusterRoleBinding{}),
		mock.IsType([]k8sclient.CreateOption(nil)),
	).Return(nil)
	client.On("Create",
		mock.IsType(ctx),
		mock.IsType(&corev1.ConfigMap{}),
		mock.IsType([]k8sclient.CreateOption(nil)),
	).Return(nil)
	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(types.NamespacedName{Name: deregisterJobName, Namespace: ControlPlaneNamespace}),
		mock.IsType(&batchv1.Job{}),
	).Return(nil)
	client.On("Delete",
		mock.IsType(ctx),
		mock.IsType(&batchv1.Job{}),
		mock.IsType([]k8sclient.DeleteOption{}),
	).Return(nil)
	client.On("Create",
		mock.IsType(ctx),
		mock.IsType(&batchv1.Job{}),
		mock.IsType([]k8sclient.CreateOption(nil)),
	).Return(nil)

	_, _, err := reconciler.handleClusterDeletion(testClusterObjWithFinalizer, ctx, expected.req)
	if expected.err != err {
		t.Error("Expected error:", expected.err, " but got ", err)
	}
}

func TestReconcilerToFailWhileCallingCreateDeregisterJob(t *testing.T) {
	expected := struct {
		ctx    context.Context
		req    reconcile.Request
		res    reconcile.Result
		errMsg string
	}{

		context.WithValue(context.Background(), types.NamespacedName{Namespace: "kube-slice", Name: "kube-slice"}, testClusterObjWithFinalizer),
		reconcile.Request{NamespacedName: types.NamespacedName{Name: testClusterName, Namespace: testProjectNamespace}},
		reconcile.Result{},
		"error updating status of deregistration on the controller",
	}
	client := utilmock.NewClient()

	gv := hubv1alpha1.GroupVersion
	testScheme := runtime.NewScheme()
	err := scheme.AddToScheme(testScheme)
	if err != nil {
		t.Fatalf("Error adding core scheme to test scheme: %v", err)
	}
	testScheme.AddKnownTypeWithName(gv.WithKind("Cluster"), &hubv1alpha1.Cluster{})

	testClusterEventRecorder := mevents.NewEventRecorder(client, testScheme, ossEvents.EventsMap, mevents.EventRecorderOptions{
		Cluster:   "test-cluster",
		Project:   "avesha",
		Component: "worker-operator",
		Namespace: controllers.ControlPlaneNamespace,
	})
	mf, _ := metrics.NewMetricsFactory(prometheus.NewRegistry(), metrics.MetricsFactoryOptions{})
	reconciler := NewReconciler(client, testClusterEventRecorder, mf)
	reconciler.InjectClient(client)
	ctx := context.WithValue(context.Background(), types.NamespacedName{Name: testClusterName, Namespace: testProjectNamespace}, testClusterObjWithFinalizer)
	clusterKey := types.NamespacedName{Namespace: testProjectNamespace, Name: testClusterName}

	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(clusterKey),
		mock.IsType(&hubv1alpha1.Cluster{}),
	).Return(nil)
	client.StatusMock.On("Update",
		mock.IsType(ctx),
		mock.IsType(&hubv1alpha1.Cluster{}),
		mock.IsType([]k8sclient.UpdateOption(nil)),
	).Return(errors.New("error updating status of deregistration on the controller"))
	client.On("Create",
		mock.IsType(ctx),
		mock.IsType(&corev1.Event{}),
		mock.IsType([]k8sclient.CreateOption(nil)),
	).Return(nil)

	_, _, err = reconciler.handleClusterDeletion(testClusterObjWithFinalizer, ctx, expected.req)
	if err != nil && expected.errMsg != err.Error() {
		t.Error("Expected error:", expected.errMsg, " but got ", err.Error())
	}
}
