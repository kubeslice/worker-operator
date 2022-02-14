/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SliceSpec defines the desired state of Slice
type SliceSpec struct {
	// Display name of the slice.
	SliceDisplayName string `json:"sliceDisplayName"`
	// SliceConfig is the spec for slice received from hub cluster
	SliceConfig *SliceConfig `json:"sliceConfig,omitempty"`
}

// QosProfileDetails is the QOS Profile for the slice
type QosProfileDetails struct {
	// Queue Type
	QueueType string `json:"queueType"`
	// Bandwidth Ceiling eg:5000
	BandwidthCeilingKbps string `json:"bandwidthCeilingKbps"`
	// Bandwidth Guaranteed eg:4000
	BandwidthGuaranteedKbps string `json:"bandwidthGuaranteedKbps"`
	// Priority 0-3
	Priority string `json:"priority"`
	// DSCP code for inter cluster traffic
	DscpClass string `json:"dscpClass"`
}

type SliceIpamConfig struct {
	// IPAM Type for slice
	SliceIpamType string `json:"sliceIpamType"`
	// Cluster specific octet for IPAM root subnet
	IpamClusterOctet int `json:"ipamClusterOctet,omitempty"`
}

// SliceConfig defines the Config retrieved from Hub
type SliceConfig struct {
	// UUID of the slice.
	SliceID string `json:"sliceId"`
	// name of the slice.
	SliceName string `json:"sliceName"`
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
}

// SliceStatus defines the observed state of Slice
type SliceStatus struct {
	// SliceConfig is the spec for slice received from hub cluster
	SliceConfig *SliceConfig `json:"sliceConfig,omitempty"`
	DnsIP       string       `json:"dnsIP"`
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
