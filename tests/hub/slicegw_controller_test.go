package hub_test

import (
	"time"

	meshv1beta1 "bitbucket.org/realtimeai/kubeslice-operator/api/v1beta1"
	spokev1alpha1 "bitbucket.org/realtimeai/mesh-apis/pkg/spoke/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Hub SlicegwController", func() {
	Context("With SpokeSliceGW created in hub", func() {
		var hubSlice *spokev1alpha1.SpokeSliceConfig
		var createdSlice *meshv1beta1.Slice
		var hubSliceGw *spokev1alpha1.SpokeSliceGateway
		var hubSecret *corev1.Secret
		var createdSliceGwOnSpoke *meshv1beta1.SliceGateway

		BeforeEach(func() {
			// Prepare k8s objects
			hubSlice = &spokev1alpha1.SpokeSliceConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slice",
					Namespace: PROJECT_NS,
					Labels: map[string]string{
						"spoke-cluster": CLUSTER_NAME,
					},
				},
				Spec: spokev1alpha1.SpokeSliceConfigSpec{
					SliceName:        "test-slice",
					SliceType:        "Application",
					SliceSubnet:      "10.0.0.1/16",
					SliceIpamType:    "Local",
					IpamClusterOctet: 100,
				},
			}
			hubSliceGw = &spokev1alpha1.SpokeSliceGateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slicegateway",
					Namespace: PROJECT_NS,
					Labels: map[string]string{
						"spoke-cluster": CLUSTER_NAME,
					},
				},
				Spec: spokev1alpha1.SpokeSliceGatewaySpec{
					SliceName: "test-slice",
				},
			}
			hubSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slicegateway",
					Namespace: PROJECT_NS,
				},
				Data: map[string][]byte{},
			}
			createdSlice = &meshv1beta1.Slice{}
			createdSliceGwOnSpoke = &meshv1beta1.SliceGateway{}

			// Cleanup after each test
			DeferCleanup(func() {
				Expect(k8sClient.Delete(ctx, hubSlice)).Should(Succeed())
				Expect(k8sClient.Delete(ctx, hubSliceGw)).Should(Succeed())
				Expect(k8sClient.Delete(ctx, hubSecret)).Should(Succeed())
				Eventually(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Namespace: "kubeslice-system", Name: createdSlice.Name}, createdSlice)
					return errors.IsNotFound(err)
				}, time.Second*10, time.Millisecond*250).Should(BeTrue())
				Expect(k8sClient.Delete(ctx, createdSliceGwOnSpoke)).Should(Succeed())
			})
		})

		It("Should create SliceGw on spoke cluster", func() {
			Expect(k8sClient.Create(ctx, hubSlice)).Should(Succeed())
			Expect(k8sClient.Create(ctx, hubSliceGw)).Should(Succeed())
			Expect(k8sClient.Create(ctx, hubSecret)).Should(Succeed())
			//once hubSlice is created controller will create a slice CR on spoke cluster
			sliceKey := types.NamespacedName{Name: "test-slice", Namespace: "kubeslice-system"}
			// Make sure slice is reconciled in spoke cluster
			Eventually(func() bool {
				err := k8sClient.Get(ctx, sliceKey, createdSlice)
				if err != nil {
					return false
				}
				return true
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			sliceGwKey := types.NamespacedName{Namespace: CONTROL_PLANE_NS, Name: hubSliceGw.Name}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, sliceGwKey, createdSliceGwOnSpoke)
				return err == nil
			}, time.Second*10, time.Second*1).Should(BeTrue())
		})

		It("Should set slice as owner of slicegw", func() {
			Expect(k8sClient.Create(ctx, hubSlice)).Should(Succeed())
			Expect(k8sClient.Create(ctx, hubSliceGw)).Should(Succeed())
			Expect(k8sClient.Create(ctx, hubSecret)).Should(Succeed())
			//once hubSlice is created controller will create a slice CR on spoke cluster
			sliceKey := types.NamespacedName{Name: "test-slice", Namespace: "kubeslice-system"}
			// Make sure slice is reconciled in spoke cluster
			Eventually(func() bool {
				err := k8sClient.Get(ctx, sliceKey, createdSlice)
				if err != nil {
					return false
				}
				return true
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			sliceGwKey := types.NamespacedName{Namespace: CONTROL_PLANE_NS, Name: hubSliceGw.Name}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, sliceGwKey, createdSliceGwOnSpoke)
				return err == nil
			}, time.Second*10, time.Second*1).Should(BeTrue())

			Expect(createdSliceGwOnSpoke.ObjectMeta.OwnerReferences[0].Name).Should(Equal("test-slice"))
		})

	})
})
