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

	sidecar "github.com/kubeslice/router-sidecar/pkg/proto"
	meshv1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

type SliceRouterConnCtx struct {
	RemoteSliceGwNsmSubnet string
	LocalNsmGwPeerIP       string
}

func GetClientConnectionInfo(ctx context.Context, addr string) ([]meshv1beta1.AppPod, error) {
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

	var appPods []meshv1beta1.AppPod

	for _, c := range info.Connection {
		appPods = append(appPods, meshv1beta1.AppPod{
			PodName:      c.PodName,
			NsmInterface: c.NsmInterface,
			NsmIP:        c.NsmIP,
			NsmPeerIP:    c.NsmPeerIP,
		})
	}

	return appPods, nil
}

func SendConnectionContext(ctx context.Context, serverAddr string, sliceRouterConnCtx *SliceRouterConnCtx) error {
	conn, err := grpc.Dial(serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer conn.Close()

	msg := &sidecar.SliceGwConContext{
		RemoteSliceGwNsmSubnet: sliceRouterConnCtx.RemoteSliceGwNsmSubnet,
		LocalNsmGwPeerIP:       sliceRouterConnCtx.LocalNsmGwPeerIP,
	}

	client := sidecar.NewSliceRouterSidecarServiceClient(conn)

	_, err = client.UpdateSliceGwConnectionContext(ctx, msg)

	return err
}
