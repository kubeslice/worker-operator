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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"bitbucket.org/realtimeai/kubeslice-operator/internal/webhook/deploy"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Deploy Webhook", func() {

	Describe("MutationRequired", func() {
		Context("New deployment without proper annotation", func() {

			table := []metav1.ObjectMeta{
				{}, // with empty meta
				{
					Annotations: map[string]string{}, // with empty annotations
				},
				{
					Annotations: map[string]string{
						"avesha.io/status": "",
					}, // with empty value for status key
				},
				{
					Annotations: map[string]string{
						"avesha.io/slice": "",
					}, // with empty value for slice key
				},
				{
					Annotations: map[string]string{
						"avesha.io/status": "not injected",
					}, // with different value for status key
				},
			}

			It("should not enable injection", func() {

				for _, meta := range table {
					is := deploy.MutationRequired(meta)
					Expect(is).To(BeFalse())
				}

			})
		})

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
					is := deploy.MutationRequired(meta)
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
					is := deploy.MutationRequired(meta)
					Expect(is).To(BeFalse())
				}

			})
		})
	})

})
