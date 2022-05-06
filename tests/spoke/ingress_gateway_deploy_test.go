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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	"github.com/kubeslice/worker-operator/internal/manifest"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("IngressGateway", func() {

	Context("With ingress not installed", func() {

		objects := []client.Object{
			&appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "green-istio-ingressgateway",
					Namespace: "kubeslice-system",
				},
			},
			&corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "green-istio-ingressgateway",
					Namespace: "kubeslice-system",
				},
			},
			&rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "green-istio-ingressgateway-sds",
					Namespace: "kubeslice-system",
				},
			},
			&corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "green-istio-ingressgateway-service-account",
					Namespace: "kubeslice-system",
				},
			},
			&rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "green-istio-ingressgateway-sds",
					Namespace: "kubeslice-system",
				},
			},
		}

		slice := &kubeslicev1beta1.Slice{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Slice",
				APIVersion: "mesh.avesha.io/v1beta1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "green",
				Namespace: "kubeslice-system",
				UID:       "test-uid",
			},
			Spec: kubeslicev1beta1.SliceSpec{},
		}

		AfterEach(func() {
			for _, o := range objects {
				Expect(k8sClient.Delete(ctx, o)).Should(Succeed())
			}
		})

		It("Should install istio ingress gateway deployment", func() {
			err := manifest.InstallIngress(ctx, k8sClient, slice)
			Expect(err).NotTo(HaveOccurred())

			// Check if deployment is there in the cluster
			deployKey := types.NamespacedName{Name: "green-istio-ingressgateway", Namespace: "kubeslice-system"}
			createdDeploy := &appsv1.Deployment{}

			// Wait until deployment is created properly
			Eventually(func() bool {
				err := k8sClient.Get(ctx, deployKey, createdDeploy)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			Expect(createdDeploy.ObjectMeta.Name).To(Equal("green-istio-ingressgateway"))

			labels := createdDeploy.ObjectMeta.Labels
			Expect(labels["slice"]).To(Equal("green"))

			ann := createdDeploy.ObjectMeta.Annotations
			Expect(ann["avesha.io/slice"]).To(Equal("green"))
			Expect(ann["avesha.io/status"]).To(Equal("injected"))
		})

		It("Should install istio ingress gateway resources", func() {
			err := manifest.InstallIngress(ctx, k8sClient, slice)
			Expect(err).NotTo(HaveOccurred())

			// Check if service is there in the cluster
			key := types.NamespacedName{Name: "green-istio-ingressgateway", Namespace: "kubeslice-system"}
			svc := &corev1.Service{}

			// Wait until deployment is created properly
			Eventually(func() bool {
				err := k8sClient.Get(ctx, key, svc)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			Expect(svc.ObjectMeta.Name).To(Equal("green-istio-ingressgateway"))

			// Check if role and rolebinding are there in the cluster
			rkey := types.NamespacedName{Name: "green-istio-ingressgateway-sds", Namespace: "kubeslice-system"}
			role := &rbacv1.Role{}
			rb := &rbacv1.RoleBinding{}

			// Wait until role is created properly
			Eventually(func() bool {
				err := k8sClient.Get(ctx, rkey, role)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			// Wait until rolebinding is created properly
			Eventually(func() bool {
				err := k8sClient.Get(ctx, rkey, rb)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			Expect(role.ObjectMeta.Name).To(Equal("green-istio-ingressgateway-sds"))
			Expect(rb.ObjectMeta.Name).To(Equal("green-istio-ingressgateway-sds"))

			// Check if sa there in the cluster
			skey := types.NamespacedName{Name: "green-istio-ingressgateway-service-account", Namespace: "kubeslice-system"}
			sa := &corev1.ServiceAccount{}

			// Wait until sa is created properly
			Eventually(func() bool {
				err := k8sClient.Get(ctx, skey, sa)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			Expect(sa.ObjectMeta.Name).To(Equal("green-istio-ingressgateway-service-account"))

		})

		It("Should add slice ownerrefernce to the objects", func() {
			err := manifest.InstallIngress(ctx, k8sClient, slice)
			Expect(err).NotTo(HaveOccurred())

			// Check if deployment is there in the cluster
			deployKey := types.NamespacedName{Name: "green-istio-ingressgateway", Namespace: "kubeslice-system"}
			createdDeploy := &appsv1.Deployment{}

			// Wait until deployment is created properly
			Eventually(func() bool {
				err := k8sClient.Get(ctx, deployKey, createdDeploy)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			Expect(createdDeploy.ObjectMeta.Name).To(Equal("green-istio-ingressgateway"))

			own := createdDeploy.ObjectMeta.OwnerReferences
			Expect(len(own)).To(Equal(1))
			Expect(own[0].APIVersion).To(Equal("mesh.avesha.io/v1beta1"))
			Expect(string(own[0].UID)).To(Equal("test-uid"))
			Expect(own[0].Kind).To(Equal("Slice"))
			Expect(own[0].Name).To(Equal("green"))

		})

	})

	Context("With ingress gw installed", func() {

		var deploy *appsv1.Deployment

		BeforeEach(func() {

			deploy = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "green-istio-ingressgateway",
					Namespace: "kubeslice-system",
				},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"test": "test"},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{"test": "test"},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{{
								Name:  "nginx",
								Image: "nginx",
							}},
						},
					},
				},
			}

		})

		It("Should cleanup the resources", func() {

			err := k8sClient.Create(ctx, deploy)
			Expect(err).ToNot(HaveOccurred())

			// Check if deployment is created
			deployKey := types.NamespacedName{Name: "green-istio-ingressgateway", Namespace: "kubeslice-system"}
			deploy := &appsv1.Deployment{}

			// Wait until deployment is deleted properly
			Eventually(func() bool {
				err := k8sClient.Get(ctx, deployKey, deploy)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			err = manifest.UninstallIngress(ctx, k8sClient, "green")
			Expect(err).ToNot(HaveOccurred())

			// Wait until deployment is deleted properly
			Eventually(func() bool {
				err := k8sClient.Get(ctx, deployKey, deploy)
				return errors.IsNotFound(err)
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

		})

	})

})
