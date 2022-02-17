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

// SliceGatewaySpec defines the desired state of SliceGateway
type SliceGatewaySpec struct {
	Slice                 string `json:"slice,omitempty"`
	SliceClusterId        string `json:"sliceClusterId,omitempty"`
	RemoteClusterId       string `json:"remoteClusterId,omitempty"`
	SliceClusterNamespace string `json:"sliceClusterNamespace,omitempty"`
	SliceSiteName         string `json:"sliceSiteName,omitempty"`
	//+kubebuilder:validation:Enum:=Unassigned;Assigned;Deployed;Registered;LinkActive;LinkInactive;InError;PendingCerts
	SliceGatewayStatus string `json:"sliceGatewayStatus,omitempty"`
	//+kubebuilder:validation:Enum:=Registered;NotRegistered
	SliceGatewayRegistration     string `json:"sliceGatewayRegistration,omitempty"`
	SliceGatewayRegisteredAtTime string `json:"sliceGatewayRegisteredAtTime,omitempty"`
	//+kubebuilder:validation:Enum:=OpenVPN
	SliceGatewayType        string `json:"sliceGatewayType,omitempty"`
	SliceGatewaySubnet      string `json:"sliceGatewaySubnet,omitempty"`
	SliceGatewayQosProfile  string `json:"sliceGatewayQosProfile,omitempty"`
	AssignedAtTime          string `json:"assignedAtTime,omitempty"`
	SliceGatewayPodIp       string `json:"sliceGatewayPodIp,omitempty"`
	SliceGatewayRemotePodIp string `json:"sliceGatewayRemotePodIp,omitempty"`
	//+kubebuilder:validation:Enum:=Client;Server
	SliceGatewayHostType  string               `json:"sliceGatewayHostType,omitempty"`
	OpenVPN               OpenVPNConfiguration `json:"openVPN,omitempty"`
	GatewayNumber         int                  `json:"gatewayNumber,omitempty"`
	ClusterInsertionIndex int                  `json:"clusterInsertionIndex,omitempty"`
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

// SliceGatewayStatus defines the observed state of SliceGateway
type SliceGatewayStatus struct {
	NodeIp             string                            `json:"nodeIp,omitempty"`
	GatewayPodIp       string                            `json:"gatewayPodIp,omitempty"`
	TunnelStatus       SliceGatewayConnectedTunnelStatus `json:"tunnelStatus,omitempty"`
	AppStatus          []SliceGatewayConnectedAppStatus  `json:"appStatus,omitempty"`
	LastHealthPingTime string                            `json:"lastHealthPingTime,omitempty"`
	//+kubebuilder:validation:Enum:=Initializing;Healthy;Unhealthy;Terminated
	PodStatus                   string `json:"podStatus,omitempty"`
	SliceGatewayRemoteNodeIp    string `json:"sliceGatewayRemoteNodeIp,omitempty"`
	SliceGatewayNodePort        int    `json:"sliceGatewayNodePort,omitempty"`
	SliceGatewayRemoteSubnet    string `json:"sliceGatewayRemoteSubnet,omitempty"`
	SliceGatewayRemoteVpnIp     string `json:"sliceGatewayRemoteVpnIp,omitempty"`
	SliceGatewayRemoteNodePort  int    `json:"sliceGatewayRemoteNodePort,omitempty"`
	SliceGatewayLocalVpnIp      string `json:"sliceGatewayLocalVpnIp,omitempty"`
	SliceGatewayRemoteGatewayId string `json:"sliceGatewayRemoteGatewayId,omitempty"`
	SliceGatewayRemoteClusterId string `json:"sliceGatewayRemoteClusterId,omitempty"`
}

type SliceGatewayConnectedTunnelStatus struct {
	NetInterface string `json:"netInterface,omitempty"`
	LocalIp      string `json:"localIp,omitempty"`
	PeerIp       string `json:"peerIp,omitempty"`
	Latency      int    `json:"latency,omitempty"`
	TxRate       string `json:"txRate,omitempty"`
	RxRate       string `json:"rxRate,omitempty"`
}

type SliceGatewayConnectedAppStatus struct {
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

// SliceGateway is the Schema for the slicegateways API
type SliceGateway struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SliceGatewaySpec   `json:"spec,omitempty"`
	Status SliceGatewayStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SliceGatewayList contains a list of SliceGateway
type SliceGatewayList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SliceGateway `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SliceGateway{}, &SliceGatewayList{})
}
