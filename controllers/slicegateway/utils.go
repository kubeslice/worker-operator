package slicegateway

import (
	"context"

	kubeslicev1beta1 "github.com/kubeslice/operator/api/v1beta1"
	"github.com/kubeslice/operator/internal/hub/controllers"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (r *SliceGwReconciler) cleanupSliceGwResources(ctx context.Context, slicegw *kubeslicev1beta1.SliceGateway) error {
	//delete gateway secret
	kubeSliceGwCerts := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      slicegw.Name,
			Namespace: controllers.ControlPlaneNamespace,
		},
	}
	if err := r.Delete(ctx, kubeSliceGwCerts); err != nil {
		r.Log.Error(err, "Error Deleting Gateway Secret while cleaning up.. Please Delete it before installing slice again")
		return err
	}
	return nil
}
