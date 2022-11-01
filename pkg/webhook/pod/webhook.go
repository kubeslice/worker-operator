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
	"net/http"

	//	"github.com/kubeslice/worker-operator/controllers"

	"github.com/kubeslice/worker-operator/controllers"
	"github.com/kubeslice/worker-operator/pkg/logger"
	v1 "k8s.io/api/admission/v1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
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

	log.Info("logging the req obj", "req", req)

	if req.Kind.Kind == "Pod" {
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

		if mutate, sliceName := wh.MutationRequired(pod.ObjectMeta, ctx, req.Kind.Kind); !mutate {
			log.Info("mutation not required", "pod metadata", pod.ObjectMeta)
		} else {
			log.Info("mutating pod", "pod metadata", pod.ObjectMeta)
			pod = MutatePod(pod, sliceName)
			log.Info("mutated pod", "pod metadata", pod.ObjectMeta)
		}

		marshaled, err := json.Marshal(pod)
		if err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}
		return admission.PatchResponseFromRaw(req.Object.Raw, marshaled)
	} else if req.Kind.Kind == "Deployment" {
		deploy := &appsv1.Deployment{}
		log := logger.FromContext(ctx)
		err := wh.decoder.Decode(req, deploy)
		if err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}

		if mutate, sliceName := wh.MutationRequired(deploy.ObjectMeta, ctx, req.Kind.Kind); !mutate {
			log.Info("mutation not required", "pod metadata", deploy.Spec.Template.ObjectMeta)
		} else {
			log.Info("mutating deploy", "pod metadata", deploy.Spec.Template.ObjectMeta)
			deploy = MutateDeployment(deploy, sliceName)
			log.Info("mutated deploy", "pod metadata", deploy.Spec.Template.ObjectMeta)
		}

		marshaled, err := json.Marshal(deploy)
		if err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}
		return admission.PatchResponseFromRaw(req.Object.Raw, marshaled)
	} else if req.Kind.Kind == "StatefulSet" {
		statefulset := &appsv1.StatefulSet{}
		err := wh.decoder.Decode(req, statefulset)
		if err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		log := logger.FromContext(ctx)

		if mutate, sliceName := wh.MutationRequired(statefulset.ObjectMeta, ctx, req.Kind.Kind); !mutate {
			log.Info("mutation not required", "pod metadata", statefulset.Spec.Template.ObjectMeta)
		} else {
			log.Info("mutating statefulset", "pod metadata", statefulset.Spec.Template.ObjectMeta)
			statefulset = MutateStatefulset(statefulset, sliceName)
			log.Info("mutated statefulset", "pod metadata", statefulset.Spec.Template.ObjectMeta)
		}

		marshaled, err := json.Marshal(statefulset)
		if err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}
		return admission.PatchResponseFromRaw(req.Object.Raw, marshaled)
	} else if req.Kind.Kind == "CronJob" {
		cronJob := &batchv1.CronJob{}
		err := wh.decoder.Decode(req, cronJob)
		if err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		log := logger.FromContext(ctx)

		if mutate, sliceName := wh.MutationRequired(cronJob.ObjectMeta, ctx, req.Kind.Kind); !mutate {
			log.Info("mutation not required", "pod metadata", cronJob.Spec.JobTemplate.ObjectMeta)
		} else {
			log.Info("mutating cronjob", "pod metadata", cronJob.Spec.JobTemplate.ObjectMeta)
			cronJob = MutateCronJobs(cronJob, sliceName)
			log.Info("mutated cronjob", "pod metadata", cronJob.Spec.JobTemplate.ObjectMeta)
		}

		marshaled, err := json.Marshal(cronJob)
		if err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}
		return admission.PatchResponseFromRaw(req.Object.Raw, marshaled)
	}

	return admission.Response{AdmissionResponse: v1.AdmissionResponse{
		Result: &metav1.Status{
			Message: "Invalid Kind",
		},
	}}
}

func (wh *WebhookServer) InjectDecoder(d *admission.Decoder) error {
	wh.decoder = d
	return nil
}

func MutatePod(pod *corev1.Pod, sliceName string) *corev1.Pod {
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

func MutateDeployment(deploy *appsv1.Deployment, sliceName string) *appsv1.Deployment {
	// Add injection status to deployment annotations
	if deploy.Spec.Template.ObjectMeta.Annotations == nil {
		deploy.Spec.Template.ObjectMeta.Annotations = map[string]string{}
	}

	deploy.Spec.Template.ObjectMeta.Annotations[AdmissionWebhookAnnotationStatusKey] = "injected"

	// Add vl3 annotation to pod template
	annotations := deploy.Spec.Template.ObjectMeta.Annotations
	annotations[nsmInjectAnnotaionKey] = "vl3-service-" + sliceName

	// Add slice identifier labels to pod template
	labels := deploy.Spec.Template.ObjectMeta.Labels
	labels[PodInjectLabelKey] = "app"
	labels[admissionWebhookAnnotationInjectKey] = sliceName

	return deploy
}

func MutateStatefulset(ss *appsv1.StatefulSet, sliceName string) *appsv1.StatefulSet {
	// Add injection status to statefulset annotations
	if ss.Spec.Template.ObjectMeta.Annotations == nil {
		ss.Spec.Template.ObjectMeta.Annotations = map[string]string{}
	}

	ss.Spec.Template.ObjectMeta.Annotations[AdmissionWebhookAnnotationStatusKey] = "injected"

	// Add vl3 annotation to pod template
	annotations := ss.Spec.Template.ObjectMeta.Annotations
	annotations[nsmInjectAnnotaionKey] = "vl3-service-" + sliceName

	// Add slice identifier labels to pod template
	labels := ss.Spec.Template.ObjectMeta.Labels
	labels[PodInjectLabelKey] = "app"
	labels[admissionWebhookAnnotationInjectKey] = sliceName

	return ss
}

func MutateCronJobs(cronJobs *batchv1.CronJob, sliceName string) *batchv1.CronJob {

	log.Info("Mutation recieved for cronjob", "cronjob name", cronJobs.Name)
	log.Info("Cronjob meta", "meta", cronJobs.Spec.JobTemplate.ObjectMeta)

	// Add injection status to jobs annotations
	if cronJobs.Spec.JobTemplate.ObjectMeta.Annotations == nil {
		cronJobs.Spec.JobTemplate.ObjectMeta.Annotations = map[string]string{}
	}

	cronJobs.Spec.JobTemplate.ObjectMeta.Annotations[AdmissionWebhookAnnotationStatusKey] = "injected"

	// Add vl3 annotation to pod template
	annotations := cronJobs.Spec.JobTemplate.ObjectMeta.Annotations
	annotations[nsmInjectAnnotaionKey] = "vl3-service-" + sliceName

	// Add slice identifier labels to pod template
	labels := cronJobs.Spec.JobTemplate.ObjectMeta.Labels
	labels[PodInjectLabelKey] = "app"
	labels[admissionWebhookAnnotationInjectKey] = sliceName

	return cronJobs
}

func (wh *WebhookServer) MutationRequired(metadata metav1.ObjectMeta, ctx context.Context, kind string) (bool, string) {
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
		log.Info("obj is already injected", "kind", kind)
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
