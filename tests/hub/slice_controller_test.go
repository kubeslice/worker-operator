package hub_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	workerv1alpha1 "github.com/kubeslice/apis/pkg/worker/v1alpha1"
	kubeslicev1beta1 "github.com/kubeslice/operator/api/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Hub SliceController", func() {

	Context("With Slice CR created in hub", func() {

		var hubSlice *workerv1alpha1.WorkerSliceConfig
		var createdSlice *kubeslicev1beta1.Slice

		BeforeEach(func() {

			// Prepare k8s objects
			hubSlice = &workerv1alpha1.WorkerSliceConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slice-1",
					Namespace: PROJECT_NS,
					Labels: map[string]string{
						"worker-cluster": CLUSTER_NAME,
					},
				},
				Spec: workerv1alpha1.WorkerSliceConfigSpec{
					SliceName:        "test-slice-1",
					SliceType:        "Application",
					SliceSubnet:      "10.0.0.1/16",
					SliceIpamType:    "Local",
					IpamClusterOctet: 100,
				},
			}

			createdSlice = &kubeslicev1beta1.Slice{}

			// Cleanup after each test
			DeferCleanup(func() {
				Expect(k8sClient.Delete(ctx, hubSlice)).Should(Succeed())
				Eventually(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Namespace: "kubeslice-system", Name: createdSlice.Name}, createdSlice)
					return errors.IsNotFound(err)
				}, time.Second*10, time.Millisecond*250).Should(BeTrue())
			})
		})

		It("Should create Slice CR in worker", func() {
			ctx := context.Background()

			Expect(k8sClient.Create(ctx, hubSlice)).Should(Succeed())

			sliceKey := types.NamespacedName{Name: "test-slice-1", Namespace: "kubeslice-system"}

			// Make sure slice is reconciled in worker cluster
			Eventually(func() bool {
				err := k8sClient.Get(ctx, sliceKey, createdSlice)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			Expect(createdSlice.Status.SliceConfig.SliceSubnet).To(Equal("10.0.0.1/16"))
			Expect(createdSlice.Status.SliceConfig.SliceDisplayName).To(Equal("test-slice-1"))
			Expect(createdSlice.Status.SliceConfig.SliceType).To(Equal("Application"))
			Expect(createdSlice.Status.SliceConfig.SliceIpam.SliceIpamType).To(Equal("Local"))
			Expect(createdSlice.Status.SliceConfig.SliceIpam.IpamClusterOctet).To(Equal(100))

		})
		It("Should register a finalizer on workerSliceConfig CR", func() {
			ctx := context.Background()
			Expect(k8sClient.Create(ctx, hubSlice)).Should(Succeed())

			sliceKey := types.NamespacedName{Name: "test-slice-1", Namespace: "kubeslice-system"}
			// Make sure slice is reconciled in worker cluster
			Eventually(func() bool {
				err := k8sClient.Get(ctx, sliceKey, createdSlice)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			//get the created hubSlice
			hubSliceKey := types.NamespacedName{Name: "test-slice-1", Namespace: PROJECT_NS}
			sliceFinalizer := "controller.kubeslice.io/hubSpokeSlice-finalizer"

			Eventually(func() bool {
				err := k8sClient.Get(ctx, hubSliceKey, hubSlice)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())
			Expect(hubSlice.ObjectMeta.Finalizers[0]).Should(Equal(sliceFinalizer))
		})

	})

	Context("With Slice CR deleted on hub", func() {
		var hubSlice *workerv1alpha1.WorkerSliceConfig
		var createdSlice *kubeslicev1beta1.Slice

		BeforeEach(func() {
			// Prepare k8s objects
			hubSlice = &workerv1alpha1.WorkerSliceConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slice-2",
					Namespace: PROJECT_NS,
					Labels: map[string]string{
						"worker-cluster": CLUSTER_NAME,
					},
				},
				Spec: workerv1alpha1.WorkerSliceConfigSpec{
					SliceName:        "test-slice-2",
					SliceType:        "Application",
					SliceSubnet:      "10.0.0.1/16",
					SliceIpamType:    "Local",
					IpamClusterOctet: 100,
				},
			}

			createdSlice = &kubeslicev1beta1.Slice{}

		})
		It("Should Delete the slice CR on worker", func() {
			ctx := context.Background()
			Expect(k8sClient.Create(ctx, hubSlice)).Should(Succeed())

			sliceKey := types.NamespacedName{Name: "test-slice-2", Namespace: "kubeslice-system"}
			// Make sure slice is reconciled in worker cluster
			Eventually(func() bool {
				err := k8sClient.Get(ctx, sliceKey, createdSlice)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())
			//delete the hubSlice , which should delete the slice CR on worker
			Expect(k8sClient.Delete(ctx, hubSlice)).Should(Succeed())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, sliceKey, createdSlice)
				return errors.IsNotFound(err)
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

		})
	})

	Context("With ExternalGatewayConfig", func() {

		var hubSlice *workerv1alpha1.WorkerSliceConfig
		var createdSlice *kubeslicev1beta1.Slice

		BeforeEach(func() {

			// Prepare k8s objects
			hubSlice = &workerv1alpha1.WorkerSliceConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slice-3",
					Namespace: PROJECT_NS,
					Labels: map[string]string{
						"worker-cluster": CLUSTER_NAME,
					},
				},
				Spec: workerv1alpha1.WorkerSliceConfigSpec{
					SliceName: "test-slice-3",
					ExternalGatewayConfig: workerv1alpha1.ExternalGatewayConfig{
						Ingress: workerv1alpha1.ExternalGatewayConfigOptions{
							Enabled: true,
						},
						Egress: workerv1alpha1.ExternalGatewayConfigOptions{
							Enabled: true,
						},
						NsIngress: workerv1alpha1.ExternalGatewayConfigOptions{
							Enabled: true,
						},
					},
				},
			}

			createdSlice = &kubeslicev1beta1.Slice{}

			// Cleanup after each test
			DeferCleanup(func() {
				Expect(k8sClient.Delete(ctx, hubSlice)).Should(Succeed())
				Eventually(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Namespace: "kubeslice-system", Name: createdSlice.Name}, createdSlice)
					return errors.IsNotFound(err)
				}, time.Second*10, time.Millisecond*250).Should(BeTrue())
			})
		})

		It("Should create Slice CR in worker", func() {
			ctx := context.Background()

			Expect(k8sClient.Create(ctx, hubSlice)).Should(Succeed())

			sliceKey := types.NamespacedName{Name: "test-slice-3", Namespace: "kubeslice-system"}

			// Make sure slice is reconciled in worker cluster
			Eventually(func() bool {
				err := k8sClient.Get(ctx, sliceKey, createdSlice)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			Expect(createdSlice.Status.SliceConfig.ExternalGatewayConfig).ToNot(BeNil())
			Expect(createdSlice.Status.SliceConfig.ExternalGatewayConfig.Ingress.Enabled).To(BeTrue())
			Expect(createdSlice.Status.SliceConfig.ExternalGatewayConfig.Egress.Enabled).To(BeTrue())
			Expect(createdSlice.Status.SliceConfig.ExternalGatewayConfig.NsIngress.Enabled).To(BeTrue())

		})

	})

})
