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
	"reflect"

	workerv1alpha1 "github.com/kubeslice/apis/pkg/worker/v1alpha1"
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"

	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Hub serviceImportController", func() {
	Context("With a spokeserviceImport CR installed on hub,verify service import is reconciled on worker cluster", func() {
		var hubServiceImport *workerv1alpha1.WorkerServiceImport
		var reflectedSvcIm *workerv1alpha1.WorkerServiceImport
		var createdSvcIm *kubeslicev1beta1.ServiceImport
		var slice *kubeslicev1beta1.Slice

		BeforeEach(func() {
			// Prepare k8s objects for slice
			slice = &kubeslicev1beta1.Slice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slice-1",
					Namespace: "kubeslice-system",
				},
				Spec: kubeslicev1beta1.SliceSpec{},
			}
			// Prepare k8s objects for serviceImport

			hubServiceImport = &workerv1alpha1.WorkerServiceImport{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service-import",
					Namespace: PROJECT_NS,
				},
				Spec: workerv1alpha1.WorkerServiceImportSpec{
					SliceName:        "test-slice-1",
					ServiceName:      "test-service-import",
					ServiceNamespace: "default",
					ServiceDiscoveryPorts: []workerv1alpha1.ServiceDiscoveryPort{
						{
							Name:     "abc",
							Port:     5000,
							Protocol: "TCP",
						},
						{
							Name:     "abcd",
							Port:     5001,
							Protocol: "UDP",
						},
						{
							Name:     "abcde",
							Port:     5002,
							Protocol: "SCTP",
						},
						{
							Name:     "abcdef",
							Port:     5003,
							Protocol: "invalidProtocol",
						},
					},
					ServiceDiscoveryEndpoints: []workerv1alpha1.ServiceDiscoveryEndpoint{
						{
							PodName: "abc",
							NsmIp:   "x.x.x.1",
							Cluster: "cluster1",
							DnsName: "cluster-dns-1",
						},
					},
				},
			}
			reflectedSvcIm = &workerv1alpha1.WorkerServiceImport{}
			createdSvcIm = &kubeslicev1beta1.ServiceImport{}
			// Cleanup after each test
			DeferCleanup(func() {
				Expect(k8sClient.Delete(ctx, hubServiceImport)).Should(Succeed())
				Eventually(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Name: hubServiceImport.Name, Namespace: hubServiceImport.Namespace}, hubServiceImport)
					return errors.IsNotFound(err)
				}, time.Second*10, time.Millisecond*250).Should(BeTrue())
				Expect(k8sClient.Delete(ctx, slice)).Should(Succeed())
			})
		})
		It("Should Create serviceimport", func() {
			ctx := context.Background()
			Expect(k8sClient.Create(ctx, slice)).Should(Succeed())
			Expect(k8sClient.Create(ctx, hubServiceImport)).Should(Succeed())

			svcKey := types.NamespacedName{Namespace: "default", Name: "test-service-import"}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, svcKey, createdSvcIm)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())
		})
		It("Should register a finalizer on ServiceImport CR", func() {
			ctx := context.Background()
			Expect(k8sClient.Create(ctx, slice)).Should(Succeed())
			Expect(k8sClient.Create(ctx, hubServiceImport)).Should(Succeed())

			serviceImportFinalizer := []string{"controller.kubeslice.io/hubWorkerServiceImport-finalizer"}

			svcKey := types.NamespacedName{Namespace: PROJECT_NS, Name: "test-service-import"}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, svcKey, reflectedSvcIm)
				return err == nil && reflect.DeepEqual(reflectedSvcIm.ObjectMeta.Finalizers, serviceImportFinalizer)
			}, time.Second*30, time.Millisecond*250).Should(BeTrue())
		})

		It("Should able to fetch slice object for service import", func() {
			ctx := context.Background()
			Expect(k8sClient.Create(ctx, slice)).Should(Succeed())
			Expect(k8sClient.Create(ctx, hubServiceImport)).Should(Succeed())

			createdSlice := &kubeslicev1beta1.Slice{}
			sliceKey := types.NamespacedName{Namespace: CONTROL_PLANE_NS, Name: hubServiceImport.Spec.SliceName}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, sliceKey, createdSlice)
				return err == nil
			}, time.Second*30, time.Microsecond*250).Should(BeTrue())
		})

		It("Should create service import on spoke cluster", func() {
			ctx := context.Background()
			Expect(k8sClient.Create(ctx, slice)).Should(Succeed())
			Expect(k8sClient.Create(ctx, hubServiceImport)).Should(Succeed())

			//created slice for service import
			createdSlice := &kubeslicev1beta1.Slice{}
			sliceKey := types.NamespacedName{Namespace: CONTROL_PLANE_NS, Name: hubServiceImport.Spec.SliceName}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, sliceKey, createdSlice)
				return err == nil
			}, time.Second*30, time.Microsecond*250).Should(BeTrue())

			//created service import on spoke cluster
			svcImKey := types.NamespacedName{Namespace: hubServiceImport.Spec.ServiceNamespace, Name: hubServiceImport.Spec.ServiceName}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, svcImKey, createdSvcIm)
				return err == nil
			}, time.Second*30, time.Microsecond*250).Should(BeTrue())
		})

		It("Should generate Events", func() {
			ctx := context.Background()
			Expect(k8sClient.Create(ctx, slice)).Should(Succeed())
			Expect(k8sClient.Create(ctx, hubServiceImport)).Should(Succeed())

			//created slice for service import
			createdSlice := &kubeslicev1beta1.Slice{}
			sliceKey := types.NamespacedName{Namespace: CONTROL_PLANE_NS, Name: hubServiceImport.Spec.SliceName}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, sliceKey, createdSlice)
				return err == nil
			}, time.Second*30, time.Microsecond*250).Should(BeTrue())

			//created service import on spoke cluster
			svcImKey := types.NamespacedName{Namespace: hubServiceImport.Spec.ServiceNamespace, Name: hubServiceImport.Spec.ServiceName}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, svcImKey, createdSvcIm)
				return err == nil
			}, time.Second*30, time.Microsecond*250).Should(BeTrue())

			events := corev1.EventList{}
			Eventually(func() bool {
				err := k8sClient.List(ctx, &events)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			for _, event := range events.Items {
				if event.Source.Component == "test-svcim-controller" && event.InvolvedObject.Kind == "WorkerServiceImport" {
					Expect(event.Message).To(Equal("Successfully created service import on spoke cluster , svc import " + hubServiceImport.Spec.ServiceName + " cluster "))
				}
			}
		})
	})
})
