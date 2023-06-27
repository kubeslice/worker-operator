package vpnkeyrotation

import (
	"context"
	"os"
	"testing"
	"time"

	"errors"

	"github.com/kubeslice/worker-operator/pkg/slicegwrecycler"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	hubv1alpha1 "github.com/kubeslice/apis/pkg/controller/v1alpha1"
	spokev1alpha1 "github.com/kubeslice/apis/pkg/worker/v1alpha1"
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"

	"github.com/kubeslice/kubeslice-monitoring/pkg/metrics"
	hub "github.com/kubeslice/worker-operator/pkg/hub/hubclient"
	utilmock "github.com/kubeslice/worker-operator/pkg/mocks"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	testProjectNamespace   = "kubeslice-avesha"
	testVPNKeyRotationName = "test-vpn"
	testWorkerNamespace    = "kubeslice-system"
)

var testVPNKeyRotationObject = &hubv1alpha1.VpnKeyRotation{
	ObjectMeta: metav1.ObjectMeta{
		Name:      testVPNKeyRotationName,
		Namespace: testProjectNamespace,
	},
	Spec: hubv1alpha1.VpnKeyRotationSpec{
		ClusterGatewayMapping: map[string][]string{
			"cluster1": {"fire-worker-2-worker-1", "fire-worker-2-worker-3", "fire-worker-2-worker-4"},
		},
		CertificateCreationTime: metav1.Time{Time: time.Now()},
	},
	Status: hubv1alpha1.VpnKeyRotationStatus{},
}

var testVPNKeyRotationObjectWithInProgressStatus = &hubv1alpha1.VpnKeyRotation{
	ObjectMeta: metav1.ObjectMeta{
		Name:      testVPNKeyRotationName,
		Namespace: testProjectNamespace,
	},
	Spec: hubv1alpha1.VpnKeyRotationSpec{
		ClusterGatewayMapping: map[string][]string{
			"cluster1": {"fire-worker-2-worker-1", "fire-worker-2-worker-3", "fire-worker-2-worker-4"},
		},
		CertificateCreationTime: metav1.Time{Time: time.Now()},
	},
	Status: hubv1alpha1.VpnKeyRotationStatus{
		CurrentRotationState: map[string]hubv1alpha1.StatusOfKeyRotation{
			"fire-worker-2-worker-1": {
				Status:               hubv1alpha1.InProgress,
				LastUpdatedTimestamp: metav1.Time{Time: time.Now()},
			},
		},
	},
}

var testVPNKeyRotationObjectWithIntervalTest = &hubv1alpha1.VpnKeyRotation{
	ObjectMeta: metav1.ObjectMeta{
		Name:      testVPNKeyRotationName,
		Namespace: testProjectNamespace,
	},
	Spec: hubv1alpha1.VpnKeyRotationSpec{
		ClusterGatewayMapping: map[string][]string{
			"cluster1": {"fire-worker-2-worker-1", "fire-worker-2-worker-3", "fire-worker-2-worker-4"},
		},
		CertificateCreationTime: metav1.Time{Time: time.Now()},
	},
	Status: hubv1alpha1.VpnKeyRotationStatus{
		CurrentRotationState: map[string]hubv1alpha1.StatusOfKeyRotation{
			"fire-worker-2-worker-1": {
				Status:               hubv1alpha1.Complete,
				LastUpdatedTimestamp: metav1.Time{Time: time.Now().AddDate(0, 0, -1)}, // anything less than current date
			},
			"fire-worker-2-worker-3": {
				Status:               hubv1alpha1.SecretUpdated,
				LastUpdatedTimestamp: metav1.Time{Time: time.Now().AddDate(0, 0, -2)}, // anything less than current date
			},
			"fire-worker-2-worker-4": {
				Status:               hubv1alpha1.Complete,
				LastUpdatedTimestamp: metav1.Time{Time: time.Now().AddDate(0, 0, -3)}, // anything less than current date
			},
		},
	},
}

var sliceGateway = &kubeslicev1beta1.SliceGateway{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "fire-worker-2-worker-1",
		Namespace: testWorkerNamespace,
	},
	Status: kubeslicev1beta1.SliceGatewayStatus{
		Config: kubeslicev1beta1.SliceGatewayConfig{
			SliceGatewayHostType:        "Server",
			SliceGatewayRemoteGatewayID: "fire-worker-2-worker-3", // mocking client status
		},
	},
}

var workerSlice = &kubeslicev1beta1.Slice{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "fire",
		Namespace: testWorkerNamespace,
	},
	Status: kubeslicev1beta1.SliceStatus{},
	Spec:   kubeslicev1beta1.SliceSpec{},
}

var podListCustom = &corev1.PodList{
	Items: []corev1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod-1",
				Namespace: testWorkerNamespace,
			},
			Spec: corev1.PodSpec{},
			Status: corev1.PodStatus{
				PodIP: "192.1.1.1",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod-2",
				Namespace: testWorkerNamespace,
			},
			Spec:   corev1.PodSpec{},
			Status: corev1.PodStatus{},
		},
	},
}

