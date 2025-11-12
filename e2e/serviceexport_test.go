package e2e

import (
	"context"
	"time"

	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("ServiceExport E2E", func() {
	var (
		ctx       context.Context
		namespace string
		sliceName string
	)

	BeforeEach(func() {
		ctx = context.Background()
		namespace = "test-namespace"
		sliceName = "e2e-slice"

		//  Create namespace if not exists
		createNamespaceIfNotExists(ctx, namespace)

	})

	It("should create a ServiceExport and update status correctly", func() {
		// Deploy app pod matching ServiceExport selector
		labels := map[string]string{"app": "test-app"}
		createTestPod(ctx, namespace, "app-pod", labels)

		// Create slice first (required for ServiceExport to reconcile)
		createSlice(ctx, sliceName)

		// Create ServiceExport object
		se := &kubeslicev1beta1.ServiceExport{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-svc-export",
				Namespace: namespace,
			},
			Spec: kubeslicev1beta1.ServiceExportSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: labels,
				},
				Slice: sliceName,
				Ports: []kubeslicev1beta1.ServicePort{
					{
						Name:     "http",
						Protocol: "TCP",
					},
				},
			},
		}

		Expect(k8sClient.Create(ctx, se)).To(Succeed())

		Eventually(func(g Gomega) {
			updated := &kubeslicev1beta1.ServiceExport{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: se.Name, Namespace: namespace}, updated)).To(Succeed())
			g.Expect(updated.Status.ExportStatus).To(Equal(kubeslicev1beta1.ExportStatusReady))
			g.Expect(updated.Status.AvailableEndpoints).To(BeNumerically(">", 0))
		}).WithTimeout(2 * time.Minute).WithPolling(2 * time.Second).Should(Succeed())
	})

})
