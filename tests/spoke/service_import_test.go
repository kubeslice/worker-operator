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
	"time"

	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	"github.com/kubeslice/worker-operator/tests/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
)

var _ = Describe("ServiceImportController", func() {

	Context("With a service import CR object installed, verify service import CR is reconciled", func() {
		var slice *kubeslicev1beta1.Slice
		var dnssvc *corev1.Service
		var dnscm *corev1.ConfigMap
		var svcim *kubeslicev1beta1.ServiceImport
		var createdSlice *kubeslicev1beta1.Slice
		BeforeEach(func() {

			// Prepare k8s objects for slice and kubeslice-dns service
			slice = &kubeslicev1beta1.Slice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slice-2",
					Namespace: "kubeslice-system",
				},
				Spec: kubeslicev1beta1.SliceSpec{},
			}

			dnssvc = &corev1.Service{
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

			dnscm = &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kubeslice-dns",
					Namespace: "kubeslice-system",
				},
				Data: map[string]string{"slice.db": ""},
			}

			// Prepare k8s objects for slice and kubeslice-dns service
			svcim = &kubeslicev1beta1.ServiceImport{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "iperf-server",
					Namespace: "default",
				},
				Spec: kubeslicev1beta1.ServiceImportSpec{
					Slice:   "test-slice-2",
					DNSName: "iperf-server.iperf.svc.slice.local",
					Ports:   getTestServiceExportPorts(),
				},
			}
			createdSlice = &kubeslicev1beta1.Slice{}

			// Cleanup after each test
			DeferCleanup(func() {
				ctx := context.Background()
				Expect(k8sClient.Delete(ctx, svcim)).Should(Succeed())
				Eventually(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Name: svcim.Name, Namespace: svcim.Namespace}, svcim)
					return errors.IsNotFound(err)
				}, time.Second*30, time.Millisecond*250).Should(BeTrue())

				Expect(k8sClient.Delete(ctx, dnscm)).Should(Succeed())
				Eventually(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Name: dnscm.Name, Namespace: dnscm.Namespace}, dnscm)
					return errors.IsNotFound(err)
				}, time.Second*30, time.Millisecond*250).Should(BeTrue())

				Expect(k8sClient.Delete(ctx, dnssvc)).Should(Succeed())
				Eventually(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Name: dnssvc.Name, Namespace: dnssvc.Namespace}, dnssvc)
					return errors.IsNotFound(err)
				}, time.Second*30, time.Millisecond*250).Should(BeTrue())

				Expect(k8sClient.Delete(ctx, slice)).Should(Succeed())
				Eventually(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Name: slice.Name, Namespace: slice.Namespace}, slice)
					return errors.IsNotFound(err)
				}, time.Second*30, time.Millisecond*250).Should(BeTrue())

			})
		})

		It("Should update service import status", func() {
			ctx := context.Background()
			Expect(k8sClient.Create(ctx, slice)).Should(Succeed())
			Expect(k8sClient.Create(ctx, dnssvc)).Should(Succeed())
			Expect(k8sClient.Create(ctx, dnscm)).Should(Succeed())
			Expect(k8sClient.Create(ctx, svcim)).Should(Succeed())

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

			svcKey := types.NamespacedName{Name: "iperf-server", Namespace: "default"}
			createdSvcIm := &kubeslicev1beta1.ServiceImport{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, svcKey, createdSvcIm)
				if err != nil {
					return false
				}
				if createdSvcIm.Status.ExposedPorts != "80/TCP" {
					return false
				}
				if createdSvcIm.Status.ImportStatus != kubeslicev1beta1.ImportStatusReady {
					return false
				}
				return true
			}, time.Second*30, time.Millisecond*250).Should(BeTrue())

			err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
				err := k8sClient.Get(ctx, svcKey, createdSvcIm)
				if err != nil {
					return err
				}
				createdSvcIm.Status.Endpoints = []kubeslicev1beta1.ServiceEndpoint{{
					Name: "test",
					IP:   "test",
				}}
				return k8sClient.Status().Update(ctx, createdSvcIm)
			})
			Expect(err).To(BeNil())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, svcKey, createdSvcIm)
				if err != nil {
					return false
				}
				if createdSvcIm.Status.AvailableEndpoints != 1 {
					return false
				}
				return true
			}, time.Second*30, time.Millisecond*250).Should(BeTrue())

			m, err := utils.GetGaugeMetricFromRegistry(MetricRegistry, "kubeslice_serviceimport_endpoints", map[string]string{
				"slice":         "test-slice-2",
				"slice_service": "iperf-server",
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(m).To(Equal(1.0))
		})
	})
})
