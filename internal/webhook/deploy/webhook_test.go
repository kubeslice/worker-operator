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

package deploy_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubeslice/worker-operator/internal/webhook/deploy"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type fakeWebhookClient struct{}

func (f fakeWebhookClient) GetSliceNamespaceIsolationPolicy(ctx context.Context, slice string) (bool, error) {
	return true, nil
}

func (f fakeWebhookClient) UpdateSliceApplicationNamespaces(ctx context.Context, slice string, namespace string) error {
	return nil
}

func (f fakeWebhookClient) SliceAppNamespaceConfigured(ctx context.Context, slice string, namespace string) (bool, error) {
	return true, nil
}
func (f fakeWebhookClient) GetNamespaceLabels(ctx context.Context, client client.Client, namespace string) (map[string]string, error) {
	return map[string]string{"kubeslice.io/slice": "green"}, nil
}

var _ = Describe("Deploy Webhook", func() {
	fakeWhClient := new(fakeWebhookClient)
	webhookServer := deploy.WebhookServer{
		WhClient: fakeWhClient,
	}
	Describe("MutationRequired", func() {
		Context("New deployment with proper annotation", func() {

			table := []metav1.ObjectMeta{
				{
					Annotations: map[string]string{
						"avesha.io/slice": "green",
					}, // with proper annotations
				},
				{
					Annotations: map[string]string{
						"avesha.io/slice":  "green",
						"avesha.io/status": "",
					}, // with empty value for status key
				},
				{
					Annotations: map[string]string{
						"avesha.io/slice":  "green",
						"avesha.io/status": "not injected",
					}, // with different value for status key
				},
			}

			It("should enable injection", func() {

				for _, meta := range table {
					is, _ := webhookServer.MutationRequired(meta)
					Expect(is).To(BeTrue())
				}

			})
		})

		Context("Already Injected", func() {

			table := []metav1.ObjectMeta{
				{
					Annotations: map[string]string{
						"avesha.io/status": "injected",
					}, // with injection status
				},
				{
					Labels: map[string]string{
						"avesha.io/pod-type": "app",
					}, // with pod type set
				},
				{
					Annotations: map[string]string{
						"avesha.io/status": "injected",
					}, // with injection status
					Labels: map[string]string{
						"avesha.io/pod-type": "app",
					}, // with pod type set
				},
				{
					Namespace: "kubeslice-system",
				}, // Deployed in control plane namespace
			}
			It("Should skip injection", func() {

				for _, meta := range table {
					is, _ := webhookServer.MutationRequired(meta)
					Expect(is).To(BeFalse())
				}

			})
		})
		Context("Empty annotations,but namespace is part of slice->auto netpol", func() {
			table := []metav1.ObjectMeta{
				{
					Annotations: map[string]string{}, //empty
					Namespace:   "test-namespace",
				},
			}
			It("should enable injection", func() {
				for _, meta := range table {
					is, _ := webhookServer.MutationRequired(meta)
					Expect(is).To(BeTrue())
				}
			})
		})
	})

})
