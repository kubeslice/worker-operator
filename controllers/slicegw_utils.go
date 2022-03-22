package controllers

import (
	meshv1beta1 "bitbucket.org/realtimeai/kubeslice-operator/api/v1beta1"
	"context"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (r *SliceGwReconciler) cleanupSliceGwResources(ctx context.Context, slicegw *meshv1beta1.SliceGateway) error {
	//delete gateway secret
	meshSliceGwCerts := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      slicegw.Name,
			Namespace: ControlPlaneNamespace,
		},
	}
	if err := r.Delete(ctx, meshSliceGwCerts); err != nil {
		r.Log.Error(err, "Error Deleting Gateway Secret while cleaning up.. Please Delete it before installing slice again")
		return err
	}
	return nil
}
