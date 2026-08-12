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

package slicegwrecycler

import (
	"testing"

	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNewVPNClientEmulator(t *testing.T) {
	client, err := NewVPNClientEmulator(nil)
	if err != nil {
		t.Errorf("NewVPNClientEmulator() error = %v", err)
	}
	if client == nil {
		t.Error("NewVPNClientEmulator() returned nil client")
	}
}

func TestVPNClientEmulator_TriggerFSM(t *testing.T) {
	emulator, _ := NewVPNClientEmulator(nil)

	sliceGw := &kubeslicev1beta1.SliceGateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-gateway",
			Namespace: "test-ns",
		},
	}

	slice := &kubeslicev1beta1.Slice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slice",
			Namespace: "test-ns",
		},
	}

	err := emulator.TriggerFSM(sliceGw, slice, "server-1", "client-1", "test-controller")
	if err != nil {
		t.Errorf("TriggerFSM() error = %v, expected nil", err)
	}
}
