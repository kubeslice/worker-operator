package manifest

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Install istio ingress gw on the given cluster
func InstallIngress(ctx context.Context, c client.Client, slice string) error {

	deploy := &appsv1.Deployment{}
	err := NewManifest("../../files/ingress/ingress-deploy.json").Parse(deploy)
	if err != nil {
		return err
	}

	// Add the deployment to slice
	deploy.Labels["slice"] = slice
	deploy.Annotations = map[string]string{
		"avesha.io/slice": slice,
	}

	svc := &corev1.Service{}
	err = NewManifest("../../files/ingress/ingress-svc.json").Parse(svc)
	if err != nil {
		return err
	}

	role := &rbacv1.Role{}
	err = NewManifest("../../files/ingress/ingress-role.json").Parse(role)
	if err != nil {
		return err
	}

	sa := &corev1.ServiceAccount{}
	err = NewManifest("../../files/ingress/ingress-sa.json").Parse(sa)
	if err != nil {
		return err
	}

	rb := &rbacv1.RoleBinding{}
	err = NewManifest("../../files/ingress/ingress-rolebinding.json").Parse(rb)
	if err != nil {
		return err
	}

	objects := []client.Object{
		deploy,
		svc,
		role,
		sa,
		rb,
	}

	for _, o := range objects {
		if err := c.Create(ctx, o); err != nil {
			return err
		}
	}

	return nil
}
