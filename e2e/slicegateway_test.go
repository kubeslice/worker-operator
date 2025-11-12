package e2e

import (
	"context"
	"time"

	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("SliceGateway E2E", Ordered, func() {
	var (
		ctx              context.Context
		sliceGwName      string
		sliceGwNamespace string
		sliceName        string
		siteName         string
	)

	BeforeAll(func() {
		ctx = context.Background()
		sliceGwName = "e2e-slicegateway"
		sliceGwNamespace = "kubeslice-system" // control plane namespace
		sliceName = "e2e-slice"
		siteName = "e2e-site"

		//  Create the namespace so the test doesn't fail
		createNamespaceIfNotExists(ctx, sliceGwNamespace)
	})

	AfterAll(func() {
		// Cleanup resources at the end of test
		gw := &kubeslicev1beta1.SliceGateway{}
		err := k8sClient.Get(ctx, types.NamespacedName{Name: sliceGwName, Namespace: sliceGwNamespace}, gw)
		if err == nil {
			_ = k8sClient.Delete(ctx, gw)
		}
	})

	It("should create a SliceGateway CR successfully", func() {
		gw := &kubeslicev1beta1.SliceGateway{
			ObjectMeta: metav1.ObjectMeta{
				Name:      sliceGwName,
				Namespace: sliceGwNamespace,
			},
			Spec: kubeslicev1beta1.SliceGatewaySpec{
				SliceName: sliceName,
				SiteName:  siteName,
			},
		}

		By("creating the SliceGateway resource")
		Expect(k8sClient.Create(ctx, gw)).To(Succeed())

		By("verifying the resource exists")
		created := &kubeslicev1beta1.SliceGateway{}
		Eventually(func() error {
			return k8sClient.Get(ctx, types.NamespacedName{Name: sliceGwName, Namespace: sliceGwNamespace}, created)
		}, 60*time.Second, 5*time.Second).Should(Succeed())

		Expect(created.Spec.SliceName).To(Equal(sliceName))
		Expect(created.Spec.SiteName).To(Equal(siteName))
	})

	It("should update SliceGateway status when reconciled", func() {
		By("waiting for controller to reconcile")
		Eventually(func() string {
			gw := &kubeslicev1beta1.SliceGateway{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: sliceGwName, Namespace: sliceGwNamespace}, gw); err != nil {
				return ""
			}
			return gw.Status.Config.SliceGatewayStatus
		}, 2*time.Minute, 5*time.Second).ShouldNot(BeEmpty())
	})

	It("should have gateway pod created by controller", func() {
		By("verifying a gateway pod exists")
		Eventually(func() int {
			podList := &corev1.PodList{}
			err := k8sClient.List(ctx, podList, client.InNamespace(sliceGwNamespace),
				client.MatchingLabels(map[string]string{
					"networking.kubeslice.io/slicegateway": sliceGwName,
				}))
			if err != nil {
				return 0
			}
			return len(podList.Items)
		}, 2*time.Minute, 10*time.Second).Should(BeNumerically(">", 0))
	})

	It("should delete SliceGateway successfully", func() {
		By("deleting the SliceGateway")
		gw := &kubeslicev1beta1.SliceGateway{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: sliceGwName, Namespace: sliceGwNamespace}, gw)).To(Succeed())
		Expect(k8sClient.Delete(ctx, gw)).To(Succeed())

		By("verifying the resource is deleted")
		Eventually(func() bool {
			err := k8sClient.Get(ctx, types.NamespacedName{Name: sliceGwName, Namespace: sliceGwNamespace}, gw)
			return err != nil
		}, 60*time.Second, 5*time.Second).Should(BeTrue())
	})
})
