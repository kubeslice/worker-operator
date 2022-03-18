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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// SliceConfigSpec defines the desired state of SliceConfig
type SliceConfigSpec struct {
	SliceSubnet string `json:"sliceSubnet,omitempty"`
	//+kubebuilder:validation:Enum:=Application
	SliceType            string                    `json:"sliceType,omitempty"`
	SliceGatewayProvider SpokeSliceGatewayProvider `json:"sliceGatewayProvider,omitempty"`
	//+kubebuilder:validation:Enum:=Local
	SliceIpamType             string                    `json:"sliceIpamType,omitempty"`
	Clusters                  []string                  `json:"clusters,omitempty"`
	StandardQosProfileName    string                    `json:"standardQosProfileName,omitempty"` // FIXME: Add OneOf StandardQosProfileName vs QosProfileDetails
	QosProfileDetails         QOSProfile                `json:"qosProfileDetails,omitempty"`      // FIXME: Add OneOf StandardQosProfileName vs QosProfileDetails
	NamespaceIsolationProfile NamespaceIsolationProfile `json:"namespaceIsolationProfile,omitempty"`
	NetworkPolicyVersion      int                       `json:"networkPolicyVersion,omitempty"` //FIXME:: Not Required. Confirm with UI
}

// SpokeSliceGatewayProvider defines the configuration for slicegateway
type SpokeSliceGatewayProvider struct {
	//+kubebuilder:validation:Enum:=OpenVPN
	SliceGatewayType string `json:"sliceGatewayType,omitempty"`
	//+kubebuilder:validation:Enum:=Local
	SliceCaType string `json:"sliceCaType,omitempty"`
}

// QOSProfile is the QOS Profile configuration from backend
type QOSProfile struct {
	//+kubebuilder:validation:Enum:=HTB
	QueueType string `json:"queueType,omitempty"`
	Priority  string `json:"priority,omitempty"`
	//+kubebuilder:validation:Enum:=BANDWIDTH_CONTROL
	TcType                  string `json:"tcType,omitempty"`
	BandwidthCeilingKbps    int    `json:"bandwidthCeilingKbps,omitempty"` //FIXME: Need research for unlimited
	BandwidthGuaranteedKbps int    `json:"bandwidthGuaranteedKbps,omitempty"`
	//+kubebuilder:validation:Enum:=Default;AF11;AF12;AF13;AF21;AF22;AF23;AF31;AF32;AF33;AF41;AF42;AF43;EF
	DscpClass string `json:"dscpClass,omitempty"`
}

type NamespaceIsolationProfile struct {
	IsolationEnabled      bool                      `json:"isolationEnabled,omitempty"`
	ApplicationNamespaces []SliceNamespaceSelection `json:"applicationNamespaces,omitempty"`
	AllowedNamespaces     []SliceNamespaceSelection `json:"allowedNamespaces,omitempty"`
}

type SliceNamespaceSelection struct {
	Namespace string   `json:"namespace,omitempty"`
	Clusters  []string `json:"clusters,omitempty"`
}

// SliceConfigStatus defines the observed state of SliceConfig
type SliceConfigStatus struct {
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// SliceConfig is the Schema for the sliceconfig API
type SliceConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SliceConfigSpec   `json:"spec,omitempty"`
	Status SliceConfigStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SliceConfigList contains a list of SliceConfig
type SliceConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SliceConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SliceConfig{}, &SliceConfigList{})
}
