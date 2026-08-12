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

package router

import (
	"context"
	"testing"

	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	sidecar "github.com/kubeslice/router-sidecar/pkg/sidecar/sidecarpb"
)

type connectionInfo struct {
	podName      string
	nsmInterface string
	nsmIP        string
	nsmPeerIP    string
}

type fakeRouterClient struct {
	connections []connectionInfo
	sendErr     error
	updateErr   error
	getErr      error
}

func (f *fakeRouterClient) GetClientConnectionInfo(ctx context.Context, addr string) ([]kubeslicev1beta1.AppPod, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	var appPods []kubeslicev1beta1.AppPod
	for _, c := range f.connections {
		appPods = append(appPods, kubeslicev1beta1.AppPod{
			PodName:      c.podName,
			NsmInterface: c.nsmInterface,
			NsmIP:        c.nsmIP,
			NsmPeerIP:    c.nsmPeerIP,
		})
	}
	return appPods, nil
}

func (f *fakeRouterClient) SendConnectionContext(ctx context.Context, serverAddr string, sliceRouterConnCtx *SliceRouterConnCtx) error {
	return f.sendErr
}

func (f *fakeRouterClient) UpdateEcmpRoutes(ctx context.Context, serverAddr string, sliceRouterConnCtx *UpdateEcmpInfo) error {
	return f.updateErr
}

func (f *fakeRouterClient) GetRouteInKernel(ctx context.Context, serverAddr string, sliceRouterConnCtx *GetRouteConfig) (*sidecar.VerifyRouteAddResponse, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	return &sidecar.VerifyRouteAddResponse{}, nil
}

func TestNewWorkerRouterClientProvider(t *testing.T) {
	client, err := NewWorkerRouterClientProvider()
	if err != nil {
		t.Errorf("NewWorkerRouterClientProvider() error = %v", err)
	}
	if client == nil {
		t.Error("NewWorkerRouterClientProvider() returned nil client")
	}
}

func TestGetClientConnectionInfo_Mapping(t *testing.T) {
	fake := &fakeRouterClient{
		connections: []connectionInfo{
			{
				podName:      "pod1",
				nsmInterface: "nsm0",
				nsmIP:        "10.0.0.1",
				nsmPeerIP:    "10.0.0.2",
			},
			{
				podName:      "pod2",
				nsmInterface: "nsm1",
				nsmIP:        "10.0.0.3",
				nsmPeerIP:    "10.0.0.4",
			},
		},
	}

	appPods, err := fake.GetClientConnectionInfo(context.Background(), "dummy:1234")
	if err != nil {
		t.Errorf("GetClientConnectionInfo() error = %v", err)
	}

	if len(appPods) != 2 {
		t.Errorf("GetClientConnectionInfo() returned %d pods, want 2", len(appPods))
	}

	if appPods[0].PodName != "pod1" || appPods[0].NsmIP != "10.0.0.1" {
		t.Errorf("GetClientConnectionInfo() pod mapping incorrect")
	}

	if appPods[1].PodName != "pod2" || appPods[1].NsmInterface != "nsm1" {
		t.Errorf("GetClientConnectionInfo() pod mapping incorrect")
	}
}

func TestGetClientConnectionInfo_Empty(t *testing.T) {
	fake := &fakeRouterClient{
		connections: []connectionInfo{},
	}

	appPods, err := fake.GetClientConnectionInfo(context.Background(), "dummy:1234")
	if err != nil {
		t.Errorf("GetClientConnectionInfo() error = %v", err)
	}

	if len(appPods) != 0 {
		t.Errorf("GetClientConnectionInfo() returned %d pods, want 0", len(appPods))
	}
}
