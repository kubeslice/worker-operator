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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kubeslice/operator/internal/manifest"
	appsv1 "k8s.io/api/apps/v1"
)

var _ = Describe("Manifest File", func() {

	Context("With k8s deployment manifest", func() {

		f := "egress-deploy"

		It("Should parse file into k8s deployment in the slice", func() {

			m := manifest.NewManifest(f, "green")

			Expect(m).ToNot(BeNil())

			deploy := &appsv1.Deployment{}
			err := m.Parse(deploy)
			Expect(err).NotTo(HaveOccurred())
			Expect(deploy).ToNot(BeNil())

			Expect(deploy.ObjectMeta.Name).Should(Equal("green-istio-egressgateway"))

		})

	})

})
