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

// SliceSpec defines the desired state of Slice
type SliceSpec struct {
}

// QosProfileDetails is the QOS Profile for the slice
type QosProfileDetails struct {
	// Queue Type
	QueueType string `json:"queueType,omitempty"`
	// Bandwidth Ceiling eg:5000
	BandwidthCeilingKbps int `json:"bandwidthCeilingKbps,omitempty"`
	// Bandwidth Guaranteed eg:4000
	BandwidthGuaranteedKbps int `json:"bandwidthGuaranteedKbps,omitempty"`
	// Priority 0-3
	Priority int `json:"priority,omitempty"`
	// DSCP code for inter cluster traffic
	DscpClass string `json:"dscpClass,omitempty"`
	// TC type
	TcType string `json:"tcType,omitempty"`
}

type SliceIpamConfig struct {
	// IPAM Type for slice
	SliceIpamType string `json:"sliceIpamType"`
	// Cluster specific octet for IPAM root subnet
	IpamClusterOctet *int `json:"ipamClusterOctet,omitempty"`
}

// SliceConfig defines the Config retrieved from Hub
type SliceConfig struct {
	// UUID of the slice.
	SliceID string `json:"sliceId"`
	// display name of the slice.
	SliceDisplayName string `json:"sliceDisplayName"`
	// IP subnet range of the slice.
	SliceSubnet string `json:"sliceSubnet"`
	// Type of the slice.
	SliceType string `json:"sliceType"`
	// QOS profile details
	QosProfileDetails QosProfileDetails `json:"qosProfileDetails"`
	// IPAM configuration for the slice
	SliceIpam SliceIpamConfig `json:"sliceIpam"`
	// ExternalGatewayConfig determines istio ingress/egress configuration
	ExternalGatewayConfig *ExternalGatewayConfig `json:"externalGatewayConfig,omitempty"`
	// Namespace Isolation profile contains fields related to namespace binding to slice
	NamespaceIsolationProfile *NamespaceIsolationProfile `json:"namespaceIsolationProfile,omitempty"`
	//ClusterSubnetCIDR is the subnet to be used by the current cluster
	ClusterSubnetCIDR string `json:"clusterSubnetCIDR,omitempty"`
}

// NamespaceIsolationProfile defines the namespace isolation policy for the slice
type NamespaceIsolationProfile struct {
	// Enable Namespace Isolation in the slice
	// +kubebuilder:default:=false
	IsolationEnabled bool `json:"isolationEnabled,omitempty"`
	//Application namespaces is a list of namespaces that are bound to the slice
	ApplicationNamespaces []string `json:"applicationNamespaces,omitempty"`
	//Allowed namespaces is a list of namespaces that can send and receive traffic to app namespaces
	AllowedNamespaces []string `json:"allowedNamespaces,omitempty"`
}

// ExternalGatewayConfig determines istio ingress/egress configuration
type ExternalGatewayConfig struct {
	Ingress     *ExternalGatewayConfigOptions `json:"ingress,omitempty"`
	Egress      *ExternalGatewayConfigOptions `json:"egress,omitempty"`
	NsIngress   *ExternalGatewayConfigOptions `json:"nsIngress,omitempty"`
	GatewayType string                        `json:"gatewayType,omitempty"`
}

type ExternalGatewayConfigOptions struct {
	Enabled bool `json:"enabled,omitempty"`
}

// AppPod defines the app pods connected to slice
type AppPod struct {
	// PodName is App Pod Name
	PodName string `json:"podName,omitempty"`
	// PodNamespace is App Pod Namespace
	PodNamespace string `json:"podNamespace,omitempty"`
	// PodIP is App Pod IP
	PodIP string `json:"podIp,omitempty"`
	// NsmIP is the nsm ip of App
	NsmIP string `json:"nsmIp,omitempty"`
	// NsmInterface is the nsm interface of App
	NsmInterface string `json:"nsmInterface,omitempty"`
	// PeerIp is the nsm peer ip of gateway
	NsmPeerIP string `json:"nsmPeerIp,omitempty"`
}

// SliceStatus defines the observed state of Slice
type SliceStatus struct {
	// SliceConfig is the spec for slice received from hub cluster
	SliceConfig *SliceConfig `json:"sliceConfig,omitempty"`
	// DNSIP is the IP of Coredns server
	DNSIP string `json:"dnsIP,omitempty"`
	// AppPods contains the list of app pods connected to the slice
	AppPods []AppPod `json:"appPods,omitempty"`
	// AppPodsUpdatedOn is the time when app pods list was updated
	AppPodsUpdatedOn int64 `json:"appPodsUpdatedOn,omitempty"`
	// NetworkPoliciesInstalled defines whether the netpol are installed in atleast one applicationNamespace
	// +kubebuilder:default:=false
	NetworkPoliciesInstalled bool `json:"networkPoliciesInstalled,omitempty"`
	// Slice Application Namespace list
	ApplicationNamespaces []string `json:"applicationNamespaces,omitempty"`
	// Slice Allowed Namespace list
	AllowedNamespaces []string `json:"allowedNamespaces,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Slice is the Schema for the slice API
type Slice struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SliceSpec   `json:"spec,omitempty"`
	Status SliceStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SliceList contains a list of Slice
type SliceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Slice `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Slice{}, &SliceList{})
}
