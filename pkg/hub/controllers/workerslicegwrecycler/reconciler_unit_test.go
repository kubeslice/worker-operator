package workerslicegwrecycler

import (
	"context"
	"errors"
	"testing"

	spokev1alpha1 "github.com/kubeslice/apis/pkg/worker/v1alpha1"
	mevents "github.com/kubeslice/kubeslice-monitoring/pkg/events"
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	ossEvents "github.com/kubeslice/worker-operator/events"
	"github.com/kubeslice/worker-operator/pkg/gwsidecar"
	utilmock "github.com/kubeslice/worker-operator/pkg/mocks"
	"github.com/kubeslice/worker-operator/pkg/router"
	sidecar "github.com/kubeslice/router-sidecar/pkg/sidecar/sidecarpb"
	"github.com/looplab/fsm"
	"github.com/stretchr/testify/mock"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
	ctrl "sigs.k8s.io/controller-runtime"
)

type MockWorkerGWSidecarClient struct {
	mock.Mock
}

func (m *MockWorkerGWSidecarClient) GetStatus(ctx context.Context, serverAddr string) (*gwsidecar.GwStatus, error) {
	args := m.Called(ctx, serverAddr)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*gwsidecar.GwStatus), args.Error(1)
}

type MockWorkerRouterClient struct {
	mock.Mock
}

func (m *MockWorkerRouterClient) UpdateEcmpRoutes(ctx context.Context, serverAddr string, ecmpUpdateInfo *router.UpdateEcmpInfo) error {
	args := m.Called(ctx, serverAddr, ecmpUpdateInfo)
	return args.Error(0)
}

func (m *MockWorkerRouterClient) GetRouteInKernel(ctx context.Context, serverAddr string, sliceRouterConnCtx *router.GetRouteConfig) (*sidecar.VerifyRouteAddResponse, error) {
	args := m.Called(ctx, serverAddr, sliceRouterConnCtx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*sidecar.VerifyRouteAddResponse), args.Error(1)
}

var testRecyclerName = "test-recycler"
var testRecyclerNamespace = "kubeslice-system"
var testServerGwName = "test-slice-server-0"
var testClientGwName = "test-slice-client-0"

var testWorkerSliceGwRecycler = &spokev1alpha1.WorkerSliceGwRecycler{
	ObjectMeta: metav1.ObjectMeta{
		Name:      testRecyclerName,
		Namespace: testRecyclerNamespace,
		Labels: map[string]string{
			"slice_name":   "test-slice",
			"slicegw_name": testServerGwName,
		},
	},
	Spec: spokev1alpha1.WorkerSliceGwRecyclerSpec{
		GwPair: spokev1alpha1.GwPair{
			ServerID: testServerGwName,
			ClientID: testClientGwName,
		},
		State:         "init",
		Request:       "verify_new_deployment_created",
		SliceGwServer: testServerGwName,
		SliceGwClient: testClientGwName,
		SliceName:     "test-slice",
	},
}

var testSliceGw = &kubeslicev1beta1.SliceGateway{
	ObjectMeta: metav1.ObjectMeta{
		Name:      testServerGwName,
		Namespace: testRecyclerNamespace,
	},
	Spec: kubeslicev1beta1.SliceGatewaySpec{
		SliceName: "test-slice",
	},
	Status: kubeslicev1beta1.SliceGatewayStatus{
		Config: kubeslicev1beta1.SliceGatewayConfig{
			SliceGatewayHostType:    "Server",
			SliceGatewayRemoteSubnet: "10.0.0.0/16",
		},
	},
}

func TestReconcilerNotFound(t *testing.T) {
	expected := struct {
		ctx context.Context
		req ctrl.Request
		res ctrl.Result
		err error
	}{
		context.Background(),
		ctrl.Request{NamespacedName: types.NamespacedName{Name: testRecyclerName, Namespace: testRecyclerNamespace}},
		ctrl.Result{},
		nil,
	}

	client := utilmock.NewClient()
	eventRecorder := mevents.NewEventRecorder(client, scheme.Scheme, ossEvents.EventsMap, mevents.EventRecorderOptions{})
	reconciler := &Reconciler{
		Client:        client,
		MeshClient:    client,
		EventRecorder: &eventRecorder,
		FSM:           make(map[string]*fsm.FSM),
	}

	recyclerKey := types.NamespacedName{Namespace: testRecyclerNamespace, Name: testRecyclerName}

	client.On("Get",
		mock.Anything,
		mock.IsType(recyclerKey),
		mock.IsType(&spokev1alpha1.WorkerSliceGwRecycler{}),
	).Return(apierrors.NewNotFound(schema.GroupResource{Resource: "workerslicegwrecycler"}, testRecyclerName))

	result, err := reconciler.Reconcile(expected.ctx, expected.req)
	if expected.res != result {
		t.Error("Expected response :", expected.res, " but got ", result)
	}
	if expected.err != err {
		t.Error("Expected error:", expected.err, " but got ", err)
	}
}

