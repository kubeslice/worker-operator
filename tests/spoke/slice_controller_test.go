package spoke_test

import (
	"context"
	"time"

	meshv1beta1 "bitbucket.org/realtimeai/kubeslice-operator/api/v1beta1"
	"bitbucket.org/realtimeai/kubeslice-operator/internal/logger"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var log = logger.NewLogger()

var _ = Describe("SliceController", func() {

	Context("With a Slice CR and mesh-dns service - slice.Status.DNSIP reconciliation", func() {

		var slice *meshv1beta1.Slice
		var svc *corev1.Service

		BeforeEach(func() {

			// Prepare k8s objects for slice and mesh-dns service
			slice = &meshv1beta1.Slice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slice",
					Namespace: "kubeslice-system",
				},
				Spec: meshv1beta1.SliceSpec{},
			}

			svc = &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mesh-dns",
					Namespace: "kubeslice-system",
				},
				Spec: corev1.ServiceSpec{
					ClusterIP: "10.0.0.20",
					Ports: []corev1.ServicePort{{
						Port: 52,
					}},
				},
			}

			// Cleanup after each test
			DeferCleanup(func() {
				ctx := context.Background()
				Expect(k8sClient.Delete(ctx, slice)).Should(Succeed())
				Expect(k8sClient.Delete(ctx, svc)).Should(Succeed())
			})
		})

		It("Should update slice status with DNS IP", func() {
			ctx := context.Background()

			// Create slice and mesh-dns service
			Expect(k8sClient.Create(ctx, slice)).Should(Succeed())
			Expect(k8sClient.Create(ctx, svc)).Should(Succeed())

			svcKey := types.NamespacedName{Name: "mesh-dns", Namespace: "kubeslice-system"}
			createdSvc := &corev1.Service{}

			// Wait until service is created properly
			Eventually(func() bool {
				err := k8sClient.Get(ctx, svcKey, createdSvc)
				if err != nil {
					return false
				}
				return true
			}, time.September*10, time.Millisecond*250).Should(BeTrue())

			sliceKey := types.NamespacedName{Name: "test-slice", Namespace: "kubeslice-system"}
			createdSlice := &meshv1beta1.Slice{}

			// Make sure slice status.Status.DNSIP is pointing to correct serviceIP
			Eventually(func() string {
				err := k8sClient.Get(ctx, sliceKey, createdSlice)
				if err != nil {
					return ""
				}
				return createdSlice.Status.DNSIP
			}, time.Second*10, time.Millisecond*250).Should(Equal("10.0.0.20"))

		})

	})

})
