package vpnkeyrotation

import (
	"context"
	"strconv"
	"time"

	hubv1alpha1 "github.com/kubeslice/apis/pkg/controller/v1alpha1"
	spokev1alpha1 "github.com/kubeslice/apis/pkg/worker/v1alpha1"

	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	nsmv1 "github.com/networkservicemesh/sdk-k8s/pkg/tools/k8s/apis/networkservicemesh.io/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
)

var (
	gws = []string{
		"fire-worker-1-worker-2",
		"fire-worker-1-worker-3",
		"fire-worker-1-worker-4",
	}
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

	Context("When VPN Key Rotation Interval Reached, should update secrets", func() {
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
		var createSecretGw1 *corev1.Secret

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
					RotationCount:           2,
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
				ObjectMeta: metav1.ObjectMeta{Name: gws[0], Namespace: PROJECT_NS},
				Data:       make(map[string][]byte),
			}

			hubSecretGw2 = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: gws[1], Namespace: PROJECT_NS},
				Data:       make(map[string][]byte),
			}

			hubSecretGw3 = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: gws[2], Namespace: PROJECT_NS},
				Data:       make(map[string][]byte),
			}

			createdSlice = &kubeslicev1beta1.Slice{}
			createdSliceGw = &kubeslicev1beta1.SliceGateway{}
			createSecretGw1 = &corev1.Secret{}

			// Cleanup after each test
			DeferCleanup(func() {
				ctx := context.Background()
				Expect(k8sClient.Delete(ctx, vpnKeyRotation)).Should(Succeed())
				Expect(k8sClient.Delete(ctx, slice))
				Expect(k8sClient.Delete(ctx, sliceGw))
				Expect(k8sClient.Delete(ctx, hubSecretGw1)).Should(Succeed())
				Expect(k8sClient.Delete(ctx, hubSecretGw2)).Should(Succeed())
				Expect(k8sClient.Delete(ctx, hubSecretGw3)).Should(Succeed())

			})
		})

		It("recycler for a gw should be secret read state", func() {
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

		It("recycler for a gw should be secret updated state", func() {
			Expect(k8sClient.Create(ctx, hubSecretGw1)).Should(Succeed())
			Expect(k8sClient.Create(ctx, hubSecretGw2)).Should(Succeed())
			Expect(k8sClient.Create(ctx, hubSecretGw3)).Should(Succeed())
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

			createdSliceGw.Status.Config.SliceGatewayHostType = "Server"
			sliceGw.Status.Config.SliceGatewayRemoteGatewayID = "fire-worker-1-worker-3"

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
			vpnKeyRotation.Spec.CertificateExpiryTime = metav1.Time{Time: time.Now().AddDate(0, 0, 60)}
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
		})

		It("secret should be created with rotation count as suffix", func() {
			Expect(k8sClient.Create(ctx, hubSecretGw1)).Should(Succeed())
			Expect(k8sClient.Create(ctx, hubSecretGw2)).Should(Succeed())
			Expect(k8sClient.Create(ctx, hubSecretGw3)).Should(Succeed())
			Expect(k8sClient.Create(ctx, slice)).Should(Succeed())
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

			createdSliceGw.Status.Config.SliceGatewayHostType = "Server"
			sliceGw.Status.Config.SliceGatewayRemoteGatewayID = "fire-worker-1-worker-3"

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
			vpnKeyRotation.Spec.CertificateExpiryTime = metav1.Time{Time: time.Now().AddDate(0, 0, 60)}
			Eventually(func() bool {
				err := k8sClient.Update(ctx, vpnKeyRotation)
				return err == nil
			}, time.Second*30, time.Millisecond*250).Should(BeTrue())

			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      hubSecretGw1.Name + "-" + strconv.Itoa(vpnKeyRotation.Spec.RotationCount),
					Namespace: CONTROL_PLANE_NS},
					createSecretGw1)

			}, time.Second*60, time.Millisecond*500).Should(BeNil())
		})

	})

	Context("VPN Key Rotation Completion", func() {
		var vpnKeyRotation *hubv1alpha1.VpnKeyRotation
		var createdvpnKeyRotation *hubv1alpha1.VpnKeyRotation
		var workerSliceGWRecycler0 *spokev1alpha1.WorkerSliceGwRecycler
		var workerSliceGWRecycler1 *spokev1alpha1.WorkerSliceGwRecycler
		var workerSliceGWRecycler2 *spokev1alpha1.WorkerSliceGwRecycler

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
					CertificateCreationTime: metav1.Time{Time: time.Now()},
					CertificateExpiryTime:   metav1.Time{Time: time.Now().AddDate(0, 0, 30)},
					RotationInterval:        30,
				},
			}
			workerSliceGWRecycler0 = &spokev1alpha1.WorkerSliceGwRecycler{
				ObjectMeta: metav1.ObjectMeta{
					Name:      gws[0],
					Namespace: PROJECT_NS,
					Labels: map[string]string{
						"slicegw_name": gws[0],
					},
				},
				Spec: spokev1alpha1.WorkerSliceGwRecyclerSpec{
					State: "Error",
				},
			}
			workerSliceGWRecycler1 = &spokev1alpha1.WorkerSliceGwRecycler{
				ObjectMeta: metav1.ObjectMeta{
					Name:      gws[1],
					Namespace: PROJECT_NS,
					Labels: map[string]string{
						"slicegw_name": gws[1],
					},
				},
				Spec: spokev1alpha1.WorkerSliceGwRecyclerSpec{
					State: "Error",
				},
			}
			workerSliceGWRecycler2 = &spokev1alpha1.WorkerSliceGwRecycler{
				ObjectMeta: metav1.ObjectMeta{
					Name:      gws[2],
					Namespace: PROJECT_NS,
					Labels: map[string]string{
						"slicegw_name": gws[2],
					},
				},
				Spec: spokev1alpha1.WorkerSliceGwRecyclerSpec{
					State: "Error",
				},
			}
			createdvpnKeyRotation = &hubv1alpha1.VpnKeyRotation{}

			// Cleanup after each test
			DeferCleanup(func() {
				ctx := context.Background()
				Expect(k8sClient.Delete(ctx, vpnKeyRotation)).Should(Succeed())
			})
		})

		It("if no workerslicegwrecylcer - vpn should be completion state", func() {
			Expect(k8sClient.Create(ctx, vpnKeyRotation)).Should(Succeed())
			Eventually(func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      vpnKeyRotation.Name,
					Namespace: PROJECT_NS},
					createdvpnKeyRotation)

				if err != nil {
					return err
				}
				return nil
			}, time.Second*60, time.Millisecond*500).Should(BeNil())

			err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      vpnKeyRotation.Name,
					Namespace: PROJECT_NS,
				}, createdvpnKeyRotation)
				if err != nil {
					return err
				}
				createdvpnKeyRotation.Status.CurrentRotationState = map[string]hubv1alpha1.StatusOfKeyRotation{
					gws[0]: {
						Status:               hubv1alpha1.InProgress,
						LastUpdatedTimestamp: metav1.Time{Time: time.Now()},
					},
					gws[1]: {
						Status:               hubv1alpha1.InProgress,
						LastUpdatedTimestamp: metav1.Time{Time: time.Now()},
					},
					gws[2]: {
						Status:               hubv1alpha1.InProgress,
						LastUpdatedTimestamp: metav1.Time{Time: time.Now()},
					},
				}
				err = k8sClient.Status().Update(ctx, createdvpnKeyRotation)
				return err
			})
			Expect(err).ToNot(HaveOccurred())

			//get the rotation object
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      vpnKeyRotation.Name,
					Namespace: PROJECT_NS},
					createdvpnKeyRotation)
				if err != nil {
					return false
				}
				for _, gw := range gws {
					value, ok := createdvpnKeyRotation.Status.CurrentRotationState[gw]
					if !ok {
						return false
					}
					return value.Status == hubv1alpha1.Complete
				}
				return true
			}, time.Second*80, time.Millisecond*500).Should(BeTrue())
		})

		It("if workerslicegwrecylcer is present and in error - vpn should be error state", func() {
			Expect(k8sClient.Create(ctx, workerSliceGWRecycler0)).Should(Succeed())
			Expect(k8sClient.Create(ctx, workerSliceGWRecycler1)).Should(Succeed())
			Expect(k8sClient.Create(ctx, workerSliceGWRecycler2)).Should(Succeed())

			Expect(k8sClient.Create(ctx, vpnKeyRotation)).Should(Succeed())

			Eventually(func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      vpnKeyRotation.Name,
					Namespace: PROJECT_NS},
					createdvpnKeyRotation)

				if err != nil {
					return err
				}
				return nil
			}, time.Second*60, time.Millisecond*500).Should(BeNil())

			err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      vpnKeyRotation.Name,
					Namespace: PROJECT_NS,
				}, createdvpnKeyRotation)
				if err != nil {
					return err
				}
				createdvpnKeyRotation.Status.CurrentRotationState = map[string]hubv1alpha1.StatusOfKeyRotation{
					gws[0]: {
						Status:               hubv1alpha1.InProgress,
						LastUpdatedTimestamp: metav1.Time{Time: time.Now()},
					},
					gws[1]: {
						Status:               hubv1alpha1.InProgress,
						LastUpdatedTimestamp: metav1.Time{Time: time.Now()},
					},
					gws[2]: {
						Status:               hubv1alpha1.InProgress,
						LastUpdatedTimestamp: metav1.Time{Time: time.Now()},
					},
				}
				err = k8sClient.Status().Update(ctx, createdvpnKeyRotation)
				return err
			})
			Expect(err).ToNot(HaveOccurred())

			//get the rotation object
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      vpnKeyRotation.Name,
					Namespace: PROJECT_NS},
					createdvpnKeyRotation)
				if err != nil {
					return false
				}
				for _, gw := range gws {
					value, ok := createdvpnKeyRotation.Status.CurrentRotationState[gw]
					if !ok {
						return false
					}
					return value.Status == hubv1alpha1.Error
				}
				return true
			}, time.Second*60, time.Millisecond*500).Should(BeTrue())
		})
	})

	Context("cluster attach/detach test", func() {
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

		It("initial state", func() {
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
				return compareMapKeysAndArrayElements(vpnKeyRotation.Status.CurrentRotationState, gws)
			}, time.Second*60, time.Millisecond*500).Should(BeTrue())
		})

		It("cluster append", func() {
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
				return compareMapKeysAndArrayElements(vpnKeyRotation.Status.CurrentRotationState, gws)
			}, time.Second*60, time.Millisecond*500).Should(BeTrue())

			gws = append(gws, "fire-worker-1-worker-5")
			vpnKeyRotation.Spec.ClusterGatewayMapping = map[string][]string{
				CLUSTER_NAME: gws,
			}
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
				return compareMapKeysAndArrayElements(vpnKeyRotation.Status.CurrentRotationState, gws)
			}, time.Second*60, time.Millisecond*500).Should(BeTrue())
		})

		It("cluster delete", func() {
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
				return compareMapKeysAndArrayElements(vpnKeyRotation.Status.CurrentRotationState, gws)
			}, time.Second*60, time.Millisecond*500).Should(BeTrue())

			gws = gws[:len(gws)-1]
			vpnKeyRotation.Spec.ClusterGatewayMapping = map[string][]string{
				CLUSTER_NAME: gws,
			}
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
				return compareMapKeysAndArrayElements(vpnKeyRotation.Status.CurrentRotationState, gws)
			}, time.Second*60, time.Millisecond*500).Should(BeTrue())
		})
	})
})

func compareMapKeysAndArrayElements(myMap map[string]hubv1alpha1.StatusOfKeyRotation, array []string) bool {
	arrayElements := make(map[string]bool)
	for _, element := range array {
		arrayElements[element] = true
	}

	for key := range myMap {
		if !arrayElements[key] {
			return false
		}
	}

	for _, element := range array {
		if _, ok := myMap[element]; !ok {
			return false
		}
	}

	return len(myMap) == len(arrayElements)
}
