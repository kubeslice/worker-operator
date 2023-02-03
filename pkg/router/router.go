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

	sidecar "github.com/kubeslice/router-sidecar/pkg/sidecar/sidecarpb"
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

type SliceRouterConnCtx struct {
	RemoteSliceGwNsmSubnet string
	LocalNsmGwPeerIPs      []string
}

type routerSidecarClient struct {
}
type UpdateEcmpInfo struct {
	RemoteSliceGwNsmSubnet string
	NsmIpToDelete          string
}
type GetRouteConfig struct {
	RemoteSliceGwNsmSubnet string
	NsmIp                  string
}

func NewWorkerRouterClientProvider() (*routerSidecarClient, error) {
	return &routerSidecarClient{}, nil
}

func (worker routerSidecarClient) GetClientConnectionInfo(ctx context.Context, addr string) ([]kubeslicev1beta1.AppPod, error) {
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	client := sidecar.NewSliceRouterSidecarServiceClient(conn)
	info, err := client.GetSliceRouterClientConnectionInfo(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, err
	}

	var appPods []kubeslicev1beta1.AppPod

	for _, c := range info.Connection {
		appPods = append(appPods, kubeslicev1beta1.AppPod{
			PodName:      c.PodName,
			NsmInterface: c.NsmInterface,
			NsmIP:        c.NsmIP,
			NsmPeerIP:    c.NsmPeerIP,
		})
	}

	return appPods, nil
}

func (worker routerSidecarClient) SendConnectionContext(ctx context.Context, serverAddr string, sliceRouterConnCtx *SliceRouterConnCtx) error {
	conn, err := grpc.Dial(serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer conn.Close()

	msg := &sidecar.SliceGwConContext{
		RemoteSliceGwNsmSubnet: sliceRouterConnCtx.RemoteSliceGwNsmSubnet,
		LocalNsmGwPeerIPList:   sliceRouterConnCtx.LocalNsmGwPeerIPs,
	}

	client := sidecar.NewSliceRouterSidecarServiceClient(conn)

	_, err = client.UpdateSliceGwConnectionContext(ctx, msg)

	return err
}
func (worker routerSidecarClient) UpdateEcmpRoutes(ctx context.Context, serverAddr string, sliceRouterConnCtx *UpdateEcmpInfo) error {
	conn, err := grpc.Dial(serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer conn.Close()
	msg := &sidecar.EcmpUpdateInfo{
		RemoteSliceGwNsmSubnet: sliceRouterConnCtx.RemoteSliceGwNsmSubnet,
		NsmIPToRemove:          sliceRouterConnCtx.NsmIpToDelete,
	}
	client := sidecar.NewSliceRouterSidecarServiceClient(conn)
	_, err = client.UpdateEcmpRoutes(ctx, msg)
	return err
}

func (worker routerSidecarClient) GetRouteInKernel(ctx context.Context, serverAddr string, sliceRouterConnCtx *GetRouteConfig) (*sidecar.VerifyRouteAddResponse, error) {
	conn, err := grpc.Dial(serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	msg := &sidecar.VerifyRouteAddRequest{
		NsmIP: sliceRouterConnCtx.NsmIp,
		DstIP: sliceRouterConnCtx.RemoteSliceGwNsmSubnet,
	}
	client := sidecar.NewSliceRouterSidecarServiceClient(conn)
	res, err := client.GetRouteInKernel(ctx, msg)
	return res, err
}
