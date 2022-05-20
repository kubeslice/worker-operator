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

package netop

import (
	"context"
	"strconv"

	sidecar "github.com/kubeslice/netops/pkg/proto"
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Generic event types enum
type EventType int32

const (
	EventType_EV_CREATE EventType = 0
	EventType_EV_UPDATE EventType = 1
	EventType_EV_DELETE EventType = 2
)

type netopSidecarClient struct {
	Client sidecar.NetOpsServiceClient
}

func NewWorkerNetOpClientProvider() (*netopSidecarClient, error) {
	return &netopSidecarClient{}, nil
}

func (spoke netopSidecarClient) UpdateSliceQosProfile(ctx context.Context, addr string, slice *kubeslicev1beta1.Slice) error {
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer conn.Close()

	client := sidecar.NewNetOpsServiceClient(conn)

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

func (spoke netopSidecarClient) SendSliceLifeCycleEventToNetOp(ctx context.Context, addr string, sliceName string, eventType EventType) error {
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer conn.Close()

	client := sidecar.NewNetOpsServiceClient(conn)

	// Transform event type enum in this pacakge to event type in netops package
	et := map[EventType]sidecar.EventType{
		EventType_EV_CREATE: 0,
		EventType_EV_UPDATE: 1,
		EventType_EV_DELETE: 2,
	}[eventType]

	sliceEvent := &sidecar.SliceLifeCycleEvent{
		SliceName: sliceName,
		Event:     et,
	}

	_, err = client.UpdateSliceLifeCycleEvent(ctx, sliceEvent)
	return err
}

// SendConnectionContext sends sonnectioncontext to netop sidecar
func (spoke netopSidecarClient) SendConnectionContext(ctx context.Context, serverAddr string, gw *kubeslicev1beta1.SliceGateway, sliceGwNodePort int32) error {
	conn, err := grpc.Dial(serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer conn.Close()

	client := sidecar.NewNetOpsServiceClient(conn)

	gwType := sidecar.SliceGwHostType_SLICE_GW_CLIENT
	remoteGwType := sidecar.SliceGwHostType_SLICE_GW_SERVER
	if gw.Status.Config.SliceGatewayHostType == "Server" {
		gwType = sidecar.SliceGwHostType_SLICE_GW_SERVER
		remoteGwType = sidecar.SliceGwHostType_SLICE_GW_CLIENT
	}

	//so many values needed?? netop only uses gwId, gwType, remoteport,localport
	c := &sidecar.NetOpConnectionContext{
		SliceId:               gw.Status.Config.SliceName,
		LocalSliceGwId:        gw.Status.Config.SliceGatewayID,
		LocalSliceGwVpnIP:     gw.Status.Config.SliceGatewayLocalVpnIP,
		LocalSliceGwHostType:  gwType,
		LocalSliceGwNsmSubnet: gw.Status.Config.SliceGatewaySubnet,
		//LocalSliceGwNodeIP:    nodeIP,
		LocalSliceGwNodePort: strconv.Itoa(int(sliceGwNodePort)),

		RemoteSliceGwId:        gw.Status.Config.SliceGatewayRemoteGatewayID,
		RemoteSliceGwVpnIP:     gw.Status.Config.SliceGatewayRemoteVpnIP,
		RemoteSliceGwHostType:  remoteGwType,
		RemoteSliceGwNsmSubnet: gw.Status.Config.SliceGatewayRemoteSubnet,
		RemoteSliceGwNodeIP:    gw.Status.Config.SliceGatewayRemoteNodeIP,
		RemoteSliceGwNodePort:  strconv.Itoa(gw.Status.Config.SliceGatewayRemoteNodePort),
	}
	_, err = client.UpdateConnectionContext(ctx, c)
	return err
}
