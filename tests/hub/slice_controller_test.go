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

package hub_test

import (
	"context"
	"reflect"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	workerv1alpha1 "github.com/kubeslice/apis/pkg/worker/v1alpha1"
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	"github.com/kubeslice/worker-operator/tests/utils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Hub SliceController", func() {

	Context("With Slice CR created in hub", func() {

		var hubSlice *workerv1alpha1.WorkerSliceConfig
		var createdSlice *kubeslicev1beta1.Slice
		var ipamOctet = 1

		BeforeEach(func() {
			// Prepare k8s objects
			hubSlice = &workerv1alpha1.WorkerSliceConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slice-1",
					Namespace: PROJECT_NS,
					Labels: map[string]string{
						"worker-cluster": CLUSTER_NAME,
					},
				},
				Spec: workerv1alpha1.WorkerSliceConfigSpec{
					SliceName:         "test-slice-1",
					SliceType:         "Application",
					SliceSubnet:       "10.0.0.1/16",
					SliceIpamType:     "Local",
					IpamClusterOctet:  ipamOctet,
					ClusterSubnetCIDR: "10.0.16.0/20",
				},
			}
			createdSlice = &kubeslicev1beta1.Slice{}

			// Cleanup after each test
			DeferCleanup(func() {
				Expect(k8sClient.Delete(ctx, hubSlice)).Should(Succeed())
				Eventually(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Namespace: hubSlice.Namespace, Name: hubSlice.Name}, hubSlice)
					return errors.IsNotFound(err)
				}, time.Second*10, time.Millisecond*250).Should(BeTrue())
			})
		})

		It("Should create Slice CR in spoke", func() {
			ctx := context.Background()

			Expect(k8sClient.Create(ctx, hubSlice)).Should(Succeed())

			sliceKey := types.NamespacedName{Name: "test-slice-1", Namespace: "kubeslice-system"}

			// Make sure slice is reconciled in spoke cluster
			Eventually(func() bool {
				err := k8sClient.Get(ctx, sliceKey, createdSlice)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			// Expect(createdSlice.Status.SliceConfig.SliceSubnet).To(Equal("10.0.0.1/16"))
			Expect(createdSlice.Status.SliceConfig.SliceDisplayName).To(Equal("test-slice-1"))
			Expect(createdSlice.Status.SliceConfig.SliceType).To(Equal("Application"))
			Expect(createdSlice.Status.SliceConfig.SliceIpam.SliceIpamType).To(Equal("Local"))
			Expect(createdSlice.Status.SliceConfig.SliceIpam.IpamClusterOctet).To(Equal(1))
			Expect(createdSlice.Status.SliceConfig.ClusterSubnetCIDR).To(Equal("10.0.16.0/20"))

			m, err := utils.GetCounterMetricFromRegistry(MetricRegistry, "kubeslice_slice_created_total", map[string]string{
				"slice": "test-slice-1",
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(m).To(Equal(1.0))

			m, err = utils.GetCounterMetricFromRegistry(MetricRegistry, "kubeslice_slice_updated_total", map[string]string{
				"slice": "test-slice-1",
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(m).To(Equal(2.0))
		})

		It("Should Generate Events", func() {
			ctx := context.Background()
			Expect(k8sClient.Create(ctx, hubSlice)).Should(Succeed())
			sliceKey := types.NamespacedName{Name: "test-slice-1", Namespace: "kubeslice-system"}

			// Make sure slice is reconciled in worker cluster
			Eventually(func() bool {
				err := k8sClient.Get(ctx, sliceKey, createdSlice)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			events := corev1.EventList{}
			Eventually(func() bool {
				err := k8sClient.List(ctx, &events)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			for _, event := range events.Items {
				if event.Source.Component == "test-slice-controller" && event.InvolvedObject.Kind == "WorkerSliceConfig" {
					Expect(event.Message).To(Equal("Created slice on spoke cluster , slice " + event.InvolvedObject.Name + " cluster "))
				}
			}
		})

		It("Should register a finalizer on spokeSliceConfig CR", func() {
			ctx := context.Background()
			Expect(k8sClient.Create(ctx, hubSlice)).Should(Succeed())

			sliceKey := types.NamespacedName{Name: "test-slice-1", Namespace: "kubeslice-system"}
			// Make sure slice is reconciled in spoke cluster
			Eventually(func() bool {
				err := k8sClient.Get(ctx, sliceKey, createdSlice)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			//get the created hubSlice
			hubSliceKey := types.NamespacedName{Name: "test-slice-1", Namespace: PROJECT_NS}
			sliceFinalizer := []string{"controller.kubeslice.io/hubSpokeSlice-finalizer"}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, hubSliceKey, hubSlice)
				return err == nil && reflect.DeepEqual(hubSlice.ObjectMeta.Finalizers, sliceFinalizer)
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())
		})

	})

	Context("With Slice CR deleted on hub", func() {
		var hubSlice *workerv1alpha1.WorkerSliceConfig
		var createdSlice *kubeslicev1beta1.Slice

		BeforeEach(func() {

			hundred := 100
			// Prepare k8s objects
			hubSlice = &workerv1alpha1.WorkerSliceConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slice-2",
					Namespace: PROJECT_NS,
					Labels: map[string]string{
						"worker-cluster": CLUSTER_NAME,
					},
				},
				Spec: workerv1alpha1.WorkerSliceConfigSpec{
					SliceName:        "test-slice-2",
					SliceType:        "Application",
					SliceSubnet:      "10.0.0.1/16",
					SliceIpamType:    "Local",
					IpamClusterOctet: hundred,
				},
			}

			createdSlice = &kubeslicev1beta1.Slice{}

		})
		It("Should Delete the slice CR on spoke", func() {
			ctx := context.Background()
			Expect(k8sClient.Create(ctx, hubSlice)).Should(Succeed())

			sliceKey := types.NamespacedName{Name: "test-slice-2", Namespace: "kubeslice-system"}
			// Make sure slice is reconciled in spoke cluster
			Eventually(func() bool {
				err := k8sClient.Get(ctx, sliceKey, createdSlice)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())
			//delete the hubSlice , which should delete the slice CR on spoke
			Expect(k8sClient.Delete(ctx, hubSlice)).Should(Succeed())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, sliceKey, createdSlice)
				return errors.IsNotFound(err)
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

		})
	})

	Context("With ExternalGatewayConfig", func() {

		var hubSlice *workerv1alpha1.WorkerSliceConfig
		var createdSlice *kubeslicev1beta1.Slice

		BeforeEach(func() {
			var ipamOcter = 16
			// Prepare k8s objects
			hubSlice = &workerv1alpha1.WorkerSliceConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slice-3",
					Namespace: PROJECT_NS,
					Labels: map[string]string{
						"worker-cluster": CLUSTER_NAME,
					},
				},
				Spec: workerv1alpha1.WorkerSliceConfigSpec{
					SliceName:        "test-slice-3",
					IpamClusterOctet: ipamOcter,
					ExternalGatewayConfig: workerv1alpha1.ExternalGatewayConfig{
						Ingress: workerv1alpha1.ExternalGatewayConfigOptions{
							Enabled: true,
						},
						Egress: workerv1alpha1.ExternalGatewayConfigOptions{
							Enabled: true,
						},
						NsIngress: workerv1alpha1.ExternalGatewayConfigOptions{
							Enabled: true,
						},
					},
				},
			}

			createdSlice = &kubeslicev1beta1.Slice{}

			// Cleanup after each test
			DeferCleanup(func() {
				Expect(k8sClient.Delete(ctx, hubSlice)).Should(Succeed())
				Eventually(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Namespace: hubSlice.Namespace, Name: hubSlice.Name}, hubSlice)
					return errors.IsNotFound(err)
				}, time.Second*10, time.Millisecond*250).Should(BeTrue())
			})
		})

		It("Should create Slice CR in spoke", func() {
			ctx := context.Background()

			Expect(k8sClient.Create(ctx, hubSlice)).Should(Succeed())

			sliceKey := types.NamespacedName{Name: "test-slice-3", Namespace: "kubeslice-system"}

			// Make sure slice is reconciled in spoke cluster
			Eventually(func() bool {
				err := k8sClient.Get(ctx, sliceKey, createdSlice)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			Expect(createdSlice.Status.SliceConfig.ExternalGatewayConfig).ToNot(BeNil())
			Expect(createdSlice.Status.SliceConfig.ExternalGatewayConfig.Ingress.Enabled).To(BeTrue())
			Expect(createdSlice.Status.SliceConfig.ExternalGatewayConfig.Egress.Enabled).To(BeTrue())
			Expect(createdSlice.Status.SliceConfig.ExternalGatewayConfig.NsIngress.Enabled).To(BeTrue())

		})

	})

	Context("Slice health check", func() {
		var hubSlice *workerv1alpha1.WorkerSliceConfig
		var pods []*corev1.Pod
		var ipamOcter = 16
		BeforeEach(func() {
			hubSlice = &workerv1alpha1.WorkerSliceConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slice-4",
					Namespace: PROJECT_NS,
					Labels: map[string]string{
						"worker-cluster":      CLUSTER_NAME,
						"original-slice-name": "test-slice-4",
					},
				},
				Spec: workerv1alpha1.WorkerSliceConfigSpec{
					SliceName:        "test-slice-4",
					IpamClusterOctet: ipamOcter,
					ExternalGatewayConfig: workerv1alpha1.ExternalGatewayConfig{
						Ingress: workerv1alpha1.ExternalGatewayConfigOptions{
							Enabled: true,
						},
						Egress: workerv1alpha1.ExternalGatewayConfigOptions{
							Enabled: true,
						},
						NsIngress: workerv1alpha1.ExternalGatewayConfigOptions{
							Enabled: true,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, hubSlice)).Should(Succeed())
			DeferCleanup(func() {
				Expect(k8sClient.Delete(ctx, hubSlice)).Should(Succeed())
				Eventually(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Namespace: hubSlice.Namespace, Name: hubSlice.Name}, hubSlice)
					return errors.IsNotFound(err)
				}, time.Second*10, time.Millisecond*250).Should(BeTrue())
				// delete all pods which were created for checking health status
				for _, pod := range pods {
					k8sClient.Delete(ctx, pod)
					pods = []*corev1.Pod{}
				}
			})
		})

		It("Should update slice CR with health status updated time", func() {
			sliceKey := types.NamespacedName{Name: hubSlice.Name, Namespace: hubSlice.Namespace}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, sliceKey, hubSlice)
				GinkgoWriter.Println(hubSlice.Status)
				return err == nil && hubSlice.Status.SliceHealth != nil
			}, time.Second*10, time.Second*1).Should(BeTrue())
			last := hubSlice.Status.SliceHealth.LastUpdated

			// LastUpdated should be updated every few seconds
			Eventually(func() bool {
				err := k8sClient.Get(ctx, sliceKey, hubSlice)
				return err == nil && hubSlice.Status.SliceHealth != nil && !hubSlice.Status.SliceHealth.LastUpdated.Equal(&last)
			}, time.Second*200, time.Second*1).Should(BeTrue())
		})

		It("Should update slice CR with component status as error for all", func() {
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: hubSlice.Name, Namespace: hubSlice.Namespace}, hubSlice)
				GinkgoWriter.Println(hubSlice.Status)
				return err == nil && hubSlice.Status.SliceHealth != nil
			}, time.Second*10, time.Second*1).Should(BeTrue())
			cs := hubSlice.Status.SliceHealth.ComponentStatuses

			Expect(cs).Should(HaveLen(2))

			Expect(string(hubSlice.Status.SliceHealth.SliceHealthStatus)).Should(Equal("Warning"))

			for _, c := range cs {
				Expect(string(c.ComponentHealthStatus)).Should(Equal("Error"))
				m, err := utils.GetGaugeMetricFromRegistry(MetricRegistry, "kubeslice_slice_component_up", map[string]string{
					"slice_component": c.Component,
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(m).To(Equal(0.0))
			}
			m, err := utils.GetGaugeMetricFromRegistry(MetricRegistry, "kubeslice_slice_up", map[string]string{})
			Expect(err).ToNot(HaveOccurred())
			Expect(m).To(Equal(0.0))
		})

		It("Should update slice CR with component status as normal when pods running", func() {
			var replicas int32 = 1
			deployments := []*appsv1.Deployment{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "slicegateway",
						Labels: map[string]string{
							"kubeslice.io/pod-type": "slicegateway",
							"kubeslice.io/slice":    "test-slice-4",
						},
						Namespace: "kubeslice-system",
					},
					Spec: appsv1.DeploymentSpec{
						Replicas: &replicas,
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"kubeslice.io/pod-type": "slicegateway",
								"kubeslice.io/slice":    "test-slice-4",
							},
						},
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"kubeslice.io/pod-type": "slicegateway",
									"kubeslice.io/slice":    "test-slice-4",
								},
							},
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name:            "kubeslice-sidecar",
										Image:           "nexus.dev.aveshalabs.io/kubeslice/gw-sidecar:1.0.0",
										ImagePullPolicy: corev1.PullAlways,
									},
								},
							},
						},
					},
				},
			}
			pods := []*corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "dns",
						Labels: map[string]string{
							"app": "kubeslice-dns",
						},
						Namespace: "kubeslice-system",
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "test",
							Image: "test",
						}},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "slicegateway",
						Labels: map[string]string{
							"kubeslice.io/pod-type": "slicegateway",
							"kubeslice.io/slice":    "test-slice-4",
						},
						Namespace: "kubeslice-system",
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "test",
							Image: "test",
						}},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "slicerouter",
						Labels: map[string]string{
							"kubeslice.io/pod-type": "router",
							"kubeslice.io/slice":    "test-slice-4",
						},
						Namespace: "kubeslice-system",
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "test",
							Image: "test",
						}},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "egress",
						Labels: map[string]string{
							"istio":              "egressgateway",
							"kubeslice.io/slice": "test-slice-4",
						},
						Namespace: "kubeslice-system",
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "test",
							Image: "test",
						}},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "ingress",
						Labels: map[string]string{
							"istio":              "ingressgateway",
							"kubeslice.io/slice": "test-slice-4",
						},
						Namespace: "kubeslice-system",
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "test",
							Image: "test",
						}},
					},
				},
			}
			for _, deployment := range deployments {
				Expect(k8sClient.Create(ctx, deployment)).ToNot(HaveOccurred())
			}
			for _, pod := range pods {
				Expect(k8sClient.Create(ctx, pod)).ToNot(HaveOccurred())
				pod.Status.Phase = corev1.PodRunning
				Expect(k8sClient.Status().Update(ctx, pod)).ToNot(HaveOccurred())
			}

			// wait for next reconcile loop
			time.Sleep(120 * time.Second)

			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: hubSlice.Name, Namespace: hubSlice.Namespace}, hubSlice)
				GinkgoWriter.Println(hubSlice.Status)
				return err == nil && hubSlice.Status.SliceHealth != nil
			}, time.Second*30, time.Second*1).Should(BeTrue())

			cs := hubSlice.Status.SliceHealth.ComponentStatuses
			Expect(cs).Should(HaveLen(5))
			Expect(string(hubSlice.Status.SliceHealth.SliceHealthStatus)).Should(Equal("Normal"))
			for i, c := range cs {
				Expect(c.Component).Should(Equal(pods[i].ObjectMeta.Name))
				Expect(string(c.ComponentHealthStatus)).Should(Equal("Normal"))
				m, err := utils.GetGaugeMetricFromRegistry(MetricRegistry, "kubeslice_slice_component_up", map[string]string{
					"slice_component": c.Component,
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(m).To(Equal(1.0))
			}
			m, err := utils.GetGaugeMetricFromRegistry(MetricRegistry, "kubeslice_slice_up", map[string]string{})
			Expect(err).ToNot(HaveOccurred())
			Expect(m).To(Equal(1.0))
		})
	})
})
