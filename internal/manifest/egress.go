package manifest

import (
	"context"

	istiov1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Install istio egress gw resources on the given cluster in a slice
// Resources:
//  deployment (adds annotations to add the egress pod to the slice)
//  serviceaccount
//  role
//  rolebinding
//  service (type clusterip)
//  gateway
func InstallEgress(ctx context.Context, c client.Client, slice string) error {
	deploy := &appsv1.Deployment{}
	err := NewManifest("../../files/egress/egress-deploy.json", slice).Parse(deploy)
	if err != nil {
		return err
	}

	svc := &corev1.Service{}
	err = NewManifest("../../files/egress/egress-svc.json", slice).Parse(svc)
	if err != nil {
		return err
	}

	role := &rbacv1.Role{}
	err = NewManifest("../../files/egress/egress-role.json", slice).Parse(role)
	if err != nil {
		return err
	}

	sa := &corev1.ServiceAccount{}
	err = NewManifest("../../files/egress/egress-sa.json", slice).Parse(sa)
	if err != nil {
		return err
	}

	rb := &rbacv1.RoleBinding{}
	err = NewManifest("../../files/egress/egress-rolebinding.json", slice).Parse(rb)
	if err != nil {
		return err
	}

	gw := &istiov1beta1.Gateway{}
	err = NewManifest("../../files/egress/egress-gw.json", slice).Parse(gw)
	if err != nil {
		return err
	}

	objects := []client.Object{
		deploy,
		svc,
		role,
		sa,
		rb,
		gw,
	}

	for _, o := range objects {
		if err := c.Create(ctx, o); err != nil {
			return err
		}
	}

	return nil
}

// Uninstall istio egress (EW) resources fo a slice from a given cluster
// Resources:
//  deployment
//  serviceaccount
//  role
//  rolebinding
//  service
//  gateway
func UninstallEgress(ctx context.Context, c client.Client, slice string) error {

	log.Info("deleting EW egress gw for the slice", "slice", slice)

	objects := []client.Object{
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      slice + "-istio-egressgateway",
				Namespace: "kubeslice-system",
			},
		},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      slice + "-istio-egressgateway",
				Namespace: "kubeslice-system",
			},
		},
		&rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      slice + "-istio-egressgateway-sds",
				Namespace: "kubeslice-system",
			},
		},
		&corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      slice + "-istio-egressgateway-service-account",
				Namespace: "kubeslice-system",
			},
		},
		&rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      slice + "-istio-egressgateway-sds",
				Namespace: "kubeslice-system",
			},
		},
	}

	for _, o := range objects {
		if err := c.Delete(ctx, o); err != nil {
			// Ignore the error if the resource is already deleted
			// return error only if there is some other error
			if !errors.IsNotFound(err) {
				return err
			}
		}
	}

	return nil
}