func TestReconcilerVPNRotationNotFound(t *testing.T) {
	expected := struct {
		ctx    context.Context
		req    reconcile.Request
		res    reconcile.Result
		errStr string
	}{

		context.WithValue(context.Background(), types.NamespacedName{Namespace: testProjectNamespace, Name: testVPNKeyRotationName}, testVPNKeyRotationObject),
		reconcile.Request{NamespacedName: types.NamespacedName{Name: testVPNKeyRotationName, Namespace: testProjectNamespace}},
		reconcile.Result{},
		"object not found",
	}
	client := utilmock.NewClient()
	mf, _ := metrics.NewMetricsFactory(prometheus.NewRegistry(), metrics.MetricsFactoryOptions{})
	reconciler := NewReconciler(client, nil, nil, mf, nil)
	reconciler.InjectClient(client)
	ctx := context.WithValue(context.Background(), types.NamespacedName{Name: testVPNKeyRotationName, Namespace: testProjectNamespace}, testVPNKeyRotationObject)
	vpmRotationKey := types.NamespacedName{Namespace: testProjectNamespace, Name: testVPNKeyRotationName}

	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(vpmRotationKey),
		mock.IsType(&hubv1alpha1.VpnKeyRotation{}),
	).Return(errors.New("object not found"))

	result, err := reconciler.Reconcile(expected.ctx, expected.req)
	if expected.res != result {
		t.Error("Expected response :", expected.res, " but got ", result)
	}
	if expected.errStr != err.Error() {
		t.Error("Expected error:", expected.errStr, " but got ", err)
	}
}

func TestReconcilerVPNRotationReconcilerForInitialStage(t *testing.T) {
	os.Setenv("CLUSTER_NAME", "cluster1")
	expected := struct {
		ctx context.Context
		req reconcile.Request
		res reconcile.Result
		err error
	}{
		context.WithValue(context.Background(), types.NamespacedName{Namespace: testProjectNamespace, Name: testVPNKeyRotationName}, testVPNKeyRotationObject),
		reconcile.Request{NamespacedName: types.NamespacedName{Name: testVPNKeyRotationName, Namespace: testProjectNamespace}},
		reconcile.Result{},
		nil,
	}
	client := utilmock.NewClient()
	mf, _ := metrics.NewMetricsFactory(prometheus.NewRegistry(), metrics.MetricsFactoryOptions{})
	reconciler := NewReconciler(client, nil, nil, mf, nil)
	reconciler.InjectClient(client)
	ctx := context.WithValue(context.Background(), types.NamespacedName{Name: testVPNKeyRotationName, Namespace: testProjectNamespace}, testVPNKeyRotationObject)
	vpmRotationKey := types.NamespacedName{Namespace: testProjectNamespace, Name: testVPNKeyRotationName}

	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(vpmRotationKey),
		mock.IsType(&hubv1alpha1.VpnKeyRotation{}),
	).Return(nil).Run(func(args mock.Arguments) {
		vpnKeyRotation := args.Get(2).(*hubv1alpha1.VpnKeyRotation)
		*vpnKeyRotation = *testVPNKeyRotationObject
	})
	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(vpmRotationKey),
		mock.IsType(&hubv1alpha1.VpnKeyRotation{}),
	).Return(nil)
	client.StatusMock.On("Update",
		mock.IsType(ctx),
		mock.IsType(&hubv1alpha1.VpnKeyRotation{}),
		mock.IsType([]k8sclient.UpdateOption(nil)),
	).Return(nil)
	result, _ := reconciler.Reconcile(expected.ctx, expected.req)
	if expected.res != result {
		t.Error("Expected response :", expected.res, " but got ", result)
	}
}

