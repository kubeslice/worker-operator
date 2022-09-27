//go:build !ignore_autogenerated
// +build !ignore_autogenerated

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

// Code generated by controller-gen. DO NOT EDIT.

package v1beta1

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AppPod) DeepCopyInto(out *AppPod) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AppPod.
func (in *AppPod) DeepCopy() *AppPod {
	if in == nil {
		return nil
	}
	out := new(AppPod)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ExternalGatewayConfig) DeepCopyInto(out *ExternalGatewayConfig) {
	*out = *in
	if in.Ingress != nil {
		in, out := &in.Ingress, &out.Ingress
		*out = new(ExternalGatewayConfigOptions)
		**out = **in
	}
	if in.Egress != nil {
		in, out := &in.Egress, &out.Egress
		*out = new(ExternalGatewayConfigOptions)
		**out = **in
	}
	if in.NsIngress != nil {
		in, out := &in.NsIngress, &out.NsIngress
		*out = new(ExternalGatewayConfigOptions)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ExternalGatewayConfig.
func (in *ExternalGatewayConfig) DeepCopy() *ExternalGatewayConfig {
	if in == nil {
		return nil
	}
	out := new(ExternalGatewayConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ExternalGatewayConfigOptions) DeepCopyInto(out *ExternalGatewayConfigOptions) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ExternalGatewayConfigOptions.
func (in *ExternalGatewayConfigOptions) DeepCopy() *ExternalGatewayConfigOptions {
	if in == nil {
		return nil
	}
	out := new(ExternalGatewayConfigOptions)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GwPodInfo) DeepCopyInto(out *GwPodInfo) {
	*out = *in
	out.TunnelStatus = in.TunnelStatus
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GwPodInfo.
func (in *GwPodInfo) DeepCopy() *GwPodInfo {
	if in == nil {
		return nil
	}
	out := new(GwPodInfo)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *IngressGwPod) DeepCopyInto(out *IngressGwPod) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new IngressGwPod.
func (in *IngressGwPod) DeepCopy() *IngressGwPod {
	if in == nil {
		return nil
	}
	out := new(IngressGwPod)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NamespaceIsolationProfile) DeepCopyInto(out *NamespaceIsolationProfile) {
	*out = *in
	if in.ApplicationNamespaces != nil {
		in, out := &in.ApplicationNamespaces, &out.ApplicationNamespaces
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.AllowedNamespaces != nil {
		in, out := &in.AllowedNamespaces, &out.AllowedNamespaces
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NamespaceIsolationProfile.
func (in *NamespaceIsolationProfile) DeepCopy() *NamespaceIsolationProfile {
	if in == nil {
		return nil
	}
	out := new(NamespaceIsolationProfile)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *QosProfileDetails) DeepCopyInto(out *QosProfileDetails) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new QosProfileDetails.
func (in *QosProfileDetails) DeepCopy() *QosProfileDetails {
	if in == nil {
		return nil
	}
	out := new(QosProfileDetails)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServiceEndpoint) DeepCopyInto(out *ServiceEndpoint) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServiceEndpoint.
func (in *ServiceEndpoint) DeepCopy() *ServiceEndpoint {
	if in == nil {
		return nil
	}
	out := new(ServiceEndpoint)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServiceExport) DeepCopyInto(out *ServiceExport) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServiceExport.
func (in *ServiceExport) DeepCopy() *ServiceExport {
	if in == nil {
		return nil
	}
	out := new(ServiceExport)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ServiceExport) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServiceExportList) DeepCopyInto(out *ServiceExportList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ServiceExport, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServiceExportList.
func (in *ServiceExportList) DeepCopy() *ServiceExportList {
	if in == nil {
		return nil
	}
	out := new(ServiceExportList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ServiceExportList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServiceExportSpec) DeepCopyInto(out *ServiceExportSpec) {
	*out = *in
	if in.Selector != nil {
		in, out := &in.Selector, &out.Selector
		*out = new(v1.LabelSelector)
		(*in).DeepCopyInto(*out)
	}
	if in.Ports != nil {
		in, out := &in.Ports, &out.Ports
		*out = make([]ServicePort, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServiceExportSpec.
func (in *ServiceExportSpec) DeepCopy() *ServiceExportSpec {
	if in == nil {
		return nil
	}
	out := new(ServiceExportSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServiceExportStatus) DeepCopyInto(out *ServiceExportStatus) {
	*out = *in
	if in.Pods != nil {
		in, out := &in.Pods, &out.Pods
		*out = make([]ServicePod, len(*in))
		copy(*out, *in)
	}
	out.IngressGwPod = in.IngressGwPod
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServiceExportStatus.
func (in *ServiceExportStatus) DeepCopy() *ServiceExportStatus {
	if in == nil {
		return nil
	}
	out := new(ServiceExportStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServiceImport) DeepCopyInto(out *ServiceImport) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServiceImport.
func (in *ServiceImport) DeepCopy() *ServiceImport {
	if in == nil {
		return nil
	}
	out := new(ServiceImport)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ServiceImport) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServiceImportList) DeepCopyInto(out *ServiceImportList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ServiceImport, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServiceImportList.
func (in *ServiceImportList) DeepCopy() *ServiceImportList {
	if in == nil {
		return nil
	}
	out := new(ServiceImportList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ServiceImportList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServiceImportSpec) DeepCopyInto(out *ServiceImportSpec) {
	*out = *in
	if in.Ports != nil {
		in, out := &in.Ports, &out.Ports
		*out = make([]ServicePort, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServiceImportSpec.
func (in *ServiceImportSpec) DeepCopy() *ServiceImportSpec {
	if in == nil {
		return nil
	}
	out := new(ServiceImportSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServiceImportStatus) DeepCopyInto(out *ServiceImportStatus) {
	*out = *in
	if in.Endpoints != nil {
		in, out := &in.Endpoints, &out.Endpoints
		*out = make([]ServiceEndpoint, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServiceImportStatus.
func (in *ServiceImportStatus) DeepCopy() *ServiceImportStatus {
	if in == nil {
		return nil
	}
	out := new(ServiceImportStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServicePod) DeepCopyInto(out *ServicePod) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServicePod.
func (in *ServicePod) DeepCopy() *ServicePod {
	if in == nil {
		return nil
	}
	out := new(ServicePod)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServicePort) DeepCopyInto(out *ServicePort) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServicePort.
func (in *ServicePort) DeepCopy() *ServicePort {
	if in == nil {
		return nil
	}
	out := new(ServicePort)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Slice) DeepCopyInto(out *Slice) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Slice.
func (in *Slice) DeepCopy() *Slice {
	if in == nil {
		return nil
	}
	out := new(Slice)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *Slice) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SliceConfig) DeepCopyInto(out *SliceConfig) {
	*out = *in
	out.QosProfileDetails = in.QosProfileDetails
	out.SliceIpam = in.SliceIpam
	if in.ExternalGatewayConfig != nil {
		in, out := &in.ExternalGatewayConfig, &out.ExternalGatewayConfig
		*out = new(ExternalGatewayConfig)
		(*in).DeepCopyInto(*out)
	}
	if in.NamespaceIsolationProfile != nil {
		in, out := &in.NamespaceIsolationProfile, &out.NamespaceIsolationProfile
		*out = new(NamespaceIsolationProfile)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SliceConfig.
func (in *SliceConfig) DeepCopy() *SliceConfig {
	if in == nil {
		return nil
	}
	out := new(SliceConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SliceGateway) DeepCopyInto(out *SliceGateway) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SliceGateway.
func (in *SliceGateway) DeepCopy() *SliceGateway {
	if in == nil {
		return nil
	}
	out := new(SliceGateway)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *SliceGateway) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SliceGatewayConfig) DeepCopyInto(out *SliceGatewayConfig) {
	*out = *in
	if in.SliceGatewayRemoteNodeIPs != nil {
		in, out := &in.SliceGatewayRemoteNodeIPs, &out.SliceGatewayRemoteNodeIPs
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SliceGatewayConfig.
func (in *SliceGatewayConfig) DeepCopy() *SliceGatewayConfig {
	if in == nil {
		return nil
	}
	out := new(SliceGatewayConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SliceGatewayList) DeepCopyInto(out *SliceGatewayList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]SliceGateway, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SliceGatewayList.
func (in *SliceGatewayList) DeepCopy() *SliceGatewayList {
	if in == nil {
		return nil
	}
	out := new(SliceGatewayList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *SliceGatewayList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SliceGatewaySpec) DeepCopyInto(out *SliceGatewaySpec) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SliceGatewaySpec.
func (in *SliceGatewaySpec) DeepCopy() *SliceGatewaySpec {
	if in == nil {
		return nil
	}
	out := new(SliceGatewaySpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SliceGatewayStatus) DeepCopyInto(out *SliceGatewayStatus) {
	*out = *in
	in.Config.DeepCopyInto(&out.Config)
	if in.PodNames != nil {
		in, out := &in.PodNames, &out.PodNames
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.PodIPs != nil {
		in, out := &in.PodIPs, &out.PodIPs
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.GatewayPodStatus != nil {
		in, out := &in.GatewayPodStatus, &out.GatewayPodStatus
		*out = make([]*GwPodInfo, len(*in))
		for i := range *in {
			if (*in)[i] != nil {
				in, out := &(*in)[i], &(*out)[i]
				*out = new(GwPodInfo)
				**out = **in
			}
		}
	}
	if in.GatewayPodStatus != nil {
		in, out := &in.GatewayPodStatus, &out.GatewayPodStatus
		*out = make([]GwPodInfo, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SliceGatewayStatus.
func (in *SliceGatewayStatus) DeepCopy() *SliceGatewayStatus {
	if in == nil {
		return nil
	}
	out := new(SliceGatewayStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SliceIpamConfig) DeepCopyInto(out *SliceIpamConfig) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SliceIpamConfig.
func (in *SliceIpamConfig) DeepCopy() *SliceIpamConfig {
	if in == nil {
		return nil
	}
	out := new(SliceIpamConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SliceList) DeepCopyInto(out *SliceList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Slice, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SliceList.
func (in *SliceList) DeepCopy() *SliceList {
	if in == nil {
		return nil
	}
	out := new(SliceList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *SliceList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SliceSpec) DeepCopyInto(out *SliceSpec) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SliceSpec.
func (in *SliceSpec) DeepCopy() *SliceSpec {
	if in == nil {
		return nil
	}
	out := new(SliceSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SliceStatus) DeepCopyInto(out *SliceStatus) {
	*out = *in
	if in.SliceConfig != nil {
		in, out := &in.SliceConfig, &out.SliceConfig
		*out = new(SliceConfig)
		(*in).DeepCopyInto(*out)
	}
	if in.AppPods != nil {
		in, out := &in.AppPods, &out.AppPods
		*out = make([]AppPod, len(*in))
		copy(*out, *in)
	}
	if in.ApplicationNamespaces != nil {
		in, out := &in.ApplicationNamespaces, &out.ApplicationNamespaces
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.AllowedNamespaces != nil {
		in, out := &in.AllowedNamespaces, &out.AllowedNamespaces
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SliceStatus.
func (in *SliceStatus) DeepCopy() *SliceStatus {
	if in == nil {
		return nil
	}
	out := new(SliceStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TunnelStatus) DeepCopyInto(out *TunnelStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TunnelStatus.
func (in *TunnelStatus) DeepCopy() *TunnelStatus {
	if in == nil {
		return nil
	}
	out := new(TunnelStatus)
	in.DeepCopyInto(out)
	return out
}
