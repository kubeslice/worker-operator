package spoke_test

import (
	"time"

	meshv1beta1 "bitbucket.org/realtimeai/kubeslice-operator/api/v1beta1"
	"bitbucket.org/realtimeai/kubeslice-operator/internal/logger"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var log = logger.NewLogger()

var _ = Describe("SliceController", func() {

	Context("With a Slice CR created", func() {
		It("Should create the slice and update status with config", func() {

			timeout := time.Second * 60
			interval := time.Second * 1

			slice := &meshv1beta1.Slice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slice",
					Namespace: "kubeslice-system",
				},
				Spec: meshv1beta1.SliceSpec{},
			}

			Expect(k8sClient.Create(ctx, slice)).Should(Succeed())
			slice.Status.SliceConfig = &meshv1beta1.SliceConfig{
				SliceDisplayName: "test-slice",
				SliceSubnet:      "10.0.0.1/8",
				SliceIpam: meshv1beta1.SliceIpamConfig{
					SliceIpamType:    "IPAM_TEST",
					IpamClusterOctet: 1,
				},
				SliceType: "APPLICATION",
			}
			Expect(k8sClient.Status().Update(ctx, slice)).Should(Succeed())

			log.Info("updating slic", "slice", slice)

			lookupKey := types.NamespacedName{Name: "test-slice", Namespace: "kubeslice-system"}
			createdSlice := &meshv1beta1.Slice{}

			// Wait until slice is created properly
			Eventually(func() bool {
				err := k8sClient.Get(ctx, lookupKey, createdSlice)
				if err != nil {
					return false
				}
				// wait for status to be updated
				return createdSlice.Status.SliceConfig != nil
			}, timeout, interval).Should(BeTrue())
			// Expect(createdSlice.Status).ToNot(BeNil())
			// Expect(createdSlice.Status.SliceConfig).ToNot(BeNil())
			Expect(createdSlice.Status.SliceConfig.SliceDisplayName).Should(Equal("test-slice"))
		})

	})

})
