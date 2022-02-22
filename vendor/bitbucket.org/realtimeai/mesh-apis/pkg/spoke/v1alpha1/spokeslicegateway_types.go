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

// SpokeSliceGatewaySpec defines the desired state of SpokeSliceGateway
type SpokeSliceGatewaySpec struct {
	Slice                 string `json:"slice,omitempty"`
	SliceClusterId        string `json:"sliceClusterId,omitempty"`
	RemoteClusterId       string `json:"remoteClusterId,omitempty"`
	SliceClusterNamespace string `json:"sliceClusterNamespace,omitempty"`
	SliceSiteName         string `json:"sliceSiteName,omitempty"`
	//+kubebuilder:validation:Enum:=Unassigned;Assigned;Deployed;Registered;LinkActive;LinkInactive;InError;PendingCerts
	SpokeSliceGatewayStatus string `json:"spokeSliceGatewayStatus,omitempty"`
	//+kubebuilder:validation:Enum:=Registered;NotRegistered
	SpokeSliceGatewayRegistration     string `json:"spokeSliceGatewayRegistration,omitempty"`
	SpokeSliceGatewayRegisteredAtTime string `json:"spokeSliceGatewayRegisteredAtTime,omitempty"`
	//+kubebuilder:validation:Enum:=OpenVPN
	SpokeSliceGatewayType        string `json:"spokeSliceGatewayType,omitempty"`
	SpokeSliceGatewaySubnet      string `json:"spokeSliceGatewaySubnet,omitempty"`
	SpokeSliceGatewayQosProfile  string `json:"spokeSliceGatewayQosProfile,omitempty"`
	AssignedAtTime               string `json:"assignedAtTime,omitempty"`
	SpokeSliceGatewayPodIp       string `json:"spokeSliceGatewayPodIp,omitempty"`
	SpokeSliceGatewayRemotePodIp string `json:"spokeSliceGatewayRemotePodIp,omitempty"`
	//+kubebuilder:validation:Enum:=Client;Server
	SpokeSliceGatewayHostType string               `json:"spokeSliceGatewayHostType,omitempty"`
	OpenVPN                   OpenVPNConfiguration `json:"openVPN,omitempty"`
	GatewayNumber             int                  `json:"gatewayNumber,omitempty"`
	ClusterInsertionIndex     int                  `json:"clusterInsertionIndex,omitempty"`
	SliceGatewayNodePort      int                  `json:"sliceGatewayNodePort,omitempty"`
	SliceGatewayNodeIp        string               `json:"sliceGatewayNodeIp,omitempty"`
	SliceGatewayLocalVpnIp    string               `json:"sliceGatewayLocalVpnIp,omitempty"`
	RemoteSliceGatewayName    string               `json:"remoteSliceGatewayName,omitempty"`
}

type OpenVPNConfiguration struct {
	Client OpenVPNClientConfiguration `json:"client,omitempty"`
	Server OpenVPNServerConfiguration `json:"server,omitempty"`
}

type OpenVPNClientConfiguration struct {
	OvpnConfigFile string `json:"ovpnConfigFile,omitempty"`
}

type OpenVPNServerConfiguration struct {
	OvpnConfFile      string `json:"ovpnConfFile,omitempty"`
	CcdFile           string `json:"ccdFile,omitempty"`
	PkiCACertFile     string `json:"pkiCACertFile,omitempty"`
	PkiTAKeyFile      string `json:"pkiTAKeyFile,omitempty"`
	PkiDhPemFile      string `json:"pkiDhPemFile,omitempty"`
	PkiIssuedCertFile string `json:"pkiIssuedCertFile,omitempty"`
	PkiPrivateKeyFile string `json:"pkiPrivateKeyFile,omitempty"`
}

// SpokeSliceGatewayStatus defines the observed state of SpokeSliceGateway
type SpokeSliceGatewayStatus struct {
	NodeIp             string                                 `json:"nodeIp,omitempty"`
	GatewayPodIp       string                                 `json:"gatewayPodIp,omitempty"`
	TunnelStatus       SpokeSliceGatewayConnectedTunnelStatus `json:"tunnelStatus,omitempty"`
	AppStatus          []SpokeSliceGatewayConnectedAppStatus  `json:"appStatus,omitempty"`
	LastHealthPingTime string                                 `json:"lastHealthPingTime,omitempty"`
	//+kubebuilder:validation:Enum:=Initializing;Healthy;Unhealthy;Terminated
	PodStatus                        string `json:"podStatus,omitempty"`
	SpokeSliceGatewayRemoteNodeIp    string `json:"spokeSliceGatewayRemoteNodeIp,omitempty"`
	SpokeSliceGatewayNodePort        int    `json:"spokeSliceGatewayNodePort,omitempty"`
	SpokeSliceGatewayRemoteSubnet    string `json:"spokeSliceGatewayRemoteSubnet,omitempty"`
	SpokeSliceGatewayRemoteVpnIp     string `json:"spokeSliceGatewayRemoteVpnIp,omitempty"`
	SpokeSliceGatewayRemoteNodePort  int    `json:"spokeSliceGatewayRemoteNodePort,omitempty"`
	SpokeSliceGatewayLocalVpnIp      string `json:"spokeSliceGatewayLocalVpnIp,omitempty"`
	SpokeSliceGatewayRemoteGatewayId string `json:"spokeSliceGatewayRemoteGatewayId,omitempty"`
	SpokeSliceGatewayRemoteClusterId string `json:"spokeSliceGatewayRemoteClusterId,omitempty"`
}

type SpokeSliceGatewayConnectedTunnelStatus struct {
	NetInterface string `json:"netInterface,omitempty"`
	LocalIp      string `json:"localIp,omitempty"`
	PeerIp       string `json:"peerIp,omitempty"`
	Latency      int    `json:"latency,omitempty"`
	TxRate       string `json:"txRate,omitempty"`
	RxRate       string `json:"rxRate,omitempty"`
}

type SpokeSliceGatewayConnectedAppStatus struct {
	PodName             string `json:"podName,omitempty"`
	NsmInterface        string `json:"nsmInterface,omitempty"`
	NsmIp               string `json:"nsmIp,omitempty"`
	PodIp               string `json:"podIp,omitempty"`
	NsmGatewayInterface string `json:"nsmGatewayInterface,omitempty"`
	NsmGatewayIp        string `json:"nsmGatewayIp,omitempty"`
	Namespace           string `json:"namespace,omitempty"`
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