func TestReconcilerVPNRotationReconcilerCompletionAsSuccess(t *testing.T) {
	os.Setenv("CLUSTER_NAME", "cluster1")
	expected := struct {
		ctx context.Context
		req reconcile.Request
		res reconcile.Result
		err error
	}{
		context.WithValue(context.Background(), types.NamespacedName{Namespace: testProjectNamespace, Name: testVPNKeyRotationName}, testVPNKeyRotationObjectWithInProgressStatus),
		reconcile.Request{NamespacedName: types.NamespacedName{Name: testVPNKeyRotationName, Namespace: testProjectNamespace}},
		reconcile.Result{},
		nil,
	}
	client := utilmock.NewClient()
	mf, _ := metrics.NewMetricsFactory(prometheus.NewRegistry(), metrics.MetricsFactoryOptions{})
	reconciler := NewReconciler(client, &hub.HubClientConfig{
		Client: client,
	}, nil, mf, nil)
	reconciler.InjectClient(client)
	ctx := context.WithValue(context.Background(), types.NamespacedName{Name: testVPNKeyRotationName, Namespace: testProjectNamespace}, testVPNKeyRotationObjectWithInProgressStatus)
	vpmRotationKey := types.NamespacedName{Namespace: testProjectNamespace, Name: testVPNKeyRotationName}

	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(vpmRotationKey),
		mock.IsType(&hubv1alpha1.VpnKeyRotation{}),
	).Return(nil).Run(func(args mock.Arguments) {
		vpnKeyRotation := args.Get(2).(*hubv1alpha1.VpnKeyRotation)
		*vpnKeyRotation = *testVPNKeyRotationObjectWithInProgressStatus
	})
	client.On("List",
		mock.IsType(ctx),
		mock.IsType(&spokev1alpha1.WorkerSliceGwRecyclerList{}),
		mock.IsType([]k8sclient.ListOption(nil)),
	).Return(nil, nil)
	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(vpmRotationKey),
		mock.IsType(&hubv1alpha1.VpnKeyRotation{}),
	).Return(nil)
	client.StatusMock.On("Update",
		mock.IsType(ctx),
		mock.IsType(&hubv1alpha1.VpnKeyRotation{}),
		mock.IsType([]k8sclient.UpdateOption(nil)),
	).Return(nil)

	result, _ := reconciler.Reconcile(expected.ctx, expected.req)
	if expected.res != result {
		t.Error("Expected response :", expected.res, " but got ", result)
	}
}

func TestReconcilerVPNRotationReconcilerIntervalTestNegative(t *testing.T) {
	os.Setenv("CLUSTER_NAME", "cluster1")
	expected := struct {
		ctx    context.Context
		req    reconcile.Request
		res    reconcile.Result
		errMsg string
	}{
		context.WithValue(context.Background(), types.NamespacedName{Namespace: testProjectNamespace, Name: testVPNKeyRotationName}, testVPNKeyRotationObjectWithIntervalTest),
		reconcile.Request{NamespacedName: types.NamespacedName{Name: testVPNKeyRotationName, Namespace: testProjectNamespace}},
		reconcile.Result{},
		"object not found",
	}
	client := utilmock.NewClient()
	mf, _ := metrics.NewMetricsFactory(prometheus.NewRegistry(), metrics.MetricsFactoryOptions{})
	reconciler := NewReconciler(client, &hub.HubClientConfig{
		Client: client,
	}, nil, mf, nil)
	reconciler.InjectClient(client)
	ctx := context.WithValue(context.Background(), types.NamespacedName{Name: testVPNKeyRotationName, Namespace: testProjectNamespace}, testVPNKeyRotationObjectWithIntervalTest)
	vpmRotationKey := types.NamespacedName{Namespace: testProjectNamespace, Name: testVPNKeyRotationName}

	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(vpmRotationKey),
		mock.IsType(&hubv1alpha1.VpnKeyRotation{}),
	).Return(nil).Run(func(args mock.Arguments) {
		vpnKeyRotation := args.Get(2).(*hubv1alpha1.VpnKeyRotation)
		*vpnKeyRotation = *testVPNKeyRotationObjectWithIntervalTest
	})
	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(vpmRotationKey),
		mock.IsType(&hubv1alpha1.VpnKeyRotation{}),
	).Return(nil)
	client.StatusMock.On("Update",
		mock.IsType(ctx),
		mock.IsType(&hubv1alpha1.VpnKeyRotation{}),
		mock.IsType([]k8sclient.UpdateOption(nil)),
	).Return(nil)
	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(types.NamespacedName{Name: "fire-worker-2-worker-1-0"}),
		mock.IsType(&corev1.Secret{}),
	).Return(nil, false, nil)
	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(vpmRotationKey),
		mock.IsType(&hubv1alpha1.VpnKeyRotation{}),
	).Return(nil)
	client.StatusMock.On("Update",
		mock.IsType(ctx),
		mock.IsType(&hubv1alpha1.VpnKeyRotation{}),
		mock.IsType([]k8sclient.UpdateOption(nil)),
	).Return(nil)
	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(types.NamespacedName{Name: "fire-worker-2-worker-1"}),
		mock.IsType(&kubeslicev1beta1.SliceGateway{}),
	).Return(errors.New("object not found"))

	_, err := reconciler.Reconcile(expected.ctx, expected.req)
	if expected.errMsg != err.Error() {
		t.Error("Expected error:", expected.errMsg, " but got ", err)
	}
}

