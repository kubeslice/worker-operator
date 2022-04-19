package spoke_test

import (
	"context"
	"time"

	meshv1beta1 "bitbucket.org/realtimeai/kubeslice-operator/api/v1beta1"
	"bitbucket.org/realtimeai/kubeslice-operator/internal/logger"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var log = logger.NewLogger()
var sliceFinalizer = "mesh.kubeslice.io/slice-finalizer"

var _ = Describe("SliceController", func() {

	Context("With a Slice CR and mesh-dns service created", func() {

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
				Eventually(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Name: slice.Name, Namespace: slice.Namespace}, slice)
					return errors.IsNotFound(err)
				}, time.Second*30, time.Millisecond*250).Should(BeTrue())
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
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			sliceKey := types.NamespacedName{Name: "test-slice", Namespace: "kubeslice-system"}
			createdSlice := &meshv1beta1.Slice{}

			// Make sure slice status.Status.DNSIP is pointing to correct serviceIP
			Eventually(func() string {
				err := k8sClient.Get(ctx, sliceKey, createdSlice)
				if err != nil {
					return ""
				}
				return createdSlice.Status.DNSIP
			}, time.Second*30, time.Millisecond*250).Should(Equal("10.0.0.20"))

		})
		It("Should create a finalizer for slice CR created", func() {
			ctx := context.Background()
			// Create slice and mesh-dns service
			Eventually(func() bool {
				err := k8sClient.Create(ctx, slice)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			Eventually(func() bool {
				err := k8sClient.Create(ctx, svc)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			createdSlice := &meshv1beta1.Slice{}
			Eventually(func() bool {
				sliceKey := types.NamespacedName{Name: "test-slice", Namespace: "kubeslice-system"}
				if err := k8sClient.Get(ctx, sliceKey, createdSlice); err != nil {
					return false
				}
				return true
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())
			Expect(createdSlice.ObjectMeta.Finalizers[0]).Should(Equal(sliceFinalizer))
		})

	})
	Context("With Slice CR Deleted", func() {
		var slice *meshv1beta1.Slice
		var svc *corev1.Service
		var svcimport *meshv1beta1.ServiceImport
		var svcexport *meshv1beta1.ServiceExport
		BeforeEach(func() {

			// Prepare k8s objects for slice and mesh-dns service
			slice = &meshv1beta1.Slice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slice-im",
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
			svcimport = &meshv1beta1.ServiceImport{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service-import",
					Namespace: "kubeslice-system",
					Labels: map[string]string{
						"kubeslice.io/slice": "test-slice-im",
					},
				},
				Spec: meshv1beta1.ServiceImportSpec{
					Slice:   "test-slice-im",
					DNSName: "pod.svc.local.cluster",
					Ports: []meshv1beta1.ServicePort{
						{
							Name:          "xyz",
							ContainerPort: 5000,
						},
					},
				},
			}
			svcexport = &meshv1beta1.ServiceExport{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service-export",
					Namespace: "kubeslice-system",
					Labels: map[string]string{
						"kubeslice.io/slice": "test-slice-im",
					},
				},
				Spec: meshv1beta1.ServiceExportSpec{
					Slice: "test-slice-im",
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "iperf",
						},
					},
					Ports: []meshv1beta1.ServicePort{
						{
							Name:          "xyz",
							ContainerPort: 5000,
						},
					},
				},
			}
			DeferCleanup(func() {
				Expect(k8sClient.Delete(ctx, svc)).Should(Succeed())
			})
		})

		It("Should Delete All the dependent resources", func() {
			Expect(k8sClient.Create(ctx, slice)).Should(Succeed())
			Expect(k8sClient.Create(ctx, svc)).Should(Succeed())
			Expect(k8sClient.Create(ctx, svcimport)).Should(Succeed())
			Expect(k8sClient.Create(ctx, svcexport)).Should(Succeed())

			createdSvc := &corev1.Service{}
			svcKey := types.NamespacedName{Name: "mesh-dns", Namespace: "kubeslice-system"}

			// Wait until service is created properly
			Eventually(func() bool {
				err := k8sClient.Get(ctx, svcKey, createdSvc)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			createdSlice := &meshv1beta1.Slice{}
			Eventually(func() bool {
				sliceKey := types.NamespacedName{Name: "test-slice-im", Namespace: "kubeslice-system"}
				if err := k8sClient.Get(ctx, sliceKey, createdSlice); err != nil {
					return false
				}
				return true
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			//Delete the slice CR which will trigger the finalizer and cleanup resources
			Expect(k8sClient.Delete(ctx, slice)).Should(Succeed())
			//check if svc import objects created are deleted or not
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "test-service-import", Namespace: "kubeslice-system"}, svcimport)
				return errors.IsNotFound(err)
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			//check if svc export objects created are deleted or not
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "test-service-export", Namespace: "kubeslice-system"}, svcexport)
				return errors.IsNotFound(err)
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

		})
	})

})
