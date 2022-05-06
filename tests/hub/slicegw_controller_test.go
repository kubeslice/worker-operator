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
	"time"

	spokev1alpha1 "github.com/kubeslice/apis/pkg/worker/v1alpha1"
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Hub SlicegwController", func() {
	Context("With SpokeSliceGW created in hub", func() {
		var hubSlice *spokev1alpha1.WorkerSliceConfig
		var createdSlice *kubeslicev1beta1.Slice
		var hubSliceGw *spokev1alpha1.WorkerSliceGateway
		var hubSecret *corev1.Secret
		var createdSliceGwOnSpoke *kubeslicev1beta1.SliceGateway

		BeforeEach(func() {
			// Prepare k8s objects
			hubSlice = &spokev1alpha1.WorkerSliceConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slice",
					Namespace: PROJECT_NS,
					Labels: map[string]string{
						"spoke-cluster": CLUSTER_NAME,
					},
				},
				Spec: spokev1alpha1.WorkerSliceConfigSpec{
					SliceName:        "test-slice",
					SliceType:        "Application",
					SliceSubnet:      "10.0.0.1/16",
					SliceIpamType:    "Local",
					IpamClusterOctet: 100,
				},
			}
			hubSliceGw = &spokev1alpha1.WorkerSliceGateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slicegateway",
					Namespace: PROJECT_NS,
					Labels: map[string]string{
						"spoke-cluster": CLUSTER_NAME,
					},
				},
				Spec: spokev1alpha1.WorkerSliceGatewaySpec{
					SliceName: "test-slice",
					LocalGatewayConfig: spokev1alpha1.SliceGatewayConfig{
						ClusterName: CLUSTER_NAME,
					},
				},
			}
			hubSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slicegateway",
					Namespace: PROJECT_NS,
				},
				Data: map[string][]byte{},
			}
			createdSlice = &kubeslicev1beta1.Slice{}
			createdSliceGwOnSpoke = &kubeslicev1beta1.SliceGateway{}

			// Cleanup after each test
			DeferCleanup(func() {
				Expect(k8sClient.Delete(ctx, hubSlice)).Should(Succeed())
				Expect(k8sClient.Delete(ctx, hubSliceGw)).Should(Succeed())
				Expect(k8sClient.Delete(ctx, hubSecret)).Should(Succeed())
				Eventually(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Namespace: "kubeslice-system", Name: createdSlice.Name}, createdSlice)
					return errors.IsNotFound(err)
				}, time.Second*10, time.Millisecond*250).Should(BeTrue())
				Expect(k8sClient.Delete(ctx, createdSliceGwOnSpoke)).Should(Succeed())
			})
		})

		It("Should create SliceGw on spoke cluster", func() {
			Expect(k8sClient.Create(ctx, hubSlice)).Should(Succeed())
			Expect(k8sClient.Create(ctx, hubSliceGw)).Should(Succeed())
			Expect(k8sClient.Create(ctx, hubSecret)).Should(Succeed())
			//once hubSlice is created controller will create a slice CR on spoke cluster
			sliceKey := types.NamespacedName{Name: "test-slice", Namespace: "kubeslice-system"}
			// Make sure slice is reconciled in spoke cluster
			Eventually(func() bool {
				err := k8sClient.Get(ctx, sliceKey, createdSlice)
				return err == nil
			}, time.Second*20, time.Millisecond*250).Should(BeTrue())

			sliceGwKey := types.NamespacedName{Namespace: CONTROL_PLANE_NS, Name: hubSliceGw.Name}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, sliceGwKey, createdSliceGwOnSpoke)
				return err == nil
			}, time.Second*20, time.Second*1).Should(BeTrue())
		})

		It("Should set slice as owner of slicegw", func() {
			Expect(k8sClient.Create(ctx, hubSlice)).Should(Succeed())
			Expect(k8sClient.Create(ctx, hubSliceGw)).Should(Succeed())
			Expect(k8sClient.Create(ctx, hubSecret)).Should(Succeed())
			//once hubSlice is created controller will create a slice CR on spoke cluster
			sliceKey := types.NamespacedName{Name: "test-slice", Namespace: "kubeslice-system"}
			// Make sure slice is reconciled in spoke cluster
			Eventually(func() bool {
				err := k8sClient.Get(ctx, sliceKey, createdSlice)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			sliceGwKey := types.NamespacedName{Namespace: CONTROL_PLANE_NS, Name: hubSliceGw.Name}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, sliceGwKey, createdSliceGwOnSpoke)
				return err == nil
			}, time.Second*10, time.Second*1).Should(BeTrue())

			Expect(createdSliceGwOnSpoke.ObjectMeta.OwnerReferences[0].Name).Should(Equal("test-slice"))
		})

		It("Should set slice as owner of slicegw", func() {
			Expect(k8sClient.Create(ctx, hubSlice)).Should(Succeed())
			Expect(k8sClient.Create(ctx, hubSliceGw)).Should(Succeed())
			Expect(k8sClient.Create(ctx, hubSecret)).Should(Succeed())
			//once hubSlice is created controller will create a slice CR on spoke cluster
			sliceKey := types.NamespacedName{Name: "test-slice", Namespace: "kubeslice-system"}
			// Make sure slice is reconciled in spoke cluster
			Eventually(func() bool {
				err := k8sClient.Get(ctx, sliceKey, createdSlice)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			sliceGwKey := types.NamespacedName{Namespace: CONTROL_PLANE_NS, Name: hubSliceGw.Name}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, sliceGwKey, createdSliceGwOnSpoke)
				return err == nil
			}, time.Second*10, time.Second*1).Should(BeTrue())

			Expect(createdSliceGwOnSpoke.ObjectMeta.OwnerReferences[0].Name).Should(Equal("test-slice"))
		})

	})
})
