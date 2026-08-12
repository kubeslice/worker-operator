package workerslicegwrecycler

import (
	"context"
	"testing"

	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	utilmock "github.com/kubeslice/worker-operator/pkg/mocks"
	"github.com/stretchr/testify/mock"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestCheckIfDeploymentIsPresent(t *testing.T) {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deployment",
			Namespace: "kubeslice-system",
			Labels: map[string]string{
				"kubeslice.io/slice":           "test-slice",
				"kubeslice.io/slice-gw":        testServerGwName,
			},
		},
	}

	deploymentList := &appsv1.DeploymentList{
		Items: []appsv1.Deployment{*deployment},
	}

	client := utilmock.NewClient()
	reconciler := &Reconciler{
		MeshClient: client,
	}

	ctx := context.Background()

	client.On("List",
		mock.Anything,
		mock.IsType(&appsv1.DeploymentList{}),
		mock.IsType([]k8sclient.ListOption(nil)),
	).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(1).(*appsv1.DeploymentList)
		*arg = *deploymentList
	})

	present := reconciler.CheckIfDeploymentIsPresent(ctx, "test-deployment", "test-slice", testServerGwName)
	if !present {
		t.Error("Expected deployment to be present but got false")
	}

	notPresent := reconciler.CheckIfDeploymentIsPresent(ctx, "non-existent-deployment", "test-slice", testServerGwName)
	if notPresent {
		t.Error("Expected deployment to be absent but got true")
	}
}

func TestCreateNewDeployment(t *testing.T) {
	sliceGw := &kubeslicev1beta1.SliceGateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testServerGwName,
			Namespace: "kubeslice-system",
		},
		Spec: kubeslicev1beta1.SliceGatewaySpec{
			SliceName: "test-slice",
		},
		Status: kubeslicev1beta1.SliceGatewayStatus{
			Config: kubeslicev1beta1.SliceGatewayConfig{
				SliceGatewayIntermediateDeployments: []string{},
			},
		},
	}

	client := utilmock.NewClient()
	reconciler := &Reconciler{
		MeshClient: client,
	}

	ctx := context.Background()

	client.On("List",
		mock.Anything,
		mock.IsType(&appsv1.DeploymentList{}),
		mock.IsType([]k8sclient.ListOption(nil)),
	).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(1).(*appsv1.DeploymentList)
		arg.Items = []appsv1.Deployment{}
	})

	client.On("Get",
		mock.Anything,
		mock.IsType(types.NamespacedName{Namespace: "kubeslice-system", Name: testServerGwName}),
		mock.IsType(&kubeslicev1beta1.SliceGateway{}),
	).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*kubeslicev1beta1.SliceGateway)
		*arg = *sliceGw
	})

	client.StatusMock.On("Update",
		mock.Anything,
		mock.IsType(&kubeslicev1beta1.SliceGateway{}),
		mock.IsType([]k8sclient.UpdateOption(nil)),
	).Return(nil)

	client.StatusMock.On("Update",
		mock.Anything,
		mock.IsType(&kubeslicev1beta1.SliceGateway{}),
		mock.IsType([]k8sclient.SubResourceUpdateOption(nil)),
	).Return(nil)

	result, err, requeue := reconciler.CreateNewDeployment(ctx, "test-deployment-1", "test-slice", testServerGwName)
	if err != nil {
		t.Error("Expected no error but got:", err)
	}
	if requeue {
		t.Error("Expected requeue to be false but got true")
	}
	if result != (ctrl.Result{}) {
		t.Error("Expected empty result but got:", result)
	}
}

func TestMarkGwRouteForDeletion(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "kubeslice-system",
			Labels: map[string]string{
				"kubeslice.io/pod-type": "slicegateway",
				"kubeslice.io/slice-gw": testServerGwName,
			},
		},
	}

	podList := &corev1.PodList{
		Items: []corev1.Pod{*pod},
	}

	sliceGw := &kubeslicev1beta1.SliceGateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testServerGwName,
			Namespace: "kubeslice-system",
		},
		Spec: kubeslicev1beta1.SliceGatewaySpec{
			SliceName: "test-slice",
		},
	}

	client := utilmock.NewClient()
	reconciler := &Reconciler{
		MeshClient: client,
	}

	ctx := context.Background()

	client.On("List",
		mock.Anything,
		mock.IsType(&corev1.PodList{}),
		mock.IsType([]k8sclient.ListOption(nil)),
	).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(1).(*corev1.PodList)
		*arg = *podList
	})

	client.On("Update",
		mock.Anything,
		mock.IsType(&corev1.Pod{}),
		mock.IsType([]k8sclient.UpdateOption(nil)),
	).Return(nil)

	err := reconciler.MarkGwRouteForDeletion(ctx, sliceGw, testServerGwName)
	if err != nil {
		t.Error("Expected no error but got:", err)
	}
}

func TestTriggerGwDeploymentDeletion(t *testing.T) {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testServerGwName,
			Namespace: "kubeslice-system",
			Labels: map[string]string{
				"kubeslice.io/slice":    "test-slice",
				"kubeslice.io/slice-gw": testServerGwName,
			},
		},
	}

	deploymentList := &appsv1.DeploymentList{
		Items: []appsv1.Deployment{*deployment},
	}

	sliceGw := &kubeslicev1beta1.SliceGateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testServerGwName,
			Namespace: "kubeslice-system",
		},
		Spec: kubeslicev1beta1.SliceGatewaySpec{
			SliceName: "test-slice",
		},
		Status: kubeslicev1beta1.SliceGatewayStatus{
			Config: kubeslicev1beta1.SliceGatewayConfig{
				SliceGatewayIntermediateDeployments: []string{"test-new-deployment"},
			},
		},
	}

	client := utilmock.NewClient()
	reconciler := &Reconciler{
		MeshClient: client,
	}

	ctx := context.Background()

	client.On("List",
		mock.Anything,
		mock.IsType(&appsv1.DeploymentList{}),
		mock.IsType([]k8sclient.ListOption(nil)),
	).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(1).(*appsv1.DeploymentList)
		*arg = *deploymentList
	})

	client.On("Update",
		mock.Anything,
		mock.IsType(&appsv1.Deployment{}),
		mock.IsType([]k8sclient.UpdateOption(nil)),
	).Return(nil)

	client.On("Get",
		mock.Anything,
		mock.IsType(types.NamespacedName{Namespace: "kubeslice-system", Name: testServerGwName}),
		mock.IsType(&kubeslicev1beta1.SliceGateway{}),
	).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*kubeslicev1beta1.SliceGateway)
		*arg = *sliceGw
	})

	client.StatusMock.On("Update",
		mock.Anything,
		mock.IsType(&kubeslicev1beta1.SliceGateway{}),
		mock.IsType([]k8sclient.UpdateOption(nil)),
	).Return(nil)

	client.StatusMock.On("Update",
		mock.Anything,
		mock.IsType(&kubeslicev1beta1.SliceGateway{}),
		mock.IsType([]k8sclient.SubResourceUpdateOption(nil)),
	).Return(nil)

	err := reconciler.TriggerGwDeploymentDeletion(ctx, "test-slice", testServerGwName, testServerGwName, "test-new-deployment")
	if err != nil {
		t.Error("Expected no error but got:", err)
	}
}
