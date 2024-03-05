package cluster

import (
	"context"
	"errors"
	"testing"

	hubv1alpha1 "github.com/kubeslice/apis/pkg/controller/v1alpha1"
	"github.com/kubeslice/kubeslice-monitoring/pkg/metrics"
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	utilmock "github.com/kubeslice/worker-operator/pkg/mocks"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var testClusterRoleRef = rbacv1.RoleRef{
	APIGroup: "rbac.authorization.k8s.io",
	Kind:     "ClusterRole",
	Name:     clusterRoleName,
}

var testClusterRoleBindingSubject = []rbacv1.Subject{{
	Kind:      "ServiceAccount",
	Name:      serviceAccountName,
	Namespace: ControlPlaneNamespace,
}}

var testOperatorClusterRole = &rbacv1.ClusterRole{
	ObjectMeta: metav1.ObjectMeta{
		Name: operatorClusterRoleName,
	},
	Rules: []rbacv1.PolicyRule{
		{
			APIGroups: []string{"apps"},
			Resources: []string{"deployments", "statefulsets", "daemonsets"},
			Verbs:     []string{"get", "list", "patch", "create", "update", "delete"},
		},
	},
}

var testClusterObj = &hubv1alpha1.Cluster{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "test-cluster",
		Namespace: "kubeslice-avesha",
	},
	Spec: hubv1alpha1.ClusterSpec{},
}

var (
	testProjectNamespace = "kubeslice-avesha"
	testClusterName      = "test-cluster"
)

func TestGetOperatorClusterRole(t *testing.T) {
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
	reconciler := NewReconciler(client, client, nil, mf)
	reconciler.InjectClient(client)
	ctx := context.WithValue(context.Background(), types.NamespacedName{Name: testClusterName, Namespace: testProjectNamespace}, testClusterObj)
	clusterRoleKey := types.NamespacedName{Name: operatorClusterRoleName}

	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(clusterRoleKey),
		mock.IsType(&rbacv1.ClusterRole{}),
	).Return(nil)
	_, err := reconciler.getOperatorClusterRole(ctx)
	assert.ErrorIs(t, expected.err, err)
}

