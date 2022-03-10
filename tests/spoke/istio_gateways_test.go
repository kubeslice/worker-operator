package spoke_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"bitbucket.org/realtimeai/kubeslice-operator/internal/manifest"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = FDescribe("IstioGateways", func() {

	Context("With ingress not installed", func() {

		objects := []client.Object{
			&appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "istio-ingressgateway",
					Namespace: "kubeslice-system",
				},
			},
			&corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "istio-ingressgateway",
					Namespace: "kubeslice-system",
				},
			},
			&rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "istio-ingressgateway-sds",
					Namespace: "kubeslice-system",
				},
			},
			&corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "istio-ingressgateway-service-account",
					Namespace: "kubeslice-system",
				},
			},
			&rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "istio-ingressgateway-sds",
					Namespace: "kubeslice-system",
				},
			},
		}

		AfterEach(func() {
			for _, o := range objects {
				Expect(k8sClient.Delete(ctx, o)).Should(Succeed())
			}
		})

		It("Should install istio ingress gateway deployment", func() {
			err := manifest.InstallIngress(context.Background(), k8sClient, "green")
			Expect(err).NotTo(HaveOccurred())

			// Check if deployment is there in the cluster
			deployKey := types.NamespacedName{Name: "istio-ingressgateway", Namespace: "kubeslice-system"}
			createdDeploy := &appsv1.Deployment{}

			// Wait until deployment is created properly
			Eventually(func() bool {
				err := k8sClient.Get(ctx, deployKey, createdDeploy)
				if err != nil {
					return false
				}
				return true
			}, time.September*10, time.Millisecond*250).Should(BeTrue())

			Expect(createdDeploy.ObjectMeta.Name).To(Equal("istio-ingressgateway"))

			labels := createdDeploy.ObjectMeta.Labels
			Expect(labels["slice"]).To(Equal("green"))

			ann := createdDeploy.ObjectMeta.Annotations
			Expect(ann["avesha.io/slice"]).To(Equal("green"))

		})

		It("Should install istio ingress gateway resources", func() {
			err := manifest.InstallIngress(context.Background(), k8sClient, "green")
			Expect(err).NotTo(HaveOccurred())

			// Check if service is there in the cluster
			svcKey := types.NamespacedName{Name: "istio-ingressgateway", Namespace: "kubeslice-system"}
			svc := &corev1.Service{}

			// Wait until deployment is created properly
			Eventually(func() bool {
				err := k8sClient.Get(ctx, svcKey, svc)
				if err != nil {
					return false
				}
				return true
			}, time.September*10, time.Millisecond*250).Should(BeTrue())

			Expect(svc.ObjectMeta.Name).To(Equal("istio-ingressgateway"))

		})

	})

})
