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

package serviceexport

import (
	"context"
	"strings"
	"testing"

	"github.com/kubeslice/worker-operator/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestValidateServiceExport(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1beta1.AddToScheme(scheme)
	
	tests := []struct {
		name        string
		svcExport   *v1beta1.ServiceExport
		slice       *v1beta1.Slice
		namespace   string
		shouldError bool
		errorMsg    string
	}{
		{
			name: "valid serviceexport",
			svcExport: &v1beta1.ServiceExport{
				ObjectMeta: metav1.ObjectMeta{Name: "test-svc", Namespace: "app-ns"},
				Spec: v1beta1.ServiceExportSpec{
					Slice: "test-slice",
				},
			},
			slice: &v1beta1.Slice{
				ObjectMeta: metav1.ObjectMeta{Name: "test-slice", Namespace: "app-ns"},
				Status: v1beta1.SliceStatus{
					SliceConfig: &v1beta1.SliceConfig{
						NamespaceIsolationProfile: &v1beta1.NamespaceIsolationProfile{
							ApplicationNamespaces: []string{"app-ns"},
						},
					},
				},
			},
			namespace:   "app-ns",
			shouldError: false,
		},
		{
			name: "slice not found",
			svcExport: &v1beta1.ServiceExport{
				ObjectMeta: metav1.ObjectMeta{Name: "test-svc", Namespace: "app-ns"},
				Spec: v1beta1.ServiceExportSpec{
					Slice: "nonexistent-slice",
				},
			},
			namespace:   "app-ns",
			shouldError: true,
			errorMsg:    "slice 'nonexistent-slice' not found on cluster",
		},
		{
			name: "namespace not in application namespaces",
			svcExport: &v1beta1.ServiceExport{
				ObjectMeta: metav1.ObjectMeta{Name: "test-svc", Namespace: "wrong-ns"},
				Spec: v1beta1.ServiceExportSpec{
					Slice: "test-slice",
				},
			},
			slice: &v1beta1.Slice{
				ObjectMeta: metav1.ObjectMeta{Name: "test-slice", Namespace: "wrong-ns"},
				Status: v1beta1.SliceStatus{
					SliceConfig: &v1beta1.SliceConfig{
						NamespaceIsolationProfile: &v1beta1.NamespaceIsolationProfile{
							ApplicationNamespaces: []string{"app-ns"},
						},
					},
				},
			},
			namespace:   "wrong-ns",
			shouldError: true,
			errorMsg:    "namespace 'wrong-ns' is not an onboarded application namespace for slice 'test-slice'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var objs []runtime.Object
			if tt.slice != nil {
				objs = append(objs, tt.slice)
			}
			
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objs...).Build()
			validator := &ServiceExportValidator{
				Client: fakeClient,
			}

			err := validator.validateServiceExport(context.Background(), tt.svcExport, tt.namespace)
			
			if tt.shouldError {
				if err == nil {
					t.Errorf("expected error but got none")
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error message to contain '%s' but got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error but got: %v", err)
				}
			}
		})
	}
}
