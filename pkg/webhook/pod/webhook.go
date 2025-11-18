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

package pod

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/kubeslice/apis/pkg/controller/v1alpha1"
	"github.com/kubeslice/worker-operator/api/v1beta1"
	"github.com/kubeslice/worker-operator/controllers"
	"github.com/kubeslice/worker-operator/pkg/logger"
	v1 "k8s.io/api/admission/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	admissionWebhookAnnotationInjectKey       = controllers.ApplicationNamespaceSelectorLabelKey
	AdmissionWebhookAnnotationStatusKey       = "kubeslice.io/status"
	PodInjectLabelKey                         = "kubeslice.io/pod-type"
	admissionWebhookSliceNamespaceSelectorKey = controllers.ApplicationNamespaceSelectorLabelKey
	controlPlaneNamespace                     = "kubeslice-system"
	nsmInjectAnnotaionKey1                    = "ns.networkservicemesh.io"
	nsmInjectAnnotaionKey2                    = "networkservicemesh.io"
	kubesliceExcludeKey                       = "kubeslice.io/exclude"
	SkipInitContainersLabelKey                = "kubeslice.io/skip-init-containers"
)

var (
	log = logger.NewWrappedLogger().WithName("Webhook").V(1)
)

type SliceInfoProvider interface {
	SliceAppNamespaceConfigured(ctx context.Context, slice string, namespace string) (bool, error)
	GetNamespaceLabels(ctx context.Context, client client.Client, namespace string) (map[string]string, error)
	GetSliceOverlayNetworkType(ctx context.Context, client client.Client, sliceName string) (v1alpha1.NetworkType, error)
	GetAllServiceExports(ctx context.Context, client client.Client, slice string) (*v1beta1.ServiceExportList, error)
}

type WebhookServer struct {
	Client          client.Client
	Decoder         admission.Decoder
	SliceInfoClient SliceInfoProvider
}

func (wh *WebhookServer) Handle(ctx context.Context, req admission.Request) admission.Response {
	if req.Kind.Kind == "Pod" {
		pod := &corev1.Pod{}
		err := wh.Decoder.Decode(req, pod)
		if err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		log := logger.FromContext(ctx)

		// handle empty namespace field when the pod is created by deployment
		if pod.ObjectMeta.Namespace == "" {
			pod.ObjectMeta.Namespace = req.Namespace
		}

		if mutate, sliceName, nsLabels := wh.MutationRequired(pod.ObjectMeta, ctx, req.Kind.Kind); !mutate {
			log.Info("mutation not required for pod", "pod metadata", pod.ObjectMeta.Name)
		} else {
			log.Info("mutating pod", "pod metadata", pod.ObjectMeta.Name)
			pod = MutatePod(pod, sliceName, nsLabels)
		}

		marshaled, err := json.Marshal(pod)
		if err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}
		return admission.PatchResponseFromRaw(req.Object.Raw, marshaled)
	} else if req.Kind.Kind == "Deployment" {
		deploy := &appsv1.Deployment{}
		log := logger.FromContext(ctx)
		err := wh.Decoder.Decode(req, deploy)
		if err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}

		if mutate, sliceName, nsLabels := wh.MutationRequired(deploy.ObjectMeta, ctx, req.Kind.Kind); !mutate {
			log.Info("mutation not required for deployment", "pod metadata", deploy.Spec.Template.ObjectMeta)
		} else {
			deploy = MutateDeployment(deploy, sliceName, nsLabels)
			log.Info("mutated deploy", "pod metadata", deploy.Spec.Template.ObjectMeta)
		}

		marshaled, err := json.Marshal(deploy)
		if err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}
		return admission.PatchResponseFromRaw(req.Object.Raw, marshaled)
	} else if req.Kind.Kind == "StatefulSet" {
		statefulset := &appsv1.StatefulSet{}
		err := wh.Decoder.Decode(req, statefulset)
		if err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		log := logger.FromContext(ctx)

		if mutate, sliceName, nsLabels := wh.MutationRequired(statefulset.ObjectMeta, ctx, req.Kind.Kind); !mutate {
			log.Info("mutation not required for statefulsets", "pod metadata", statefulset.Spec.Template.ObjectMeta)
		} else {
			statefulset = MutateStatefulset(statefulset, sliceName, nsLabels)
			log.Info("mutated statefulset", "pod metadata", statefulset.Spec.Template.ObjectMeta)
		}

		marshaled, err := json.Marshal(statefulset)
		if err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}
		return admission.PatchResponseFromRaw(req.Object.Raw, marshaled)
	} else if req.Kind.Kind == "DaemonSet" {
		daemonset := &appsv1.DaemonSet{}
		err := wh.Decoder.Decode(req, daemonset)
		if err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		log := logger.FromContext(ctx)

		if mutate, sliceName, nsLabels := wh.MutationRequired(daemonset.ObjectMeta, ctx, req.Kind.Kind); !mutate {
			log.Info("mutation not required for daemonset", "pod metadata", daemonset.Spec.Template.ObjectMeta)
		} else {
			daemonset = MutateDaemonSet(daemonset, sliceName, nsLabels)
			log.Info("mutated daemonset", "pod metadata", daemonset.Spec.Template.ObjectMeta)
		}

		marshaled, err := json.Marshal(daemonset)
		if err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}
		return admission.PatchResponseFromRaw(req.Object.Raw, marshaled)
	} else if req.Kind.Kind == "ServiceExport" {
		serviceexport := &v1beta1.ServiceExport{}
		err := wh.Decoder.Decode(req, serviceexport)
		if err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		log := logger.FromContext(ctx)

		log.Info("validating serviceexport", "serviceexport spec", serviceexport.Spec)
		validation, conflictingAlias, err := wh.ValidateServiceExport(serviceexport, ctx)
		if err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}
		if !validation {
			log.Info("serviceexport validation failed: alias already exist", "serviceexport-name", serviceexport.ObjectMeta.Name)
			return admission.Denied(fmt.Sprintf("Alias %s already exist", conflictingAlias))
		}
		return admission.Allowed("")
	}

	return admission.Response{AdmissionResponse: v1.AdmissionResponse{
		Result: &metav1.Status{
			Message: "Invalid Kind",
		},
	}}
}

