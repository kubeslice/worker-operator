package deploy

import (
	"context"
	"encoding/json"
	"net/http"

	//	"bitbucket.org/realtimeai/kubeslice-operator/controllers"
	"bitbucket.org/realtimeai/kubeslice-operator/internal/logger"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	admissionWebhookAnnotationInjectKey = "avesha.io/slice"
	admissionWebhookAnnotationStatusKey = "avesha.io/status"
)

var (
	log = logger.NewLogger().WithName("Webhook").V(1)
)

type WebhookServer struct {
	Client  client.Client
	decoder *admission.Decoder
}

func (wh *WebhookServer) Handle(ctx context.Context, req admission.Request) admission.Response {
	deploy := &appsv1.Deployment{}
	err := wh.decoder.Decode(req, deploy)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	log := logger.FromContext(ctx)

	if !MutationRequired(deploy.ObjectMeta) {
		log.Info("mutation not required", "pod metadata", deploy.Spec.Template.ObjectMeta)
	} else {
		log.Info("mutating deploy", "pod metadata", deploy.Spec.Template.ObjectMeta)
		slice := deploy.ObjectMeta.Annotations[admissionWebhookAnnotationInjectKey]
		deploy = Mutate(deploy, slice)
		log.Info("mutated deploy", "pod metadata", deploy.Spec.Template.ObjectMeta)
	}

	marshaled, err := json.Marshal(deploy)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	return admission.PatchResponseFromRaw(req.Object.Raw, marshaled)
}

func (wh *WebhookServer) InjectDecoder(d *admission.Decoder) error {
	wh.decoder = d
	return nil
}

func Mutate(deploy *appsv1.Deployment, sliceName string) *appsv1.Deployment {
	// Add injection status to deployment annotations
	deploy.Annotations[admissionWebhookAnnotationInjectKey] = "injected"

	if deploy.Spec.Template.ObjectMeta.Annotations == nil {
		deploy.Spec.Template.ObjectMeta.Annotations = map[string]string{}
	}

	// Add vl3 annotation to pod template
	annotations := deploy.Spec.Template.ObjectMeta.Annotations
	annotations["ns.networkservicemesh.io"] = "vl3-service-" + sliceName

	// Add slice identifier labels to pod template
	labels := deploy.Spec.Template.ObjectMeta.Labels
	labels["avesha.io/pod-type"] = "app"
	labels["avesha.io/slice"] = sliceName

	return deploy
}

func MutationRequired(metadata metav1.ObjectMeta) bool {
	annotations := metadata.GetAnnotations()

	// no annotations available for the object, skip mutation
	if annotations == nil {
		return false
	}

	if metadata.GetAnnotations()[admissionWebhookAnnotationStatusKey] == "injected" {
		log.Info("Deployment is already injected")
		return false
	}

	if metadata.GetLabels()["avesha.io/pod-type"] == "app" {
		log.Info("Pod is already injected")
		return false
	}

	// TODO namespace isolation policy

	// The annotation avesha.io/slice:SLICENAME is present, enable mutation
	return annotations[admissionWebhookAnnotationInjectKey] != ""
}