func TestReconcilerVPNRotationReconcilerIntervalTest(t *testing.T) {
	os.Setenv("CLUSTER_NAME", "cluster1")
	expected := struct {
		ctx    context.Context
		req    reconcile.Request
		res    reconcile.Result
		errStr string
	}{
		context.WithValue(context.Background(), types.NamespacedName{Namespace: testProjectNamespace, Name: testVPNKeyRotationName}, testVPNKeyRotationObjectWithIntervalTest),
		reconcile.Request{NamespacedName: types.NamespacedName{Name: testVPNKeyRotationName, Namespace: testProjectNamespace}},
		reconcile.Result{},
		"object not found",
	}
	client := utilmock.NewClient()
	mf, _ := metrics.NewMetricsFactory(prometheus.NewRegistry(), metrics.MetricsFactoryOptions{})
	reconciler := NewReconciler(client, &hub.HubClientConfig{
		Client: client,
	}, nil, mf, nil)
	reconciler.InjectClient(client)
	ctx := context.WithValue(context.Background(), types.NamespacedName{Name: testVPNKeyRotationName, Namespace: testProjectNamespace}, testVPNKeyRotationObjectWithIntervalTest)
	vpmRotationKey := types.NamespacedName{Namespace: testProjectNamespace, Name: testVPNKeyRotationName}

	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(vpmRotationKey),
		mock.IsType(&hubv1alpha1.VpnKeyRotation{}),
	).Return(nil).Run(func(args mock.Arguments) {
		vpnKeyRotation := args.Get(2).(*hubv1alpha1.VpnKeyRotation)
		*vpnKeyRotation = *testVPNKeyRotationObjectWithIntervalTest
	})
	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(vpmRotationKey),
		mock.IsType(&hubv1alpha1.VpnKeyRotation{}),
	).Return(nil)
	client.StatusMock.On("Update",
		mock.IsType(ctx),
		mock.IsType(&hubv1alpha1.VpnKeyRotation{}),
		mock.IsType([]k8sclient.UpdateOption(nil)),
	).Return(nil)
	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(types.NamespacedName{Name: "fire-worker-2-worker-1-0"}),
		mock.IsType(&corev1.Secret{}),
	).Return(nil, false, nil)
	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(vpmRotationKey),
		mock.IsType(&hubv1alpha1.VpnKeyRotation{}),
	).Return(nil)
	client.StatusMock.On("Update",
		mock.IsType(ctx),
		mock.IsType(&hubv1alpha1.VpnKeyRotation{}),
		mock.IsType([]k8sclient.UpdateOption(nil)),
	).Return(nil)
	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(types.NamespacedName{Name: "fire-worker-2-worker-1"}),
		mock.IsType(&kubeslicev1beta1.SliceGateway{}),
	).Return(nil).Run(func(args mock.Arguments) {
		sliceGatewayCR := args.Get(2).(*kubeslicev1beta1.SliceGateway)
		*sliceGatewayCR = *sliceGateway
	})
	client.On("List",
		mock.IsType(ctx),
		mock.IsType(&corev1.PodList{}),
		mock.IsType([]k8sclient.ListOption(nil)),
	).Return(nil).Run(func(args mock.Arguments) {
		podsList := args.Get(1).(*corev1.PodList)
		*podsList = *podListCustom
	})
	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(vpmRotationKey),
		mock.IsType(&hubv1alpha1.VpnKeyRotation{}),
	).Return(nil)
	client.StatusMock.On("Update",
		mock.IsType(ctx),
		mock.IsType(&hubv1alpha1.VpnKeyRotation{}),
		mock.IsType([]k8sclient.UpdateOption(nil)),
	).Return(nil)
	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(types.NamespacedName{Name: "fire-worker-2-worker-1"}),
		mock.IsType(&kubeslicev1beta1.Slice{}),
	).Return(errors.New("object not found"))
	result, err := reconciler.Reconcile(expected.ctx, expected.req)
	if expected.res != result {
		t.Error("Expected response :", expected.res, " but got ", result)
	}
	if expected.errStr != err.Error() {
		t.Error("Expected error:", expected.errStr, " but got ", err)
	}
}