func TestCreateDeregisterJobPositiveScenarios(t *testing.T) {
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
	reconciler := NewReconciler(client, client, nil, mf)
	reconciler.InjectClient(client)
	ctx := context.WithValue(context.Background(), types.NamespacedName{Name: testClusterName, Namespace: testProjectNamespace}, testClusterObj)
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
	client.StatusMock.On("Update",
		mock.IsType(ctx),
		mock.IsType(&hubv1alpha1.Cluster{}),
		mock.IsType([]k8sclient.SubResourceUpdateOption(nil)),
	).Return(nil)
	client.On("List",
		mock.IsType(ctx),
		mock.IsType(&kubeslicev1beta1.SliceList{}),
		mock.IsType([]k8sclient.ListOption(nil)),
	).Return(nil)
	client.On("List",
		mock.IsType(ctx),
		mock.IsType(&kubeslicev1beta1.SliceGatewayList{}),
		mock.IsType([]k8sclient.ListOption(nil)),
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
	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(types.NamespacedName{Name: ControlPlaneNamespace}),
		mock.IsType(&corev1.Namespace{}),
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
	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(types.NamespacedName{Name: clusterDeregisterConfigMap, Namespace: ControlPlaneNamespace}),
		mock.IsType(&corev1.ConfigMap{}),
	).Return(nil)
	client.On("Delete",
		mock.IsType(ctx),
		mock.IsType(&corev1.ConfigMap{}),
		mock.IsType([]k8sclient.DeleteOption{}),
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
	err := reconciler.createDeregisterJob(ctx, testClusterObj)
	assert.ErrorIs(t, expected.err, err)
}

func TestReconcilerFailToUpdateClusterRegistrationStatus(t *testing.T) {
	expected := struct {
		ctx    context.Context
		req    reconcile.Request
		res    reconcile.Result
		errMsg string
	}{

		context.WithValue(context.Background(), types.NamespacedName{Namespace: "kube-slice", Name: "kube-slice"}, testClusterObj),
		reconcile.Request{NamespacedName: types.NamespacedName{Name: testClusterName, Namespace: testProjectNamespace}},
		reconcile.Result{},
		"error updating status of deregistration on the controller",
	}
	client := utilmock.NewClient()

	mf, _ := metrics.NewMetricsFactory(prometheus.NewRegistry(), metrics.MetricsFactoryOptions{})
	reconciler := NewReconciler(client, client, nil, mf)
	reconciler.InjectClient(client)
	ctx := context.WithValue(context.Background(), types.NamespacedName{Name: testClusterName, Namespace: testProjectNamespace}, testClusterObj)
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
	client.StatusMock.On("Update",
		mock.IsType(ctx),
		mock.IsType(&hubv1alpha1.Cluster{}),
		mock.IsType([]k8sclient.SubResourceUpdateOption(nil)),
	).Return(errors.New("error updating status of deregistration on the controller"))

	err := reconciler.createDeregisterJob(ctx, testClusterObj)
	assert.Equal(t, expected.errMsg, err.Error())
}

func TestReconcilerFailToCreateServiceAccount(t *testing.T) {
	expected := struct {
		ctx    context.Context
		req    reconcile.Request
		res    reconcile.Result
		errMsg string
	}{

		context.WithValue(context.Background(), types.NamespacedName{Namespace: "kube-slice", Name: "kube-slice"}, testClusterObj),
		reconcile.Request{NamespacedName: types.NamespacedName{Name: testClusterName, Namespace: testProjectNamespace}},
		reconcile.Result{},
		"unable to create service account",
	}
	client := utilmock.NewClient()

	mf, _ := metrics.NewMetricsFactory(prometheus.NewRegistry(), metrics.MetricsFactoryOptions{})
	reconciler := NewReconciler(client, client, nil, mf)
	reconciler.InjectClient(client)
	ctx := context.WithValue(context.Background(), types.NamespacedName{Name: testClusterName, Namespace: testProjectNamespace}, testClusterObj)
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
	client.StatusMock.On("Update",
		mock.IsType(ctx),
		mock.IsType(&hubv1alpha1.Cluster{}),
		mock.IsType([]k8sclient.SubResourceUpdateOption(nil)),
	).Return(nil)
	client.On("List",
		mock.IsType(ctx),
		mock.IsType(&kubeslicev1beta1.SliceList{}),
		mock.IsType([]k8sclient.ListOption(nil)),
	).Return(nil)
	client.On("List",
		mock.IsType(ctx),
		mock.IsType(&kubeslicev1beta1.SliceGatewayList{}),
		mock.IsType([]k8sclient.ListOption(nil)),
	).Return(nil)
	client.On("Create",
		mock.IsType(ctx),
		mock.IsType(&corev1.ServiceAccount{}),
		mock.IsType([]k8sclient.CreateOption(nil)),
	).Return(errors.New("unable to create service account"))

	err := reconciler.createDeregisterJob(ctx, testClusterObj)
	assert.Equal(t, expected.errMsg, err.Error())
}

func TestReconcilerFailToFetchOperatorClusterRole(t *testing.T) {
	expected := struct {
		ctx    context.Context
		req    reconcile.Request
		res    reconcile.Result
		errMsg string
	}{

		context.WithValue(context.Background(), types.NamespacedName{Namespace: "kube-slice", Name: "kube-slice"}, testClusterObj),
		reconcile.Request{NamespacedName: types.NamespacedName{Name: testClusterName, Namespace: testProjectNamespace}},
		reconcile.Result{},
		"unable to fetch operator clusterrole",
	}
	client := utilmock.NewClient()

	mf, _ := metrics.NewMetricsFactory(prometheus.NewRegistry(), metrics.MetricsFactoryOptions{})
	reconciler := NewReconciler(client, client, nil, mf)
	reconciler.InjectClient(client)
	ctx := context.WithValue(context.Background(), types.NamespacedName{Name: testClusterName, Namespace: testProjectNamespace}, testClusterObj)
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
	client.StatusMock.On("Update",
		mock.IsType(ctx),
		mock.IsType(&hubv1alpha1.Cluster{}),
		mock.IsType([]k8sclient.SubResourceUpdateOption(nil)),
	).Return(nil)
	client.On("List",
		mock.IsType(ctx),
		mock.IsType(&kubeslicev1beta1.SliceList{}),
		mock.IsType([]k8sclient.ListOption(nil)),
	).Return(nil)
	client.On("List",
		mock.IsType(ctx),
		mock.IsType(&kubeslicev1beta1.SliceGatewayList{}),
		mock.IsType([]k8sclient.ListOption(nil)),
	).Return(nil)
	client.On("Create",
		mock.IsType(ctx),
		mock.IsType(&corev1.ServiceAccount{}),
		mock.IsType([]k8sclient.CreateOption(nil)),
	).Return(nil)
	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(&corev1.ServiceAccount{}),
		mock.IsType([]k8sclient.CreateOption(nil)),
	).Return(nil)
	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(types.NamespacedName{Name: operatorClusterRoleName}),
		mock.IsType(&rbacv1.ClusterRole{}),
	).Return(errors.New("unable to fetch operator clusterrole"))

	err := reconciler.createDeregisterJob(ctx, testClusterObj)
	assert.Equal(t, expected.errMsg, err.Error())
}

