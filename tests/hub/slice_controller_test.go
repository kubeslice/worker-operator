package hub_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	meshv1beta1 "bitbucket.org/realtimeai/kubeslice-operator/api/v1beta1"
	"bitbucket.org/realtimeai/kubeslice-operator/internal/logger"
	spokev1alpha1 "bitbucket.org/realtimeai/mesh-apis/pkg/spoke/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var log = logger.NewLogger()

var _ = Describe("Hub SliceController", func() {

	Context("With Slice CR created in hub", func() {

		var hubSlice *spokev1alpha1.SpokeSliceConfig
		var createdSlice *meshv1beta1.Slice

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

			createdSlice = &meshv1beta1.Slice{}

			// Cleanup after each test
			DeferCleanup(func() {
				ctx := context.Background()
				Expect(k8sClient.Delete(ctx, hubSlice)).Should(Succeed())
				Expect(k8sClient.Delete(ctx, createdSlice)).Should(Succeed())
			})
		})

		It("Should create Slice CR in spoke", func() {
			ctx := context.Background()

			Expect(k8sClient.Create(ctx, hubSlice)).Should(Succeed())

			sliceKey := types.NamespacedName{Name: "test-slice", Namespace: "kubeslice-system"}

			// Make sure slice is reconciled in spoke cluster
			Eventually(func() bool {
				err := k8sClient.Get(ctx, sliceKey, createdSlice)
				if err != nil {
					return false
				}
				return true
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			Expect(createdSlice.Status.SliceConfig.SliceSubnet).To(Equal("10.0.0.1/16"))
			Expect(createdSlice.Status.SliceConfig.SliceDisplayName).To(Equal("test-slice"))
			Expect(createdSlice.Status.SliceConfig.SliceIpam.SliceIpamType).To(Equal("Local"))
			Expect(createdSlice.Status.SliceConfig.SliceType).To(Equal("Application"))
			Expect(createdSlice.Status.SliceConfig.SliceIpam.IpamClusterOctet).To(Equal(100))

		})

	})

})
