package pod

import (
	"context"
	"strings"

	"github.com/kubeslice/apis/pkg/controller/v1alpha1"
	"github.com/kubeslice/worker-operator/api/v1beta1"
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

// Fetch all serviceexport objects belonging to the slice
func (w *webhookClient) GetAllServiceExports(ctx context.Context, c client.Client, slice string) (*v1beta1.ServiceExportList, error) {

	listOpts := []client.ListOption{
		client.MatchingLabels(map[string]string{
			controllers.ApplicationNamespaceSelectorLabelKey: slice,
		},
		),
	}

	serviceExportList := &v1beta1.ServiceExportList{}
	if err := c.List(ctx, serviceExportList, listOpts...); err != nil {
		log.Info("Failed to get ServiceExportList", "slice", slice)
		return nil, err
	}
	return serviceExportList, nil
}

func aliasExist(existingAliases []string, newAlias string) bool {
	for _, alias := range existingAliases {
		if strings.EqualFold(alias, newAlias) {
			return true
		}
	}
	return false
}

func (w *webhookClient) GetSliceOverlayNetworkType(ctx context.Context, client client.Client, sliceName string) (v1alpha1.NetworkType, error) {
	return controllers.GetSliceOverlayNetworkType(ctx, client, sliceName)
}
