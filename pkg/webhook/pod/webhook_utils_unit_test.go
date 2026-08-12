package pod

import (
	"context"
	"testing"

	"github.com/kubeslice/worker-operator/api/v1beta1"
	"github.com/kubeslice/worker-operator/controllers"
	utilmock "github.com/kubeslice/worker-operator/pkg/mocks"
	"github.com/stretchr/testify/mock"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func TestGetNamespaceLabels(t *testing.T) {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-namespace",
			Labels: map[string]string{
				controllers.ApplicationNamespaceSelectorLabelKey: "test-slice",
				"custom-label": "custom-value",
			},
		},
	}

	client := utilmock.NewClient()
	webhookClient := NewWebhookClient()
	ctx := context.Background()

	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(types.NamespacedName{Name: "test-namespace"}),
		mock.IsType(&corev1.Namespace{}),
	).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*corev1.Namespace)
		*arg = *namespace
	})

	labels, err := webhookClient.GetNamespaceLabels(ctx, client, "test-namespace")
	if err != nil {
		t.Error("Expected no error but got:", err)
	}

	if labels == nil {
		t.Fatal("Expected labels to be non-nil")
	}

	if labels[controllers.ApplicationNamespaceSelectorLabelKey] != "test-slice" {
		t.Errorf("Expected slice label to be 'test-slice' but got '%s'",
			labels[controllers.ApplicationNamespaceSelectorLabelKey])
	}
}

func TestGetNamespaceLabelsNotFound(t *testing.T) {
	client := utilmock.NewClient()
	webhookClient := NewWebhookClient()
	ctx := context.Background()

	client.On("Get",
		mock.IsType(ctx),
		mock.IsType(types.NamespacedName{Name: "test-namespace"}),
		mock.IsType(&corev1.Namespace{}),
	).Return(apierrors.NewNotFound(schema.GroupResource{Resource: "namespace"}, "test-namespace"))

	labels, err := webhookClient.GetNamespaceLabels(ctx, client, "test-namespace")
	if err == nil {
		t.Error("Expected error but got nil")
	}

	if labels != nil {
		t.Error("Expected labels to be nil when namespace not found")
	}
}

func TestGetAllServiceExports(t *testing.T) {
	serviceExportList := &v1beta1.ServiceExportList{
		Items: []v1beta1.ServiceExport{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "svcex-1",
					Namespace: "ns-1",
					Labels: map[string]string{
						controllers.ApplicationNamespaceSelectorLabelKey: "test-slice",
					},
				},
				Spec: v1beta1.ServiceExportSpec{
					Slice:   "test-slice",
					Aliases: []string{"service1.example.com"},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "svcex-2",
					Namespace: "ns-2",
					Labels: map[string]string{
						controllers.ApplicationNamespaceSelectorLabelKey: "test-slice",
					},
				},
				Spec: v1beta1.ServiceExportSpec{
					Slice:   "test-slice",
					Aliases: []string{"service2.example.com"},
				},
			},
		},
	}

	client := utilmock.NewClient()
	webhookClient := NewWebhookClient()
	ctx := context.Background()

	client.On("List",
		mock.IsType(ctx),
		mock.IsType(&v1beta1.ServiceExportList{}),
		mock.IsType([]k8sclient.ListOption(nil)),
	).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(1).(*v1beta1.ServiceExportList)
		*arg = *serviceExportList
	})

	result, err := webhookClient.GetAllServiceExports(ctx, client, "test-slice")
	if err != nil {
		t.Error("Expected no error but got:", err)
	}

	if result == nil {
		t.Fatal("Expected result to be non-nil")
	}

	if len(result.Items) != 2 {
		t.Errorf("Expected 2 service exports but got %d", len(result.Items))
	}
}

func TestAliasExist(t *testing.T) {
	tests := []struct {
		existingAliases []string
		newAlias        string
		expected        bool
	}{
		{[]string{"service1.com", "service2.com"}, "service1.com", true},
		{[]string{"service1.com", "service2.com"}, "SERVICE1.COM", true}, // Case insensitive
		{[]string{"service1.com", "service2.com"}, "service3.com", false},
		{[]string{}, "service1.com", false},
	}

	for _, test := range tests {
		result := aliasExist(test.existingAliases, test.newAlias)
		if result != test.expected {
			t.Errorf("For existing aliases %v and new alias %s, expected %v but got %v",
				test.existingAliases, test.newAlias, test.expected, result)
		}
	}
}

func TestMutatePod(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "test-namespace",
		},
	}

	sliceName := "test-slice"
	mutatedPod := MutatePod(pod, sliceName)

	// Check annotations
	if mutatedPod.Annotations[AdmissionWebhookAnnotationStatusKey] != "injected" {
		t.Error("Expected status annotation to be 'injected'")
	}

	expectedNsmAnnotation := "vl3-service-" + sliceName
	if mutatedPod.Annotations[nsmInjectAnnotaionKey1] != expectedNsmAnnotation {
		t.Errorf("Expected NSM annotation to be '%s' but got '%s'",
			expectedNsmAnnotation, mutatedPod.Annotations[nsmInjectAnnotaionKey1])
	}

	// Check labels
	if mutatedPod.Labels[PodInjectLabelKey] != "app" {
		t.Error("Expected pod-type label to be 'app'")
	}

	if mutatedPod.Labels[admissionWebhookAnnotationInjectKey] != sliceName {
		t.Errorf("Expected slice label to be '%s' but got '%s'",
			sliceName, mutatedPod.Labels[admissionWebhookAnnotationInjectKey])
	}
}

