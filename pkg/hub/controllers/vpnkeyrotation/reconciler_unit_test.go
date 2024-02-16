package vpnkeyrotation

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	gwsidecarpb "github.com/kubeslice/gateway-sidecar/pkg/sidecar/sidecarpb"

	hubv1alpha1 "github.com/kubeslice/apis/pkg/controller/v1alpha1"
	spokev1alpha1 "github.com/kubeslice/apis/pkg/worker/v1alpha1"
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	ossEvents "github.com/kubeslice/worker-operator/events"

	mevents "github.com/kubeslice/kubeslice-monitoring/pkg/events"
	"github.com/kubeslice/kubeslice-monitoring/pkg/metrics"
	"github.com/kubeslice/worker-operator/pkg/hub/controllers"
	hub "github.com/kubeslice/worker-operator/pkg/hub/hubclient"
	utilmock "github.com/kubeslice/worker-operator/pkg/mocks"
	"github.com/kubeslice/worker-operator/pkg/slicegwrecycler"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
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
		CertificateCreationTime: &metav1.Time{Time: time.Now()},
	},
	Status: hubv1alpha1.VpnKeyRotationStatus{},
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
		CertificateCreationTime: &metav1.Time{Time: time.Now()},
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
		GatewayPodStatus: []*kubeslicev1beta1.GwPodInfo{
			{
				PodName: "pod-1",
				TunnelStatus: kubeslicev1beta1.TunnelStatus{
					Status: int32(gwsidecarpb.TunnelStatusType_GW_TUNNEL_STATE_UP),
				},
				PeerPodName: "fire-worker-3-worker-2",
			},
			{
				PodName: "pod-2", TunnelStatus: kubeslicev1beta1.TunnelStatus{
					Status: int32(gwsidecarpb.TunnelStatusType_GW_TUNNEL_STATE_UP),
				},
				PeerPodName: "fire-worker-3-worker-2"},
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
	vpnKeyRotationObj := &hubv1alpha1.VpnKeyRotation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testVPNKeyRotationName,
			Namespace: testProjectNamespace,
		},
		Spec: hubv1alpha1.VpnKeyRotationSpec{
			ClusterGatewayMapping: map[string][]string{
				"cluster1": {"fire-worker-2-worker-1"},
			},
		},
		Status: hubv1alpha1.VpnKeyRotationStatus{},
	}
	expected := struct {
		ctx context.Context
		req reconcile.Request
		res reconcile.Result
		err error
	}{
		context.WithValue(context.Background(), types.NamespacedName{Namespace: testProjectNamespace, Name: testVPNKeyRotationName}, vpnKeyRotationObj),
		reconcile.Request{NamespacedName: types.NamespacedName{Name: testVPNKeyRotationName, Namespace: testProjectNamespace}},
		reconcile.Result{RequeueAfter: time.Second},
		nil,
	}
	client := utilmock.NewClient()
	mf, _ := metrics.NewMetricsFactory(prometheus.NewRegistry(), metrics.MetricsFactoryOptions{})
	reconciler := NewReconciler(client, nil, nil, mf, nil)
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
	result, _ := reconciler.Reconcile(expected.ctx, expected.req)
	if expected.res != result {
		t.Error("Expected response :", expected.res, " but got ", result)
	}
}

