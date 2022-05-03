/*
##########################################################
serviceexport_types.go

Avesha LLC
Feb 2021

Copyright (c) Avesha LLC. 2021

Avesha Sice Operator
##########################################################
*/

package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ControllerType defines the type of service mesh running in the cluster
type ControllerType string

const (
	// ControllerTypeIstio indicates that service is exported through istio
	ControllerTypeIstio ControllerType = "istio"
	// ControllerTypeNone indicates that service is running as normal kubernetes service without any mesh
	ControllerTypeNone ControllerType = "none"
)

// ServicePort is the port exposed by ServicePod
type ServicePort struct {
	// Name of the port
	Name string `json:"name,omitempty"`
	// Port number exposed from the container
	ContainerPort int32 `json:"containerPort"`
	// Protocol for port. Must be UDP, TCP, or SCTP.
	// Defaults to "TCP".
	Protocol corev1.Protocol `json:"protocol,omitempty"`
}

// ServicePod contains pod information which offers a service
type ServicePod struct {
	// Name of the pod
	Name string `json:"name"`
	// NsmIP of the pod which is reachable within slice
	NsmIP string `json:"nsmIp,omitempty"`
	// PodIp of the pod which is reachable within cluster
	PodIp string `json:"podIp"`
	// DNSName is the dns A record name for the pod
	DNSName string `json:"dnsName"`
}

// ServiceExportSpec defines the desired state of ServiceExport
type ServiceExportSpec struct {
	// Slice denotes the slice which the app is part of
	Slice string `json:"slice"`
	// Selector is a label query over pods that should be exposed as a service
	Selector *metav1.LabelSelector `json:"selector"`
	// ControllerType denotes the type of service mesh the app pods are part of
	ControllerType ControllerType `json:"controllerType,omitempty"`
	// IngressEnabled denotes whether the traffic should be proxied through an ingress gateway
	IngressEnabled bool `json:"ingressEnabled,omitempty"`
	// Ports which should be exposed through the service
	Ports []ServicePort `json:"ports"`
}

// ExportStatus is the status of Service Discovery reconciliation
type ExportStatus string

const (
	// ExportStatusInitial is the initial state
	ExportStatusInitial ExportStatus = ""
	// ExportStatusPending indicates that the service reconciliation is in progress and endpoints are not exported yet
	ExportStatusPending ExportStatus = "PENDING"
	// ExportStatusReady indicates that the Service is ready to serve requests
	ExportStatusReady ExportStatus = "READY"
	// ExportStatusError indicates that service is in error state and cannot process requests
	ExportStatusError ExportStatus = "ERROR"
)

// ServiceProtocol is the protocol exposed by the service
type ServiceProtocol string

const (
	// ServiceProtocolTCP is tcp protocol
	ServiceProtocolTCP ServiceProtocol = "tcp"
	//ServiceProtocolHTTP is http protocol
	ServiceProtocolHTTP ServiceProtocol = "http"
)

// ServiceExportStatus defines the observed state of ServiceExport
type ServiceExportStatus struct {
	// DNSName is the FQDN to reach the service
	DNSName string `json:"dnsName,omitempty"`
	// ExportStatus denotes the export status of the service
	ExportStatus ExportStatus `json:"exportStatus,omitempty"`
	// Pods denotes the service endpoint pods
	Pods []ServicePod `json:"pods,omitempty"`
	// Last sync time with backend
	LastSync int64 `json:"lastSync,omitempty"`
	// ExposedPorts shows a one line representation of ports and protocols exposed
	// only used to show as a printercolumn
	ExposedPorts string `json:"exposedPorts,omitempty"`
	// AvailableEndpoints shows the number of available endpoints
	AvailableEndpoints int `json:"availableEndpoints,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Slice",type=string,JSONPath=`.spec.slice`
// +kubebuilder:printcolumn:name="Ingress",type=boolean,JSONPath=`.spec.ingressEnabled`
// +kubebuilder:printcolumn:name="Port(s)",type=string,JSONPath=`.status.exposedPorts`
// +kubebuilder:printcolumn:name="Endpoints",type=integer,JSONPath=`.status.availableEndpoints`
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.exportStatus`
// +kubebuilder:resource:path=serviceexports,singular=serviceexport,shortName=svcex

// ServiceExport is the Schema for the serviceexports API
type ServiceExport struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ServiceExportSpec   `json:"spec,omitempty"`
	Status ServiceExportStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ServiceExportList contains a list of ServiceExport
type ServiceExportList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ServiceExport `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ServiceExport{}, &ServiceExportList{})
}
