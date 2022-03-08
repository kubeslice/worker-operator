package manifest

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Install istio ingress gw on the given cluster
func InstallIngress(ctx context.Context, c client.Client) error {

	deployManifest := NewManifest("../../files/ingress/ingress-deploy.json")
	deploy, err := deployManifest.ParseDeployment()
	if err != nil {
		return err
	}

	if err := c.Create(ctx, deploy); err != nil {
		return err
	}

	return nil
}
