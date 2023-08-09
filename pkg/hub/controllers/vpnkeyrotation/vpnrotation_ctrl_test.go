package vpnkeyrotation

import (
	"context"
	"strconv"
	"time"

	hubv1alpha1 "github.com/kubeslice/apis/pkg/controller/v1alpha1"
	spokev1alpha1 "github.com/kubeslice/apis/pkg/worker/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	ossEvents "github.com/kubeslice/worker-operator/events"
	nsmv1 "github.com/networkservicemesh/sdk-k8s/pkg/tools/k8s/apis/networkservicemesh.io/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
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

func createPod(client client.Client, name, gwName string) {
	labels := map[string]string{
		"kubeslice.io/slice-gw": gwName,
		"kubeslice.io/pod-type": "slicegateway",
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: CONTROL_PLANE_NS,
			Labels:    labels,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Image: "nginx",
					Name:  "gw",
					Ports: []corev1.ContainerPort{
						{
							ContainerPort: 80,
						},
					},
				},
			},
		},
	}

	// Create the pod using the fake client.
	Expect(k8sClient.Create(ctx, pod))
}

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
						CLUSTER_NAME: {gws[0]},
					},
					Clusters:                []string{"worker-1"},
					CertificateCreationTime: &metav1.Time{Time: time.Now()},
				},
			}
			for _, v := range gws {
				createPod(k8sClient, v+"-0", v)
				createPod(k8sClient, v+"-1", v)
			}

			// Cleanup after each test
			DeferCleanup(func() {
				ctx := context.Background()
				Expect(k8sClient.Delete(ctx, vpnKeyRotation)).Should(Succeed())
			})
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
				return len(vpnKeyRotation.Status.CurrentRotationState) != 0
			}, time.Second*120, time.Millisecond*500).Should(BeTrue())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      vpnKeyRotation.Name,
					Namespace: PROJECT_NS},
					vpnKeyRotation)
				if err != nil {
					return false
				}
				_, ok := vpnKeyRotation.Status.CurrentRotationState[gws[0]]
				if !ok {
					return false
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
		var createdvpnKeyRotation *hubv1alpha1.VpnKeyRotation

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
					CertificateCreationTime: &metav1.Time{Time: time.Now()},
					CertificateExpiryTime:   &metav1.Time{Time: time.Now().AddDate(0, 0, 30)},
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
			createdvpnKeyRotation = &hubv1alpha1.VpnKeyRotation{}

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

			vpnKeyRotation.Spec.CertificateCreationTime = &metav1.Time{Time: time.Now().AddDate(0, 0, 30)}
			vpnKeyRotation.Spec.CertificateExpiryTime = &metav1.Time{Time: time.Now().AddDate(0, 0, 60)}
			Eventually(func() bool {
				err := k8sClient.Update(ctx, vpnKeyRotation)
				return err == nil
			}, time.Second*30, time.Millisecond*250).Should(BeTrue())

			events := &corev1.EventList{}
			Eventually(func() bool {
				err := k8sClient.List(ctx, events, client.InNamespace("kubeslice-system"))
				return err == nil && len(events.Items) > 0 && eventFound(events, string(ossEvents.EventGatewayCertificateRecyclingTriggered))
			}, 30*time.Second, 1*time.Second).Should(BeTrue())

			Eventually(func() bool {
				err := k8sClient.List(ctx, events, client.InNamespace("kubeslice-system"))
				return err == nil && len(events.Items) > 0 && eventFound(events, string(ossEvents.EventGatewayCertificateUpdated))
			}, 30*time.Second, 1*time.Second).Should(BeTrue())

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

			vpnKeyRotation.Spec.CertificateCreationTime = &metav1.Time{Time: time.Now().AddDate(0, 0, 30)}
			vpnKeyRotation.Spec.CertificateExpiryTime = &metav1.Time{Time: time.Now().AddDate(0, 0, 60)}
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

		It("should delete previous certificates", func() {
			Expect(k8sClient.Create(ctx, vpnKeyRotation)).Should(Succeed())
			Expect(k8sClient.Create(ctx, slice)).Should(Succeed())
			Expect(k8sClient.Create(ctx, sliceGw)).Should(Succeed())
			Expect(k8sClient.Create(ctx, hubSecretGw1)).Should(Succeed())
			Expect(k8sClient.Create(ctx, hubSecretGw2)).Should(Succeed())
			Expect(k8sClient.Create(ctx, hubSecretGw3)).Should(Succeed())
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

			events := &corev1.EventList{}
			Eventually(func() bool {
				err := k8sClient.List(ctx, events, client.InNamespace("kubeslice-system"))
				return err == nil && len(events.Items) > 0 && eventFound(events, string(ossEvents.EventGatewayRecyclingSuccessful))
			}, 30*time.Second, 1*time.Second).Should(BeTrue())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      hubSecretGw1.Name + "-" + strconv.Itoa(vpnKeyRotation.Spec.RotationCount-1),
					Namespace: CONTROL_PLANE_NS},
					createSecretGw1)
				return errors.IsNotFound(err)
			}, time.Second*60, time.Millisecond*500).Should(BeTrue())
		})
	})

	Context("VPN Key Rotation Completion", func() {
		var vpnKeyRotation *hubv1alpha1.VpnKeyRotation
		var workerSliceGWRecycler0 *spokev1alpha1.WorkerSliceGwRecycler
		var createdWorkerSliceGWRecycler0 *spokev1alpha1.WorkerSliceGwRecycler

		var slice *kubeslicev1beta1.Slice
		var sliceGw *kubeslicev1beta1.SliceGateway
		var createdSlice *kubeslicev1beta1.Slice
		var createdSliceGw *kubeslicev1beta1.SliceGateway
		var createdvpnKeyRotation *hubv1alpha1.VpnKeyRotation
		BeforeEach(func() {
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
						CLUSTER_NAME: {gws[0]},
					},
					CertificateCreationTime: &metav1.Time{Time: time.Now()},
					CertificateExpiryTime:   &metav1.Time{Time: time.Now().AddDate(0, 0, 30)},
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
					State: "error",
				},
			}
			createdWorkerSliceGWRecycler0 = &spokev1alpha1.WorkerSliceGwRecycler{}
			createdSlice = &kubeslicev1beta1.Slice{}
			createdSliceGw = &kubeslicev1beta1.SliceGateway{}
			createdvpnKeyRotation = &hubv1alpha1.VpnKeyRotation{}

			// Cleanup after each test
			DeferCleanup(func() {
				ctx := context.Background()
				Expect(k8sClient.Delete(ctx, vpnKeyRotation)).Should(Succeed())
			})
		})

		It("if no workerslicegwrecylcer - vpn should be completion state", func() {
			Expect(k8sClient.Create(ctx, slice)).Should(Succeed())
			Expect(k8sClient.Create(ctx, sliceGw)).Should(Succeed())
			Expect(k8sClient.Create(ctx, vpnKeyRotation)).Should(Succeed())

			sliceKey := types.NamespacedName{Name: slice.Name, Namespace: CONTROL_PLANE_NS}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, sliceKey, createdSlice)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			sliceGwKey := types.NamespacedName{Name: sliceGw.Name, Namespace: CONTROL_PLANE_NS}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, sliceGwKey, createdSliceGw)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			createdSliceGw.Status.Config.SliceGatewayHostType = "Server"
			sliceGw.Status.Config.SliceGatewayRemoteGatewayID = "fire-worker-1-worker-3"

			Eventually(func() bool {
				err := k8sClient.Status().Update(ctx, createdSliceGw)
				return err == nil
			}, time.Second*30, time.Millisecond*250).Should(BeTrue())

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

			events := &corev1.EventList{}
			Eventually(func() bool {
				err := k8sClient.List(ctx, events, client.InNamespace("kubeslice-system"))
				return err == nil && len(events.Items) > 0 && eventFound(events, string(ossEvents.EventGatewayRecyclingSuccessful))
			}, 30*time.Second, 1*time.Second).Should(BeTrue())
		})

		It("if workerslicegwrecylcer is present and in error - vpn should be error state", func() {
			Expect(k8sClient.Create(ctx, workerSliceGWRecycler0)).Should(Succeed())
			Eventually(func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      workerSliceGWRecycler0.Name,
					Namespace: PROJECT_NS},
					createdWorkerSliceGWRecycler0)
				if err != nil {
					return err
				}
				return nil
			}, time.Second*60, time.Millisecond*500).Should(BeNil())

			Expect(k8sClient.Create(ctx, vpnKeyRotation)).Should(Succeed())

			sliceGwKey := types.NamespacedName{Name: sliceGw.Name, Namespace: CONTROL_PLANE_NS}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, sliceGwKey, createdSliceGw)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			createdSliceGw.Status.Config.SliceGatewayHostType = "Server"
			createdSliceGw.Status.Config.SliceGatewayRemoteGatewayID = "fire-worker-1-worker-3"

			Eventually(func() bool {
				err := k8sClient.Status().Update(ctx, createdSliceGw)
				return err == nil
			}, time.Second*30, time.Millisecond*250).Should(BeTrue())

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
				}
				err = k8sClient.Status().Update(ctx, createdvpnKeyRotation)
				return err
			})
			Expect(err).ToNot(HaveOccurred())

			events := &corev1.EventList{}
			Eventually(func() bool {
				err := k8sClient.List(ctx, events, client.InNamespace("kubeslice-system"))
				return err == nil && len(events.Items) > 0 && eventFound(events, string(ossEvents.EventGatewayRecyclingFailed))
			}, 30*time.Second, 1*time.Second).Should(BeTrue())

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
		var gws = []string{
			"fire-worker-1-worker-2",
			"fire-worker-1-worker-3",
			"fire-worker-3-worker-1",
			"fire-worker-2-worker-1",
			"fire-worker-2-worker-3",
			"fire-worker-1-worker-4",
		}
		BeforeEach(func() {
			vpnKeyRotation = &hubv1alpha1.VpnKeyRotation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vpn-rotation",
					Namespace: PROJECT_NS,
				},
				Spec: hubv1alpha1.VpnKeyRotationSpec{
					SliceName: "fire",
					ClusterGatewayMapping: map[string][]string{
						"worker-1": gws,
					},
					CertificateCreationTime: &metav1.Time{Time: time.Now()},
				},
			}

			for _, v := range gws {
				createPod(k8sClient, v+"-0", v)
				createPod(k8sClient, v+"-1", v)
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

		It("cluster attach", func() {
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

			newGw := "fire-worker-1-worker-5"
			gws = append(gws, newGw)
			createPod(k8sClient, newGw+"-0", newGw)
			createPod(k8sClient, newGw+"-1", newGw)

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

			Expect(keyShouldPresent("fire-worker-1-worker-5", vpnKeyRotation.Status.CurrentRotationState)).Should(BeTrue()) //attach

		})

		It("cluster detach", func() {
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

			Expect(keyShouldPresent("fire-worker-1-worker-5", vpnKeyRotation.Status.CurrentRotationState)).Should(BeFalse()) //detach
		})
	})
})

func keyShouldPresent(keyToCheck string, myMap map[string]hubv1alpha1.StatusOfKeyRotation) bool {
	for key := range myMap {
		if key == keyToCheck {
			return true
		}
	}
	return false
}

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

func eventFound(events *corev1.EventList, eventTitle string) bool {
	for _, event := range events.Items {
		if event.Labels["eventTitle"] == eventTitle {
			return true
		}
	}
	return false
}
