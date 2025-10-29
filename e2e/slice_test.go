package e2e

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
)

var _ = Describe("Slice E2E", func() {
	var (
		ctx       context.Context
		namespace string
		sliceName string
	)

	BeforeEach(func() {
		ctx = context.Background()
		namespace = "test-namespace"
		// sliceName = "test-slice"
		sliceName = "e2e-slice"

		//  Create namespace if not exists
		createNamespaceIfNotExists(ctx, namespace)
	})

	It("should create a Slice and reconcile its status", func() {
		//  Create Slice object
		slice := &kubeslicev1beta1.Slice{
			ObjectMeta: metav1.ObjectMeta{
				Name:      sliceName,
				Namespace: namespace,
				Labels: map[string]string{
					"kubeslice.io/origin": "hub",
				},
				Annotations: map[string]string{
					"kubeslice.io/reconciled-from-hub": "true",
				},
			},
			Spec: kubeslicev1beta1.SliceSpec{},
		}
		Expect(k8sClient.Create(ctx, slice)).To(Succeed())

		Eventually(func(g Gomega) {
			updated := &kubeslicev1beta1.Slice{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: sliceName, Namespace: namespace}, updated)).To(Succeed())
			g.Expect(updated.Status.SliceConfig).NotTo(BeNil())
		}).WithTimeout(2 * time.Minute).WithPolling(2 * time.Second).Should(Succeed())
	})

	It("should update app pods in Slice status when pods are created", func() {
		//  Create the Slice first
		slice := &kubeslicev1beta1.Slice{
			ObjectMeta: metav1.ObjectMeta{
				Name:      sliceName,
				Namespace: namespace,
				Labels: map[string]string{
					"kubeslice.io/origin": "hub",
				},
				Annotations: map[string]string{
					"kubeslice.io/reconciled-from-hub": "true",
				},
			},
			Spec: kubeslicev1beta1.SliceSpec{},
		}
		Expect(k8sClient.Create(ctx, slice)).To(Succeed())

		// Create a test app pod with the application namespace selector label
		labels := map[string]string{
			"app":                                "test-app",
			"kubeslice.io/application-namespace": sliceName,
		}

		createTestPod(ctx, namespace, "app-pod", labels)

		Eventually(func(g Gomega) {
			updated := &kubeslicev1beta1.Slice{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: sliceName, Namespace: namespace}, updated)).To(Succeed())
			g.Expect(len(updated.Status.AppPods)).To(BeNumerically(">", 0))
			g.Expect(updated.Status.AppPods[0].PodName).To(Equal("app-pod"))
		}).WithTimeout(2 * time.Minute).WithPolling(2 * time.Second).Should(Succeed())
	})

	It("should delete Slice successfully", func() {
		//  Create Slice
		slice := &kubeslicev1beta1.Slice{
			ObjectMeta: metav1.ObjectMeta{
				Name:      sliceName,
				Namespace: namespace,
				Labels: map[string]string{
					"kubeslice.io/origin": "hub",
				},
				Annotations: map[string]string{
					"kubeslice.io/reconciled-from-hub": "true",
				},
			},
			Spec: kubeslicev1beta1.SliceSpec{},
		}
		Expect(k8sClient.Create(ctx, slice)).To(Succeed())

		Expect(k8sClient.Delete(ctx, slice)).To(Succeed())

		//  Verify Slice is deleted
		Eventually(func() error {
			s := &kubeslicev1beta1.Slice{}
			return k8sClient.Get(ctx, client.ObjectKey{Name: sliceName, Namespace: namespace}, s)
		}).WithTimeout(1 * time.Minute).WithPolling(2 * time.Second).ShouldNot(Succeed())
	})
})