// applyCustomLabels applies custom labels from namespace to the pod template labels
func applyCustomLabels(labels map[string]string, nsLabels map[string]string) {
	if labels == nil {
		return
	}
	if nsLabels == nil {
		return
	}
	// Copy custom labels from namespace to pod
	// For initial phase: check if namespace has skip-init-containers label
	if val, exists := nsLabels[SkipInitContainersLabelKey]; exists {
		labels[SkipInitContainersLabelKey] = val
	}
	// Add more custom label mappings here as needed
}

func MutatePod(pod *corev1.Pod, sliceName string, nsLabels map[string]string) *corev1.Pod {
	// Add injection status to pod annotations
	if pod.ObjectMeta.Annotations == nil {
		pod.ObjectMeta.Annotations = map[string]string{}
	}
	pod.ObjectMeta.Annotations[AdmissionWebhookAnnotationStatusKey] = "injected"

	// Initialize the Labels field as an empty map
	if pod.ObjectMeta.Labels == nil {
		pod.ObjectMeta.Labels = map[string]string{}
	}

	// Add vl3 annotation to pod template
	annotations := pod.ObjectMeta.Annotations
	annotations[nsmInjectAnnotaionKey1] = "vl3-service-" + sliceName
	annotations[nsmInjectAnnotaionKey2] = fmt.Sprintf("kernel://vl3-service-%s/nsm0", sliceName)

	// Add slice identifier labels to pod template
	labels := pod.ObjectMeta.Labels
	labels[PodInjectLabelKey] = "app"
	labels[admissionWebhookAnnotationInjectKey] = sliceName

	// Apply custom labels from namespace
	applyCustomLabels(labels, nsLabels)

	return pod
}

