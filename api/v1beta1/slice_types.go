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
}

// SliceConfig defines the Config retrieved from backend
type SliceConfig struct {
	// SliceId is the UUID of the slice.
	SliceID string `json:"sliceId"`
	// SliceSubnet refers to the subnet the slice is part of
	SliceSubnet string `json:"sliceSubnet"`
}

// SliceStatus defines the observed state of Slice
type SliceStatus struct {
	// SliceConfig is the spec for slice received from hub cluster
	SliceConfig *SliceConfig `json:"sliceConfig,omitempty"`
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
