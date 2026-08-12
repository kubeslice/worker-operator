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

package manifest

import (
	"context"
	"errors"
	"os"
	"testing"

	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	utilmock "github.com/kubeslice/worker-operator/pkg/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestUninstallIngress(t *testing.T) {
	tests := []struct {
		name          string
		sliceName     string
		setupMock     func(*utilmock.MockClient)
		expectedError bool
	}{
		{
			name:      "successfully uninstall all resources",
			sliceName: "test-slice",
			setupMock: func(mc *utilmock.MockClient) {
				mc.On("Delete",
					mock.Anything,
					mock.IsType(&appsv1.Deployment{}),
					mock.Anything,
				).Return(nil)
				mc.On("Delete",
					mock.Anything,
					mock.IsType(&corev1.Service{}),
					mock.Anything,
				).Return(nil)
				mc.On("Delete",
					mock.Anything,
					mock.IsType(&rbacv1.Role{}),
					mock.Anything,
				).Return(nil)
				mc.On("Delete",
					mock.Anything,
					mock.IsType(&corev1.ServiceAccount{}),
					mock.Anything,
				).Return(nil)
				mc.On("Delete",
					mock.Anything,
					mock.IsType(&rbacv1.RoleBinding{}),
					mock.Anything,
				).Return(nil)
				mc.On("Delete",
					mock.Anything,
					mock.Anything,
					mock.Anything,
				).Return(nil)
			},
			expectedError: false,
		},
		{
			name:      "resources already deleted (not found)",
			sliceName: "test-slice",
			setupMock: func(mc *utilmock.MockClient) {
				notFoundErr := kerrors.NewNotFound(schema.GroupResource{}, "resource")
				mc.On("Delete",
					mock.Anything,
					mock.Anything,
					mock.Anything,
				).Return(notFoundErr)
			},
			expectedError: false,
		},
		{
			name:      "error deleting resource",
			sliceName: "test-slice",
			setupMock: func(mc *utilmock.MockClient) {
				mc.On("Delete",
					mock.Anything,
					mock.Anything,
					mock.Anything,
				).Return(errors.New("delete failed"))
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := utilmock.NewClient()
			tt.setupMock(mockClient)

			err := UninstallIngress(context.Background(), mockClient, tt.sliceName)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUninstallEgress(t *testing.T) {
	tests := []struct {
		name          string
		sliceName     string
		setupMock     func(*utilmock.MockClient)
		expectedError bool
	}{
		{
			name:      "successfully uninstall all resources",
			sliceName: "test-slice",
			setupMock: func(mc *utilmock.MockClient) {
				mc.On("Delete",
					mock.Anything,
					mock.IsType(&appsv1.Deployment{}),
					mock.Anything,
				).Return(nil)
				mc.On("Delete",
					mock.Anything,
					mock.IsType(&corev1.Service{}),
					mock.Anything,
				).Return(nil)
				mc.On("Delete",
					mock.Anything,
					mock.IsType(&rbacv1.Role{}),
					mock.Anything,
				).Return(nil)
				mc.On("Delete",
					mock.Anything,
					mock.IsType(&corev1.ServiceAccount{}),
					mock.Anything,
				).Return(nil)
				mc.On("Delete",
					mock.Anything,
					mock.IsType(&rbacv1.RoleBinding{}),
					mock.Anything,
				).Return(nil)
				mc.On("Delete",
					mock.Anything,
					mock.Anything,
					mock.Anything,
				).Return(nil)
			},
			expectedError: false,
		},
		{
			name:      "resources already deleted (not found)",
			sliceName: "test-slice",
			setupMock: func(mc *utilmock.MockClient) {
				notFoundErr := kerrors.NewNotFound(schema.GroupResource{}, "resource")
				mc.On("Delete",
					mock.Anything,
					mock.Anything,
					mock.Anything,
				).Return(notFoundErr)
			},
			expectedError: false,
		},
		{
			name:      "error deleting resource",
			sliceName: "test-slice",
			setupMock: func(mc *utilmock.MockClient) {
				mc.On("Delete",
					mock.Anything,
					mock.Anything,
					mock.Anything,
				).Return(errors.New("delete failed"))
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := utilmock.NewClient()
			tt.setupMock(mockClient)

			err := UninstallEgress(context.Background(), mockClient, tt.sliceName)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestIstioProxyImageDefault(t *testing.T) {
	tests := []struct {
		name         string
		envValue     string
		setEnv       bool
		expectedUsed string
	}{
		{
			name:         "uses default when env not set",
			envValue:     "",
			setEnv:       false,
			expectedUsed: ISTIO_PROXY_DEFAULT_IMAGE,
		},
		{
			name:         "uses custom image when env set",
			envValue:     "custom/istio:latest",
			setEnv:       true,
			expectedUsed: "custom/istio:latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setEnv {
				os.Setenv("AVESHA_ISTIO_PROXY_IMAGE", tt.envValue)
				defer os.Unsetenv("AVESHA_ISTIO_PROXY_IMAGE")
			} else {
				os.Unsetenv("AVESHA_ISTIO_PROXY_IMAGE")
			}

			result := os.Getenv("AVESHA_ISTIO_PROXY_IMAGE")
			if result == "" {
				result = ISTIO_PROXY_DEFAULT_IMAGE
			}

			assert.Equal(t, tt.expectedUsed, result)
		})
	}
}

func TestInstallIngressCreateError(t *testing.T) {
	tests := []struct {
		name          string
		errorOnCreate bool
		alreadyExists bool
	}{
		{
			name:          "create error is returned",
			errorOnCreate: true,
			alreadyExists: false,
		},
		{
			name:          "already exists error is ignored",
			errorOnCreate: true,
			alreadyExists: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := utilmock.NewClient()
			scheme := runtime.NewScheme()
			_ = kubeslicev1beta1.AddToScheme(scheme)
			_ = corev1.AddToScheme(scheme)
			_ = appsv1.AddToScheme(scheme)
			_ = rbacv1.AddToScheme(scheme)

			mockClient.On("Scheme").Return(scheme)

			var createErr error
			if tt.alreadyExists {
				createErr = kerrors.NewAlreadyExists(schema.GroupResource{}, "resource")
			} else {
				createErr = errors.New("create failed")
			}

			mockClient.On("Create",
				mock.Anything,
				mock.Anything,
				mock.Anything,
			).Return(createErr)

			slice := &kubeslicev1beta1.Slice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slice",
					Namespace: "kubeslice-system",
				},
			}

			err := InstallIngress(context.Background(), mockClient, slice)

			if tt.alreadyExists {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestInstallEgressCreateError(t *testing.T) {
	tests := []struct {
		name          string
		errorOnCreate bool
		alreadyExists bool
	}{
		{
			name:          "create error is returned",
			errorOnCreate: true,
			alreadyExists: false,
		},
		{
			name:          "already exists error is ignored",
			errorOnCreate: true,
			alreadyExists: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := utilmock.NewClient()
			scheme := runtime.NewScheme()
			_ = kubeslicev1beta1.AddToScheme(scheme)
			_ = corev1.AddToScheme(scheme)
			_ = appsv1.AddToScheme(scheme)
			_ = rbacv1.AddToScheme(scheme)

			mockClient.On("Scheme").Return(scheme)

			var createErr error
			if tt.alreadyExists {
				createErr = kerrors.NewAlreadyExists(schema.GroupResource{}, "resource")
			} else {
				createErr = errors.New("create failed")
			}

			mockClient.On("Create",
				mock.Anything,
				mock.Anything,
				mock.Anything,
			).Return(createErr)

			slice := &kubeslicev1beta1.Slice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slice",
					Namespace: "kubeslice-system",
				},
			}

			err := InstallEgress(context.Background(), mockClient, slice)

			if tt.alreadyExists {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}
