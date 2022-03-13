package spoke_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"bitbucket.org/realtimeai/kubeslice-operator/internal/manifest"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = FDescribe("EgressGatewayDeploy", func() {

	Context("With egress gateway not installed", func() {
		objects := []client.Object{
			&appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "green-istio-egressgateway",
					Namespace: "kubeslice-system",
				},
			},
		}

		AfterEach(func() {
			for _, o := range objects {
				Expect(k8sClient.Delete(ctx, o)).Should(Succeed())
			}
		})

		It("Should install istio egress gateway deployment in the slice", func() {
			err := manifest.InstallEgress(ctx, k8sClient, "green")
			Expect(err).NotTo(HaveOccurred())

			// Check if deployment is there in the cluster
			deployKey := types.NamespacedName{Name: "green-istio-egressgateway", Namespace: "kubeslice-system"}
			createdDeploy := &appsv1.Deployment{}

			// Wait until deployment is created properly
			Eventually(func() bool {
				err := k8sClient.Get(ctx, deployKey, createdDeploy)
				if err != nil {
					return false
				}
				return true
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			Expect(createdDeploy.ObjectMeta.Name).To(Equal("green-istio-egressgateway"))

			labels := createdDeploy.ObjectMeta.Labels
			Expect(labels["slice"]).To(Equal("green"))

			ann := createdDeploy.ObjectMeta.Annotations
			Expect(ann["avesha.io/slice"]).To(Equal("green"))

		})

	})

})