func MutateDeployment(deploy *appsv1.Deployment, sliceName string, nsLabels map[string]string) *appsv1.Deployment {
	// Add injection status to deployment annotations
	if deploy.Spec.Template.ObjectMeta.Annotations == nil {
		deploy.Spec.Template.ObjectMeta.Annotations = map[string]string{}
	}

	deploy.Spec.Template.ObjectMeta.Annotations[AdmissionWebhookAnnotationStatusKey] = "injected"

	// Add vl3 annotation to pod template
	annotations := deploy.Spec.Template.ObjectMeta.Annotations
	annotations[nsmInjectAnnotaionKey1] = "vl3-service-" + sliceName
	annotations[nsmInjectAnnotaionKey2] = fmt.Sprintf("kernel://vl3-service-%s/nsm0", sliceName)

	if deploy.Spec.Template.ObjectMeta.Labels == nil {
		deploy.Spec.Template.ObjectMeta.Labels = map[string]string{}
	}
	// Add slice identifier labels to pod template
	labels := deploy.Spec.Template.ObjectMeta.Labels
	labels[PodInjectLabelKey] = "app"
	labels[admissionWebhookAnnotationInjectKey] = sliceName

	// Apply custom labels from namespace
	applyCustomLabels(labels, nsLabels)

	if deploy.ObjectMeta.Labels == nil {
		deploy.ObjectMeta.Labels = make(map[string]string)
	}
	deploy.ObjectMeta.Labels[admissionWebhookAnnotationInjectKey] = sliceName

	return deploy
}

func MutateStatefulset(ss *appsv1.StatefulSet, sliceName string, nsLabels map[string]string) *appsv1.StatefulSet {
	// Add injection status to statefulset annotations
	if ss.Spec.Template.ObjectMeta.Annotations == nil {
		ss.Spec.Template.ObjectMeta.Annotations = map[string]string{}
	}

	ss.Spec.Template.ObjectMeta.Annotations[AdmissionWebhookAnnotationStatusKey] = "injected"

	// Add vl3 annotation to pod template
	annotations := ss.Spec.Template.ObjectMeta.Annotations
	annotations[nsmInjectAnnotaionKey1] = "vl3-service-" + sliceName
	annotations[nsmInjectAnnotaionKey2] = fmt.Sprintf("kernel://vl3-service-%s/nsm0", sliceName)

	if ss.Spec.Template.ObjectMeta.Labels == nil {
		ss.Spec.Template.ObjectMeta.Labels = map[string]string{}
	}
	// Add slice identifier labels to pod template
	labels := ss.Spec.Template.ObjectMeta.Labels
	labels[PodInjectLabelKey] = "app"
	labels[admissionWebhookAnnotationInjectKey] = sliceName

	// Apply custom labels from namespace
	applyCustomLabels(labels, nsLabels)

	if ss.ObjectMeta.Labels == nil {
		ss.ObjectMeta.Labels = make(map[string]string)
	}
	ss.ObjectMeta.Labels[admissionWebhookAnnotationInjectKey] = sliceName

	return ss
}

func MutateDaemonSet(ds *appsv1.DaemonSet, sliceName string, nsLabels map[string]string) *appsv1.DaemonSet {
	// Add injection status to statefulset annotations
	if ds.Spec.Template.ObjectMeta.Annotations == nil {
		ds.Spec.Template.ObjectMeta.Annotations = map[string]string{}
	}

	ds.Spec.Template.ObjectMeta.Annotations[AdmissionWebhookAnnotationStatusKey] = "injected"

	// Add vl3 annotation to pod template
	annotations := ds.Spec.Template.ObjectMeta.Annotations
	annotations[nsmInjectAnnotaionKey1] = "vl3-service-" + sliceName
	annotations[nsmInjectAnnotaionKey2] = fmt.Sprintf("kernel://vl3-service-%s/nsm0", sliceName)

	if ds.Spec.Template.ObjectMeta.Labels == nil {
		ds.Spec.Template.ObjectMeta.Labels = map[string]string{}
	}
	// Add slice identifier labels to pod template
	labels := ds.Spec.Template.ObjectMeta.Labels
	labels[PodInjectLabelKey] = "app"
	labels[admissionWebhookAnnotationInjectKey] = sliceName

	// Apply custom labels from namespace
	applyCustomLabels(labels, nsLabels)

	// add slice identifier labels to object
	if ds.ObjectMeta.Labels == nil {
		ds.ObjectMeta.Labels = make(map[string]string)
	}
	ds.ObjectMeta.Labels[admissionWebhookAnnotationInjectKey] = sliceName

	return ds
}