func TestReconcilerVPNRotationReconcilerTriggerFSM(t *testing.T) {
	os.Setenv("CLUSTER_NAME", "cluster1")
	expected := struct {
		ctx context.Context
		req reconcile.Request
		res reconcile.Result
		err error
	}{
		context.WithValue(context.Background(), types.NamespacedName{Namespace: testProjectNamespace, Name: testVPNKeyRotationName}, testVPNKeyRotationObjectWithIntervalTest),
		reconcile.Request{NamespacedName: types.NamespacedName{Name: testVPNKeyRotationName, Namespace: testProjectNamespace}},
		reconcile.Result{},
		nil,
	}
	client := utilmock.NewClient()
	mf, _ := metrics.NewMetricsFactory(prometheus.NewRegistry(), metrics.MetricsFactoryOptions{})

	workerRecyclerClient, err := slicegwrecycler.NewVPNClientEmulator(k8sClient)
	if err != nil {
		os.Exit(1)
	}

	reconciler := NewReconciler(client, &hub.HubClientConfig{
		Client: client,
	}, nil, mf, workerRecyclerClient)
	reconciler.InjectClient(client)

	ctx := context.WithValue(context.Background(), types.NamespacedName{Name: testVPNKeyRotationName, Namespace: testProjectNamespace}, testVPNKeyRotationObjectWithIntervalTest)
	vpmRotationKey := types.NamespacedName{Namespace: testProjectNamespace, Name: testVPNKeyRotationName}
	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(vpmRotationKey),
		mock.IsType(&hubv1alpha1.VpnKeyRotation{}),
	).Return(nil).Run(func(args mock.Arguments) {
		vpnKeyRotation := args.Get(2).(*hubv1alpha1.VpnKeyRotation)
		*vpnKeyRotation = *testVPNKeyRotationObjectWithIntervalTest
	})
	client.On("List",
		mock.IsType(ctx),
		mock.IsType(&spokev1alpha1.WorkerSliceGwRecyclerList{}),
		mock.IsType([]k8sclient.ListOption(nil)),
	).Return(nil, nil)
	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(vpmRotationKey),
		mock.IsType(&hubv1alpha1.VpnKeyRotation{}),
	).Return(nil)
	client.StatusMock.On("Update",
		mock.IsType(ctx),
		mock.IsType(&hubv1alpha1.VpnKeyRotation{}),
		mock.IsType([]k8sclient.UpdateOption(nil)),
	).Return(nil)
	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(types.NamespacedName{Name: "fire-worker-2-worker-1-0"}),
		mock.IsType(&corev1.Secret{}),
	).Return(nil, false, nil)
	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(vpmRotationKey),
		mock.IsType(&hubv1alpha1.VpnKeyRotation{}),
	).Return(nil)
	client.StatusMock.On("Update",
		mock.IsType(ctx),
		mock.IsType(&hubv1alpha1.VpnKeyRotation{}),
		mock.IsType([]k8sclient.UpdateOption(nil)),
	).Return(nil)
	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(types.NamespacedName{Name: "fire-worker-2-worker-1"}),
		mock.IsType(&kubeslicev1beta1.SliceGateway{}),
	).Return(nil).Run(func(args mock.Arguments) {
		sliceGatewayCR := args.Get(2).(*kubeslicev1beta1.SliceGateway)
		*sliceGatewayCR = *sliceGateway
	})
	client.On("List",
		mock.IsType(ctx),
		mock.IsType(&corev1.PodList{}),
		mock.IsType([]k8sclient.ListOption(nil)),
	).Return(nil).Run(func(args mock.Arguments) {
		podsList := args.Get(1).(*corev1.PodList)
		*podsList = *podListCustom
	})
	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(vpmRotationKey),
		mock.IsType(&hubv1alpha1.VpnKeyRotation{}),
	).Return(nil)
	client.StatusMock.On("Update",
		mock.IsType(ctx),
		mock.IsType(&hubv1alpha1.VpnKeyRotation{}),
		mock.IsType([]k8sclient.UpdateOption(nil)),
	).Return(nil)
	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(types.NamespacedName{Name: "fire-worker-2-worker-1"}),
		mock.IsType(&kubeslicev1beta1.Slice{}),
	).Return(nil).Run(func(args mock.Arguments) {
		sliceCR := args.Get(2).(*kubeslicev1beta1.Slice)
		*sliceCR = *workerSlice
	})
	_, err = reconciler.Reconcile(expected.ctx, expected.req)
	if err != nil {
		t.Error("Expected response :", expected.res)
	}
}