func TestReconcilerFailToCreateClusterRole(t *testing.T) {
	expected := struct {
		ctx    context.Context
		req    reconcile.Request
		res    reconcile.Result
		errMsg string
	}{

		context.WithValue(context.Background(), types.NamespacedName{Namespace: "kube-slice", Name: "kube-slice"}, testClusterObj),
		reconcile.Request{NamespacedName: types.NamespacedName{Name: testClusterName, Namespace: testProjectNamespace}},
		reconcile.Result{},
		"unable to create cluster role",
	}
	client := utilmock.NewClient()

	mf, _ := metrics.NewMetricsFactory(prometheus.NewRegistry(), metrics.MetricsFactoryOptions{})
	reconciler := NewReconciler(client, client, nil, mf)
	reconciler.InjectClient(client)
	ctx := context.WithValue(context.Background(), types.NamespacedName{Name: testClusterName, Namespace: testProjectNamespace}, testClusterObj)
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
	client.StatusMock.On("Update",
		mock.IsType(ctx),
		mock.IsType(&hubv1alpha1.Cluster{}),
		mock.IsType([]k8sclient.SubResourceUpdateOption(nil)),
	).Return(nil)
	client.On("List",
		mock.IsType(ctx),
		mock.IsType(&kubeslicev1beta1.SliceList{}),
		mock.IsType([]k8sclient.ListOption(nil)),
	).Return(nil)
	client.On("List",
		mock.IsType(ctx),
		mock.IsType(&kubeslicev1beta1.SliceGatewayList{}),
		mock.IsType([]k8sclient.ListOption(nil)),
	).Return(nil)
	client.On("Create",
		mock.IsType(ctx),
		mock.IsType(&corev1.ServiceAccount{}),
		mock.IsType([]k8sclient.CreateOption(nil)),
	).Return(nil)
	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(&corev1.ServiceAccount{}),
		mock.IsType([]k8sclient.CreateOption(nil)),
	).Return(nil)
	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(types.NamespacedName{Name: operatorClusterRoleName}),
		mock.IsType(&rbacv1.ClusterRole{}),
	).Return(nil)
	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(types.NamespacedName{Name: ControlPlaneNamespace}),
		mock.IsType(&corev1.Namespace{}),
	).Return(nil)
	client.On("Create",
		mock.IsType(ctx),
		mock.IsType(&rbacv1.ClusterRole{}),
		mock.IsType([]k8sclient.CreateOption(nil)),
	).Return(errors.New("unable to create cluster role"))

	err := reconciler.createDeregisterJob(ctx, testClusterObj)
	assert.Equal(t, expected.errMsg, err.Error())
}