func TestReconcilerVPNRotationReconcilerCompletionAsSuccess(t *testing.T) {
	vpnRotationObject := &hubv1alpha1.VpnKeyRotation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testVPNKeyRotationName,
			Namespace: testProjectNamespace,
		},
		Spec: hubv1alpha1.VpnKeyRotationSpec{
			ClusterGatewayMapping: map[string][]string{
				"cluster1": {"fire-worker-2-worker-1"},
			},
			CertificateCreationTime: &metav1.Time{Time: time.Now()},
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
	os.Setenv("CLUSTER_NAME", "cluster1")
	expected := struct {
		ctx context.Context
		req reconcile.Request
		res reconcile.Result
		err error
	}{
		context.WithValue(context.Background(), types.NamespacedName{Namespace: testProjectNamespace, Name: testVPNKeyRotationName}, vpnRotationObject),
		reconcile.Request{NamespacedName: types.NamespacedName{Name: testVPNKeyRotationName, Namespace: testProjectNamespace}},
		reconcile.Result{},
		nil,
	}
	client := utilmock.NewClient()
	mf, _ := metrics.NewMetricsFactory(prometheus.NewRegistry(), metrics.MetricsFactoryOptions{})
	gv := hubv1alpha1.GroupVersion
	testScheme := runtime.NewScheme()
	err := scheme.AddToScheme(testScheme)
	if err != nil {
		t.Fatalf("Error adding core scheme to test scheme: %v", err)
	}
	testScheme.AddKnownTypeWithName(gv.WithKind("VpnKeyRotation"), &hubv1alpha1.VpnKeyRotation{})
	testClusterEventRecorder := mevents.NewEventRecorder(client, testScheme, ossEvents.EventsMap, mevents.EventRecorderOptions{
		Cluster:   "test-cluster",
		Project:   "avesha",
		Component: "worker-operator",
		Namespace: controllers.ControlPlaneNamespace,
	})
	reconciler := NewReconciler(client,
		&hub.HubClientConfig{
			Client: client,
		}, &testClusterEventRecorder, mf, nil)
	reconciler.InjectClient(client)
	ctx := context.WithValue(context.Background(), types.NamespacedName{Name: testVPNKeyRotationName, Namespace: testProjectNamespace}, vpnRotationObject)
	vpmRotationKey := types.NamespacedName{Namespace: testProjectNamespace, Name: testVPNKeyRotationName}

	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(vpmRotationKey),
		mock.IsType(&hubv1alpha1.VpnKeyRotation{}),
	).Return(nil).Run(func(args mock.Arguments) {
		vpnKeyRotation := args.Get(2).(*hubv1alpha1.VpnKeyRotation)
		*vpnKeyRotation = *vpnRotationObject
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
	client.StatusMock.On("Update",
		mock.IsType(ctx),
		mock.IsType(&hubv1alpha1.VpnKeyRotation{}),
		mock.IsType([]k8sclient.SubResourceUpdateOption(nil)),
	).Return(nil)
	client.On("Create",
		mock.IsType(ctx),
		mock.IsType(&corev1.Event{}),
		mock.IsType([]k8sclient.CreateOption(nil)),
	).Return(nil)
	client.On("Delete",
		mock.IsType(ctx),
		mock.IsType(&corev1.Secret{}),
		mock.IsType([]k8sclient.DeleteOption{}),
	).Return(nil)
	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(types.NamespacedName{Name: "fire-worker-2-worker-1"}),
		mock.IsType(&kubeslicev1beta1.SliceGateway{}),
	).Return(errors.New("object not found"))
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
	gv := hubv1alpha1.GroupVersion
	testScheme := runtime.NewScheme()
	err := scheme.AddToScheme(testScheme)
	if err != nil {
		t.Fatalf("Error adding core scheme to test scheme: %v", err)
	}
	testScheme.AddKnownTypeWithName(gv.WithKind("VpnKeyRotation"), &hubv1alpha1.VpnKeyRotation{})
	testClusterEventRecorder := mevents.NewEventRecorder(client, testScheme, ossEvents.EventsMap, mevents.EventRecorderOptions{
		Cluster:   "test-cluster",
		Project:   "avesha",
		Component: "worker-operator",
		Namespace: controllers.ControlPlaneNamespace,
	})
	reconciler := NewReconciler(client, &hub.HubClientConfig{
		Client: client,
	}, &testClusterEventRecorder, mf, nil)
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
	client.StatusMock.On("Update",
		mock.IsType(ctx),
		mock.IsType(&hubv1alpha1.VpnKeyRotation{}),
		mock.IsType([]k8sclient.SubResourceUpdateOption(nil)),
	).Return(nil)
	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(types.NamespacedName{Name: "fire-worker-2-worker-1-1"}),
		mock.IsType(&corev1.Secret{}),
	).Return(nil, false, nil)
	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(vpmRotationKey),
		mock.IsType(&hubv1alpha1.VpnKeyRotation{}),
	).Return(nil)
	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(types.NamespacedName{Name: "fire-worker-2-worker-1"}),
		mock.IsType(&kubeslicev1beta1.SliceGateway{}),
	).Return(errors.New("object not found"))
	client.On("Create",
		mock.IsType(ctx),
		mock.IsType(&corev1.Event{}),
		mock.IsType([]k8sclient.CreateOption(nil)),
	).Return(nil)
	_, err = reconciler.Reconcile(expected.ctx, expected.req)
	if err != nil && expected.errMsg != err.Error() {
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
	gv := hubv1alpha1.GroupVersion
	testScheme := runtime.NewScheme()
	err := scheme.AddToScheme(testScheme)
	if err != nil {
		t.Fatalf("Error adding core scheme to test scheme: %v", err)
	}
	testScheme.AddKnownTypeWithName(gv.WithKind("VpnKeyRotation"), &hubv1alpha1.VpnKeyRotation{})
	testClusterEventRecorder := mevents.NewEventRecorder(client, testScheme, ossEvents.EventsMap, mevents.EventRecorderOptions{
		Cluster:   "test-cluster",
		Project:   "avesha",
		Component: "worker-operator",
		Namespace: controllers.ControlPlaneNamespace,
	})
	reconciler := NewReconciler(client, &hub.HubClientConfig{
		Client: client,
	}, &testClusterEventRecorder, mf, nil)
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
	client.StatusMock.On("Update",
		mock.IsType(ctx),
		mock.IsType(&hubv1alpha1.VpnKeyRotation{}),
		mock.IsType([]k8sclient.SubResourceUpdateOption(nil)),
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
	client.StatusMock.On("Update",
		mock.IsType(ctx),
		mock.IsType(&hubv1alpha1.VpnKeyRotation{}),
		mock.IsType([]k8sclient.SubResourceUpdateOption(nil)),
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
	client.StatusMock.On("Update",
		mock.IsType(ctx),
		mock.IsType(&hubv1alpha1.VpnKeyRotation{}),
		mock.IsType([]k8sclient.SubResourceUpdateOption(nil)),
	).Return(nil)
	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(types.NamespacedName{Name: "fire-worker-2-worker-1"}),
		mock.IsType(&kubeslicev1beta1.Slice{}),
	).Return(errors.New("object not found"))
	client.On("Create",
		mock.IsType(ctx),
		mock.IsType(&corev1.Event{}),
		mock.IsType([]k8sclient.CreateOption(nil)),
	).Return(nil)

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

	gv := hubv1alpha1.GroupVersion
	testScheme := runtime.NewScheme()
	err = scheme.AddToScheme(testScheme)
	if err != nil {
		t.Fatalf("Error adding core scheme to test scheme: %v", err)
	}
	testScheme.AddKnownTypeWithName(gv.WithKind("VpnKeyRotation"), &hubv1alpha1.VpnKeyRotation{})
	testClusterEventRecorder := mevents.NewEventRecorder(client, testScheme, ossEvents.EventsMap, mevents.EventRecorderOptions{
		Cluster:   "test-cluster",
		Project:   "avesha",
		Component: "worker-operator",
		Namespace: controllers.ControlPlaneNamespace,
	})

	reconciler := NewReconciler(client, &hub.HubClientConfig{
		Client: client,
	}, &testClusterEventRecorder, mf, workerRecyclerClient)
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
	client.StatusMock.On("Update",
		mock.IsType(ctx),
		mock.IsType(&hubv1alpha1.VpnKeyRotation{}),
		mock.IsType([]k8sclient.SubResourceUpdateOption(nil)),
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
	client.StatusMock.On("Update",
		mock.IsType(ctx),
		mock.IsType(&hubv1alpha1.VpnKeyRotation{}),
		mock.IsType([]k8sclient.SubResourceUpdateOption(nil)),
	).Return(nil)
	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(types.NamespacedName{Name: "fire-worker-2-worker-1"}),
		mock.IsType(&kubeslicev1beta1.Slice{}),
	).Return(nil).Run(func(args mock.Arguments) {
		sliceCR := args.Get(2).(*kubeslicev1beta1.Slice)
		*sliceCR = *workerSlice
	})
	client.On("Create",
		mock.IsType(ctx),
		mock.IsType(&corev1.Event{}),
		mock.IsType([]k8sclient.CreateOption(nil)),
	).Return(nil)
	client.On("Update",
		mock.IsType(ctx),
		mock.IsType(&corev1.Event{}),
		mock.IsType([]k8sclient.UpdateOption(nil)),
	).Return(nil)
	client.On("Delete",
		mock.IsType(ctx),
		mock.IsType(&corev1.Secret{}),
		mock.IsType([]k8sclient.DeleteOption{}),
	).Return(nil)

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
			CertificateCreationTime: &metav1.Time{Time: time.Now()},
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
	gv := hubv1alpha1.GroupVersion
	testScheme := runtime.NewScheme()
	err := scheme.AddToScheme(testScheme)
	if err != nil {
		t.Fatalf("Error adding core scheme to test scheme: %v", err)
	}
	testScheme.AddKnownTypeWithName(gv.WithKind("VpnKeyRotation"), &hubv1alpha1.VpnKeyRotation{})
	testClusterEventRecorder := mevents.NewEventRecorder(client, testScheme, ossEvents.EventsMap, mevents.EventRecorderOptions{
		Cluster:   "test-cluster",
		Project:   "avesha",
		Component: "worker-operator",
		Namespace: controllers.ControlPlaneNamespace,
	})

	reconciler := NewReconciler(client, &hub.HubClientConfig{
		Client: client,
	}, &testClusterEventRecorder, mf, nil)
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
	client.StatusMock.On("Update",
		mock.IsType(ctx),
		mock.IsType(&hubv1alpha1.VpnKeyRotation{}),
		mock.IsType([]k8sclient.SubResourceUpdateOption(nil)),
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
	client.On("Create",
		mock.IsType(ctx),
		mock.IsType(&corev1.Event{}),
		mock.IsType([]k8sclient.CreateOption(nil)),
	).Return(nil)

	_, err = reconciler.Reconcile(expected.ctx, expected.req)
	if err != nil && expected.errMsg != err.Error() {
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
			CertificateCreationTime: &metav1.Time{Time: time.Now()},
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
	gv := hubv1alpha1.GroupVersion
	testScheme := runtime.NewScheme()
	err := scheme.AddToScheme(testScheme)
	if err != nil {
		t.Fatalf("Error adding core scheme to test scheme: %v", err)
	}
	testScheme.AddKnownTypeWithName(gv.WithKind("VpnKeyRotation"), &hubv1alpha1.VpnKeyRotation{})
	testClusterEventRecorder := mevents.NewEventRecorder(client, testScheme, ossEvents.EventsMap, mevents.EventRecorderOptions{
		Cluster:   "test-cluster",
		Project:   "avesha",
		Component: "worker-operator",
		Namespace: controllers.ControlPlaneNamespace,
	})
	reconciler := NewReconciler(client, &hub.HubClientConfig{
		Client: client,
	}, &testClusterEventRecorder, mf, nil)
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
	client.StatusMock.On("Update",
		mock.IsType(ctx),
		mock.IsType(&hubv1alpha1.VpnKeyRotation{}),
		mock.IsType([]k8sclient.SubResourceUpdateOption(nil)),
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
	client.On("Create",
		mock.IsType(ctx),
		mock.IsType(&corev1.Event{}),
		mock.IsType([]k8sclient.CreateOption(nil)),
	).Return(nil)

	_, err = reconciler.Reconcile(expected.ctx, expected.req)
	if err != nil && expected.errMsg != err.Error() {
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
				"cluster1": {"fire-worker-2-worker-1"},
			},
			CertificateCreationTime: &metav1.Time{Time: time.Now()},
		},
		Status: hubv1alpha1.VpnKeyRotationStatus{
			CurrentRotationState: map[string]hubv1alpha1.StatusOfKeyRotation{
				"fire-worker-2-worker-1": {
					Status:               hubv1alpha1.Complete,
					LastUpdatedTimestamp: metav1.Time{Time: time.Now().AddDate(0, 0, -1)}, // anything less than current date
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
	gv := hubv1alpha1.GroupVersion
	testScheme := runtime.NewScheme()
	err := scheme.AddToScheme(testScheme)
	if err != nil {
		t.Fatalf("Error adding core scheme to test scheme: %v", err)
	}
	testScheme.AddKnownTypeWithName(gv.WithKind("VpnKeyRotation"), &hubv1alpha1.VpnKeyRotation{})
	testClusterEventRecorder := mevents.NewEventRecorder(client, testScheme, ossEvents.EventsMap, mevents.EventRecorderOptions{
		Cluster:   "test-cluster",
		Project:   "avesha",
		Component: "worker-operator",
		Namespace: controllers.ControlPlaneNamespace,
	})
	reconciler := NewReconciler(client, &hub.HubClientConfig{
		Client: client,
	}, &testClusterEventRecorder, mf, nil)
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
	client.StatusMock.On("Update",
		mock.IsType(ctx),
		mock.IsType(&hubv1alpha1.VpnKeyRotation{}),
		mock.IsType([]k8sclient.SubResourceUpdateOption(nil)),
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
	client.On("Create",
		mock.IsType(ctx),
		mock.IsType(&corev1.Event{}),
		mock.IsType([]k8sclient.CreateOption(nil)),
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
	client.StatusMock.On("Update",
		mock.IsType(ctx),
		mock.IsType(&hubv1alpha1.VpnKeyRotation{}),
		mock.IsType([]k8sclient.SubResourceUpdateOption(nil)),
	).Return(nil)
	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(types.NamespacedName{Name: "fire-worker-2-worker-1"}),
		mock.IsType(&kubeslicev1beta1.Slice{}),
	).Return(errors.New("object not found"))
	client.On("Create",
		mock.IsType(ctx),
		mock.IsType(&corev1.Event{}),
		mock.IsType([]k8sclient.CreateOption(nil)),
	).Return(nil)
	result, err := reconciler.Reconcile(expected.ctx, expected.req)
	if err != nil && expected.res != result {
		t.Error("Expected response :", expected.res, " but got ", result)
	}
}

func TestReconcileRotationSyncingClusterAttach(t *testing.T) {
	allGws := []string{"fire-worker-2-worker-1", "fire-worker-2-worker-3", "fire-worker-2-worker-4"}
	vpnRotationObject := &hubv1alpha1.VpnKeyRotation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testVPNKeyRotationName,
			Namespace: testProjectNamespace,
		},
		Spec: hubv1alpha1.VpnKeyRotationSpec{
			ClusterGatewayMapping: map[string][]string{
				"cluster1": allGws,
			},
			CertificateCreationTime: &metav1.Time{Time: time.Now()},
		},
		Status: hubv1alpha1.VpnKeyRotationStatus{},
	}
	os.Setenv("CLUSTER_NAME", "cluster1")
	expected := struct {
		ctx context.Context
		req reconcile.Request
		res reconcile.Result
		err error
	}{
		context.WithValue(context.Background(), types.NamespacedName{Namespace: testProjectNamespace, Name: testVPNKeyRotationName}, vpnRotationObject),
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
	ctx := context.WithValue(context.Background(), types.NamespacedName{Name: testVPNKeyRotationName, Namespace: testProjectNamespace}, vpnRotationObject)
	vpmRotationKey := types.NamespacedName{Namespace: testProjectNamespace, Name: testVPNKeyRotationName}

	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(vpmRotationKey),
		mock.IsType(&hubv1alpha1.VpnKeyRotation{}),
	).Return(nil).Run(func(args mock.Arguments) {
		vpnKeyRotation := args.Get(2).(*hubv1alpha1.VpnKeyRotation)
		*vpnKeyRotation = *vpnRotationObject
	})
	client.StatusMock.On("Update",
		mock.IsType(ctx),
		mock.IsType(&hubv1alpha1.VpnKeyRotation{}),
		mock.IsType([]k8sclient.UpdateOption(nil)),
	).Return(nil)
	client.StatusMock.On("Update",
		mock.IsType(ctx),
		mock.IsType(&hubv1alpha1.VpnKeyRotation{}),
		mock.IsType([]k8sclient.SubResourceUpdateOption(nil)),
	).Return(nil)
	client.On("List",
		mock.IsType(ctx),
		mock.IsType(&corev1.PodList{}),
		mock.IsType([]k8sclient.ListOption(nil)),
	).Return(nil).Run(func(args mock.Arguments) {
		podsList := args.Get(1).(*corev1.PodList)
		*podsList = *podListCustom
	})

	_, err := reconciler.syncCurrentRotationState(expected.ctx, vpnRotationObject, allGws)
	if err != nil {
		t.Error("Expected response :", expected.res)
	}
	updatedVpnKeyRotation := &hubv1alpha1.VpnKeyRotation{}
	err = client.Get(ctx, types.NamespacedName{Name: testVPNKeyRotationName, Namespace: testProjectNamespace}, updatedVpnKeyRotation)
	expectedSyncedRotationState := map[string]hubv1alpha1.StatusOfKeyRotation{
		"fire-worker-2-worker-1": {
			Status:               hubv1alpha1.Complete,
			LastUpdatedTimestamp: *vpnRotationObject.Spec.CertificateCreationTime,
		},
		"fire-worker-2-worker-3": {
			Status:               hubv1alpha1.Complete,
			LastUpdatedTimestamp: *vpnRotationObject.Spec.CertificateCreationTime,
		},
		"fire-worker-2-worker-4": {
			Status:               hubv1alpha1.Complete,
			LastUpdatedTimestamp: *vpnRotationObject.Spec.CertificateCreationTime,
		},
	}
	assert.Equal(t, expectedSyncedRotationState, updatedVpnKeyRotation.Status.CurrentRotationState)
}

func TestReconcileRotationSyncingClusterDetach(t *testing.T) {
	allGws := []string{"fire-worker-2-worker-1", "fire-worker-2-worker-3"}
	nowTime := metav1.Time{Time: time.Now()}
	vpnRotationObject := &hubv1alpha1.VpnKeyRotation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testVPNKeyRotationName,
			Namespace: testProjectNamespace,
		},
		Spec: hubv1alpha1.VpnKeyRotationSpec{
			ClusterGatewayMapping: map[string][]string{
				"worker-2": allGws,
			},
			CertificateCreationTime: &nowTime,
		},
		Status: hubv1alpha1.VpnKeyRotationStatus{
			CurrentRotationState: map[string]hubv1alpha1.StatusOfKeyRotation{
				"fire-worker-2-worker-1": {
					Status:               hubv1alpha1.Complete,
					LastUpdatedTimestamp: nowTime,
				},
				"fire-worker-2-worker-3": {
					Status:               hubv1alpha1.Complete,
					LastUpdatedTimestamp: nowTime,
				},
				"fire-worker-2-worker-4": {
					Status:               hubv1alpha1.Complete,
					LastUpdatedTimestamp: nowTime, // reconciler should remove this
				},
				"fire-worker-1-worker-3": {
					Status:               hubv1alpha1.Complete,
					LastUpdatedTimestamp: nowTime,
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
		context.WithValue(context.Background(), types.NamespacedName{Namespace: testProjectNamespace, Name: testVPNKeyRotationName}, vpnRotationObject),
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
	ctx := context.WithValue(context.Background(), types.NamespacedName{Name: testVPNKeyRotationName, Namespace: testProjectNamespace}, vpnRotationObject)
	vpmRotationKey := types.NamespacedName{Namespace: testProjectNamespace, Name: testVPNKeyRotationName}

	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(vpmRotationKey),
		mock.IsType(&hubv1alpha1.VpnKeyRotation{}),
	).Return(nil).Run(func(args mock.Arguments) {
		vpnKeyRotation := args.Get(2).(*hubv1alpha1.VpnKeyRotation)
		*vpnKeyRotation = *vpnRotationObject
	})
	client.StatusMock.On("Update",
		mock.IsType(ctx),
		mock.IsType(&hubv1alpha1.VpnKeyRotation{}),
		mock.IsType([]k8sclient.UpdateOption(nil)),
	).Return(nil)
	client.StatusMock.On("Update",
		mock.IsType(ctx),
		mock.IsType(&hubv1alpha1.VpnKeyRotation{}),
		mock.IsType([]k8sclient.SubResourceUpdateOption(nil)),
	).Return(nil)

	os.Setenv("CLUSTER_NAME", "worker-2")

	_, err := reconciler.syncCurrentRotationState(expected.ctx, vpnRotationObject, allGws)
	if err != nil {
		t.Error("Expected response :", expected.res)
	}
	updatedVpnKeyRotation := &hubv1alpha1.VpnKeyRotation{}
	err = client.Get(ctx, types.NamespacedName{Name: vpnRotationObject.Name, Namespace: vpmRotationKey.Namespace}, updatedVpnKeyRotation)
	expectedSyncedRotationState := map[string]hubv1alpha1.StatusOfKeyRotation{
		"fire-worker-2-worker-1": {
			Status:               hubv1alpha1.Complete,
			LastUpdatedTimestamp: *vpnRotationObject.Spec.CertificateCreationTime,
		},
		"fire-worker-2-worker-3": {
			Status:               hubv1alpha1.Complete,
			LastUpdatedTimestamp: *vpnRotationObject.Spec.CertificateCreationTime,
		},
		"fire-worker-1-worker-3": {
			Status:               hubv1alpha1.Complete,
			LastUpdatedTimestamp: *vpnRotationObject.Spec.CertificateCreationTime,
		},
	}
	assert.Equal(t, expectedSyncedRotationState, updatedVpnKeyRotation.Status.CurrentRotationState)
}
