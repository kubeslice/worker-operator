/*
 *  Copyright (c) 2022 Avesha, Inc. All rights reserved.
 *
 *  SPDX-License-Identifier: Apache-2.0
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */

package spoke_test

import (
	"context"
	"fmt"
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	"github.com/kubeslice/worker-operator/controllers"
	slicegatewaycontroller "github.com/kubeslice/worker-operator/controllers/slicegateway"
	webhook "github.com/kubeslice/worker-operator/pkg/webhook/pod"
	nsmv1 "github.com/networkservicemesh/sdk-k8s/pkg/tools/k8s/apis/networkservicemesh.io/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"reflect"
	_ "sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
	"time"
)

var sliceGwFinalizer = []string{
	"networking.kubeslice.io/slicegw-finalizer"}

var _ = Describe("Worker SlicegwController", func() {

	var sliceGw *kubeslicev1beta1.SliceGateway
	var createdSliceGw *kubeslicev1beta1.SliceGateway
	var slice *kubeslicev1beta1.Slice
	var svc *corev1.Service
	var createdSlice *kubeslicev1beta1.Slice
	var vl3ServiceEndpoint *nsmv1.NetworkServiceEndpoint
	var appPod *corev1.Pod
	var podDisruptionBudget *policyv1.PodDisruptionBudget
	var createdPodDisruptionBudget *policyv1.PodDisruptionBudget

	Context("With SliceGW CR created", func() {

		BeforeEach(func() {
			sliceGw = &kubeslicev1beta1.SliceGateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slicegw",
					Namespace: CONTROL_PLANE_NS,
				},
				Spec: kubeslicev1beta1.SliceGatewaySpec{
					SliceName: "test-slice-4",
				},
			}

			slice = &kubeslicev1beta1.Slice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slice-4",
					Namespace: CONTROL_PLANE_NS,
				},
				Spec: kubeslicev1beta1.SliceSpec{},
			}
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
			labels := map[string]string{
				"kubeslice.io/slice":         "test-slice-4",
				"kubeslice.io/pod-type":      "slicegateway",
				"networkservicemesh.io/app":  "test-slicegw",
				"networkservicemesh.io/impl": "vl3-service-test-slice-4",
			}

			ann := map[string]string{
				"ns.networkservicemesh.io": "vl3-service-test-slice",
			}
			appPod = &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "nginx-pod",
					Namespace:   CONTROL_PLANE_NS,
					Labels:      labels,
					Annotations: ann,
				},
				Spec: corev1.PodSpec{

					Containers: []corev1.Container{
						{
							Image: "nginx",
							Name:  "nginx",
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 80,
								},
							},
						},
					},
				},
			}

			vl3ServiceEndpoint = &nsmv1.NetworkServiceEndpoint{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "vl3-nse-" + "test-slice-4",
					Namespace: "kubeslice-system",
					Labels: map[string]string{
						"app":                "vl3-nse-" + "test-slice-4",
						"networkservicename": "vl3-service-" + "test-slice-4",
					},
				},
				Spec: nsmv1.NetworkServiceEndpointSpec{
					Name: "vl3-service-" + "test-slice-4",
					NetworkServiceNames: []string{
						"vl3-service-" + "test-slice-4",
					},
				},
			}

			podDisruptionBudget = &policyv1.PodDisruptionBudget{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("%s-pdb", sliceGw.Name),
					Namespace: CONTROL_PLANE_NS,
					Labels: map[string]string{
						controllers.ApplicationNamespaceSelectorLabelKey: slice.Name,
						controllers.SliceGatewaySelectorLabelKey:         sliceGw.Name,
					},
				},
				Spec: policyv1.PodDisruptionBudgetSpec{
					MinAvailable: &slicegatewaycontroller.DefaultMinAvailablePodsInPDB,
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							controllers.ApplicationNamespaceSelectorLabelKey: slice.Name,
							webhook.PodInjectLabelKey:                        "slicegateway",
							"kubeslice.io/slicegw":                           sliceGw.Name,
						},
					},
				},
			}

			createdSlice = &kubeslicev1beta1.Slice{}
			createdSliceGw = &kubeslicev1beta1.SliceGateway{}
			createdPodDisruptionBudget = &policyv1.PodDisruptionBudget{}
			founddepl := &appsv1.Deployment{}
			deplKey1 := types.NamespacedName{Name: "test-slicegw-0-0", Namespace: CONTROL_PLANE_NS}
			deplKey2 := types.NamespacedName{Name: "test-slicegw-1-0", Namespace: CONTROL_PLANE_NS}

			DeferCleanup(func() {
				ctx := context.Background()
				Expect(k8sClient.Delete(ctx, slice)).Should(Succeed())
				Eventually(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Name: slice.Name, Namespace: slice.Namespace}, createdSlice)
					return errors.IsNotFound(err)
				}, time.Second*30, time.Millisecond*250).Should(BeTrue())
				Expect(k8sClient.Delete(ctx, sliceGw)).Should(Succeed())
				Eventually(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Name: sliceGw.Name, Namespace: sliceGw.Namespace}, createdSliceGw)
					return errors.IsNotFound(err)
				}, time.Second*30, time.Millisecond*250).Should(BeTrue())
				Expect(k8sClient.Delete(ctx, vl3ServiceEndpoint)).Should(Succeed())
				Eventually(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Name: sliceGw.Name, Namespace: sliceGw.Namespace}, appPod)
					return errors.IsNotFound(err)
				}, time.Second*40, time.Millisecond*250).Should(BeTrue())
				Expect(k8sClient.Delete(ctx, appPod)).Should(Succeed())
				Eventually(func() bool {
					err := k8sClient.Get(ctx, deplKey1, founddepl)
					if err != nil {
						return errors.IsNotFound(err)
					}
					Expect(k8sClient.Delete(ctx, founddepl)).Should(Succeed())
					err = k8sClient.Get(ctx, deplKey2, founddepl)
					if err != nil {
						return errors.IsNotFound(err)
					}
					Expect(k8sClient.Delete(ctx, founddepl)).Should(Succeed())
					return true
				}, time.Second*50, time.Millisecond*250).Should(BeTrue())
				Expect(k8sClient.Delete(ctx, svc)).Should(Succeed())
				Eventually(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Name: svc.Name, Namespace: svc.Namespace}, svc)
					return errors.IsNotFound(err)
				}, time.Second*30, time.Millisecond*250).Should(BeTrue())

				Eventually(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      podDisruptionBudget.Name,
						Namespace: podDisruptionBudget.Namespace},
						createdPodDisruptionBudget,
					)
					if err != nil {
						return errors.IsNotFound(err)
					}
					Expect(k8sClient.Delete(ctx, createdPodDisruptionBudget)).Should(Succeed())
					return true
				}, time.Second*30, time.Millisecond*250).Should(BeTrue())
			})
		})

		It("should create a gw nodeport service if gw type is Server", func() {
			ctx := context.Background()
			Expect(k8sClient.Create(ctx, svc)).Should(Succeed())
			Expect(k8sClient.Create(ctx, slice)).Should(Succeed())
			Expect(k8sClient.Create(ctx, vl3ServiceEndpoint)).Should(Succeed())
			Expect(k8sClient.Create(ctx, sliceGw)).Should(Succeed())
			Expect(k8sClient.Create(ctx, appPod)).Should(Succeed())

			sliceKey := types.NamespacedName{Name: "test-slice-4", Namespace: CONTROL_PLANE_NS}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, sliceKey, createdSlice)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				err := k8sClient.Get(ctx, sliceKey, createdSlice)
				if err != nil {
					return err
				}
				// Update the minimum required values in the slice cr status field
				if createdSlice.Status.SliceConfig == nil {
					createdSlice.Status.SliceConfig = &kubeslicev1beta1.SliceConfig{
						SliceDisplayName: slice.Name,
						SliceSubnet:      "192.168.0.0/16",
					}
				}
				if err := k8sClient.Status().Update(ctx, createdSlice); err != nil {
					return err
				}
				return nil
			})
			Expect(err).To(BeNil())
			Expect(createdSlice.Status.SliceConfig).NotTo(BeNil())

			slicegwkey := types.NamespacedName{Name: "test-slicegw", Namespace: CONTROL_PLANE_NS}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, slicegwkey, createdSliceGw)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			createdSliceGw.Status.Config.SliceGatewayHostType = "Server"
			Eventually(func() bool {
				err := k8sClient.Status().Update(ctx, createdSliceGw)
				return err == nil
			}, time.Second*30, time.Millisecond*250).Should(BeTrue())

			foundsvc := &corev1.Service{}
			svckey := types.NamespacedName{Name: "svc-test-slicegw-0-0", Namespace: CONTROL_PLANE_NS}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, svckey, foundsvc)
				return err == nil
			}, time.Second*30, time.Millisecond*250).Should(BeTrue())

			foundsvc = &corev1.Service{}
			svckey = types.NamespacedName{Name: "svc-test-slicegw-1-0", Namespace: CONTROL_PLANE_NS}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, svckey, foundsvc)
				return err == nil
			}, time.Second*30, time.Millisecond*250).Should(BeTrue())

		})

		It("Should create a deployment for gw server", func() {
			ctx := context.Background()
			Expect(k8sClient.Create(ctx, svc)).Should(Succeed())
			Expect(k8sClient.Create(ctx, slice)).Should(Succeed())
			Expect(k8sClient.Create(ctx, vl3ServiceEndpoint)).Should(Succeed())
			Expect(k8sClient.Create(ctx, sliceGw)).Should(Succeed())
			Expect(k8sClient.Create(ctx, appPod)).Should(Succeed())

			sliceKey := types.NamespacedName{Name: "test-slice-4", Namespace: CONTROL_PLANE_NS}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, sliceKey, createdSlice)
				return err == nil
			}, time.Second*250, time.Millisecond*250).Should(BeTrue())

			err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				err := k8sClient.Get(ctx, sliceKey, createdSlice)
				if err != nil {
					return err
				}
				// Update the minimum required values in the slice cr status field
				if createdSlice.Status.SliceConfig == nil {
					createdSlice.Status.SliceConfig = &kubeslicev1beta1.SliceConfig{
						SliceDisplayName: slice.Name,
						SliceSubnet:      "192.168.0.0/16",
					}
				}
				if err := k8sClient.Status().Update(ctx, createdSlice); err != nil {
					return err
				}
				return nil
			})
			Expect(err).To(BeNil())
			Expect(createdSlice.Status.SliceConfig).NotTo(BeNil())

			slicegwkey := types.NamespacedName{Name: "test-slicegw", Namespace: CONTROL_PLANE_NS}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, slicegwkey, createdSliceGw)
				return err == nil
			}, time.Second*250, time.Millisecond*250).Should(BeTrue())

			createdSliceGw.Status.Config.SliceGatewayHostType = "Server"
			Eventually(func() bool {
				err := k8sClient.Status().Update(ctx, createdSliceGw)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			foundsvc := &corev1.Service{}
			svckey := types.NamespacedName{Name: "svc-test-slicegw-0-0", Namespace: CONTROL_PLANE_NS}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, svckey, foundsvc)
				return err == nil
			}, time.Second*30, time.Millisecond*250).Should(BeTrue())

			foundsvc = &corev1.Service{}
			svckey = types.NamespacedName{Name: "svc-test-slicegw-1-0", Namespace: CONTROL_PLANE_NS}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, svckey, foundsvc)
				return err == nil
			}, time.Second*30, time.Millisecond*250).Should(BeTrue())

			founddepl := &appsv1.Deployment{}
			deplKey := types.NamespacedName{Name: "test-slicegw-0-0", Namespace: CONTROL_PLANE_NS}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, deplKey, founddepl)
				return err == nil
			}, time.Second*40, time.Millisecond*250).Should(BeTrue())

			founddepl = &appsv1.Deployment{}
			deplKey = types.NamespacedName{Name: "test-slicegw-1-0", Namespace: CONTROL_PLANE_NS}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, deplKey, founddepl)
				return err == nil
			}, time.Second*40, time.Millisecond*250).Should(BeTrue())

		})

		It("Should create a PodDisruptionBudget for gateway server's pods", func() {
			ctx := context.Background()
			Expect(k8sClient.Create(ctx, svc)).Should(Succeed())
			Expect(k8sClient.Create(ctx, slice)).Should(Succeed())
			Expect(k8sClient.Create(ctx, vl3ServiceEndpoint)).Should(Succeed())
			Expect(k8sClient.Create(ctx, sliceGw)).Should(Succeed())
			Expect(k8sClient.Create(ctx, appPod)).Should(Succeed())

			sliceKey := types.NamespacedName{Name: "test-slice-4", Namespace: CONTROL_PLANE_NS}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, sliceKey, createdSlice)
				return err == nil
			}, time.Second*250, time.Millisecond*250).Should(BeTrue())

			err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				err := k8sClient.Get(ctx, sliceKey, createdSlice)
				if err != nil {
					return err
				}
				// Update the minimum required values in the slice cr status field
				if createdSlice.Status.SliceConfig == nil {
					createdSlice.Status.SliceConfig = &kubeslicev1beta1.SliceConfig{
						SliceDisplayName: slice.Name,
						SliceSubnet:      "192.168.0.0/16",
					}
				}
				if err := k8sClient.Status().Update(ctx, createdSlice); err != nil {
					return err
				}
				return nil
			})
			Expect(err).To(BeNil())
			Expect(createdSlice.Status.SliceConfig).NotTo(BeNil())

			slicegwkey := types.NamespacedName{Name: "test-slicegw", Namespace: CONTROL_PLANE_NS}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, slicegwkey, createdSliceGw)
				return err == nil
			}, time.Second*250, time.Millisecond*250).Should(BeTrue())

			createdSliceGw.Status.Config.SliceGatewayHostType = "Server"
			Eventually(func() bool {
				err := k8sClient.Status().Update(ctx, createdSliceGw)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			foundsvc := &corev1.Service{}
			svckey := types.NamespacedName{Name: "svc-test-slicegw-0-0", Namespace: CONTROL_PLANE_NS}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, svckey, foundsvc)
				return err == nil
			}, time.Second*30, time.Millisecond*250).Should(BeTrue())

			foundsvc = &corev1.Service{}
			svckey = types.NamespacedName{Name: "svc-test-slicegw-1-0", Namespace: CONTROL_PLANE_NS}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, svckey, foundsvc)
				return err == nil
			}, time.Second*30, time.Millisecond*250).Should(BeTrue())

			founddepl := &appsv1.Deployment{}
			deplKey := types.NamespacedName{Name: "test-slicegw-0-0", Namespace: CONTROL_PLANE_NS}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, deplKey, founddepl)
				return err == nil
			}, time.Second*40, time.Millisecond*250).Should(BeTrue())

			founddepl = &appsv1.Deployment{}
			deplKey = types.NamespacedName{Name: "test-slicegw-1-0", Namespace: CONTROL_PLANE_NS}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, deplKey, founddepl)
				return err == nil
			}, time.Second*40, time.Millisecond*250).Should(BeTrue())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      podDisruptionBudget.Name,
					Namespace: podDisruptionBudget.Namespace,
				}, createdPodDisruptionBudget)

				return err == nil
			}, time.Second*40, time.Millisecond*250).Should(BeTrue())
		})

		It("Should create a finalizer for the slicegw cr created", func() {
			ctx := context.Background()

			Expect(k8sClient.Create(ctx, svc)).Should(Succeed())
			Eventually(func() bool {
				err := k8sClient.Create(ctx, slice)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			sliceKey := types.NamespacedName{Name: "test-slice-4", Namespace: CONTROL_PLANE_NS}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, sliceKey, createdSlice)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			//create vl3 endpoint
			Eventually(func() bool {
				err := k8sClient.Create(ctx, vl3ServiceEndpoint)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			//create slicegw
			Eventually(func() bool {
				err := k8sClient.Create(ctx, sliceGw)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			slicegwkey := types.NamespacedName{Name: "test-slicegw", Namespace: CONTROL_PLANE_NS}
			Eventually(func() bool {

				err := k8sClient.Get(ctx, slicegwkey, createdSliceGw)
				if err != nil {
					return false
				}
				return reflect.DeepEqual(createdSliceGw.GetFinalizers(), sliceGwFinalizer)
			}, time.Second*30, time.Millisecond*250).Should(BeTrue())

			Eventually(func() bool {
				err := k8sClient.Create(ctx, appPod)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())
		})

		It("Should create a deployment for gw client", func() {
			ctx := context.Background()
			Expect(k8sClient.Create(ctx, svc)).Should(Succeed())
			Expect(k8sClient.Create(ctx, slice)).Should(Succeed())
			Expect(k8sClient.Create(ctx, vl3ServiceEndpoint)).Should(Succeed())
			Expect(k8sClient.Create(ctx, sliceGw)).Should(Succeed())
			Expect(k8sClient.Create(ctx, appPod)).Should(Succeed())

			sliceKey := types.NamespacedName{Name: "test-slice-4", Namespace: CONTROL_PLANE_NS}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, sliceKey, createdSlice)
				return err == nil
			}, time.Second*250, time.Millisecond*250).Should(BeTrue())

			err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				err := k8sClient.Get(ctx, sliceKey, createdSlice)
				if err != nil {
					return err
				}
				// Update the minimum required values in the slice cr status field
				if createdSlice.Status.SliceConfig == nil {
					createdSlice.Status.SliceConfig = &kubeslicev1beta1.SliceConfig{
						SliceDisplayName: slice.Name,
						SliceSubnet:      "192.168.0.0/16",
					}
				}
				if err := k8sClient.Status().Update(ctx, createdSlice); err != nil {
					return err
				}
				return nil
			})
			Expect(err).To(BeNil())
			Expect(createdSlice.Status.SliceConfig).NotTo(BeNil())

			slicegwkey := types.NamespacedName{Name: "test-slicegw", Namespace: CONTROL_PLANE_NS}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, slicegwkey, createdSliceGw)
				return err == nil
			}, time.Second*250, time.Millisecond*250).Should(BeTrue())

			createdSliceGw.Status.Config.SliceGatewayHostType = "Client"
			createdSliceGw.Status.Config.SliceGatewayRemoteGatewayID = "remote-gateway-id"
			createdSliceGw.Status.Config.SliceGatewayRemoteNodeIPs = []string{"192.168.1.1"}
			createdSliceGw.Status.Config.SliceGatewayRemoteNodePorts = []int{8080, 8090}

			Eventually(func() bool {
				err := k8sClient.Status().Update(ctx, createdSliceGw)
				return err == nil
			}, time.Second*30, time.Millisecond*250).Should(BeTrue())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, slicegwkey, createdSliceGw)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			founddepl := &appsv1.Deployment{}
			deplKey := types.NamespacedName{Name: "test-slicegw-0-0", Namespace: CONTROL_PLANE_NS}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, deplKey, founddepl)
				return err == nil
			}, time.Second*40, time.Millisecond*250).Should(BeTrue())

			founddepl = &appsv1.Deployment{}
			deplKey = types.NamespacedName{Name: "test-slicegw-1-0", Namespace: CONTROL_PLANE_NS}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, deplKey, founddepl)
				return err == nil
			}, time.Second*40, time.Millisecond*250).Should(BeTrue())

		})

		It("Should create a PodDisruptionBudget for gateway client's pods", func() {
			ctx := context.Background()
			Expect(k8sClient.Create(ctx, svc)).Should(Succeed())
			Expect(k8sClient.Create(ctx, slice)).Should(Succeed())
			Expect(k8sClient.Create(ctx, vl3ServiceEndpoint)).Should(Succeed())
			Expect(k8sClient.Create(ctx, sliceGw)).Should(Succeed())
			Expect(k8sClient.Create(ctx, appPod)).Should(Succeed())

			sliceKey := types.NamespacedName{Name: "test-slice-4", Namespace: CONTROL_PLANE_NS}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, sliceKey, createdSlice)
				return err == nil
			}, time.Second*250, time.Millisecond*250).Should(BeTrue())

			err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				err := k8sClient.Get(ctx, sliceKey, createdSlice)
				if err != nil {
					return err
				}
				// Update the minimum required values in the slice cr status field
				if createdSlice.Status.SliceConfig == nil {
					createdSlice.Status.SliceConfig = &kubeslicev1beta1.SliceConfig{
						SliceDisplayName: slice.Name,
						SliceSubnet:      "192.168.0.0/16",
					}
				}
				if err := k8sClient.Status().Update(ctx, createdSlice); err != nil {
					return err
				}
				return nil
			})
			Expect(err).To(BeNil())
			Expect(createdSlice.Status.SliceConfig).NotTo(BeNil())

			slicegwkey := types.NamespacedName{Name: "test-slicegw", Namespace: CONTROL_PLANE_NS}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, slicegwkey, createdSliceGw)
				return err == nil
			}, time.Second*250, time.Millisecond*250).Should(BeTrue())

			createdSliceGw.Status.Config.SliceGatewayHostType = "Client"
			createdSliceGw.Status.Config.SliceGatewayRemoteGatewayID = "remote-gateway-id"
			createdSliceGw.Status.Config.SliceGatewayRemoteNodeIPs = []string{"192.168.1.1"}
			createdSliceGw.Status.Config.SliceGatewayRemoteNodePorts = []int{8080, 8090}

			Eventually(func() bool {
				err := k8sClient.Status().Update(ctx, createdSliceGw)
				return err == nil
			}, time.Second*30, time.Millisecond*250).Should(BeTrue())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, slicegwkey, createdSliceGw)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			founddepl := &appsv1.Deployment{}
			deplKey := types.NamespacedName{Name: "test-slicegw-0-0", Namespace: CONTROL_PLANE_NS}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, deplKey, founddepl)
				return err == nil
			}, time.Second*40, time.Millisecond*250).Should(BeTrue())

			founddepl = &appsv1.Deployment{}
			deplKey = types.NamespacedName{Name: "test-slicegw-1-0", Namespace: CONTROL_PLANE_NS}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, deplKey, founddepl)
				return err == nil
			}, time.Second*40, time.Millisecond*250).Should(BeTrue())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      podDisruptionBudget.Name,
					Namespace: podDisruptionBudget.Namespace,
				}, createdPodDisruptionBudget)

				return err == nil
			}, time.Second*40, time.Millisecond*250).Should(BeTrue())
		})

		It("Should create create headless service for gw client", func() {
			ctx := context.Background()
			Expect(k8sClient.Create(ctx, svc)).Should(Succeed())
			Expect(k8sClient.Create(ctx, slice)).Should(Succeed())
			Expect(k8sClient.Create(ctx, vl3ServiceEndpoint)).Should(Succeed())
			Expect(k8sClient.Create(ctx, sliceGw)).Should(Succeed())
			Expect(k8sClient.Create(ctx, appPod)).Should(Succeed())

			sliceKey := types.NamespacedName{Name: "test-slice-4", Namespace: CONTROL_PLANE_NS}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, sliceKey, createdSlice)
				return err == nil
			}, time.Second*250, time.Millisecond*250).Should(BeTrue())

			err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				err := k8sClient.Get(ctx, sliceKey, createdSlice)
				if err != nil {
					return err
				}
				// Update the minimum required values in the slice cr status field
				if createdSlice.Status.SliceConfig == nil {
					createdSlice.Status.SliceConfig = &kubeslicev1beta1.SliceConfig{
						SliceDisplayName: slice.Name,
						SliceSubnet:      "192.168.0.0/16",
					}
				}
				if err := k8sClient.Status().Update(ctx, createdSlice); err != nil {
					return err
				}
				fmt.Println(createdSlice.Status.SliceConfig, "=============================")
				return nil
			})
			Expect(err).To(BeNil())
			fmt.Println(createdSlice.Status.SliceConfig, "=============================")

			slicegwkey := types.NamespacedName{Name: "test-slicegw", Namespace: CONTROL_PLANE_NS}
			err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
				err := k8sClient.Get(ctx, slicegwkey, createdSliceGw)
				if err != nil {
					return err
				}
				createdSliceGw.Status.Config.SliceGatewayHostType = "Client"
				createdSliceGw.Status.Config.SliceGatewayRemoteGatewayID = "remote-gateway-id"
				createdSliceGw.Status.Config.SliceGatewayRemoteNodeIPs = []string{"192.168.1.1"}
				createdSliceGw.Status.Config.SliceGatewayRemoteNodePorts = []int{8080, 8090}

				err = k8sClient.Status().Update(ctx, createdSliceGw)
				if err != nil {
					return err
				}
				return nil
			})
			Expect(err).To(BeNil())

			svcKey := types.NamespacedName{Namespace: CONTROL_PLANE_NS, Name: "remote-gateway-id"}
			foundSvc := &corev1.Service{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, svcKey, foundSvc)
				if err != nil {
					return false
				}
				return foundSvc != nil
			}, time.Second*30, time.Millisecond*250).Should(BeTrue())
			Expect(foundSvc.Name).To(Equal(createdSliceGw.Status.Config.SliceGatewayRemoteGatewayID))
		})
		It("Should create create endpoint for gw client", func() {
			ctx := context.Background()
			Expect(k8sClient.Create(ctx, svc)).Should(Succeed())
			Expect(k8sClient.Create(ctx, slice)).Should(Succeed())
			Expect(k8sClient.Create(ctx, vl3ServiceEndpoint)).Should(Succeed())
			Expect(k8sClient.Create(ctx, sliceGw)).Should(Succeed())
			Expect(k8sClient.Create(ctx, appPod)).Should(Succeed())

			sliceKey := types.NamespacedName{Name: "test-slice-4", Namespace: CONTROL_PLANE_NS}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, sliceKey, createdSlice)
				return err == nil
			}, time.Second*250, time.Millisecond*250).Should(BeTrue())

			err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				err := k8sClient.Get(ctx, sliceKey, createdSlice)
				if err != nil {
					return err
				}
				// Update the minimum required values in the slice cr status field
				if createdSlice.Status.SliceConfig == nil {
					createdSlice.Status.SliceConfig = &kubeslicev1beta1.SliceConfig{
						SliceDisplayName: slice.Name,
						SliceSubnet:      "192.168.0.0/16",
					}
				}
				if err := k8sClient.Status().Update(ctx, createdSlice); err != nil {
					return err
				}
				return nil
			})
			Expect(err).To(BeNil())
			Expect(createdSlice.Status.SliceConfig).NotTo(BeNil())

			slicegwkey := types.NamespacedName{Name: "test-slicegw", Namespace: CONTROL_PLANE_NS}
			err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
				err := k8sClient.Get(ctx, slicegwkey, createdSliceGw)
				if err != nil {
					return err
				}
				createdSliceGw.Status.Config.SliceGatewayHostType = "Client"
				createdSliceGw.Status.Config.SliceGatewayRemoteGatewayID = "remote-gateway-id"
				createdSliceGw.Status.Config.SliceGatewayRemoteNodeIPs = []string{"192.168.1.1"}
				createdSliceGw.Status.Config.SliceGatewayRemoteNodePorts = []int{8080, 8090}

				err = k8sClient.Status().Update(ctx, createdSliceGw)
				if err != nil {
					return err
				}
				return nil
			})
			Expect(err).To(BeNil())

			endpointKey := types.NamespacedName{Name: createdSliceGw.Status.Config.SliceGatewayRemoteGatewayID, Namespace: CONTROL_PLANE_NS}
			endpointFound := &corev1.Endpoints{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, endpointKey, endpointFound)
				if err != nil {
					return false
				}
				return endpointFound != nil
			}, time.Second*30, time.Millisecond*250).Should(BeTrue())
			Expect(endpointFound.Name).To(Equal(createdSliceGw.Status.Config.SliceGatewayRemoteGatewayID))
			//ip should be same as remote node IP
			Expect(endpointFound.Subsets[0].Addresses[0].IP).To(Equal(createdSliceGw.Status.Config.SliceGatewayRemoteNodeIPs[0]))
		})
		It("Should restart the gw client deployment if there is mismatch in the nodePorts", func() {
			ctx := context.Background()
			Expect(k8sClient.Create(ctx, svc)).Should(Succeed())
			Expect(k8sClient.Create(ctx, slice)).Should(Succeed())
			Expect(k8sClient.Create(ctx, vl3ServiceEndpoint)).Should(Succeed())
			Expect(k8sClient.Create(ctx, sliceGw)).Should(Succeed())
			Expect(k8sClient.Create(ctx, appPod)).Should(Succeed())
			sliceKey := types.NamespacedName{Name: "test-slice-4", Namespace: CONTROL_PLANE_NS}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, sliceKey, createdSlice)
				return err == nil
			}, time.Second*250, time.Millisecond*250).Should(BeTrue())

			err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				err := k8sClient.Get(ctx, sliceKey, createdSlice)
				if err != nil {
					return err
				}
				// Update the minimum required values in the slice cr status field
				if createdSlice.Status.SliceConfig == nil {
					createdSlice.Status.SliceConfig = &kubeslicev1beta1.SliceConfig{
						SliceDisplayName: slice.Name,
						SliceSubnet:      "192.168.0.0/16",
					}
				}
				if err := k8sClient.Status().Update(ctx, createdSlice); err != nil {
					return err
				}
				return nil
			})
			Expect(err).To(BeNil())
			Expect(createdSlice.Status.SliceConfig).NotTo(BeNil())

			slicegwkey := types.NamespacedName{Name: "test-slicegw", Namespace: CONTROL_PLANE_NS}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, slicegwkey, createdSliceGw)
				return err == nil
			}, time.Second*250, time.Millisecond*250).Should(BeTrue())

			createdSliceGw.Status.Config.SliceGatewayHostType = "Client"
			createdSliceGw.Status.Config.SliceGatewayRemoteGatewayID = "remote-gateway-id"
			createdSliceGw.Status.Config.SliceGatewayRemoteNodeIPs = []string{"192.168.1.1"}
			createdSliceGw.Status.Config.SliceGatewayRemoteNodePorts = []int{8080, 8090}

			Eventually(func() bool {
				err := k8sClient.Status().Update(ctx, createdSliceGw)
				return err == nil
			}, time.Second*30, time.Millisecond*250).Should(BeTrue())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, slicegwkey, createdSliceGw)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			founddepl := &appsv1.Deployment{}
			deplKey := types.NamespacedName{Name: "test-slicegw-0-0", Namespace: CONTROL_PLANE_NS}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, deplKey, founddepl)
				return err == nil
			}, time.Second*40, time.Millisecond*250).Should(BeTrue())

			founddepl = &appsv1.Deployment{}
			deplKey = types.NamespacedName{Name: "test-slicegw-1-0", Namespace: CONTROL_PLANE_NS}

			createdSliceGw.Status.Config.SliceGatewayRemoteNodePorts = []int{6080, 7090}

			Eventually(func() bool {
				err := k8sClient.Status().Update(ctx, createdSliceGw)
				return err == nil
			}, time.Second*30, time.Millisecond*250).Should(BeTrue())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, deplKey, founddepl)
				return err == nil
			}, time.Second*40, time.Millisecond*250).Should(BeTrue())
			time.Sleep(time.Second * 30)
			var portFromDep int
			Eventually(func(portFromDep *int) []int {
				err := k8sClient.Get(ctx, deplKey, founddepl)
				if err != nil {
					return []int{}
				}
				cont := founddepl.Spec.Template.Spec.Containers[0]
				for _, key := range cont.Env {
					if key.Name == "NODE_PORT" {
						*portFromDep, err = strconv.Atoi(key.Value)
						if err != nil {
							fmt.Println("error converting string to int")
							return []int{}
						}

					}
				}
				return createdSliceGw.Status.Config.SliceGatewayRemoteNodePorts
			}(&portFromDep), time.Second*120, time.Millisecond*250).Should(ContainElement(portFromDep))
		})
	})
	Context("With SliceGw CR deleted", func() {

		BeforeEach(func() {
			sliceGw = &kubeslicev1beta1.SliceGateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slicegw-del",
					Namespace: CONTROL_PLANE_NS,
				},
				Spec: kubeslicev1beta1.SliceGatewaySpec{
					SliceName: "test-slice-del",
				},
			}

			slice = &kubeslicev1beta1.Slice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slice-del",
					Namespace: CONTROL_PLANE_NS,
				},
				Spec: kubeslicev1beta1.SliceSpec{},
			}

			vl3ServiceEndpoint = &nsmv1.NetworkServiceEndpoint{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "networkservicemesh.io/v1",
					Kind:       "NetworkServiceEndpoint",
				},
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "vl3-service-" + "test-slice-del",
					Namespace:    "kubeslice-system",
					Labels: map[string]string{
						"app":                "vl3-nse-" + "test-slice-del",
						"networkservicename": "vl3-service-" + "test-slice-del",
					},
				},
				Spec: nsmv1.NetworkServiceEndpointSpec{
					Name: "vl3-service-" + "test-slice-del",
					NetworkServiceNames: []string{
						"vl3-service-" + "test-slice-del",
					},
				},
			}
		})

		It("Should Delete All the dependent resources", func() {
			ctx := context.Background()
			Expect(k8sClient.Create(ctx, slice)).Should(Succeed())
			Expect(k8sClient.Create(ctx, vl3ServiceEndpoint)).Should(Succeed())
			Expect(k8sClient.Create(ctx, sliceGw)).Should(Succeed())
			// Expect(k8sClient.Create(ctx, appPod)).Should(Succeed())

			sliceKey := types.NamespacedName{Name: "test-slice-del", Namespace: CONTROL_PLANE_NS}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, sliceKey, createdSlice)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			slicegwkey := types.NamespacedName{Name: "test-slicegw-del", Namespace: CONTROL_PLANE_NS}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, slicegwkey, createdSliceGw)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			//Delete the sliceGw CR which will trigger the finalizer and cleanup resources
			Expect(k8sClient.Delete(ctx, sliceGw)).Should(Succeed())

			createdSecret := &corev1.Secret{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, slicegwkey, createdSecret)
				return errors.IsNotFound(err)
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			founddepl := &appsv1.Deployment{}
			deplKey := types.NamespacedName{Name: "test-slicegw-del-0-0", Namespace: CONTROL_PLANE_NS}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, deplKey, founddepl)
				return errors.IsNotFound(err)
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			createdSliceGw := &kubeslicev1beta1.SliceGateway{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, slicegwkey, createdSliceGw)
				return errors.IsNotFound(err)
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			createdPodDisruptionBudget = &policyv1.PodDisruptionBudget{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      podDisruptionBudget.Name,
					Namespace: podDisruptionBudget.Namespace,
				}, createdPodDisruptionBudget)
				return errors.IsNotFound(err)
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())
		})
	})
})
