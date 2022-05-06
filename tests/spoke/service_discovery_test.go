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
	"reflect"
	"strings"
	"time"

	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
)

func getTestServiceExportPorts() []kubeslicev1beta1.ServicePort {
	return []kubeslicev1beta1.ServicePort{
		{
			Name:          "proto-tcp",
			ContainerPort: 80,
			Protocol:      corev1.ProtocolTCP,
		},
	}
}

var _ = Describe("ServiceExportController", func() {

	Context("With a service export CR object installed, verify service export CR is reconciled", func() {
		var slice *kubeslicev1beta1.Slice
		var dnssvc *corev1.Service
		var svcex *kubeslicev1beta1.ServiceExport
		var createdSlice *kubeslicev1beta1.Slice

		BeforeEach(func() {
			// Prepare k8s objects for slice and mesh-dns service
			slice = &kubeslicev1beta1.Slice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slice-1",
					Namespace: "kubeslice-system",
				},
				Spec: kubeslicev1beta1.SliceSpec{},
			}

			dnssvc = &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mesh-dns",
					Namespace: "kubeslice-system",
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{{
						Port: 52,
					}},
				},
			}

			// Prepare k8s objects for slice and mesh-dns service
			svcex = &kubeslicev1beta1.ServiceExport{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "iperf-server",
					Namespace: "default",
				},
				Spec: kubeslicev1beta1.ServiceExportSpec{
					Slice: "test-slice-1",
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "iperf"},
					},
					Ports: getTestServiceExportPorts(),
				},
			}
			createdSlice = &kubeslicev1beta1.Slice{}

			// Cleanup after each test
			DeferCleanup(func() {
				ctx := context.Background()
				Expect(k8sClient.Delete(ctx, svcex)).Should(Succeed())
				Eventually(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Name: svcex.Name, Namespace: svcex.Namespace}, svcex)
					return errors.IsNotFound(err)
				}, time.Second*30, time.Millisecond*250).Should(BeTrue())

				Expect(k8sClient.Delete(ctx, createdSlice)).Should(Succeed())
				Eventually(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Name: slice.Name, Namespace: slice.Namespace}, createdSlice)
					return errors.IsNotFound(err)
				}, time.Second*30, time.Millisecond*250).Should(BeTrue())

				Expect(k8sClient.Delete(ctx, dnssvc)).Should(Succeed())
				Eventually(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Name: dnssvc.Name, Namespace: dnssvc.Namespace}, dnssvc)
					return errors.IsNotFound(err)
				}, time.Second*30, time.Millisecond*250).Should(BeTrue())
				log.Info("cleaned up after test")
			})
		})

		It("Should update service export status", func() {
			Expect(k8sClient.Create(ctx, dnssvc)).Should(Succeed())
			Expect(k8sClient.Create(ctx, slice)).Should(Succeed())

			err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      slice.Name,
					Namespace: slice.Namespace,
				}, createdSlice)
				if err != nil {
					return err
				}
				createdSlice.Status.SliceConfig = &kubeslicev1beta1.SliceConfig{}
				return k8sClient.Status().Update(ctx, createdSlice)
			})
			Expect(err).To(BeNil())

			Expect(k8sClient.Create(ctx, svcex)).Should(Succeed())
			svcKey := types.NamespacedName{Name: "iperf-server", Namespace: "default"}
			createdSvcEx := &kubeslicev1beta1.ServiceExport{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, svcKey, createdSvcEx)
				if err != nil {
					return false
				}
				if createdSvcEx.Status.ExposedPorts != "80/TCP" {
					return false
				}
				if createdSvcEx.Status.ExportStatus != kubeslicev1beta1.ExportStatusReady {
					return false
				}
				return true
			}, time.Second*30, time.Millisecond*250).Should(BeTrue())
		})

		It("Should update service export ports", func() {
			log.Info("Creating slice", "slice", slice)
			Expect(k8sClient.Create(ctx, dnssvc)).Should(Succeed())
			Expect(k8sClient.Create(ctx, slice)).Should(Succeed())

			err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      slice.Name,
					Namespace: slice.Namespace,
				}, createdSlice)
				if err != nil {
					return err
				}
				createdSlice.Status.SliceConfig = &kubeslicev1beta1.SliceConfig{}
				return k8sClient.Status().Update(ctx, createdSlice)
			})
			Expect(err).To(BeNil())

			Expect(k8sClient.Create(ctx, svcex)).Should(Succeed())
			svcKey := types.NamespacedName{Name: "iperf-server", Namespace: "default"}
			createdSvcEx := &kubeslicev1beta1.ServiceExport{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, svcKey, createdSvcEx)
				if err != nil {
					return false
				}
				if createdSvcEx.Status.ExportStatus != kubeslicev1beta1.ExportStatusReady {
					return false
				}
				return true
			}, time.Second*30, time.Millisecond*250).Should(BeTrue())

			createdSvcEx.Spec.Ports = append(createdSvcEx.Spec.Ports, kubeslicev1beta1.ServicePort{
				Name:          "proto-2",
				ContainerPort: 5353,
				Protocol:      corev1.ProtocolUDP,
			})

			Expect(k8sClient.Update(ctx, createdSvcEx)).Should(Succeed())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, svcKey, createdSvcEx)
				if err != nil {
					return false
				}
				if !strings.Contains(createdSvcEx.Status.ExposedPorts, "5353/UDP") {
					return false
				}
				if createdSvcEx.Status.ExportStatus != kubeslicev1beta1.ExportStatusReady {
					return false
				}
				return true
			}, time.Second*30, time.Millisecond*250).Should(BeTrue())

		})
		It("Should Add a slice label to service export", func() {
			log.Info("Creating slice", "slice", slice)

			Expect(k8sClient.Create(ctx, dnssvc)).Should(Succeed())
			Expect(k8sClient.Create(ctx, slice)).Should(Succeed())

			err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      slice.Name,
					Namespace: slice.Namespace,
				}, createdSlice)
				if err != nil {
					return err
				}
				createdSlice.Status.SliceConfig = &kubeslicev1beta1.SliceConfig{}
				return k8sClient.Status().Update(ctx, createdSlice)
			})
			Expect(err).To(BeNil())

			Expect(k8sClient.Create(ctx, svcex)).Should(Succeed())
			expectedLabel := map[string]string{
				"kubeslice.io/slice": svcex.Spec.Slice,
			}
			svcKey := types.NamespacedName{Name: "iperf-server", Namespace: "default"}
			createdSvcEx := &kubeslicev1beta1.ServiceExport{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, svcKey, createdSvcEx)
				if err != nil {
					return false
				}
				return reflect.DeepEqual(createdSvcEx.GetLabels(), expectedLabel)
			}, time.Second*30, time.Millisecond*250).Should(BeTrue())
		})
	})
})
