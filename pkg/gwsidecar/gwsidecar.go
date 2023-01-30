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

	empty "github.com/golang/protobuf/ptypes/empty"
	sidecar "github.com/kubeslice/gateway-sidecar/pkg/sidecar/sidecarpb"
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	"github.com/kubeslice/worker-operator/pkg/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type NsmStatus struct {
	IntfName string
	LocalIP  string
}
type TunnelStatus struct {
	IntfName   string
	LocalIP    string
	RemoteIP   string
	Latency    uint64
	TxRate     uint64
	RxRate     uint64
	PacketLoss uint64
	Status     int32
}
type GwStatus struct {
	NsmStatus
	TunnelStatus
}

type GwConnectionContext struct {
	RemoteSliceGwVpnIP     string
	RemoteSliceGwNsmSubnet string
}

type gwSidecarClient struct {
}

func NewWorkerGWSidecarClientProvider() (*gwSidecarClient, error) {
	return &gwSidecarClient{}, nil
}

func (worker gwSidecarClient) GetSliceGwRemotePodName(ctx context.Context, gwRemoteVpnIP string, serverAddr string) (string, error) {
	conn, err := grpc.Dial(serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return "", err
	}
	defer conn.Close()
	client := sidecar.NewGwSidecarServiceClient(conn)

	gwPodIP := &sidecar.RemoteGwVpnIP{
		RemoteGwVpnIP: gwRemoteVpnIP,
	}

	res, err := client.GetSliceGwRemotePodName(ctx, gwPodIP)
	if err != nil {
		return "", err
	}
	return res.GatewayPodName, nil
}

// GetStatus retrieves sidecar status
func (worker gwSidecarClient) GetStatus(ctx context.Context, serverAddr string) (*GwStatus, error) {
	conn, err := grpc.Dial(serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
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
			IntfName:   res.TunnelStatus.NetInterface,
			LocalIP:    res.TunnelStatus.LocalIP,
			RemoteIP:   res.TunnelStatus.PeerIP,
			PacketLoss: res.TunnelStatus.PacketLoss,
			Status:     int32(res.TunnelStatus.Status),
		}
	} else {
		gwStatus.TunnelStatus.Status = int32(sidecar.TunnelStatusType_GW_TUNNEL_STATE_DOWN)
	}

	return gwStatus, err
}

// SendConnectionContext sends connection context info to sidecar
func (worker gwSidecarClient) SendConnectionContext(ctx context.Context, serverAddr string, gwConnCtx *GwConnectionContext) error {
	log := logger.FromContext(ctx)
	conn, err := grpc.Dial(serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer conn.Close()

	client := sidecar.NewGwSidecarServiceClient(conn)

	msg := &sidecar.SliceGwConnectionContext{
		RemoteSliceGwVpnIP:     gwConnCtx.RemoteSliceGwVpnIP,
		RemoteSliceGwNsmSubnet: gwConnCtx.RemoteSliceGwNsmSubnet,
	}

	log.Info("SliceGwConnectionContext", "SliceGwConnectionContext", msg)

	_, err = client.UpdateConnectionContext(ctx, msg)

	return err
}

func (worker gwSidecarClient) UpdateSliceQosProfile(ctx context.Context, serverAddr string, slice *kubeslicev1beta1.Slice) error {
	conn, err := grpc.Dial(serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer conn.Close()

	client := sidecar.NewGwSidecarServiceClient(conn)

	// TODO change later if we add more TC types
	tcType := sidecar.TcType_BANDWIDTH_CONTROL

	qop := &sidecar.SliceQosProfile{
		SliceName:      slice.Name,
		SliceId:        slice.Name,
		QosProfileName: slice.Name,
		TcType:         tcType,
		ClassType:      sidecar.ClassType_HTB,
		BwCeiling:      uint32(slice.Status.SliceConfig.QosProfileDetails.BandwidthCeilingKbps),
		BwGuaranteed:   uint32(slice.Status.SliceConfig.QosProfileDetails.BandwidthGuaranteedKbps),
		Priority:       uint32(slice.Status.SliceConfig.QosProfileDetails.Priority),
		DscpClass:      slice.Status.SliceConfig.QosProfileDetails.DscpClass,
	}
	_, err = client.UpdateSliceQosProfile(ctx, qop)

	return err
}
