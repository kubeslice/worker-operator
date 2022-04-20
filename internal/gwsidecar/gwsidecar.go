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

package gwsidecar

import (
	"context"

	sidecar "bitbucket.org/realtimeai/kubeslice-gw-sidecar/pkg/sidecar/sidecarpb"
	empty "github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"
)

type NsmStatus struct {
	IntfName string
	LocalIP  string
}

type TunnelStatus struct {
	IntfName string
	LocalIP  string
	RemoteIP string
	Latency  uint64
	TxRate   uint64
	RxRate   uint64
}

type GwStatus struct {
	NsmStatus
	TunnelStatus
}

type GwConnectionContext struct {
	RemoteSliceGwVpnIP     string
	RemoteSliceGwNsmSubnet string
}

// GetStatus retrieves sidecar status
func GetStatus(ctx context.Context, serverAddr string) (*GwStatus, error) {
	conn, err := grpc.Dial(serverAddr, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	client := sidecar.NewGwSidecarServiceClient(conn)
	res, err := client.GetStatus(ctx, &empty.Empty{})
	if err != nil {
		return nil, err
	}

	gwStatus := &GwStatus{}

	if res.NsmIntfStatus != nil {
		gwStatus.NsmStatus = NsmStatus{
			IntfName: res.NsmIntfStatus.NsmInterfaceName,
			LocalIP:  res.NsmIntfStatus.NsmIP,
		}
	}
	if res.TunnelStatus != nil {
		gwStatus.TunnelStatus = TunnelStatus{
			IntfName: res.TunnelStatus.NetInterface,
			LocalIP:  res.TunnelStatus.LocalIP,
			RemoteIP: res.TunnelStatus.PeerIP,
		}
	}

	return gwStatus, err
}

// SendConnectionContext sends connection context info to sidecar
func SendConnectionContext(ctx context.Context, serverAddr string, gwConnCtx *GwConnectionContext) error {
	conn, err := grpc.Dial(serverAddr, grpc.WithInsecure())
	if err != nil {
		return err
	}
	defer conn.Close()

	client := sidecar.NewGwSidecarServiceClient(conn)

	msg := &sidecar.SliceGwConnectionContext{
		RemoteSliceGwVpnIP:     gwConnCtx.RemoteSliceGwVpnIP,
		RemoteSliceGwNsmSubnet: gwConnCtx.RemoteSliceGwNsmSubnet,
	}

	_, err = client.UpdateConnectionContext(ctx, msg)

	return err
}
