/*
 *  Copyright (c) 2022 Avesha, Inc. All rights reserved.
 *
 *	SPDX-License-Identifier: Apache-2.0
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

package mocks

import (
	"context"

	monitoringEvents "github.com/kubeslice/kubeslice-monitoring/pkg/events"
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	hub "github.com/kubeslice/worker-operator/pkg/hub/hubclient"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Client is a mock for the controller-runtime dynamic client interface.
type MockClient struct {
	client.Client
	mock.Mock
	StatusMock *StatusClient
}

var _ client.Client = &MockClient{}

func NewClient() *MockClient {
	return &MockClient{
		StatusMock: &StatusClient{},
	}
}

func (c *MockClient) Status() client.StatusWriter {
	return c.StatusMock
}

// Reader interface
func (c *MockClient) Get(ctx context.Context, key types.NamespacedName, obj client.Object, opts ...client.GetOption) error {
	args := c.Called(ctx, key, obj)
	return args.Error(0)
}
func (c *MockClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	args := c.Called(ctx, list, opts)
	return args.Error(0)
}

// Writer interface
func (c *MockClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	args := c.Called(ctx, obj, opts)
	return args.Error(0)
}

func (c *MockClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	args := c.Called(ctx, obj, opts)
	return args.Error(0)
}

func (c *MockClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	args := c.Called(ctx, obj, opts)
	return args.Error(0)
}

func (c *MockClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	args := c.Called(ctx, obj, patch, opts)
	return args.Error(0)
}

func (c *MockClient) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	args := c.Called(ctx, obj, opts)
	return args.Error(0)
}

func (c *MockClient) TriggerFSM(ctx context.Context, sliceGw *kubeslicev1beta1.SliceGateway,
	slice *kubeslicev1beta1.Slice, hubClient *hub.HubClientConfig, meshClient client.Client, gatewayPod *corev1.Pod,
	eventRecorder *monitoringEvents.EventRecorder, controllerName, gwRecyclerName string) (bool, error) {
	// Define the arguments you expect in the method call
	args := c.Called(ctx, sliceGw, slice, hubClient, meshClient, gatewayPod, eventRecorder, controllerName, gwRecyclerName)
	// Extract the return values from the arguments
	return args.Bool(0), args.Error(1)
}

func (c *MockClient) Scheme() *runtime.Scheme {
	args := c.Called()
	return args.Get(0).(*runtime.Scheme)
}

func (c *MockClient) RESTMapper() meta.RESTMapper {
	args := c.Called()
	return args.Get(0).(meta.RESTMapper)
}

type StatusClient struct {
	mock.Mock
}

var _ client.StatusWriter = &StatusClient{}

func (c *StatusClient) Create(ctx context.Context, obj1 client.Object, obj2 client.Object, opts ...client.SubResourceCreateOption) error {
	args := c.Called(ctx, obj1, obj2, opts)
	return args.Error(0)
}

func (c *StatusClient) Update(
	ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
	args := c.Called(ctx, obj, opts)
	return args.Error(0)
}

func (c *StatusClient) Patch(
	ctx context.Context, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
	args := c.Called(ctx, obj, patch, opts)
	return args.Error(0)
}
