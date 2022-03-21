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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SpokeSliceConfigSpec defines the desired state of Slice
type SpokeSliceConfigSpec struct {
	SliceName   string `json:"sliceName,omitempty"`
	SliceSubnet string `json:"sliceSubnet,omitempty"`
	//+kubebuilder:validation:Enum:=Application
	SliceType                 string                    `json:"sliceType,omitempty"`
	SpokeSliceGatewayProvider SpokeSliceGatewayProvider `json:"spokeSliceGatewayProvider,omitempty"`
	//+kubebuilder:validation:Enum:=Local
	SliceIpamType             string                    `json:"sliceIpamType,omitempty"`
	QosProfileDetails         QOSProfile                `json:"qosProfileDetails,omitempty"`
	NamespaceIsolationProfile NamespaceIsolationProfile `json:"namespaceIsolationProfile,omitempty"`
	IpamClusterOctet          int                       `json:"ipamClusterOctet,omitempty"`
	ExternalGatewayConfig     ExternalGatewayConfig     `json:"externalGatewayConfig,omitempty"`
}

// SpokeSliceGatewayProvider defines the configuration for slicegateway
type SpokeSliceGatewayProvider struct {
	//+kubebuilder:validation:Enum:=OpenVPN
	SpokeSliceGatewayType string `json:"spokeSliceGatewayType,omitempty"`
	//+kubebuilder:validation:Enum:=Local
	SliceCAType string `json:"sliceCaType,omitempty"`
}

// QOSProfile is the QOS Profile configuration from backend
type QOSProfile struct {
	//+kubebuilder:validation:Enum:=HTB
	QueueType               string `json:"queueType,omitempty"`
	Priority                string `json:"priority,omitempty"`
	TcType                  string `json:"tcType,omitempty"`
	BandwidthCeilingKbps    int    `json:"bandwidthCeilingKbps,omitempty"`
	BandwidthGuaranteedKbps int    `json:"bandwidthGuaranteedKbps,omitempty"`
	//+kubebuilder:validation:Enum:=Default;AF11;AF12;AF13;AF21;AF22;AF23;AF31;AF32;AF33;AF41;AF42;AF43;EF
	DscpClass string `json:"dscpClass,omitempty"`
}

type NamespaceIsolationProfile struct {
	IsolationEnabled      bool     `json:"isolationEnabled,omitempty"`
	ApplicationNamespaces []string `json:"applicationNamespaces,omitempty"`
	AllowedNamespaces     []string `json:"allowedNamespaces,omitempty"`
}

type ExternalGatewayConfig struct {
	Ingress   ExternalGatewayConfigOptions `json:"ingress,omitempty"`
	Egress    ExternalGatewayConfigOptions `json:"egress,omitempty"`
	NsIngress ExternalGatewayConfigOptions `json:"nsIngress,omitempty"`
	//+kubebuilder:validation:Enum:=none,istio
	GatewayType string `json:"gatewayType,omitempty"`
}

type ExternalGatewayConfigOptions struct {
	Enabled bool `json:"enabled,omitempty"`
}

// SpokeSliceConfigStatus defines the observed state of Slice
type SpokeSliceConfigStatus struct {
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// SpokeSliceConfig is the Schema for the slice API
type SpokeSliceConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SpokeSliceConfigSpec   `json:"spec,omitempty"`
	Status SpokeSliceConfigStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SpokeSliceConfigList contains a list of Slice
type SpokeSliceConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SpokeSliceConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SpokeSliceConfig{}, &SpokeSliceConfigList{})
}
