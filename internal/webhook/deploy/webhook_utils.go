package deploy

import (
	"context"

	"github.com/kubeslice/worker-operator/controllers"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type webhookClient struct {
}

func NewWebhookClient() *webhookClient {
	return &webhookClient{}
}

func (w *webhookClient) SliceAppNamespaceConfigured(ctx context.Context, slice string, namespace string) (bool, error) {
	return controllers.SliceAppNamespaceConfigured(ctx, slice, namespace)
}

func (w *webhookClient) GetNamespaceLabels(ctx context.Context, client client.Client, namespace string) (map[string]string, error) {

	nS := &corev1.Namespace{}
	err := client.Get(context.Background(), types.NamespacedName{Name: namespace}, nS)
	if err != nil {
		log.Info("Failed to find namespace", "namespace", namespace)
		return nil, err
	}
	nsLabels := nS.ObjectMeta.GetLabels()
	return nsLabels, nil
}
