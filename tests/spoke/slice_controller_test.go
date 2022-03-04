package spoke_test

import (
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

	Context("With a Slice CR and mesh dns service", func() {
		It("Should update slice status with DNS IP", func() {

			timeout := time.Second * 10
			interval := time.Millisecond * 250

			slice := &meshv1beta1.Slice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slice",
					Namespace: "kubeslice-system",
				},
				Spec: meshv1beta1.SliceSpec{},
			}

			svc := &corev1.Service{
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
			}, timeout, interval).Should(BeTrue())

			sliceKey := types.NamespacedName{Name: "test-slice", Namespace: "kubeslice-system"}
			createdSlice := &meshv1beta1.Slice{}

			// Make sure slice status.Status.DNSIP is pointing to correct serviceIP
			Eventually(func() string {
				err := k8sClient.Get(ctx, sliceKey, createdSlice)
				if err != nil {
					return ""
				}
				return createdSlice.Status.DNSIP
			}, timeout, interval).Should(Equal("10.0.0.20"))

		})

	})

})