func TestReconcilerFailToCreateClusterRoleBinding(t *testing.T) {
	expected := struct {
		ctx    context.Context
		req    reconcile.Request
		res    reconcile.Result
		errMsg string
	}{

		context.WithValue(context.Background(), types.NamespacedName{Namespace: "kube-slice", Name: "kube-slice"}, testClusterObj),
		reconcile.Request{NamespacedName: types.NamespacedName{Name: testClusterName, Namespace: testProjectNamespace}},
		reconcile.Result{},
		"unable to create cluster rolebinding",
	}
	client := utilmock.NewClient()

	mf, _ := metrics.NewMetricsFactory(prometheus.NewRegistry(), metrics.MetricsFactoryOptions{})
	reconciler := NewReconciler(client, client, nil, mf)
	reconciler.InjectClient(client)
	ctx := context.WithValue(context.Background(), types.NamespacedName{Name: testClusterName, Namespace: testProjectNamespace}, testClusterObj)
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
	client.StatusMock.On("Update",
		mock.IsType(ctx),
		mock.IsType(&hubv1alpha1.Cluster{}),
		mock.IsType([]k8sclient.SubResourceUpdateOption(nil)),
	).Return(nil)
	client.On("List",
		mock.IsType(ctx),
		mock.IsType(&kubeslicev1beta1.SliceList{}),
		mock.IsType([]k8sclient.ListOption(nil)),
	).Return(nil)
	client.On("List",
		mock.IsType(ctx),
		mock.IsType(&kubeslicev1beta1.SliceGatewayList{}),
		mock.IsType([]k8sclient.ListOption(nil)),
	).Return(nil)
	client.On("Create",
		mock.IsType(ctx),
		mock.IsType(&corev1.ServiceAccount{}),
		mock.IsType([]k8sclient.CreateOption(nil)),
	).Return(nil)
	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(&corev1.ServiceAccount{}),
		mock.IsType([]k8sclient.CreateOption(nil)),
	).Return(nil)
	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(types.NamespacedName{Name: operatorClusterRoleName}),
		mock.IsType(&rbacv1.ClusterRole{}),
	).Return(nil)
	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(types.NamespacedName{Name: ControlPlaneNamespace}),
		mock.IsType(&corev1.Namespace{}),
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
	).Return(errors.New("unable to create cluster rolebinding"))

	err := reconciler.createDeregisterJob(ctx, testClusterObj)
	assert.Equal(t, expected.errMsg, err.Error())
}

func TestReconcilerFailToCreateConfigmap(t *testing.T) {
	expected := struct {
		ctx    context.Context
		req    reconcile.Request
		res    reconcile.Result
		errMsg string
	}{

		context.WithValue(context.Background(), types.NamespacedName{Namespace: "kube-slice", Name: "kube-slice"}, testClusterObj),
		reconcile.Request{NamespacedName: types.NamespacedName{Name: testClusterName, Namespace: testProjectNamespace}},
		reconcile.Result{},
		"Unable to create configmap",
	}
	client := utilmock.NewClient()

	mf, _ := metrics.NewMetricsFactory(prometheus.NewRegistry(), metrics.MetricsFactoryOptions{})
	reconciler := NewReconciler(client, client, nil, mf)
	reconciler.InjectClient(client)
	ctx := context.WithValue(context.Background(), types.NamespacedName{Name: testClusterName, Namespace: testProjectNamespace}, testClusterObj)
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
	client.StatusMock.On("Update",
		mock.IsType(ctx),
		mock.IsType(&hubv1alpha1.Cluster{}),
		mock.IsType([]k8sclient.SubResourceUpdateOption(nil)),
	).Return(nil)
	client.On("List",
		mock.IsType(ctx),
		mock.IsType(&kubeslicev1beta1.SliceList{}),
		mock.IsType([]k8sclient.ListOption(nil)),
	).Return(nil)
	client.On("List",
		mock.IsType(ctx),
		mock.IsType(&kubeslicev1beta1.SliceGatewayList{}),
		mock.IsType([]k8sclient.ListOption(nil)),
	).Return(nil)
	client.On("Create",
		mock.IsType(ctx),
		mock.IsType(&corev1.ServiceAccount{}),
		mock.IsType([]k8sclient.CreateOption(nil)),
	).Return(nil)
	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(&corev1.ServiceAccount{}),
		mock.IsType([]k8sclient.CreateOption(nil)),
	).Return(nil)
	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(types.NamespacedName{Name: operatorClusterRoleName}),
		mock.IsType(&rbacv1.ClusterRole{}),
	).Return(nil)
	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(types.NamespacedName{Name: ControlPlaneNamespace}),
		mock.IsType(&corev1.Namespace{}),
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
	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(types.NamespacedName{Name: clusterDeregisterConfigMap, Namespace: ControlPlaneNamespace}),
		mock.IsType(&corev1.ConfigMap{}),
	).Return(nil)
	client.On("Delete",
		mock.IsType(ctx),
		mock.IsType(&corev1.ConfigMap{}),
		mock.IsType([]k8sclient.DeleteOption{}),
	).Return(nil)
	client.On("Create",
		mock.IsType(ctx),
		mock.IsType(&corev1.ConfigMap{}),
		mock.IsType([]k8sclient.CreateOption(nil)),
	).Return(errors.New("Unable to create configmap"))
	err := reconciler.createDeregisterJob(ctx, testClusterObj)
	assert.Equal(t, expected.errMsg, err.Error())
}

