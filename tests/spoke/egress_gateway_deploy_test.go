package spoke_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"bitbucket.org/realtimeai/kubeslice-operator/internal/manifest"
	istiov1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
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
			&corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "green-istio-egressgateway",
					Namespace: "kubeslice-system",
				},
			},
			&rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "green-istio-egressgateway-sds",
					Namespace: "kubeslice-system",
				},
			},
			&corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "green-istio-egressgateway-service-account",
					Namespace: "kubeslice-system",
				},
			},
			&rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "green-istio-egressgateway-sds",
					Namespace: "kubeslice-system",
				},
			},
			&istiov1beta1.Gateway{
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

		It("Should install istio egress gateway resources", func() {
			err := manifest.InstallEgress(ctx, k8sClient, "green")
			Expect(err).NotTo(HaveOccurred())

			// Check if service and gateway are there in the cluster
			key := types.NamespacedName{Name: "green-istio-egressgateway", Namespace: "kubeslice-system"}
			svc := &corev1.Service{}
			gw := &istiov1beta1.Gateway{}

			// Wait until service is created properly
			Eventually(func() bool {
				err := k8sClient.Get(ctx, key, svc)
				if err != nil {
					return false
				}
				return true
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			Expect(svc.ObjectMeta.Name).To(Equal("green-istio-egressgateway"))

			// Wait until Gateway is created properly
			Eventually(func() bool {
				err := k8sClient.Get(ctx, key, gw)
				if err != nil {
					return false
				}
				return true
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			Expect(gw.ObjectMeta.Name).To(Equal("green-istio-egressgateway"))

			// Check if role and rolebinding are there in the cluster
			rkey := types.NamespacedName{Name: "green-istio-egressgateway-sds", Namespace: "kubeslice-system"}
			role := &rbacv1.Role{}
			rb := &rbacv1.RoleBinding{}

			// Wait until role is created properly
			Eventually(func() bool {
				err := k8sClient.Get(ctx, rkey, role)
				if err != nil {
					return false
				}
				return true
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			// Wait until rolebinding is created properly
			Eventually(func() bool {
				err := k8sClient.Get(ctx, rkey, rb)
				if err != nil {
					return false
				}
				return true
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			Expect(role.ObjectMeta.Name).To(Equal("green-istio-egressgateway-sds"))
			Expect(rb.ObjectMeta.Name).To(Equal("green-istio-egressgateway-sds"))

			// Check if sa there in the cluster
			skey := types.NamespacedName{Name: "green-istio-egressgateway-service-account", Namespace: "kubeslice-system"}
			sa := &corev1.ServiceAccount{}

			// Wait until sa is created properly
			Eventually(func() bool {
				err := k8sClient.Get(ctx, skey, sa)
				if err != nil {
					return false
				}
				return true
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			Expect(sa.ObjectMeta.Name).To(Equal("green-istio-egressgateway-service-account"))

		})

	})

})
