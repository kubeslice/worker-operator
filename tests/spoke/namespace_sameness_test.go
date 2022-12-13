package spoke_test

import (
	"context"

	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	"github.com/kubeslice/worker-operator/controllers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
)

var _ = Describe("SliceNetpol", func() {
	var slice *kubeslicev1beta1.Slice
	var createdSlice *kubeslicev1beta1.Slice
	var appNs *corev1.Namespace

	When("Slice CR is created with application namespaces specified,", func() {
		BeforeEach(func() {
			// Prepare k8s objects for slice and kubeslice-dns service
			slice = &kubeslicev1beta1.Slice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slice",
					Namespace: "kubeslice-system",
				},
				Spec: kubeslicev1beta1.SliceSpec{},
			}

			appNs = &corev1.Namespace{}

			createdSlice = &kubeslicev1beta1.Slice{}
			// Cleanup after each test
			DeferCleanup(func() {
				ctx := context.Background()
				Expect(k8sClient.Delete(ctx, slice)).Should(Succeed())
				Eventually(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Name: slice.Name, Namespace: slice.Namespace}, slice)
					return errors.IsNotFound(err)
				}, timeout, interval).Should(BeTrue())
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: "test-appns"}, appNs); err != nil {
					Expect(k8sClient.Delete(ctx, appNs)).Should(Succeed())
				}
			})
		})

		It("should create & label app ns if it's not present in specified cluster", func() {
			// create slice
			Expect(k8sClient.Create(ctx, slice)).Should(Succeed())
			// eventually slice comes up
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      "test-slice",
					Namespace: "kubeslice-system",
				}, createdSlice)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			// onboard app ns
			err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      "test-slice",
					Namespace: "kubeslice-system",
				}, createdSlice)
				if err != nil {
					return err
				}
				createdSlice.Status.SliceConfig = &kubeslicev1beta1.SliceConfig{
					NamespaceIsolationProfile: &kubeslicev1beta1.NamespaceIsolationProfile{
						ApplicationNamespaces: []string{
							"test-appns",
						},
					},
				}
				err = k8sClient.Status().Update(ctx, createdSlice)
				return err
			})
			Expect(err).ToNot(HaveOccurred())
			// namespace should get created and labeled
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "test-appns"}, appNs)
				if err != nil {
					return false
				}
				labels := appNs.ObjectMeta.GetLabels()
				if labels == nil {
					return false
				}
				sliceLabel, ok := labels[controllers.ApplicationNamespaceSelectorLabelKey]
				if !ok {
					return false
				}
				Expect(sliceLabel).To(Equal(slice.Name))
				return true
			}, timeout, interval).Should(BeTrue())
		})

		When("A mix of valid & invalid app NS specified in sliceconfig,", func() {
			It("should create & label all valid app ns", func() {
				// create slice
				Expect(k8sClient.Create(ctx, slice)).Should(Succeed())
				// eventually slice comes up
				Eventually(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      "test-slice",
						Namespace: "kubeslice-system",
					}, createdSlice)
					return err == nil
				}, timeout, interval).Should(BeTrue())
				// onboard app ns
				err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      "test-slice",
						Namespace: "kubeslice-system",
					}, createdSlice)
					if err != nil {
						return err
					}
					createdSlice.Status.SliceConfig = &kubeslicev1beta1.SliceConfig{
						NamespaceIsolationProfile: &kubeslicev1beta1.NamespaceIsolationProfile{
							ApplicationNamespaces: []string{
								"test-appNS", // must be a lowercase RFC 1123 label
								"test-appns",
							},
						},
					}
					err = k8sClient.Status().Update(ctx, createdSlice)
					return err
				})
				Expect(err).ToNot(HaveOccurred())
				// namespace should get created and labeled
				Eventually(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Name: "test-appns"}, appNs)
					if err != nil {
						return false
					}
					labels := appNs.ObjectMeta.GetLabels()
					if labels == nil {
						return false
					}
					sliceLabel, ok := labels[controllers.ApplicationNamespaceSelectorLabelKey]
					if !ok {
						return false
					}
					Expect(sliceLabel).To(Equal(slice.Name))
					return true
				}, timeout, interval).Should(BeTrue())
			})

		})

	})
})
