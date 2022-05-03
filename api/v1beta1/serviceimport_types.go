package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ServiceEndpoint contains details of a single endpoint which offers a particular service
type ServiceEndpoint struct {
	// Name of the endpoint
	Name string `json:"name,omitempty"`
	// IP of the pod which is reachable within slice
	IP string `json:"ip"`
	// Port to reach the endpoint
	Port int32 `json:"port"`
	// ClusterID which the endpoint belongs to
	ClusterID string `json:"clusterId"`
	// DNSName
	DNSName string `json:"dnsName"`
}

// ServiceImportSpec defines the desired state of ServiceImport
type ServiceImportSpec struct {
	// Slice denotes the slice which the app is part of
	Slice string `json:"slice"`
	// DNSName shows the FQDN to reach the service
	DNSName string `json:"dnsName"`
	// Ports which should be exposed through the service
	Ports []ServicePort `json:"ports"`
}

// ImportStatus is the status of Service Discovery reconciliation
type ImportStatus string

const (
	// ImportStatusInitial is the initial state
	ImportStatusInitial ImportStatus = ""
	// ImportStatusPending indicates that the service reconciliation is in progress and endpoints are not ready to communicate
	ImportStatusPending ImportStatus = "PENDING"
	// ImportStatusReady indicates that the Service is ready to serve requests
	ImportStatusReady ImportStatus = "READY"
	// ImportStatusError indicates that service is in error state and cannot process requests
	ImportStatusError ImportStatus = "ERROR"
)

// ServiceImportStatus defines the observed state of ServiceImport
type ServiceImportStatus struct {
	// ImportStatus denotes the status of the imported service
	ImportStatus ImportStatus `json:"importStatus,omitempty"`
	// Last sync time with backend
	LastSync int64 `json:"lastSync,omitempty"`
	// Used to match if the service is updated from backend
	UpdatedOn int64 `json:"updatedOn,omitempty"`
	// ExposedPorts shows a one line representation of ports and protocols exposed
	// only used to show as a printercolumn
	ExposedPorts string `json:"exposedPorts,omitempty"`
	// AvailableEndpoints shows the number of available endpoints
	AvailableEndpoints int `json:"availableEndpoints,omitempty"`
	// Endpoints which provide the service
	Endpoints []ServiceEndpoint `json:"endpoints,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Slice",type=string,JSONPath=`.spec.slice`
// +kubebuilder:printcolumn:name="Port(s)",type=string,JSONPath=`.status.exposedPorts`
// +kubebuilder:printcolumn:name="Endpoints",type=integer,JSONPath=`.status.availableEndpoints`
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.importStatus`
// +kubebuilder:resource:path=serviceimports,singular=serviceimport,shortName=svcim

// ServiceImport is the Schema for the serviceimports API
type ServiceImport struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ServiceImportSpec   `json:"spec,omitempty"`
	Status ServiceImportStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ServiceImportList contains a list of ServiceImport
type ServiceImportList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ServiceImport `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ServiceImport{}, &ServiceImportList{})
}
