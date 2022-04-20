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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// SpokeSliceGatewaySpec defines the desired state of SpokeSliceGateway
type SpokeSliceGatewaySpec struct {
	SliceName string `json:"sliceName,omitempty"`
	//+kubebuilder:validation:Enum:=OpenVPN
	GatewayType string `json:"gatewayType,omitempty"`
	//+kubebuilder:validation:Enum:=Client;Server
	GatewayHostType     string             `json:"gatewayHostType,omitempty"`
	GatewayCredentials  GatewayCredentials `json:"gatewayCredentials,omitempty"`
	LocalGatewayConfig  SliceGatewayConfig `json:"localGatewayConfig,omitempty"`
	RemoteGatewayConfig SliceGatewayConfig `json:"remoteGatewayConfig,omitempty"`
	GatewayNumber       int                `json:"gatewayNumber,omitempty"`
}

type SliceGatewayConfig struct {
	NodeIp        string `json:"nodeIp,omitempty"`
	NodePort      int    `json:"nodePort,omitempty"`
	GatewayName   string `json:"gatewayName,omitempty"`
	ClusterName   string `json:"clusterName,omitempty"`
	VpnIp         string `json:"vpnIp,omitempty"`
	GatewaySubnet string `json:"gatewaySubnet,omitempty"`
}

type GatewayCredentials struct {
	SecretName string `json:"secretName,omitempty"`
}

// SpokeSliceGatewayStatus defines the observed state of SpokeSliceGateway
type SpokeSliceGatewayStatus struct {
	GatewayNumber         int `json:"gatewayNumber,omitempty"`
	ClusterInsertionIndex int `json:"clusterInsertionIndex,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// SpokeSliceGateway is the Schema for the slicegateways API
type SpokeSliceGateway struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SpokeSliceGatewaySpec   `json:"spec,omitempty"`
	Status SpokeSliceGatewayStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SpokeSliceGatewayList contains a list of SpokeSliceGateway
type SpokeSliceGatewayList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SpokeSliceGateway `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SpokeSliceGateway{}, &SpokeSliceGatewayList{})
}
