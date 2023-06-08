package vpnkeyrotation

import (
	"context"
	"time"

	hubv1alpha1 "github.com/kubeslice/apis/pkg/controller/v1alpha1"
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	nsmv1 "github.com/networkservicemesh/sdk-k8s/pkg/tools/k8s/apis/networkservicemesh.io/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var (
	gws = []string{"fire-worker-1-worker-2", "fire-worker-1-worker-3", "fire-worker-1-worker-4"}
)

var _ = Describe("Hub VPN Key Rotation", func() {
	Context("With Rotation Created at hub cluster", func() {
		var vpnKeyRotation *hubv1alpha1.VpnKeyRotation
		BeforeEach(func() {

			vpnKeyRotation = &hubv1alpha1.VpnKeyRotation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vpn-rotation",
					Namespace: PROJECT_NS,
				},
				Spec: hubv1alpha1.VpnKeyRotationSpec{
					SliceName: "fire",
					ClusterGatewayMapping: map[string][]string{
						CLUSTER_NAME: gws,
					},
				},
			}
			// Cleanup after each test
			DeferCleanup(func() {
				ctx := context.Background()
				Expect(k8sClient.Delete(ctx, vpnKeyRotation)).Should(Succeed())
			})
		})

		It("current rotation state should not be empty", func() {
			Expect(k8sClient.Create(ctx, vpnKeyRotation)).Should(Succeed())
			//get the rotation object
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      vpnKeyRotation.Name,
					Namespace: PROJECT_NS},
					vpnKeyRotation)
				if err != nil {
					return false
				}
				return len(vpnKeyRotation.Status.CurrentRotationState) != 0
			}, time.Second*60, time.Millisecond*500).Should(BeTrue())
		})

		It("current rotation state should have all gws status populated", func() {
			Expect(k8sClient.Create(ctx, vpnKeyRotation)).Should(Succeed())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      vpnKeyRotation.Name,
					Namespace: PROJECT_NS},
					vpnKeyRotation)
				if err != nil {
					return false
				}
				for _, gw := range gws {
					_, ok := vpnKeyRotation.Status.CurrentRotationState[gw]
					if !ok {
						return false
					}
				}
				return true
			}, time.Second*60, time.Millisecond*500).Should(BeTrue())
		})
	})

	Context("When VPN Key Rotation Interval Reached", func() {
		var vpnKeyRotation *hubv1alpha1.VpnKeyRotation
		var hubSecretGw1 *corev1.Secret
		var hubSecretGw2 *corev1.Secret
		var hubSecretGw3 *corev1.Secret
		var svc *corev1.Service
		var slice *kubeslicev1beta1.Slice
		var vl3ServiceEndpoint *nsmv1.NetworkServiceEndpoint
		var sliceGw *kubeslicev1beta1.SliceGateway
		var createdSlice *kubeslicev1beta1.Slice
		var createdSliceGw *kubeslicev1beta1.SliceGateway

		BeforeEach(func() {
			svc = &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kubeslice-dns",
					Namespace: "kubeslice-system",
				},
				Spec: corev1.ServiceSpec{
					ClusterIP: "10.0.0.20",
					Ports: []corev1.ServicePort{{
						Port: 52,
					}},
				},
			}
			sliceGw = &kubeslicev1beta1.SliceGateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      gws[0],
					Namespace: CONTROL_PLANE_NS,
				},
				Spec: kubeslicev1beta1.SliceGatewaySpec{
					SliceName: "fire",
				},
			}

			slice = &kubeslicev1beta1.Slice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "fire",
					Namespace: CONTROL_PLANE_NS,
				},
				Spec: kubeslicev1beta1.SliceSpec{},
			}

			vpnKeyRotation = &hubv1alpha1.VpnKeyRotation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vpn-rotation",
					Namespace: PROJECT_NS,
				},
				Spec: hubv1alpha1.VpnKeyRotationSpec{
					SliceName: "fire",
					ClusterGatewayMapping: map[string][]string{
						CLUSTER_NAME: gws,
					},
					CertificateCreationTime: metav1.Time{Time: time.Now()},
					CertificateExpiryTime:   metav1.Time{Time: time.Now().AddDate(0, 0, 30)},
					RotationInterval:        30,
				},
			}

			vl3ServiceEndpoint = &nsmv1.NetworkServiceEndpoint{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "vl3-nse-" + "fire",
					Namespace: "kubeslice-system",
					Labels: map[string]string{
						"app":                "vl3-nse-" + "fire",
						"networkservicename": "vl3-service-" + "fire",
					},
				},
				Spec: nsmv1.NetworkServiceEndpointSpec{
					Name: "vl3-service-" + "fire",
					NetworkServiceNames: []string{
						"vl3-service-" + "fire",
					},
				},
			}
			hubSecretGw1 = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: gws[0] + "-0", Namespace: PROJECT_NS},
				Data:       make(map[string][]byte),
			}
			hubSecretGw2 = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: gws[1] + "-0", Namespace: PROJECT_NS},
				Data:       make(map[string][]byte),
			}
			hubSecretGw3 = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: gws[2] + "-0", Namespace: PROJECT_NS},
				Data:       make(map[string][]byte),
			}

			createdSlice = &kubeslicev1beta1.Slice{}
			createdSliceGw = &kubeslicev1beta1.SliceGateway{}

			// Cleanup after each test
			DeferCleanup(func() {
				ctx := context.Background()
				Expect(k8sClient.Delete(ctx, vpnKeyRotation)).Should(Succeed())
			})
		})

		It("current rotation state should not be empty", func() {
			Expect(k8sClient.Create(ctx, hubSecretGw1)).Should(Succeed())
			Expect(k8sClient.Create(ctx, hubSecretGw2)).Should(Succeed())
			Expect(k8sClient.Create(ctx, hubSecretGw3)).Should(Succeed())

			Expect(k8sClient.Create(ctx, vpnKeyRotation)).Should(Succeed())

			Eventually(func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      vpnKeyRotation.Name,
					Namespace: PROJECT_NS},
					vpnKeyRotation)

				if err != nil {
					return err
				}
				return nil
			}, time.Second*60, time.Millisecond*500).Should(BeNil())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      vpnKeyRotation.Name,
					Namespace: PROJECT_NS},
					vpnKeyRotation)
				if err != nil {
					return false
				}
				return len(vpnKeyRotation.Status.CurrentRotationState) != 0
			}, time.Second*60, time.Millisecond*500).Should(BeTrue())

			vpnKeyRotation.Spec.CertificateCreationTime = metav1.Time{Time: time.Now().AddDate(0, 0, 30)}
			vpnKeyRotation.Spec.CertificateExpiryTime = metav1.Time{Time: time.Now().AddDate(0, 0, 30)}
			Eventually(func() bool {
				err := k8sClient.Update(ctx, vpnKeyRotation)
				return err == nil
			}, time.Second*30, time.Millisecond*250).Should(BeTrue())

			//get the rotation object
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      vpnKeyRotation.Name,
					Namespace: PROJECT_NS},
					vpnKeyRotation)
				if err != nil {
					return false
				}

				for _, gw := range gws {
					value, ok := vpnKeyRotation.Status.CurrentRotationState[gw]
					if !ok {
						return false
					}
					return value.Status == hubv1alpha1.SecretReadInProgress
				}
				return true
			}, time.Second*60, time.Millisecond*500).Should(BeTrue())
		})

		It("current rotation state should not be empty", func() {
			Expect(k8sClient.Create(ctx, svc)).Should(Succeed())
			Expect(k8sClient.Create(ctx, slice)).Should(Succeed())
			Expect(k8sClient.Create(ctx, vl3ServiceEndpoint)).Should(Succeed())
			Expect(k8sClient.Create(ctx, sliceGw)).Should(Succeed())

			sliceKey := types.NamespacedName{Name: slice.Name, Namespace: CONTROL_PLANE_NS}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, sliceKey, createdSlice)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: sliceGw.Name, Namespace: sliceGw.Namespace}, createdSliceGw)
				return err == nil
			}, time.Second*60, time.Millisecond*250).Should(BeTrue())

			// secrets for gateways
			createdSliceGw.Status.Config.SliceGatewayHostType = "Server"
			Eventually(func() bool {
				err := k8sClient.Status().Update(ctx, createdSliceGw)
				return err == nil
			}, time.Second*30, time.Millisecond*250).Should(BeTrue())

			Expect(k8sClient.Create(ctx, vpnKeyRotation)).Should(Succeed())

			Eventually(func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      vpnKeyRotation.Name,
					Namespace: PROJECT_NS},
					vpnKeyRotation)

				if err != nil {
					return err
				}
				return nil
			}, time.Second*60, time.Millisecond*500).Should(BeNil())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      vpnKeyRotation.Name,
					Namespace: PROJECT_NS},
					vpnKeyRotation)
				if err != nil {
					return false
				}
				return len(vpnKeyRotation.Status.CurrentRotationState) != 0
			}, time.Second*60, time.Millisecond*500).Should(BeTrue())

			vpnKeyRotation.Spec.CertificateCreationTime = metav1.Time{Time: time.Now().AddDate(0, 0, 30)}
			vpnKeyRotation.Spec.CertificateExpiryTime = metav1.Time{Time: time.Now().AddDate(0, 0, 30)}
			Eventually(func() bool {
				err := k8sClient.Update(ctx, vpnKeyRotation)
				return err == nil
			}, time.Second*30, time.Millisecond*250).Should(BeTrue())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      vpnKeyRotation.Name,
					Namespace: PROJECT_NS},
					vpnKeyRotation)
				if err != nil {
					return false
				}

				value, ok := vpnKeyRotation.Status.CurrentRotationState[gws[0]]
				if !ok {
					return false
				}
				return value.Status == hubv1alpha1.SecretUpdated
			}, time.Second*60, time.Millisecond*500).Should(BeTrue())

			// This is server then write test cases for server

		})
	})

})
