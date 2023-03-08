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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// IngressGwPod contains ingress gw pod information
type IngressGwPod struct {
	// Name of the pod
	Name string `json:"name"`
	// NsmIP of the pod which is reachable within slice
	NsmIP string `json:"nsmIp,omitempty"`
}

// ServiceExportSpec defines the desired state of ServiceExport
type ServiceExportSpec struct {
	// Slice denotes the slice which the app is part of
	Slice string `json:"slice"`
	// Selector is a label query over pods that should be exposed as a service
	Selector *metav1.LabelSelector `json:"selector"`
	// IngressEnabled denotes whether the traffic should be proxied through an ingress gateway
	IngressEnabled bool `json:"ingressEnabled,omitempty"`
	// Ports which should be exposed through the service
	Ports []ServicePort `json:"ports"`
	// Alias names for the exported service. The service could be addressed by the alias names
	// in addition to the slice.local name.
	Aliases []string `json:"aliases,omitempty"`
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
	// IngressGwEnabled denotes ingress gw is enabled for the serviceexport
	IngressGwEnabled bool `json:"ingressGwEnabled,omitempty"`
	// IngressGwPod contains ingress gateway pod info
	IngressGwPod IngressGwPod `json:"ingressGwPod,omitempty"`
	// Alias names for the exported service. The service could be addressed by the alias names
	// in addition to the slice.local name.
	Aliases []string `json:"aliases,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Slice",type=string,JSONPath=`.spec.slice`
// +kubebuilder:printcolumn:name="Ingress",type=boolean,JSONPath=`.spec.ingressEnabled`
// +kubebuilder:printcolumn:name="Port(s)",type=string,JSONPath=`.status.exposedPorts`
// +kubebuilder:printcolumn:name="Endpoints",type=integer,JSONPath=`.status.availableEndpoints`
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.exportStatus`
// +kubebuilder:printcolumn:name="Alias",type=string,JSONPath=`.spec.aliases`
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
