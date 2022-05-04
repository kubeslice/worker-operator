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

	kubeslicev1beta1 "github.com/kubeslice/operator/api/v1beta1"
	"github.com/kubeslice/operator/internal/logger"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var log = logger.NewLogger()
var sliceFinalizer = "networking.kubeslice.io/slice-finalizer"

var _ = Describe("SliceController", func() {

	Context("With a Slice CR and kubeslice-dns service created", func() {

		var slice *kubeslicev1beta1.Slice
		var svc *corev1.Service
		BeforeEach(func() {

			// Prepare k8s objects for slice and kubeslice-dns service
			slice = &kubeslicev1beta1.Slice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slice",
					Namespace: "kubeslice-system",
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

			// Cleanup after each test
			DeferCleanup(func() {
				ctx := context.Background()
				Expect(k8sClient.Delete(ctx, slice)).Should(Succeed())
				Eventually(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Name: slice.Name, Namespace: slice.Namespace}, slice)
					return errors.IsNotFound(err)
				}, time.Second*30, time.Millisecond*250).Should(BeTrue())
				Expect(k8sClient.Delete(ctx, svc)).Should(Succeed())
			})
		})

		It("Should update slice status with DNS IP", func() {
			ctx := context.Background()

			// Create slice and kubeslice-dns service
			Expect(k8sClient.Create(ctx, slice)).Should(Succeed())
			Expect(k8sClient.Create(ctx, svc)).Should(Succeed())

			svcKey := types.NamespacedName{Name: "kubeslice-dns", Namespace: "kubeslice-system"}
			createdSvc := &corev1.Service{}

			// Wait until service is created properly
			Eventually(func() bool {
				err := k8sClient.Get(ctx, svcKey, createdSvc)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			sliceKey := types.NamespacedName{Name: "test-slice", Namespace: "kubeslice-system"}
			createdSlice := &kubeslicev1beta1.Slice{}

			// Make sure slice status.Status.DNSIP is pointing to correct serviceIP
			Eventually(func() string {
				err := k8sClient.Get(ctx, sliceKey, createdSlice)
				if err != nil {
					return ""
				}
				return createdSlice.Status.DNSIP
			}, time.Second*30, time.Millisecond*250).Should(Equal("10.0.0.20"))

		})
		It("Should create a finalizer for slice CR created", func() {
			ctx := context.Background()
			// Create slice and kubeslice-dns service
			Eventually(func() bool {
				err := k8sClient.Create(ctx, slice)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			Eventually(func() bool {
				err := k8sClient.Create(ctx, svc)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			createdSlice := &kubeslicev1beta1.Slice{}
			Eventually(func() bool {
				sliceKey := types.NamespacedName{Name: "test-slice", Namespace: "kubeslice-system"}
				if err := k8sClient.Get(ctx, sliceKey, createdSlice); err != nil {
					return false
				}
				return true
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())
			Expect(createdSlice.ObjectMeta.Finalizers[0]).Should(Equal(sliceFinalizer))
		})

	})
	Context("With Slice CR Deleted", func() {
		var slice *kubeslicev1beta1.Slice
		var svc *corev1.Service
		var svcimport *kubeslicev1beta1.ServiceImport
		var svcexport *kubeslicev1beta1.ServiceExport
		BeforeEach(func() {

			// Prepare k8s objects for slice and kubeslice-dns service
			slice = &kubeslicev1beta1.Slice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slice-im",
					Namespace: "kubeslice-system",
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
			svcimport = &kubeslicev1beta1.ServiceImport{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service-import",
					Namespace: "kubeslice-system",
					Labels: map[string]string{
						"kubeslice.io/slice": "test-slice-im",
					},
				},
				Spec: kubeslicev1beta1.ServiceImportSpec{
					Slice:   "test-slice-im",
					DNSName: "pod.svc.local.cluster",
					Ports: []kubeslicev1beta1.ServicePort{
						{
							Name:          "xyz",
							ContainerPort: 5000,
						},
					},
				},
			}
			svcexport = &kubeslicev1beta1.ServiceExport{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service-export",
					Namespace: "kubeslice-system",
					Labels: map[string]string{
						"kubeslice.io/slice": "test-slice-im",
					},
				},
				Spec: kubeslicev1beta1.ServiceExportSpec{
					Slice: "test-slice-im",
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "iperf",
						},
					},
					Ports: []kubeslicev1beta1.ServicePort{
						{
							Name:          "xyz",
							ContainerPort: 5000,
						},
					},
				},
			}
			DeferCleanup(func() {
				Expect(k8sClient.Delete(ctx, svc)).Should(Succeed())
			})
		})

		It("Should Delete All the dependent resources", func() {
			Expect(k8sClient.Create(ctx, slice)).Should(Succeed())
			Expect(k8sClient.Create(ctx, svc)).Should(Succeed())
			Expect(k8sClient.Create(ctx, svcimport)).Should(Succeed())
			Expect(k8sClient.Create(ctx, svcexport)).Should(Succeed())

			createdSvc := &corev1.Service{}
			svcKey := types.NamespacedName{Name: "kubeslice-dns", Namespace: "kubeslice-system"}

			// Wait until service is created properly
			Eventually(func() bool {
				err := k8sClient.Get(ctx, svcKey, createdSvc)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			createdSlice := &kubeslicev1beta1.Slice{}
			Eventually(func() bool {
				sliceKey := types.NamespacedName{Name: "test-slice-im", Namespace: "kubeslice-system"}
				if err := k8sClient.Get(ctx, sliceKey, createdSlice); err != nil {
					return false
				}
				return true
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			//Delete the slice CR which will trigger the finalizer and cleanup resources
			Expect(k8sClient.Delete(ctx, slice)).Should(Succeed())
			//check if svc import objects created are deleted or not
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "test-service-import", Namespace: "kubeslice-system"}, svcimport)
				return errors.IsNotFound(err)
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			//check if svc export objects created are deleted or not
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "test-service-export", Namespace: "kubeslice-system"}, svcexport)
				return errors.IsNotFound(err)
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

		})
	})

})
