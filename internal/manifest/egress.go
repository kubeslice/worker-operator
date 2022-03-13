package manifest

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Install istio egress gw resources on the given cluster in a slice
// Resources:
//  deployment (adds annotations to add the ingress pod to the slice)
//  serviceaccount
//  role
//  rolebinding
//  service (type clusterip)
func InstallEgress(ctx context.Context, c client.Client, slice string) error {
	deploy := &appsv1.Deployment{}
	err := NewManifest("../../files/egress/egress-deploy.json", slice).Parse(deploy)
	if err != nil {
		return err
	}

	objects := []client.Object{
		deploy,
	}

	for _, o := range objects {
		if err := c.Create(ctx, o); err != nil {
			return err
		}
	}

	return nil
}

// Uninstall istio ingress (EW) resources fo a slice from a given cluster
// Resources:
//  deployment
//  serviceaccount
//  role
//  rolebinding
//  service
func UninstallEgress(ctx context.Context, c client.Client, slice string) error {
	// TODO objects should be unique to slice

	log.Info("deleting EW ingress gw for the slice", "slice", slice)

	objects := []client.Object{
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "istio-ingressgateway",
				Namespace: "kubeslice-system",
			},
		},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "istio-ingressgateway",
				Namespace: "kubeslice-system",
			},
		},
		&rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "istio-ingressgateway-sds",
				Namespace: "kubeslice-system",
			},
		},
		&corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "istio-ingressgateway-service-account",
				Namespace: "kubeslice-system",
			},
		},
		&rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "istio-ingressgateway-sds",
				Namespace: "kubeslice-system",
			},
		},
	}

	for _, o := range objects {
		if err := c.Delete(ctx, o); err != nil {
			// Ignore the error if the resource is already deleted
			if !errors.IsNotFound(err) {
				return err
			}
		}
	}

	return nil
}
