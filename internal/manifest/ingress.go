package manifest

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Install istio ingress gw on the given cluster
func InstallIngress(ctx context.Context, c client.Client, slice string) error {

	deployManifest := NewManifest("../../files/ingress/ingress-deploy.json")
	deploy, err := deployManifest.ParseDeployment()
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
