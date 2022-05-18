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

package controllers

import (
	"context"

	"github.com/stretchr/testify/mock"
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

func (c *MockClient) Get(ctx context.Context, key types.NamespacedName, obj client.Object) error {
	// obj = obj.(*spokev1alpha1.SpokeSliceConfig)
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

func (c *StatusClient) Update(
	ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	args := c.Called(ctx, obj, opts)
	return args.Error(0)
}

func (c *StatusClient) Patch(
	ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	args := c.Called(ctx, obj, patch, opts)
	return args.Error(0)
}
