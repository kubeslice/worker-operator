package spoke_test

import (
	"context"
	"strings"
	"time"

	meshv1beta1 "bitbucket.org/realtimeai/kubeslice-operator/api/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func getTestServiceExportPorts() []meshv1beta1.ServicePort {
	return []meshv1beta1.ServicePort{
		meshv1beta1.ServicePort{
			Name:          "proto-tcp",
			ContainerPort: 80,
			Protocol:      corev1.ProtocolTCP,
		},
	}
}

var _ = Describe("ServiceExportController", func() {

	Context("With a service export CR object installed, verify service export CR is reconciled", func() {
		var slice *meshv1beta1.Slice
		var dnssvc *corev1.Service
		var svcex *meshv1beta1.ServiceExport

		BeforeEach(func() {
			// Prepare k8s objects for slice and mesh-dns service
			slice = &meshv1beta1.Slice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slice",
					Namespace: "kubeslice-system",
				},
				Spec: meshv1beta1.SliceSpec{},
			}
			Expect(k8sClient.Create(ctx, slice)).Should(Succeed())

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
			Expect(k8sClient.Create(ctx, dnssvc)).Should(Succeed())

			// Prepare k8s objects for slice and mesh-dns service
			svcex = &meshv1beta1.ServiceExport{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "iperf-server",
					Namespace: "default",
				},
				Spec: meshv1beta1.ServiceExportSpec{
					Slice: "test-slice",
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "iperf"},
					},
					Ports: getTestServiceExportPorts(),
				},
			}
			Expect(k8sClient.Create(ctx, svcex)).Should(Succeed())

			// Cleanup after each test
			DeferCleanup(func() {
				ctx := context.Background()
				Expect(k8sClient.Delete(ctx, svcex)).Should(Succeed())
				Expect(k8sClient.Delete(ctx, dnssvc)).Should(Succeed())
				Expect(k8sClient.Delete(ctx, slice)).Should(Succeed())
			})
		})

		It("Should update service export status", func() {
			ctx := context.Background()

			svcKey := types.NamespacedName{Name: "iperf-server", Namespace: "default"}
			createdSvcEx := &meshv1beta1.ServiceExport{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, svcKey, createdSvcEx)
				if err != nil {
					return false
				}
				if createdSvcEx.Status.ExposedPorts != "80/TCP" {
					return false
				}
				if createdSvcEx.Status.ExportStatus != meshv1beta1.ExportStatusReady {
					return false
				}
				return true
			}, time.Second*30, time.Millisecond*250).Should(BeTrue())
		})

		It("Should update service export ports", func() {
			ctx := context.Background()

			svcKey := types.NamespacedName{Name: "iperf-server", Namespace: "default"}
			createdSvcEx := &meshv1beta1.ServiceExport{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, svcKey, createdSvcEx)
				if err != nil {
					return false
				}
				if createdSvcEx.Status.ExportStatus != meshv1beta1.ExportStatusReady {
					return false
				}
				return true
			}, time.Second*30, time.Millisecond*250).Should(BeTrue())

			createdSvcEx.Spec.Ports = append(createdSvcEx.Spec.Ports, meshv1beta1.ServicePort{
				Name:          "proto-2",
				ContainerPort: 5353,
				Protocol:      corev1.ProtocolUDP,
			})

			Expect(k8sClient.Update(ctx, createdSvcEx)).Should(Succeed())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, svcKey, createdSvcEx)
				if err != nil {
					return false
				}
				if !strings.Contains(createdSvcEx.Status.ExposedPorts, "5353/UDP") {
					return false
				}
				if createdSvcEx.Status.ExportStatus != meshv1beta1.ExportStatusReady {
					return false
				}
				return true
			}, time.Second*30, time.Millisecond*250).Should(BeTrue())

		})
	})
})

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
					Name:      "test-slice",
					Namespace: "kubeslice-system",
				},
				Spec: meshv1beta1.SliceSpec{},
			}
			Expect(k8sClient.Create(ctx, slice)).Should(Succeed())

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
			Expect(k8sClient.Create(ctx, dnssvc)).Should(Succeed())

			dnscm = &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mesh-dns",
					Namespace: "kubeslice-system",
				},
				Data: map[string]string{"slice.db": ""},
			}
			Expect(k8sClient.Create(ctx, dnscm)).Should(Succeed())

			// Prepare k8s objects for slice and mesh-dns service
			svcim = &meshv1beta1.ServiceImport{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "iperf-server",
					Namespace: "default",
				},
				Spec: meshv1beta1.ServiceImportSpec{
					Slice:   "test-slice",
					DNSName: "iperf-server.iperf.svc.slice.local",
					Ports:   getTestServiceExportPorts(),
				},
			}
			Expect(k8sClient.Create(ctx, svcim)).Should(Succeed())

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