func TestReconcilerFailToDeleteJob(t *testing.T) {
	expected := struct {
		ctx    context.Context
		req    reconcile.Request
		res    reconcile.Result
		errMsg string
	}{

		context.WithValue(context.Background(), types.NamespacedName{Namespace: "kube-slice", Name: "kube-slice"}, testClusterObj),
		reconcile.Request{NamespacedName: types.NamespacedName{Name: testClusterName, Namespace: testProjectNamespace}},
		reconcile.Result{},
		"Unable to delete deregister job",
	}
	client := utilmock.NewClient()

	mf, _ := metrics.NewMetricsFactory(prometheus.NewRegistry(), metrics.MetricsFactoryOptions{})
	reconciler := NewReconciler(client, client, nil, mf)
	reconciler.InjectClient(client)
	ctx := context.WithValue(context.Background(), types.NamespacedName{Name: testClusterName, Namespace: testProjectNamespace}, testClusterObj)
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
	client.StatusMock.On("Update",
		mock.IsType(ctx),
		mock.IsType(&hubv1alpha1.Cluster{}),
		mock.IsType([]k8sclient.SubResourceUpdateOption(nil)),
	).Return(nil)
	client.On("List",
		mock.IsType(ctx),
		mock.IsType(&kubeslicev1beta1.SliceList{}),
		mock.IsType([]k8sclient.ListOption(nil)),
	).Return(nil)
	client.On("List",
		mock.IsType(ctx),
		mock.IsType(&kubeslicev1beta1.SliceGatewayList{}),
		mock.IsType([]k8sclient.ListOption(nil)),
	).Return(nil)
	client.On("Create",
		mock.IsType(ctx),
		mock.IsType(&corev1.ServiceAccount{}),
		mock.IsType([]k8sclient.CreateOption(nil)),
	).Return(nil)
	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(&corev1.ServiceAccount{}),
		mock.IsType([]k8sclient.CreateOption(nil)),
	).Return(nil)
	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(types.NamespacedName{Name: operatorClusterRoleName}),
		mock.IsType(&rbacv1.ClusterRole{}),
	).Return(nil)
	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(types.NamespacedName{Name: ControlPlaneNamespace}),
		mock.IsType(&corev1.Namespace{}),
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
	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(types.NamespacedName{Name: clusterDeregisterConfigMap, Namespace: ControlPlaneNamespace}),
		mock.IsType(&corev1.ConfigMap{}),
	).Return(nil)
	client.On("Delete",
		mock.IsType(ctx),
		mock.IsType(&corev1.ConfigMap{}),
		mock.IsType([]k8sclient.DeleteOption{}),
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
	).Return(errors.New("Unable to delete deregister job"))

	err := reconciler.createDeregisterJob(ctx, testClusterObj)
	assert.Equal(t, expected.errMsg, err.Error())
}