func TestReconcilerServerInitState(t *testing.T) {
	recycler := testWorkerSliceGwRecycler.DeepCopy()
	recycler.Spec.State = ST_init

	expected := struct {
		ctx context.Context
		req ctrl.Request
		res ctrl.Result
		err error
	}{
		context.Background(),
		ctrl.Request{NamespacedName: types.NamespacedName{Name: testRecyclerName, Namespace: testRecyclerNamespace}},
		ctrl.Result{},
		nil,
	}

	client := utilmock.NewClient()
	meshClient := utilmock.NewClient()
	eventRecorder := mevents.NewEventRecorder(client, scheme.Scheme, ossEvents.EventsMap, mevents.EventRecorderOptions{})
	reconciler := &Reconciler{
		Client:        client,
		MeshClient:    meshClient,
		EventRecorder: &eventRecorder,
		FSM:           make(map[string]*fsm.FSM),
	}

	recyclerKey := types.NamespacedName{Namespace: testRecyclerNamespace, Name: testRecyclerName}

	client.On("Get",
		mock.Anything,
		mock.IsType(recyclerKey),
		mock.IsType(&spokev1alpha1.WorkerSliceGwRecycler{}),
	).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*spokev1alpha1.WorkerSliceGwRecycler)
		*arg = *recycler
	})

	meshClient.On("Get",
		mock.Anything,
		mock.IsType(types.NamespacedName{Namespace: testRecyclerNamespace, Name: testServerGwName}),
		mock.IsType(&kubeslicev1beta1.SliceGateway{}),
	).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*kubeslicev1beta1.SliceGateway)
		*arg = *testSliceGw
	})

	meshClient.On("List",
		mock.Anything,
		mock.IsType(&appsv1.DeploymentList{}),
		mock.IsType([]k8sclient.ListOption(nil)),
	).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(1).(*appsv1.DeploymentList)
		arg.Items = []appsv1.Deployment{}
	})

	meshClient.On("Get",
		mock.Anything,
		mock.IsType(types.NamespacedName{Namespace: testRecyclerNamespace, Name: testServerGwName}),
		mock.IsType(&kubeslicev1beta1.SliceGateway{}),
	).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*kubeslicev1beta1.SliceGateway)
		*arg = *testSliceGw
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

	client.On("Get",
		mock.Anything,
		mock.IsType(recyclerKey),
		mock.IsType(&spokev1alpha1.WorkerSliceGwRecycler{}),
	).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*spokev1alpha1.WorkerSliceGwRecycler)
		*arg = *recycler
	})

	client.On("Update",
		mock.Anything,
		mock.IsType(&spokev1alpha1.WorkerSliceGwRecycler{}),
		mock.IsType([]k8sclient.UpdateOption(nil)),
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

func TestReconcilerClientSide(t *testing.T) {
	recycler := testWorkerSliceGwRecycler.DeepCopy()
	recycler.Spec.State = ST_new_deployment_created
	recycler.Spec.Request = getRequestString(REQ_create_new_deployment)

	clientSliceGw := testSliceGw.DeepCopy()
	clientSliceGw.Name = testClientGwName
	clientSliceGw.Status.Config.SliceGatewayHostType = "Client"

	expected := struct {
		ctx context.Context
		req ctrl.Request
		res ctrl.Result
		err error
	}{
		context.Background(),
		ctrl.Request{NamespacedName: types.NamespacedName{Name: testRecyclerName, Namespace: testRecyclerNamespace}},
		ctrl.Result{},
		nil,
	}

	client := utilmock.NewClient()
	meshClient := utilmock.NewClient()
	eventRecorder := mevents.NewEventRecorder(client, scheme.Scheme, ossEvents.EventsMap, mevents.EventRecorderOptions{})
	reconciler := &Reconciler{
		Client:        client,
		MeshClient:    meshClient,
		EventRecorder: &eventRecorder,
		FSM:           make(map[string]*fsm.FSM),
	}

	recyclerKey := types.NamespacedName{Namespace: testRecyclerNamespace, Name: testRecyclerName}

	client.On("Get",
		mock.Anything,
		mock.IsType(recyclerKey),
		mock.IsType(&spokev1alpha1.WorkerSliceGwRecycler{}),
	).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*spokev1alpha1.WorkerSliceGwRecycler)
		*arg = *recycler
	})

	meshClient.On("Get",
		mock.Anything,
		mock.IsType(types.NamespacedName{Namespace: testRecyclerNamespace, Name: testServerGwName}),
		mock.IsType(&kubeslicev1beta1.SliceGateway{}),
	).Return(apierrors.NewNotFound(schema.GroupResource{Resource: "slicegateway"}, testServerGwName))

	meshClient.On("Get",
		mock.Anything,
		mock.IsType(types.NamespacedName{Namespace: testRecyclerNamespace, Name: testClientGwName}),
		mock.IsType(&kubeslicev1beta1.SliceGateway{}),
	).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*kubeslicev1beta1.SliceGateway)
		*arg = *clientSliceGw
	})

	meshClient.On("List",
		mock.Anything,
		mock.IsType(&appsv1.DeploymentList{}),
		mock.IsType([]k8sclient.ListOption(nil)),
	).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(1).(*appsv1.DeploymentList)
		arg.Items = []appsv1.Deployment{}
	})

	meshClient.On("Get",
		mock.Anything,
		mock.IsType(types.NamespacedName{Namespace: testRecyclerNamespace, Name: testClientGwName}),
		mock.IsType(&kubeslicev1beta1.SliceGateway{}),
	).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*kubeslicev1beta1.SliceGateway)
		*arg = *clientSliceGw
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

	client.On("Get",
		mock.Anything,
		mock.IsType(recyclerKey),
		mock.IsType(&spokev1alpha1.WorkerSliceGwRecycler{}),
	).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*spokev1alpha1.WorkerSliceGwRecycler)
		*arg = *recycler
	})

	client.StatusMock.On("Update",
		mock.Anything,
		mock.IsType(&spokev1alpha1.WorkerSliceGwRecycler{}),
		mock.IsType([]k8sclient.UpdateOption(nil)),
	).Return(nil)

	client.StatusMock.On("Update",
		mock.Anything,
		mock.IsType(&spokev1alpha1.WorkerSliceGwRecycler{}),
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

func TestReconcilerGetError(t *testing.T) {
	expected := struct {
		ctx    context.Context
		req    ctrl.Request
		res    ctrl.Result
		errMsg string
	}{
		context.Background(),
		ctrl.Request{NamespacedName: types.NamespacedName{Name: testRecyclerName, Namespace: testRecyclerNamespace}},
		ctrl.Result{},
		"internal error",
	}

	client := utilmock.NewClient()
	eventRecorder := mevents.NewEventRecorder(client, scheme.Scheme, ossEvents.EventsMap, mevents.EventRecorderOptions{})
	reconciler := &Reconciler{
		Client:        client,
		MeshClient:    client,
		EventRecorder: &eventRecorder,
		FSM:           make(map[string]*fsm.FSM),
	}

	recyclerKey := types.NamespacedName{Namespace: testRecyclerNamespace, Name: testRecyclerName}

	client.On("Get",
		mock.Anything,
		mock.IsType(recyclerKey),
		mock.IsType(&spokev1alpha1.WorkerSliceGwRecycler{}),
	).Return(errors.New("internal error"))

	_, err := reconciler.Reconcile(expected.ctx, expected.req)
	if err == nil || expected.errMsg != err.Error() {
		t.Error("Expected error:", expected.errMsg, " but got ", err)
	}
}

func TestGetUniqueIdentifier(t *testing.T) {
	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "test", Namespace: "ns"}}
	id := getUniqueIdentifier(req)
	if id == "" {
		t.Error("Expected non-empty identifier")
	}
}
