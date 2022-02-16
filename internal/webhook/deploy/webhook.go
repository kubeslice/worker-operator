package deploy

import (
	"context"
	"encoding/json"
	"net/http"

	"bitbucket.org/realtimeai/kubeslice-operator/internal/logger"
	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
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

	log.Info("mutating deploy", "deploy", deploy)

	// TODO: mutate

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