func TestReconcilerSecretCreationError(t *testing.T) {
	vpnKeyObject := &hubv1alpha1.VpnKeyRotation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testVPNKeyRotationName,
			Namespace: testProjectNamespace,
		},
		Spec: hubv1alpha1.VpnKeyRotationSpec{
			ClusterGatewayMapping: map[string][]string{
				"cluster1": {"fire-worker-2-worker-1", "fire-worker-2-worker-3", "fire-worker-2-worker-4"},
			},
			CertificateCreationTime: metav1.Time{Time: time.Now()},
		},
		Status: hubv1alpha1.VpnKeyRotationStatus{
			CurrentRotationState: map[string]hubv1alpha1.StatusOfKeyRotation{
				"fire-worker-2-worker-1": {
					Status:               hubv1alpha1.Complete,
					LastUpdatedTimestamp: metav1.Time{Time: time.Now().AddDate(0, 0, -1)}, // anything less than current date
				},
				"fire-worker-2-worker-3": {
					Status:               hubv1alpha1.SecretUpdated,
					LastUpdatedTimestamp: metav1.Time{Time: time.Now().AddDate(0, 0, -2)}, // anything less than current date
				},
				"fire-worker-2-worker-4": {
					Status:               hubv1alpha1.Complete,
					LastUpdatedTimestamp: metav1.Time{Time: time.Now().AddDate(0, 0, -3)}, // anything less than current date
				},
			},
		},
	}
	os.Setenv("CLUSTER_NAME", "cluster1")
	expected := struct {
		ctx    context.Context
		req    reconcile.Request
		res    reconcile.Result
		errMsg string
	}{
		context.WithValue(context.Background(), types.NamespacedName{Namespace: testProjectNamespace, Name: testVPNKeyRotationName}, vpnKeyObject),
		reconcile.Request{NamespacedName: types.NamespacedName{Name: testVPNKeyRotationName, Namespace: testProjectNamespace}},
		reconcile.Result{},
		"unable to create secret to store slicegw certs in worker cluster",
	}
	client := utilmock.NewClient()
	mf, _ := metrics.NewMetricsFactory(prometheus.NewRegistry(), metrics.MetricsFactoryOptions{})
	reconciler := NewReconciler(client, &hub.HubClientConfig{
		Client: client,
	}, nil, mf, nil)
	reconciler.InjectClient(client)
	ctx := context.WithValue(context.Background(), types.NamespacedName{Name: testVPNKeyRotationName, Namespace: testProjectNamespace}, vpnKeyObject)
	vpmRotationKey := types.NamespacedName{Namespace: testProjectNamespace, Name: testVPNKeyRotationName}

	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(vpmRotationKey),
		mock.IsType(&hubv1alpha1.VpnKeyRotation{}),
	).Return(nil).Run(func(args mock.Arguments) {
		vpnKeyRotation := args.Get(2).(*hubv1alpha1.VpnKeyRotation)
		*vpnKeyRotation = *vpnKeyObject
	})
	client.On("List",
		mock.IsType(ctx),
		mock.IsType(&spokev1alpha1.WorkerSliceGwRecyclerList{}),
		mock.IsType([]k8sclient.ListOption(nil)),
	).Return(nil, nil)
	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(vpmRotationKey),
		mock.IsType(&hubv1alpha1.VpnKeyRotation{}),
	).Return(nil)
	client.StatusMock.On("Update",
		mock.IsType(ctx),
		mock.IsType(&hubv1alpha1.VpnKeyRotation{}),
		mock.IsType([]k8sclient.UpdateOption(nil)),
	).Return(nil)
	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(types.NamespacedName{Name: "fire-worker-2-worker-1-0", Namespace: ControlPlaneNamespace}),
		mock.IsType(&corev1.Secret{}),
	).Return(&apierrors.StatusError{
		ErrStatus: metav1.Status{
			Status:  metav1.StatusFailure,
			Message: "object not found",
			Reason:  metav1.StatusReasonNotFound,
			Code:    404,
		}},
	).Once()
	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(
			types.NamespacedName{
				Name:      "fire-worker-2-worker-1",
				Namespace: ControlPlaneNamespace}),
		mock.IsType(&corev1.Secret{}),
	).Return(nil)
	client.On("Create",
		mock.IsType(ctx),
		mock.IsType(&corev1.Secret{}),
		mock.IsType([]k8sclient.CreateOption(nil)),
	).Return(errors.New("unable to create secret to store slicegw certs in worker cluster"))
	_, err := reconciler.Reconcile(expected.ctx, expected.req)
	if expected.errMsg != err.Error() {
		t.Error("Expected error:", expected.errMsg, " but got ", err)
	}
}

func TestReconcilerControllerSecretNotFound(t *testing.T) {
	var vpnKeyRotationObj = &hubv1alpha1.VpnKeyRotation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testVPNKeyRotationName,
			Namespace: testProjectNamespace,
		},
		Spec: hubv1alpha1.VpnKeyRotationSpec{
			ClusterGatewayMapping: map[string][]string{
				"cluster1": {"fire-worker-2-worker-1", "fire-worker-2-worker-3"},
			},
			CertificateCreationTime: metav1.Time{Time: time.Now()},
		},
		Status: hubv1alpha1.VpnKeyRotationStatus{
			CurrentRotationState: map[string]hubv1alpha1.StatusOfKeyRotation{
				"fire-worker-2-worker-1": {
					Status:               hubv1alpha1.Complete,
					LastUpdatedTimestamp: metav1.Time{Time: time.Now().AddDate(0, 0, -1)}, // anything less than current date
				},
				"fire-worker-2-worker-3": {
					Status:               hubv1alpha1.Complete,
					LastUpdatedTimestamp: metav1.Time{Time: time.Now().AddDate(0, 0, -2)}, // anything less than current date
				},
			},
		},
	}
	os.Setenv("CLUSTER_NAME", "cluster1")
	expected := struct {
		ctx    context.Context
		req    reconcile.Request
		res    reconcile.Result
		errMsg string
	}{
		context.WithValue(context.Background(), types.NamespacedName{Namespace: testProjectNamespace, Name: testVPNKeyRotationName}, vpnKeyRotationObj),
		reconcile.Request{NamespacedName: types.NamespacedName{Name: testVPNKeyRotationName, Namespace: testProjectNamespace}},
		reconcile.Result{},
		"object not found",
	}
	client := utilmock.NewClient()
	mf, _ := metrics.NewMetricsFactory(prometheus.NewRegistry(), metrics.MetricsFactoryOptions{})
	reconciler := NewReconciler(client, &hub.HubClientConfig{
		Client: client,
	}, nil, mf, nil)
	reconciler.InjectClient(client)
	ctx := context.WithValue(context.Background(), types.NamespacedName{Name: testVPNKeyRotationName, Namespace: testProjectNamespace}, vpnKeyRotationObj)
	vpmRotationKey := types.NamespacedName{Namespace: testProjectNamespace, Name: testVPNKeyRotationName}

	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(vpmRotationKey),
		mock.IsType(&hubv1alpha1.VpnKeyRotation{}),
	).Return(nil).Run(func(args mock.Arguments) {
		vpnKeyRotation := args.Get(2).(*hubv1alpha1.VpnKeyRotation)
		*vpnKeyRotation = *vpnKeyRotationObj
	})
	client.On("List",
		mock.IsType(ctx),
		mock.IsType(&spokev1alpha1.WorkerSliceGwRecyclerList{}),
		mock.IsType([]k8sclient.ListOption(nil)),
	).Return(nil, nil)
	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(vpmRotationKey),
		mock.IsType(&hubv1alpha1.VpnKeyRotation{}),
	).Return(nil)
	client.StatusMock.On("Update",
		mock.IsType(ctx),
		mock.IsType(&hubv1alpha1.VpnKeyRotation{}),
		mock.IsType([]k8sclient.UpdateOption(nil)),
	).Return(nil)
	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(types.NamespacedName{Name: "fire-worker-2-worker-1-0", Namespace: ControlPlaneNamespace}),
		mock.IsType(&corev1.Secret{}),
	).Return(&apierrors.StatusError{
		ErrStatus: metav1.Status{
			Status:  metav1.StatusFailure,
			Message: "object not found",
			Reason:  metav1.StatusReasonNotFound,
			Code:    404,
		}},
	).Once()
	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(
			types.NamespacedName{
				Name:      "fire-worker-2-worker-1",
				Namespace: ControlPlaneNamespace}),
		mock.IsType(&corev1.Secret{}),
	).Return(errors.New("object not found"))
	_, err := reconciler.Reconcile(expected.ctx, expected.req)
	if expected.errMsg != err.Error() {
		t.Error("Expected error:", expected.errMsg, " but got ", err)
	}
}

