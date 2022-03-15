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
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("IstioGateways", func() {

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
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

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
			key := types.NamespacedName{Name: "istio-ingressgateway", Namespace: "kubeslice-system"}
			svc := &corev1.Service{}

			// Wait until deployment is created properly
			Eventually(func() bool {
				err := k8sClient.Get(ctx, key, svc)
				if err != nil {
					return false
				}
				return true
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			Expect(svc.ObjectMeta.Name).To(Equal("istio-ingressgateway"))

			// Check if role and rolebinding are there in the cluster
			rkey := types.NamespacedName{Name: "istio-ingressgateway-sds", Namespace: "kubeslice-system"}
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

			Expect(role.ObjectMeta.Name).To(Equal("istio-ingressgateway-sds"))
			Expect(rb.ObjectMeta.Name).To(Equal("istio-ingressgateway-sds"))

			// Check if sa there in the cluster
			skey := types.NamespacedName{Name: "istio-ingressgateway-service-account", Namespace: "kubeslice-system"}
			sa := &corev1.ServiceAccount{}

			// Wait until sa is created properly
			Eventually(func() bool {
				err := k8sClient.Get(ctx, skey, sa)
				if err != nil {
					return false
				}
				return true
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			Expect(sa.ObjectMeta.Name).To(Equal("istio-ingressgateway-service-account"))

		})

	})

	Context("With ingress gw installed", func() {

		var deploy *appsv1.Deployment

		BeforeEach(func() {

			deploy = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "istio-ingressgateway",
					Namespace: "kubeslice-system",
				},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"test": "test"},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{"test": "test"},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{{
								Name:  "nginx",
								Image: "nginx",
							}},
						},
					},
				},
			}

		})

		It("Should cleanup the resources", func() {

			err := k8sClient.Create(ctx, deploy)
			Expect(err).ToNot(HaveOccurred())

			// Check if deployment is created
			deployKey := types.NamespacedName{Name: "istio-ingressgateway", Namespace: "kubeslice-system"}
			deploy := &appsv1.Deployment{}

			// Wait until deployment is deleted properly
			Eventually(func() bool {
				err := k8sClient.Get(ctx, deployKey, deploy)
				if err != nil {
					return false
				}
				return true
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			err = manifest.UninstallIngress(ctx, k8sClient, "green")
			Expect(err).ToNot(HaveOccurred())

			// Wait until deployment is deleted properly
			Eventually(func() bool {
				err := k8sClient.Get(ctx, deployKey, deploy)
				if errors.IsNotFound(err) {
					return true
				}
				return false
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

		})

	})

})
