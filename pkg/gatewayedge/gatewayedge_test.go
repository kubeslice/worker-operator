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

package gatewayedge

import (
	"context"
	"errors"
	"testing"
)

// test-only interface; production source keeps returning *gwEdgeClient.
type gatewayEdgeUpdater interface {
	UpdateSliceGwServiceMap(ctx context.Context, serverAddr string, gwSvcMap *SliceGwServiceMap) (*GwEdgeResponse, error)
}

type fakeGatewayEdgeClient struct {
	updateErr error
}

func (f *fakeGatewayEdgeClient) UpdateSliceGwServiceMap(ctx context.Context, serverAddr string, gwSvcMap *SliceGwServiceMap) (*GwEdgeResponse, error) {
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	return &GwEdgeResponse{}, nil
}

func TestNewWorkerGatewayEdgeClientProvider(t *testing.T) {
	client, err := NewWorkerGatewayEdgeClientProvider()
	if err != nil {
		t.Errorf("NewWorkerGatewayEdgeClientProvider() error = %v", err)
	}
	if client == nil {
		t.Error("NewWorkerGatewayEdgeClientProvider() returned nil client")
	}
}

func TestUpdateSliceGwServiceMap_FakeClient(t *testing.T) {
	tests := []struct {
		name      string
		client    gatewayEdgeUpdater
		expectErr bool
	}{
		{
			name:      "successful update",
			client:    &fakeGatewayEdgeClient{},
			expectErr: false,
		},
		{
			name:      "update error",
			client:    &fakeGatewayEdgeClient{updateErr: errors.New("update failed")},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.client.UpdateSliceGwServiceMap(context.Background(), "dummy:1234", &SliceGwServiceMap{})
			if (err != nil) != tt.expectErr {
				t.Errorf("UpdateSliceGwServiceMap() error = %v, expectErr %v", err, tt.expectErr)
			}
		})
	}
}

func TestUpdateSliceGwServiceMap_DialError(t *testing.T) {
	client := &gwEdgeClient{}
	_, err := client.UpdateSliceGwServiceMap(context.Background(), "127.0.0.1:1", &SliceGwServiceMap{})
	if err == nil {
		t.Error("expected dial/RPC error for unreachable address, got nil")
	}
}
