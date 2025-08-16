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

package manifest_test

import (
	"context"
	"os"

	"github.com/kubeslice/worker-operator/api/v1beta1"
	"github.com/kubeslice/worker-operator/pkg/manifest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	istiov1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("Egress Manifest", func() {
	var (
		ctx        context.Context
		fakeClient client.Client
		slice      *v1beta1.Slice
		scheme     *runtime.Scheme
	)

	BeforeEach(func() {
		ctx = context.Background()

		scheme = runtime.NewScheme()
		_ = v1beta1.AddToScheme(scheme)
		_ = appsv1.AddToScheme(scheme)
		_ = corev1.AddToScheme(scheme)
		_ = rbacv1.AddToScheme(scheme)
		_ = istiov1beta1.AddToScheme(scheme)

		fakeClient = fake.NewClientBuilder().
			WithScheme(scheme).
			Build()

		slice = &v1beta1.Slice{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "green",
				Namespace: "kubeslice-system",
				UID:       types.UID("test-uid"),
			},
		}
	})

	Context("With InstallEgress function", func() {
		It("Should install all egress resources successfully in the slice", func() {
			err := manifest.InstallEgress(ctx, fakeClient, slice)
			Expect(err).NotTo(HaveOccurred())

			deploy := &appsv1.Deployment{}
			err = fakeClient.Get(ctx, types.NamespacedName{
				Name:      "green-istio-egressgateway",
				Namespace: "kubeslice-system",
			}, deploy)
			Expect(err).NotTo(HaveOccurred())
			Expect(deploy.ObjectMeta.Name).Should(Equal("green-istio-egressgateway"))
		})

		It("Should set controller reference on created resources", func() {
			err := manifest.InstallEgress(ctx, fakeClient, slice)
			Expect(err).NotTo(HaveOccurred())

			deploy := &appsv1.Deployment{}
			err = fakeClient.Get(ctx, types.NamespacedName{
				Name:      "green-istio-egressgateway",
				Namespace: "kubeslice-system",
			}, deploy)
			Expect(err).NotTo(HaveOccurred())
			Expect(deploy.OwnerReferences).ToNot(BeEmpty())
			Expect(deploy.OwnerReferences[0].UID).To(Equal(slice.UID))
		})

		It("Should use custom istio proxy image when environment variable is set", func() {
			customImage := "custom/istio-proxy:1.17.0"
			os.Setenv("AVESHA_ISTIO_PROXY_IMAGE", customImage)
			defer os.Unsetenv("AVESHA_ISTIO_PROXY_IMAGE")

			err := manifest.InstallEgress(ctx, fakeClient, slice)
			Expect(err).NotTo(HaveOccurred())

			deploy := &appsv1.Deployment{}
			err = fakeClient.Get(ctx, types.NamespacedName{
				Name:      "green-istio-egressgateway",
				Namespace: "kubeslice-system",
			}, deploy)
			Expect(err).NotTo(HaveOccurred())
			Expect(deploy.Spec.Template.Spec.Containers[0].Image).To(Equal(customImage))
		})

		It("Should use default image when no environment variable is set", func() {
			os.Unsetenv("AVESHA_ISTIO_PROXY_IMAGE")

			err := manifest.InstallEgress(ctx, fakeClient, slice)
			Expect(err).NotTo(HaveOccurred())

			deploy := &appsv1.Deployment{}
			err = fakeClient.Get(ctx, types.NamespacedName{
				Name:      "green-istio-egressgateway",
				Namespace: "kubeslice-system",
			}, deploy)
			Expect(err).NotTo(HaveOccurred())
			Expect(deploy.Spec.Template.Spec.Containers[0].Image).To(Equal(manifest.ISTIO_PROXY_DEFAULT_IMAGE))
		})

		It("Should handle already existing resources gracefully", func() {
			err := manifest.InstallEgress(ctx, fakeClient, slice)
			Expect(err).NotTo(HaveOccurred())

			err = manifest.InstallEgress(ctx, fakeClient, slice)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("With UninstallEgress function", func() {
		BeforeEach(func() {
			err := manifest.InstallEgress(ctx, fakeClient, slice)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should uninstall all egress resources successfully from the slice", func() {
			err := manifest.UninstallEgress(ctx, fakeClient, "green")
			Expect(err).NotTo(HaveOccurred())

			deploy := &appsv1.Deployment{}
			err = fakeClient.Get(ctx, types.NamespacedName{
				Name:      "green-istio-egressgateway",
				Namespace: "kubeslice-system",
			}, deploy)
			Expect(errors.IsNotFound(err)).To(BeTrue())
		})

		It("Should handle already deleted resources gracefully", func() {
			err := manifest.UninstallEgress(ctx, fakeClient, "green")
			Expect(err).NotTo(HaveOccurred())

			err = manifest.UninstallEgress(ctx, fakeClient, "green")
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
