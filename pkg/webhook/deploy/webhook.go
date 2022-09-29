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

package deploy

import (
	"context"
	"encoding/json"
	"net/http"

	//	"github.com/kubeslice/worker-operator/controllers"

	"github.com/kubeslice/worker-operator/controllers"
	"github.com/kubeslice/worker-operator/pkg/logger"
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
	nsmInjectAnnotaionKey                     = "ns.networkservicemesh.io"
)

var (
	log = logger.NewLogger().WithName("Webhook").V(1)
)

type SliceInfoProvider interface {
	SliceAppNamespaceConfigured(ctx context.Context, slice string, namespace string) (bool, error)
	GetNamespaceLabels(ctx context.Context, client client.Client, namespace string) (map[string]string, error)
}
type WebhookServer struct {
	Client          client.Client
	decoder         *admission.Decoder
	SliceInfoClient SliceInfoProvider
}

func (wh *WebhookServer) Handle(ctx context.Context, req admission.Request) admission.Response {
	pod := &corev1.Pod{}
	err := wh.decoder.Decode(req, pod)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	log := logger.FromContext(ctx)

	// handle empty namespace field when the pod is created by deployment
	if pod.ObjectMeta.Namespace == "" {
		pod.ObjectMeta.Namespace = req.Namespace
	}

	if mutate, sliceName := wh.MutationRequired(pod.ObjectMeta, ctx); !mutate {
		log.Info("mutation not required", "pod metadata", pod.ObjectMeta)
	} else {
		log.Info("mutating pod", "pod metadata", pod.ObjectMeta)
		pod = Mutate(pod, sliceName)
		log.Info("mutated pod", "pod metadata", pod.ObjectMeta)
	}

	marshaled, err := json.Marshal(pod)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	return admission.PatchResponseFromRaw(req.Object.Raw, marshaled)
}

func (wh *WebhookServer) InjectDecoder(d *admission.Decoder) error {
	wh.decoder = d
	return nil
}

func Mutate(pod *corev1.Pod, sliceName string) *corev1.Pod {
	// Add injection status to pod annotations
	if pod.ObjectMeta.Annotations == nil {
		pod.ObjectMeta.Annotations = map[string]string{}
	}
	pod.ObjectMeta.Annotations[AdmissionWebhookAnnotationStatusKey] = "injected"

	if pod.ObjectMeta.Annotations == nil {
		pod.ObjectMeta.Annotations = map[string]string{}
	}

	// Add vl3 annotation to pod template
	annotations := pod.ObjectMeta.Annotations
	annotations[nsmInjectAnnotaionKey] = "vl3-service-" + sliceName

	// Add slice identifier labels to pod template
	labels := pod.ObjectMeta.Labels
	labels[PodInjectLabelKey] = "app"
	labels[admissionWebhookAnnotationInjectKey] = sliceName

	return pod
}

func (wh *WebhookServer) MutationRequired(metadata metav1.ObjectMeta, ctx context.Context) (bool, string) {
	log := logger.FromContext(ctx)
	annotations := metadata.GetAnnotations()
	//early exit if metadata in nil
	//we allow empty annotation, but namespace should not be empty
	if metadata.GetNamespace() == "" {
		log.Info("namespace is empty")
		return false, ""
	}
	// do not inject if it is already injected
	//TODO(rahulsawra): need better way to define injected status
	if annotations[AdmissionWebhookAnnotationStatusKey] == "injected" {
		log.Info("pod is already injected")
		return false, ""
	}

	// Do not auto onboard control plane namespace. Ideally, we should not have any deployment/pod in the control plane ns connect to a slice
	if metadata.Namespace == controlPlaneNamespace {
		log.Info("namespace is same as controle plane")
		return false, ""
	}

	nsLabels, err := wh.SliceInfoClient.GetNamespaceLabels(context.Background(), wh.Client, metadata.Namespace)
	if err != nil {
		log.Error(err, "Error getting namespace labels")
		return false, ""
	}
	if nsLabels == nil {
		log.Info("Namespace has no labels")
		return false, ""
	}

	sliceNameInNs, sliceLabelPresent := nsLabels[admissionWebhookSliceNamespaceSelectorKey]
	if !sliceLabelPresent {
		log.Info("Namespace has no slice labels")
		return false, ""
	}

	nsConfigured, err := wh.SliceInfoClient.SliceAppNamespaceConfigured(context.Background(), sliceNameInNs, metadata.Namespace)
	if err != nil {
		log.Error(err, "Failed to get app namespace info for slice",
			"slice", sliceNameInNs, "namespace", metadata.Namespace)
		return false, ""
	}
	if !nsConfigured {
		log.Info("Namespace not part of slice", "namespace", metadata.Namespace, "slice", sliceNameInNs)
		return false, ""
	}
	// The annotation kubeslice.io/slice:SLICENAME is present, enable mutation
	return true, sliceNameInNs
}