func (wh *WebhookServer) ValidateServiceExport(svcex *v1beta1.ServiceExport, ctx context.Context) (bool, string, error) {

	log := logger.FromContext(ctx)
	log.Info("fetching all serviceexport objects belonging to the slice", "slice", svcex.Spec.Slice)
	serviceExportList, err := wh.SliceInfoClient.GetAllServiceExports(context.Background(), wh.Client, svcex.Spec.Slice)
	if err != nil {
		return false, "", err
	}

	newAliases := svcex.Spec.Aliases

	for _, serviceExport := range serviceExportList.Items {
		// In case we are updating an existing ServiceExport resource
		if svcex.ObjectMeta.Name == serviceExport.ObjectMeta.Name &&
			svcex.ObjectMeta.Namespace == serviceExport.ObjectMeta.Namespace {
			continue
		}
		existingAliases := serviceExport.Spec.Aliases
		for _, newAlias := range newAliases {
			if aliasExist(existingAliases, newAlias) {
				return false, newAlias, nil
			}
		}
	}
	return true, "", nil
}

// returns mutationRequired bool, sliceName string, nsLabels map[string]string
func (wh *WebhookServer) MutationRequired(metadata metav1.ObjectMeta, ctx context.Context, kind string) (bool, string, map[string]string) {
	log := logger.FromContext(ctx)
	annotations := metadata.GetAnnotations()

	labels := metadata.GetLabels()
	if labels != nil {
		val, exists := labels[kubesliceExcludeKey]
		// don't mutate if kubeslice.io/exclude=true
		if exists && val == "true" {
			return false, "", nil
		}
	}

	//early exit if metadata in nil
	//we allow empty annotation, but namespace should not be empty
	if metadata.GetNamespace() == "" {
		log.Info("namespace is empty")
		return false, "", nil
	}

	// do not inject if it is already injected
	//TODO(rahulsawra): need better way to define injected status
	if annotations[AdmissionWebhookAnnotationStatusKey] == "injected" {
		log.Info("obj is already injected", "kind", kind)
		return false, "", nil
	}

	// Do not auto onboard control plane namespace. Ideally, we should not have any deployment/pod in the control plane ns connect to a slice
	if metadata.Namespace == controlPlaneNamespace {
		log.Info("namespace is same as controle plane")
		return false, "", nil
	}

	// Get namespace labels early to use for custom label propagation
	nsLabels, err := wh.SliceInfoClient.GetNamespaceLabels(context.Background(), wh.Client, metadata.Namespace)
	if err != nil {
		log.Error(err, "Error getting namespace labels")
		return false, "", nil
	}
	if nsLabels == nil {
		log.Info("Namespace has no labels")
		return false, "", nil
	}

	sliceNameInNs, sliceLabelPresent := nsLabels[admissionWebhookSliceNamespaceSelectorKey]
	if !sliceLabelPresent {
		log.Info("Namespace has no slice labels")
		return false, "", nil
	}

	sliceNetworkType, err := wh.SliceInfoClient.GetSliceOverlayNetworkType(context.Background(), wh.Client, sliceNameInNs)
	if err != nil {
		log.Error(err, "Error getting slice overlay network type")
		return false, "", nil
	}
	if sliceNetworkType != "" && sliceNetworkType != v1alpha1.SINGLENET {
		log.Info("Slice overlay type is not single-network. Skip pod mutation...")
		return false, "", nil
	}

	nsConfigured, err := wh.SliceInfoClient.SliceAppNamespaceConfigured(context.Background(), sliceNameInNs, metadata.Namespace)
	if err != nil {
		log.Error(err, "Failed to get app namespace info for slice",
			"slice", sliceNameInNs, "namespace", metadata.Namespace)
		return false, "", nil
	}
	if !nsConfigured {
		log.Info("Namespace not part of slice", "namespace", metadata.Namespace, "slice", sliceNameInNs)
		return false, "", nil
	}
	// The annotation kubeslice.io/slice:SLICENAME is present, enable mutation
	return true, sliceNameInNs, nsLabels
}
