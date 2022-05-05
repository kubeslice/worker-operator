/*  Copyright (c) 2022 Avesha, Inc. All rights reserved.
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

package sidecar

import (
	"context"
	"net"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/vishvananda/netlink"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GwSidecar struct {
	UnimplementedGwSidecarServiceServer
}

// Stores the node port of the remote cluster.
// We need this to set the dscp config when the mode of the Gw is of type CLIENT, in
// which case the remote cluster would be the SERVER and the client is connected to
// it over this node port.
var SliceGwRemoteClusterNodePort string = ""

// Checks if vpphost interface is present on the host.
// The vpphost interface is used to set up a network connection between the host kernel stack
// and the vpp data plane stack.
func checkIfVppIntfPresent() bool {
	vppInterface, err := net.InterfaceByName("vpphost")
	if err != nil {
		return false
	}
	return vppInterface != nil
}

// GetStatus get the status of sidecar.
func (s *GwSidecar) GetStatus(ctx context.Context, in *empty.Empty) (*GwPodStatus, error) {
	if ctx.Err() == context.Canceled {
		return nil, status.Errorf(codes.Canceled, "Client cancelled, abandoning.")
	}
	podStatus, err := getGwPodStatus()
	return podStatus, err
}

func (s *GwSidecar) UpdateConnectionContext(ctx context.Context, conContext *SliceGwConnectionContext) (*SidecarResponse, error) {
	if ctx.Err() == context.Canceled {
		return nil, status.Errorf(codes.Canceled, "Client cancelled, abandoning.")
	}
	if conContext == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Connection Context is Empty")
	}
	if conContext.GetRemoteSliceGwVpnIP() == "" {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid Remote Slice Gateway VPN IP")
	}
	log.Infof("conContext : %v", conContext)
	err := updateGwStatusWithConContext(conContext)
	if err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "Failed to update the Connection context,tunnel is not up yet!")
	}
	if conContext.GetRemoteSliceGwNsmSubnet() == "" {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid Remote Slice Gateway Subnet")
	}

	if SliceGwRemoteClusterNodePort == "" {
		SliceGwRemoteClusterNodePort = conContext.GetRemoteSliceGwNodePort()
	}

	// Add Gateway Route as follows
	// route add -net  <remote-subnet> netmask <255.255.255.0> gw <remove-vpn-ip>
	_, dstIPNet, err := net.ParseCIDR(conContext.GetRemoteSliceGwNsmSubnet())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Error in Parsing CIDR")
	}
	gwIP := net.ParseIP(conContext.GetRemoteSliceGwVpnIP())

	route := netlink.Route{Dst: dstIPNet, Gw: gwIP}
	log.Infof("RouteAdd args %v, %v ", dstIPNet, gwIP)

	if err := netlink.RouteAdd(&route); err != nil {
		log.Errorf("Gateway Pod RouteAdd Failed : %v", err)
	}

	if checkIfVppIntfPresent() {
		vppGwIP := net.ParseIP("10.255.255.254")
		_, localGwNsmSubnetIP, err := net.ParseCIDR(conContext.GetLocalSliceGwNsmSubnet())
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "Error in Parsing CIDR: local nsm subnet")
		}
		vppGwRoute := netlink.Route{Dst: localGwNsmSubnetIP, Gw: vppGwIP}
		log.Infof("RouteAdd args %v, %v ", localGwNsmSubnetIP, vppGwIP)
		if err := netlink.RouteAdd(&vppGwRoute); err != nil {
			log.Errorf("VPP Gateway RouteAdd Failed : %v", err)
		}
	}
	log.Infof("Connection Context Updated Successfully")

	return &SidecarResponse{StatusMsg: "Connection Context Updated Successfully"}, nil
}

func (s *GwSidecar) UpdateSliceQosProfile(ctx context.Context, qosProfile *SliceQosProfile) (*SidecarResponse, error) {
	if ctx.Err() == context.Canceled {
		return nil, status.Errorf(codes.Canceled, "Client canceled, ignoring qos update message.")
	}
	if qosProfile == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Qos profile message is empty")
	}

	//settings.Log.Infof("SliceQosProfile : %v", qosProfile)

	err := s.enforceSliceQosPolicy(
		&SliceQosProfile{
			ClassType:    qosProfile.GetClassType(),
			BwCeiling:    qosProfile.GetBwCeiling(),
			BwGuaranteed: qosProfile.GetBwGuaranteed(),
			Priority:     qosProfile.GetPriority(),
			DscpClass:    qosProfile.GetDscpClass(),
		},
	)
	if err != nil {
		//settings.Log.Errorf("Failed to enforce QoS policy: %v", err)
		return nil, status.Errorf(codes.Internal, "Failed to enforce QoS policy: %v", err)
	}
	log.Infof("Slice QoS policy enforced successfully")
	return &SidecarResponse{StatusMsg: "Slice QoS policy enforced successfully"}, nil
}
