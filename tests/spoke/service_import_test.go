package spoke_test

import (
	"context"
	"time"

	meshv1beta1 "bitbucket.org/realtimeai/kubeslice-operator/api/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("ServiceImportController", func() {

	Context("With a service import CR object installed, verify service import CR is reconciled", func() {
		var slice *meshv1beta1.Slice
		var dnssvc *corev1.Service
		var dnscm *corev1.ConfigMap
		var svcim *meshv1beta1.ServiceImport

		BeforeEach(func() {
			// Prepare k8s objects for slice and mesh-dns service
			slice = &meshv1beta1.Slice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slice-2",
					Namespace: "kubeslice-system",
				},
				Spec: meshv1beta1.SliceSpec{},
			}

			dnssvc = &corev1.Service{
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

			dnscm = &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mesh-dns",
					Namespace: "kubeslice-system",
				},
				Data: map[string]string{"slice.db": ""},
			}

			// Prepare k8s objects for slice and mesh-dns service
			svcim = &meshv1beta1.ServiceImport{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "iperf-server",
					Namespace: "default",
				},
				Spec: meshv1beta1.ServiceImportSpec{
					Slice:   "test-slice-2",
					DNSName: "iperf-server.iperf.svc.slice.local",
					Ports:   getTestServiceExportPorts(),
				},
			}

			// Cleanup after each test
			DeferCleanup(func() {
				ctx := context.Background()
				Expect(k8sClient.Delete(ctx, svcim)).Should(Succeed())
				Expect(k8sClient.Delete(ctx, dnscm)).Should(Succeed())
				Expect(k8sClient.Delete(ctx, dnssvc)).Should(Succeed())
				Expect(k8sClient.Delete(ctx, slice)).Should(Succeed())
			})
		})

		It("Should update service import status", func() {
			ctx := context.Background()
			Expect(k8sClient.Create(ctx, slice)).Should(Succeed())
			Expect(k8sClient.Create(ctx, dnssvc)).Should(Succeed())
			Expect(k8sClient.Create(ctx, dnscm)).Should(Succeed())
			Expect(k8sClient.Create(ctx, svcim)).Should(Succeed())
			svcKey := types.NamespacedName{Name: "iperf-server", Namespace: "default"}
			createdSvcIm := &meshv1beta1.ServiceImport{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, svcKey, createdSvcIm)
				if err != nil {
					return false
				}
				if createdSvcIm.Status.ExposedPorts != "80/TCP" {
					return false
				}
				if createdSvcIm.Status.ImportStatus != meshv1beta1.ImportStatusReady {
					return false
				}
				return true
			}, time.Second*30, time.Millisecond*250).Should(BeTrue())
		})
	})
})
