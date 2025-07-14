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

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// SliceGatewaySpec defines the desired state of SliceGateway
type SliceGatewaySpec struct {
	// SliceName is the Name of the slice this gateway is attached into
	SliceName string `json:"sliceName,omitempty"`
	// SiteName is site name
	SiteName string `json:"siteName,omitempty"`
}

// SliceGatewayConfig defines the config received from backend
type SliceGatewayConfig struct {
	// UUID of the slice gateway.
	SliceGatewayID string `json:"sliceGatewayId,omitempty"`
	// Slice Gateway Name
	SliceGatewayName string `json:"sliceGatewayName,omitempty"`
	// Name of the slice.
	SliceName string `json:"sliceName,omitempty"`
	// Slice gateway subnet range.
	SliceSiteName string `json:"sliceSiteName,omitempty"`
	// Slice gateway vpn type
	SliceGatewayType string `json:"sliceGatewayType,omitempty"`
	// Slice gateway subnet range.
	SliceGatewaySubnet string `json:"sliceGatewaySubnet,omitempty"`
	// SliceGateway status
	SliceGatewayStatus string `json:"sliceGatewayStatus,omitempty"`
	// Host Type : server or client
	SliceGatewayHostType string `json:"sliceGatewayHostType,omitempty"`
	// Node port
	SliceGatewayNodePorts []int `json:"sliceGatewayNodePorts,omitempty"`
	// Remote Node IPs
	SliceGatewayRemoteNodeIPs []string `json:"sliceGatewayRemoteNodeIps,omitempty"`
	// Remote Node Port
	SliceGatewayRemoteNodePorts []int `json:"sliceGatewayRemoteNodePorts,omitempty"`
	// Remote Node Subnet
	SliceGatewayRemoteSubnet string `json:"sliceGatewayRemoteSubnet,omitempty"`
	// Remote VPN IP
	SliceGatewayRemoteVpnIP string `json:"sliceGatewayRemoteVpnIp,omitempty"`
	// Local VPN IP
	SliceGatewayLocalVpnIP string `json:"sliceGatewayLocalVpnIp,omitempty"`
	// Remote Gateway ID
	SliceGatewayRemoteGatewayID string `json:"sliceGatewayRemoteGatewayId,omitempty"`
	// Remote Cluster ID
	SliceGatewayRemoteClusterID string `json:"sliceGatewayRemoteClusterId,omitempty"`
	// Intermediate Slice Gw Deployments
	SliceGatewayIntermediateDeployments []string `json:"sliceGatewayIntermediateDeployments,omitempty"`
	// SliceGateway Connectivity Type
	SliceGatewayConnectivityType string `json:"sliceGatewayConnectivityType,omitempty"`
	// SliceGateway Protocol Type: UDP or TCP
	SliceGatewayProtocol string `json:"sliceGatewayProtocol,omitempty"`
	// Slice gateway server LB IPs
	SliceGatewayServerLBIPs []string `json:"sliceGatewayServerLBIps,omitempty"`
}

// SliceGatewayStatus defines the observed state of SliceGateway
type SliceGatewayStatus struct {
	// SliceGatewayConfig defines the config received from backend
	Config SliceGatewayConfig `json:"config,omitempty"`
	// ConfigUpdatedOn is the time when Config updated from backend
	ConfigUpdatedOn int64 `json:"configUpdatedOn,omitempty"`
	// Deprecated PodName is the name of the gateway pod running in cluster
	PodName string `json:"podName,omitempty"`
	// PodNames is the list of names of the gateway pods running in cluster
	PodNames []string `json:"podNames,omitempty"`
	// PodStatus shows whether gateway pod is healthy
	PodStatus string `json:"podStatus,omitempty"`
	// PodIPs is the list of Ip of the gateway pods running in cluster
	PodIPs []string `json:"podIps,omitempty"`
	// PeerIP is the gateway tunnel peer ip
	PeerIP string `json:"peerIp,omitempty"`
	// ConnectionContextUpdated is the time when context updated in pod
	ConnectionContextUpdatedOn int64 `json:"connectionContextUpdatedOn,omitempty"`
	//gatewayPodStatus is a list that consists of status of individual gatewaypods
	GatewayPodStatus []*GwPodInfo `json:"gatewayPodStatus,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Subnet",type=string,JSONPath=`.status.config.sliceGatewaySubnet`
// +kubebuilder:printcolumn:name="Remote Subnet",type=string,JSONPath=`.status.config.sliceGatewayRemoteSubnet`
// +kubebuilder:printcolumn:name="Remote Cluster",type=string,JSONPath=`.status.config.sliceGatewayRemoteClusterId`
// +kubebuilder:printcolumn:name="GW Status",type=string,JSONPath=`.status.config.sliceGatewayStatus`
// +kubebuilder:resource:path=slicegateways,singular=slicegateway,shortName=gw;slicegw

// SliceGateway is the Schema for the slicegateways API
type SliceGateway struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SliceGatewaySpec   `json:"spec,omitempty"`
	Status SliceGatewayStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SliceGatewayList contains a list of SliceGateway
type SliceGatewayList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SliceGateway `json:"items"`
}

type GwPodInfo struct {
	PodName string `json:"podName,omitempty"`
	// podCreationTS indicates the creation TS of a pod
	PodCreationTS *metav1.Time `json:"podCreationTS,omitempty"`
	// originalPodCreationTS indicates how old the gw pod is even if is restarted
	OriginalPodCreationTS *metav1.Time `json:"originalPodCreationTS,omitempty"`
	PeerPodName           string       `json:"peerPodName,omitempty"`
	PodIP                 string       `json:"podIP,omitempty"`
	LocalNsmIP            string       `json:"localNsmIP,omitempty"`
	// TunnelStatus is the status of the tunnel between this gw pod and its peer
	TunnelStatus TunnelStatus `json:"tunnelStatus,omitempty"`
	RouteRemoved int32        `json:"routeRemoved,omitempty"`
	// RemotePort is the port number this gw pod is connected to on the remote cluster.
	// Applicable only for gw clients. Would be set to 0 for gw servers.
	RemotePort int32 `json:"remotePort,omitempty"`
}

type TunnelStatus struct {
	IntfName   string `json:"IntfName,omitempty"`
	LocalIP    string `json:"LocalIP,omitempty"`
	RemoteIP   string `json:"RemoteIP,omitempty"`
	Latency    uint64 `json:"Latency,omitempty"`
	TxRate     uint64 `json:"TxRate,omitempty"`
	RxRate     uint64 `json:"RxRate,omitempty"`
	PacketLoss uint64 `json:"PacketLoss,omitempty"`
	// Status is the status of the tunnel. 0: DOWN, 1: UP
	Status int32 `json:"Status,omitempty"`
	// TunnelState is the state of the tunnel in string format: UP, DOWN, UNKNOWN
	TunnelState string `json:"TunnelState,omitempty"`
}

func init() {
	SchemeBuilder.Register(&SliceGateway{}, &SliceGatewayList{})
}