func TestMutateDeployment(t *testing.T) {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deployment",
			Namespace: "test-namespace",
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{},
			},
		},
	}

	sliceName := "test-slice"
	mutatedDeployment := MutateDeployment(deployment, sliceName)

	// Check pod template annotations
	if mutatedDeployment.Spec.Template.Annotations[AdmissionWebhookAnnotationStatusKey] != "injected" {
		t.Error("Expected status annotation to be 'injected'")
	}

	// Check pod template labels
	if mutatedDeployment.Spec.Template.Labels[PodInjectLabelKey] != "app" {
		t.Error("Expected pod-type label to be 'app'")
	}

	if mutatedDeployment.Spec.Template.Labels[admissionWebhookAnnotationInjectKey] != sliceName {
		t.Errorf("Expected slice label to be '%s' but got '%s'",
			sliceName, mutatedDeployment.Spec.Template.Labels[admissionWebhookAnnotationInjectKey])
	}

	// Check deployment labels
	if mutatedDeployment.Labels[admissionWebhookAnnotationInjectKey] != sliceName {
		t.Errorf("Expected deployment slice label to be '%s' but got '%s'",
			sliceName, mutatedDeployment.Labels[admissionWebhookAnnotationInjectKey])
	}
}

func TestMutateStatefulset(t *testing.T) {
	statefulset := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-statefulset",
			Namespace: "test-namespace",
		},
		Spec: appsv1.StatefulSetSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{},
			},
		},
	}

	sliceName := "test-slice"
	mutatedStatefulset := MutateStatefulset(statefulset, sliceName)

	// Check pod template annotations
	if mutatedStatefulset.Spec.Template.Annotations[AdmissionWebhookAnnotationStatusKey] != "injected" {
		t.Error("Expected status annotation to be 'injected'")
	}

	// Check pod template labels
	if mutatedStatefulset.Spec.Template.Labels[PodInjectLabelKey] != "app" {
		t.Error("Expected pod-type label to be 'app'")
	}

	// Check statefulset labels
	if mutatedStatefulset.Labels[admissionWebhookAnnotationInjectKey] != sliceName {
		t.Errorf("Expected statefulset slice label to be '%s' but got '%s'",
			sliceName, mutatedStatefulset.Labels[admissionWebhookAnnotationInjectKey])
	}
}

func TestMutateDaemonSet(t *testing.T) {
	daemonset := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-daemonset",
			Namespace: "test-namespace",
		},
		Spec: appsv1.DaemonSetSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{},
			},
		},
	}

	sliceName := "test-slice"
	mutatedDaemonset := MutateDaemonSet(daemonset, sliceName)

	// Check pod template annotations
	if mutatedDaemonset.Spec.Template.Annotations[AdmissionWebhookAnnotationStatusKey] != "injected" {
		t.Error("Expected status annotation to be 'injected'")
	}

	// Check pod template labels
	if mutatedDaemonset.Spec.Template.Labels[PodInjectLabelKey] != "app" {
		t.Error("Expected pod-type label to be 'app'")
	}

	// Check daemonset labels
	if mutatedDaemonset.Labels[admissionWebhookAnnotationInjectKey] != sliceName {
		t.Errorf("Expected daemonset slice label to be '%s' but got '%s'",
			sliceName, mutatedDaemonset.Labels[admissionWebhookAnnotationInjectKey])
	}
}

func TestGetSliceOverlayNetworkType(t *testing.T) {
	webhookClient := NewWebhookClient()
	ctx := context.Background()
	client := utilmock.NewClient()

	client.On("Get",
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(apierrors.NewNotFound(schema.GroupResource{Resource: "slice"}, "test-slice"))

	networkType, err := webhookClient.GetSliceOverlayNetworkType(ctx, client, "test-slice")
	if err == nil {
		t.Error("Expected error when slice is not found")
	}
	t.Logf("Network type: %v, err: %v", networkType, err)
}

func TestMutateWithNilAnnotations(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "test-pod",
			Namespace:   "test-namespace",
			Annotations: nil, // nil annotations
		},
	}

	sliceName := "test-slice"
	mutatedPod := MutatePod(pod, sliceName)

	if mutatedPod.Annotations == nil {
		t.Error("Expected annotations to be initialized")
	}

	if mutatedPod.Annotations[AdmissionWebhookAnnotationStatusKey] != "injected" {
		t.Error("Expected status annotation to be 'injected'")
	}
}

func TestMutateWithNilLabels(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "test-namespace",
			Labels:    nil, // nil labels
		},
	}

	sliceName := "test-slice"
	mutatedPod := MutatePod(pod, sliceName)

	if mutatedPod.Labels == nil {
		t.Error("Expected labels to be initialized")
	}

	if mutatedPod.Labels[PodInjectLabelKey] != "app" {
		t.Error("Expected pod-type label to be 'app'")
	}
}

func TestMutateDeploymentWithExistingAnnotations(t *testing.T) {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deployment",
			Namespace: "test-namespace",
			Annotations: map[string]string{
				"existing-annotation": "value",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"existing-pod-annotation": "value",
					},
				},
			},
		},
	}

	sliceName := "test-slice"
	mutatedDeployment := MutateDeployment(deployment, sliceName)

	// Check existing annotations are preserved
	if mutatedDeployment.Spec.Template.Annotations["existing-pod-annotation"] != "value" {
		t.Error("Expected existing pod annotation to be preserved")
	}

	// Check new annotations are added
	if mutatedDeployment.Spec.Template.Annotations[AdmissionWebhookAnnotationStatusKey] != "injected" {
		t.Error("Expected status annotation to be added")
	}
}
