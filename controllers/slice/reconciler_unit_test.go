package slice

import (
	"context"
	"errors"
	"testing"

	mevents "github.com/kubeslice/kubeslice-monitoring/pkg/events"
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	ossEvents "github.com/kubeslice/worker-operator/events"
	utilmock "github.com/kubeslice/worker-operator/pkg/mocks"
	corev1 "k8s.io/api/core/v1"

	"github.com/stretchr/testify/mock"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type MockClientCall func(*utilmock.MockClient, context.Context)

var testSliceObj = &kubeslicev1beta1.Slice{}

func Test_SliceReconciler(t *testing.T) {
	tests := []struct {
		name string
		// params
		ctx context.Context
		req reconcile.Request
		// mocked calls
		loadMocks MockClientCall
		// returns
		res reconcile.Result
		err error
	}{
		{
			"When status.SliceConfig is not set",
			context.WithValue(context.Background(), types.NamespacedName{}, testSliceObj),
			reconcile.Request{NamespacedName: types.NamespacedName{}},
			func(clientMock *utilmock.MockClient, ctx context.Context) {
				// mock k8s client calls
				clientMock.On("Get",
					mock.IsType(ctx),
					mock.IsType(types.NamespacedName{}),
					mock.IsType(&kubeslicev1beta1.Slice{}),
				).Return(nil)
				// Get(*context.valueCtx,types.NamespacedName,*v1.Namespace)
				clientMock.On("Get",
					mock.IsType(ctx),
					mock.IsType(types.NamespacedName{}),
					mock.IsType(&corev1.Namespace{}),
				).Return(nil)
				clientMock.On("Update",
					mock.IsType(ctx),
					mock.IsType(&corev1.Namespace{}),
					mock.Anything,
				).Return(nil)
				// Update(*context.valueCtx,*v1beta1.Slice,[]client.UpdateOption)
				clientMock.On("Update",
					mock.IsType(ctx),
					mock.IsType(&kubeslicev1beta1.Slice{}),
					mock.Anything,
				).Return(nil)
			},
			reconcile.Result{},
			errors.New("slice not reconciled from hub"),
		},
		{
			"when slice obj is not found",
			context.WithValue(context.Background(), types.NamespacedName{}, testSliceObj),
			reconcile.Request{
				NamespacedName: types.NamespacedName{}},
			func(clientMock *utilmock.MockClient, ctx context.Context) {
				clientMock.On("Get",
					mock.IsType(ctx),
					mock.IsType(types.NamespacedName{}),
					mock.IsType(&kubeslicev1beta1.Slice{}),
				).Return(errors.New("slice not found"))
			},
			reconcile.Result{},
			errors.New("slice not found"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// init
			client := utilmock.NewClient()
			test.loadMocks(client, test.ctx)
			mockEventRecorder := mevents.NewEventRecorder(client,
				runtime.NewScheme(), ossEvents.EventsMap, mevents.EventRecorderOptions{})
			reconciler := &SliceReconciler{
				Client:        client,
				EventRecorder: &mockEventRecorder,
			}
			// call function
			result, err := reconciler.Reconcile(test.ctx, test.req)
			// assert re
			if test.res != result {
				t.Error("Expected response :", test.res, " but got ", result)
			}
			if test.err != nil && err != nil {
				if test.err.Error() != err.Error() {
					t.Error("Expected error:", test.err, " but got ", err)
				}
			} else if test.err != err {
				t.Error("Expected error:", test.err, " but got ", err)
			}
		})
	}
}

func TestHandleDnsSvc(t *testing.T) {
	tests := []struct {
		name string
		// input parameters
		ctx      context.Context
		sliceObj *kubeslicev1beta1.Slice
		// mocked calls
		loadMocks MockClientCall
		// return values
		expectedRequeue bool
		expectedRes     reconcile.Result
		expectedErr     error
	}{
		{
			name:            "DNS service not found",
			ctx:             context.WithValue(context.Background(), types.NamespacedName{}, testSliceObj),
			sliceObj:        testSliceObj,
			expectedRequeue: true,
			expectedRes:     reconcile.Result{},
			expectedErr:     errors.New("DNS service not found"),
			loadMocks: func(mc *utilmock.MockClient, ctx context.Context) {
				mc.On("Get",
					mock.IsType(ctx),
					mock.IsType(types.NamespacedName{}),
					mock.IsType(&corev1.Service{}),
				).Return(errors.New("DNS service not found"))
			},
		},
		{
			name:     "DNS service found",
			ctx:      context.WithValue(context.Background(), types.NamespacedName{}, testSliceObj),
			sliceObj: testSliceObj,
			loadMocks: func(mc *utilmock.MockClient, ctx context.Context) {
				mc.On("Get",
					mock.Anything,
					mock.Anything,
					mock.IsType(&corev1.Service{}),
				).Return(nil)
				mc.On("Get",
					mock.Anything,
					mock.Anything,
					mock.IsType(&kubeslicev1beta1.Slice{}),
				).Return(nil)
				statusClient := mc.StatusMock
				statusClient.On("Update",
					mock.Anything,
					mock.Anything,
					mock.Anything,
				).Return(nil)
			},
			expectedRequeue: true,
			expectedRes:     reconcile.Result{},
			expectedErr:     nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// init
			client := utilmock.NewClient()
			mockEventRecorder := mevents.NewEventRecorder(client,
				runtime.NewScheme(), ossEvents.EventsMap, mevents.EventRecorderOptions{})
			r := &SliceReconciler{
				Client:        client,
				EventRecorder: &mockEventRecorder,
			}
			test.loadMocks(client, test.ctx)
			// call function
			requeue, result, err := r.handleDnsSvc(test.ctx, test.sliceObj)
			// check return values
			if requeue != test.expectedRequeue {
				t.Errorf("Expected requeue: %v, got: %v", test.expectedRequeue, result.Requeue)
			}
			if test.expectedRes != result {
				t.Error("Expected response :", test.expectedRes, " but got ", result)
			}
			if test.expectedErr != nil && err != nil {
				if test.expectedErr.Error() != err.Error() {
					t.Error("Expected error:", test.expectedErr, " but got ", err)
				}
			} else if test.expectedErr != err {
				t.Error("Expected error:", test.expectedErr, " but got ", err)
			}
		})
	}
}
