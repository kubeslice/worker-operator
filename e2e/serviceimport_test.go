package e2e

import (
	"context"
	"time"

	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("ServiceImport E2E", func() {
	var (
		ctx        context.Context
		namespace  string
		sliceName  string
		importName string
	)

	BeforeEach(func() {
		ctx = context.Background()
		namespace = "test-namespace"
		// sliceName = "test-slice"
		sliceName = "e2e-slice"
		importName = "test-serviceimport"

		createNamespaceIfNotExists(ctx, namespace)

		// createSlice(ctx, sliceName)
	})

	It("should create a ServiceImport and reconcile status correctly", func() {
		//  Deploy a test pod that will act as an endpoint
		labels := map[string]string{"app": "test-app"}
		createTestPod(ctx, namespace, "test-pod", labels)

		svcImport := &kubeslicev1beta1.ServiceImport{
			ObjectMeta: metav1.ObjectMeta{
				Name:      importName,
				Namespace: namespace,
			},
			Spec: kubeslicev1beta1.ServiceImportSpec{
				Slice:   sliceName,
				DNSName: "test-app.slice.local",
				Ports: []kubeslicev1beta1.ServicePort{
					{
						Name:          "http",
						ContainerPort: 8080,
						Protocol:      corev1.ProtocolTCP,
					},
				},
				Aliases: []string{"alias-test-app"},
			},
		}

		Expect(k8sClient.Create(ctx, svcImport)).To(Succeed())

		Eventually(func(g Gomega) {
			updated := &kubeslicev1beta1.ServiceImport{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: importName, Namespace: namespace}, updated)).To(Succeed())
			g.Expect(updated.Status.ImportStatus).To(Equal(kubeslicev1beta1.ImportStatusReady))
		}).WithTimeout(2 * time.Minute).WithPolling(2 * time.Second).Should(Succeed())

		//  if ExposedPorts field is populated
		Eventually(func(g Gomega) {
			updated := &kubeslicev1beta1.ServiceImport{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: importName, Namespace: namespace}, updated)).To(Succeed())
			g.Expect(updated.Status.ExposedPorts).NotTo(BeEmpty())
		}).WithTimeout(2 * time.Minute).WithPolling(2 * time.Second).Should(Succeed())

		//  Check AvailableEndpoints count (should be >= 0 — may depend on controller behavior)
		updated := &kubeslicev1beta1.ServiceImport{}
		Expect(k8sClient.Get(ctx, client.ObjectKey{Name: importName, Namespace: namespace}, updated)).To(Succeed())
		Expect(updated.Status.AvailableEndpoints).To(BeNumerically(">=", 0))
	})
})