func TestReconcilerFailToCreateDeregisterJob(t *testing.T) {
	expected := struct {
		ctx    context.Context
		req    reconcile.Request
		res    reconcile.Result
		errMsg string
	}{

		context.WithValue(context.Background(), types.NamespacedName{Namespace: "kube-slice", Name: "kube-slice"}, testClusterObj),
		reconcile.Request{NamespacedName: types.NamespacedName{Name: testClusterName, Namespace: testProjectNamespace}},
		reconcile.Result{},
		"Unable to create deregister job",
	}
	client := utilmock.NewClient()

	mf, _ := metrics.NewMetricsFactory(prometheus.NewRegistry(), metrics.MetricsFactoryOptions{})
	reconciler := NewReconciler(client, client, nil, mf)
	reconciler.InjectClient(client)
	ctx := context.WithValue(context.Background(), types.NamespacedName{Name: testClusterName, Namespace: testProjectNamespace}, testClusterObj)
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
	client.StatusMock.On("Update",
		mock.IsType(ctx),
		mock.IsType(&hubv1alpha1.Cluster{}),
		mock.IsType([]k8sclient.SubResourceUpdateOption(nil)),
	).Return(nil)
	client.On("List",
		mock.IsType(ctx),
		mock.IsType(&kubeslicev1beta1.SliceList{}),
		mock.IsType([]k8sclient.ListOption(nil)),
	).Return(nil)
	client.On("List",
		mock.IsType(ctx),
		mock.IsType(&kubeslicev1beta1.SliceGatewayList{}),
		mock.IsType([]k8sclient.ListOption(nil)),
	).Return(nil)
	client.On("Create",
		mock.IsType(ctx),
		mock.IsType(&corev1.ServiceAccount{}),
		mock.IsType([]k8sclient.CreateOption(nil)),
	).Return(nil)
	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(&corev1.ServiceAccount{}),
		mock.IsType([]k8sclient.CreateOption(nil)),
	).Return(nil)
	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(types.NamespacedName{Name: operatorClusterRoleName}),
		mock.IsType(&rbacv1.ClusterRole{}),
	).Return(nil)
	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(types.NamespacedName{Name: ControlPlaneNamespace}),
		mock.IsType(&corev1.Namespace{}),
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
	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(types.NamespacedName{Name: clusterDeregisterConfigMap, Namespace: ControlPlaneNamespace}),
		mock.IsType(&corev1.ConfigMap{}),
	).Return(nil)
	client.On("Delete",
		mock.IsType(ctx),
		mock.IsType(&corev1.ConfigMap{}),
		mock.IsType([]k8sclient.DeleteOption{}),
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
	).Return(errors.New("Unable to create deregister job"))

	err := reconciler.createDeregisterJob(ctx, testClusterObj)
	assert.Equal(t, expected.errMsg, err.Error())
}

func TestGetConfigmapScriptData(t *testing.T) {
	data, err := getCleanupScript()
	assert.NoError(t, err)
	assert.NotZero(t, len(data), "unable to get configmap data")
}

func TestConstructJobForClusterDeregister(t *testing.T) {
	job := constructJobForClusterDeregister()
	assert.Equal(t, job.Name, deregisterJobName)
	assert.Equal(t, job.Namespace, ControlPlaneNamespace)
}

func TestConstructServiceAccount(t *testing.T) {
	sa := constructServiceAccount()
	assert.Equal(t, sa.Name, serviceAccountName)
	assert.Equal(t, sa.Namespace, ControlPlaneNamespace)
}

func TestConstructClusterRole(t *testing.T) {
	cr := constructClusterRole(testOperatorClusterRole, "random-uid")
	assert.Equal(t, cr.Rules, testOperatorClusterRole.Rules)
}

func TestConstructClusterRoleBinding(t *testing.T) {
	crb := constructClusterRoleBinding("random-uid")
	assert.Equal(t, crb.Name, clusterRoleBindingName)
	assert.Equal(t, crb.RoleRef, testClusterRoleRef)
	assert.Len(t, crb.Subjects, 1)
	assert.Equal(t, crb.Subjects[0], testClusterRoleBindingSubject[0])
}

func TestConstructConfigMap(t *testing.T) {
	data := "this is the data."
	cm := constructConfigMap(data)
	assert.Equal(t, cm.Data["kubeslice-cleanup.sh"], data)
}
