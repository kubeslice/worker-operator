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

// SpokeServiceImportSpec defines the desired state of SpokeServiceImport
type SpokeServiceImportSpec struct {
	//ServiceName is the name of the service
	ServiceName string `json:"serviceName,omitempty"`
	//ServiceNamespace is the namespace of the service
	ServiceNamespace string `json:"serviceNamespace,omitempty"` // Required
	// clusterId is the id of the cluster where the service is available.
	SourceClusters []string `json:"sourceClusters,omitempty"`
	// The name of the slice.
	SliceName string `json:"sliceName,omitempty"`
	// The type of service mesh running in the cluster
	//+kubebuilder:validation:Enum:=istio;none
	MeshType string `json:"meshType,omitempty"`
	// Proxy enabled or disabled.
	Proxy bool `json:"proxy,omitempty"`
	// the service discovery endpoint array
	ServiceDiscoveryEndpoints []ServiceDiscoveryEndpoint `json:"serviceDiscoveryEndpoints,omitempty"`
	// The ports for the given service.
	ServiceDiscoveryPorts []ServiceDiscoveryPort `json:"serviceDiscoveryPorts,omitempty"`
}

type ServiceDiscoveryEndpoint struct {
	// The name of the pod.
	PodName string `json:"podName,omitempty"`
	// The ID of the cluster.
	Cluster string `json:"cluster,omitempty"`
	// The NSM IP address.
	NsmIp string `json:"nsmIp,omitempty"`
	// the dns_name of the service
	DnsName string `json:"dnsName,omitempty"`
	// port of the service
	Port int32 `json:"port,omitempty"`
}

type ServiceDiscoveryPort struct {
	// The name of the port.
	Name string `json:"name,omitempty"`
	// The port number.
	Port int32 `json:"port,omitempty"`
	// The protocol.
	Protocol string `json:"protocol,omitempty"`
}

// SpokeServiceImportStatus defines the observed state of SpokeServiceImport
type SpokeServiceImportStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// SpokeServiceImport is the Schema for the spokeserviceimport API
type SpokeServiceImport struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SpokeServiceImportSpec   `json:"spec,omitempty"`
	Status SpokeServiceImportStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SpokeServiceImportList contains a list of SpokeServiceImport
type SpokeServiceImportList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SpokeServiceImport `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SpokeServiceImport{}, &SpokeServiceImportList{})
}
