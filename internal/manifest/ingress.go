package manifest

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Install istio ingress gw on the given cluster
func InstallIngress(ctx context.Context, c client.Client, slice string) error {

	deployManifest := NewManifest("../../files/ingress/ingress-deploy.json")
	deploy := &appsv1.Deployment{}
	err := deployManifest.Parse(deploy)
	if err != nil {
		return err
	}

	// Add the deployment to slice
	deploy.Labels["slice"] = slice
	deploy.Annotations = map[string]string{
		"avesha.io/slice": slice,
	}

	if err := c.Create(ctx, deploy); err != nil {
		return err
	}

	return nil
}