func TestReconcilerSecretCreationAndRequeue(t *testing.T) {
	vpnKeyRotationObj := &hubv1alpha1.VpnKeyRotation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testVPNKeyRotationName,
			Namespace: testProjectNamespace,
		},
		Spec: hubv1alpha1.VpnKeyRotationSpec{
			ClusterGatewayMapping: map[string][]string{
				"cluster1": {"fire-worker-2-worker-1", "fire-worker-2-worker-3"},
			},
			CertificateCreationTime: metav1.Time{Time: time.Now()},
		},
		Status: hubv1alpha1.VpnKeyRotationStatus{
			CurrentRotationState: map[string]hubv1alpha1.StatusOfKeyRotation{
				"fire-worker-2-worker-1": {
					Status:               hubv1alpha1.Complete,
					LastUpdatedTimestamp: metav1.Time{Time: time.Now().AddDate(0, 0, -1)}, // anything less than current date
				},
				"fire-worker-2-worker-3": {
					Status:               hubv1alpha1.Complete,
					LastUpdatedTimestamp: metav1.Time{Time: time.Now().AddDate(0, 0, -2)}, // anything less than current date
				},
			},
		},
	}

	os.Setenv("CLUSTER_NAME", "cluster1")
	expected := struct {
		ctx context.Context
		req reconcile.Request
		res reconcile.Result
		err error
	}{
		context.WithValue(context.Background(), types.NamespacedName{Namespace: testProjectNamespace, Name: testVPNKeyRotationName}, vpnKeyRotationObj),
		reconcile.Request{NamespacedName: types.NamespacedName{Name: testVPNKeyRotationName, Namespace: testProjectNamespace}},
		reconcile.Result{},
		nil,
	}
	client := utilmock.NewClient()
	mf, _ := metrics.NewMetricsFactory(prometheus.NewRegistry(), metrics.MetricsFactoryOptions{})
	reconciler := NewReconciler(client, &hub.HubClientConfig{
		Client: client,
	}, nil, mf, nil)
	reconciler.InjectClient(client)
	ctx := context.WithValue(context.Background(), types.NamespacedName{Name: testVPNKeyRotationName, Namespace: testProjectNamespace}, vpnKeyRotationObj)
	vpmRotationKey := types.NamespacedName{Namespace: testProjectNamespace, Name: testVPNKeyRotationName}

	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(vpmRotationKey),
		mock.IsType(&hubv1alpha1.VpnKeyRotation{}),
	).Return(nil).Run(func(args mock.Arguments) {
		vpnKeyRotation := args.Get(2).(*hubv1alpha1.VpnKeyRotation)
		*vpnKeyRotation = *vpnKeyRotationObj
	})
	client.On("List",
		mock.IsType(ctx),
		mock.IsType(&spokev1alpha1.WorkerSliceGwRecyclerList{}),
		mock.IsType([]k8sclient.ListOption(nil)),
	).Return(nil, nil)
	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(vpmRotationKey),
		mock.IsType(&hubv1alpha1.VpnKeyRotation{}),
	).Return(nil)
	client.StatusMock.On("Update",
		mock.IsType(ctx),
		mock.IsType(&hubv1alpha1.VpnKeyRotation{}),
		mock.IsType([]k8sclient.UpdateOption(nil)),
	).Return(nil)
	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(types.NamespacedName{Name: "fire-worker-2-worker-1-0", Namespace: ControlPlaneNamespace}),
		mock.IsType(&corev1.Secret{}),
	).Return(&apierrors.StatusError{
		ErrStatus: metav1.Status{
			Status:  metav1.StatusFailure,
			Message: "object not found",
			Reason:  metav1.StatusReasonNotFound,
			Code:    404,
		}},
	).Once()
	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(
			types.NamespacedName{
				Name:      "fire-worker-2-worker-1",
				Namespace: ControlPlaneNamespace}),
		mock.IsType(&corev1.Secret{}),
	).Return(nil)
	client.On("Create",
		mock.IsType(ctx),
		mock.IsType(&corev1.Secret{}),
		mock.IsType([]k8sclient.CreateOption(nil)),
	).Return(nil)

	result, err := reconciler.Reconcile(expected.ctx, expected.req)
	if err != nil && expected.res != result {
		t.Error("Expected response :", expected.res, " but got ", result)
	}
}

func TestReconcilerClusterSync(t *testing.T) {
	vpnKeyRotationObj := &hubv1alpha1.VpnKeyRotation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testVPNKeyRotationName,
			Namespace: testProjectNamespace,
		},
		Spec: hubv1alpha1.VpnKeyRotationSpec{
			ClusterGatewayMapping: map[string][]string{
				"cluster1": {"fire-worker-2-worker-1", "fire-worker-2-worker-3"},
			},
			CertificateCreationTime: metav1.Time{Time: time.Now()},
		},
		Status: hubv1alpha1.VpnKeyRotationStatus{
			CurrentRotationState: map[string]hubv1alpha1.StatusOfKeyRotation{
				"fire-worker-2-worker-1": {
					Status:               hubv1alpha1.Complete,
					LastUpdatedTimestamp: metav1.Time{Time: time.Now().AddDate(0, 0, -1)}, // anything less than current date
				},
				"fire-worker-2-worker-3": {
					Status:               hubv1alpha1.Complete,
					LastUpdatedTimestamp: metav1.Time{Time: time.Now().AddDate(0, 0, -2)}, // anything less than current date
				},
				"fire-worker-2-worker-4": {
					Status:               hubv1alpha1.Complete,
					LastUpdatedTimestamp: metav1.Time{Time: time.Now().AddDate(0, 0, -2)}, // anything less than current date
				},
			},
		},
	}

	os.Setenv("CLUSTER_NAME", "cluster1")
	expected := struct {
		ctx context.Context
		req reconcile.Request
		res reconcile.Result
		err error
	}{
		context.WithValue(context.Background(), types.NamespacedName{Namespace: testProjectNamespace, Name: testVPNKeyRotationName}, vpnKeyRotationObj),
		reconcile.Request{NamespacedName: types.NamespacedName{Name: testVPNKeyRotationName, Namespace: testProjectNamespace}},
		reconcile.Result{},
		nil,
	}
	client := utilmock.NewClient()
	mf, _ := metrics.NewMetricsFactory(prometheus.NewRegistry(), metrics.MetricsFactoryOptions{})
	reconciler := NewReconciler(client, &hub.HubClientConfig{
		Client: client,
	}, nil, mf, nil)
	reconciler.InjectClient(client)
	ctx := context.WithValue(context.Background(), types.NamespacedName{Name: testVPNKeyRotationName, Namespace: testProjectNamespace}, vpnKeyRotationObj)
	vpmRotationKey := types.NamespacedName{Namespace: testProjectNamespace, Name: testVPNKeyRotationName}

	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(vpmRotationKey),
		mock.IsType(&hubv1alpha1.VpnKeyRotation{}),
	).Return(nil).Run(func(args mock.Arguments) {
		vpnKeyRotation := args.Get(2).(*hubv1alpha1.VpnKeyRotation)
		*vpnKeyRotation = *vpnKeyRotationObj
	})
	client.On("List",
		mock.IsType(ctx),
		mock.IsType(&spokev1alpha1.WorkerSliceGwRecyclerList{}),
		mock.IsType([]k8sclient.ListOption(nil)),
	).Return(nil, nil)
	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(vpmRotationKey),
		mock.IsType(&hubv1alpha1.VpnKeyRotation{}),
	).Return(nil)
	client.StatusMock.On("Update",
		mock.IsType(ctx),
		mock.IsType(&hubv1alpha1.VpnKeyRotation{}),
		mock.IsType([]k8sclient.UpdateOption(nil)),
	).Return(nil)
	result, err := reconciler.Reconcile(expected.ctx, expected.req)
	if err != nil && expected.res != result {
		t.Error("Expected response :", expected.res, " but got ", result)
	}
}
