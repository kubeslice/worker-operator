package workerslicegwrecycler

import (
	"context"

	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	"github.com/kubeslice/worker-operator/controllers"
	"github.com/kubeslice/worker-operator/controllers/slicegateway"
	"github.com/kubeslice/worker-operator/pkg/router"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *Reconciler) CreateNewDeployment(ctx context.Context, depName string, sliceName, sliceGwName string) (ctrl.Result, error, bool) {
	log := r.Log
	// Check if the deployment is already created
	deployments, err := slicegateway.GetDeployments(ctx, r.MeshClient, sliceName, sliceGwName)
	if err != nil {
		return ctrl.Result{}, err, true
	}

	for _, dep := range deployments.Items {
		if dep.Name == depName {
			return ctrl.Result{}, nil, false
		}
	}

	// Trigger slicegateway controller to create a new intermediate deployment.
	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		sliceGw := kubeslicev1beta1.SliceGateway{}
		err := r.MeshClient.Get(ctx, types.NamespacedName{Namespace: "kubeslice-system", Name: sliceGwName}, &sliceGw)
		if err != nil {
			return err
		}
		for _, intrmDep := range sliceGw.Status.Config.SliceGatewayIntermediateDeployments {
			if intrmDep == depName {
				return nil
			}
		}
		intermediateDeployments := append(sliceGw.Status.Config.SliceGatewayIntermediateDeployments, depName)
		sliceGw.Status.Config.SliceGatewayIntermediateDeployments = intermediateDeployments
		log.Info("Updating slicegw status:", "Added dep to interim list:", intermediateDeployments)
		return r.MeshClient.Status().Update(ctx, &sliceGw)
	})
	if err != nil {
		return ctrl.Result{}, err, true
	}

	return ctrl.Result{}, nil, false
}

func (r *Reconciler) CheckIfDeploymentIsPresent(ctx context.Context, depName string, sliceName, sliceGwName string) bool {
	// Check if the deployment is already created
	deployments, err := slicegateway.GetDeployments(ctx, r.MeshClient, sliceName, sliceGwName)
	if err != nil {
		return false
	}

	for _, dep := range deployments.Items {
		if dep.Name == depName {
			return true
		}
	}

	return false
}

func (r *Reconciler) MarkGwRouteForDeletion(ctx context.Context, sliceGw *kubeslicev1beta1.SliceGateway, depName string) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		pod, err := slicegateway.GetPodForGwDeployment(ctx, r.MeshClient, depName)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}
		if pod == nil {
			return nil
		}

		podLabels := pod.ObjectMeta.Labels
		if podLabels == nil {
			return err
		}
		podLabels["kubeslice.io/exclude-gw-route"] = "true"
		pod.ObjectMeta.Labels = podLabels

		return r.MeshClient.Update(ctx, pod)
	})
	if err != nil {
		return err
	}

	return nil
}

func (r *Reconciler) GetNsmIPForNewDeployment(ctx context.Context, sliceGw *kubeslicev1beta1.SliceGateway, depName string) (string, error) {
	nsmIPs, err := slicegateway.GetNsmIPsForGwDeployment(ctx, r.MeshClient, sliceGw.Name, depName)
	if err != nil {
		return "", err
	}
	if len(nsmIPs) == 0 {
		return "", nil
	}

	return nsmIPs[0], nil
}

func (r *Reconciler) CheckRouteInSliceRouter(ctx context.Context, sliceGw *kubeslicev1beta1.SliceGateway, gwNsmIP string) (bool, error) {
	_, podIP, err := controllers.GetSliceRouterPodNameAndIP(ctx, r.MeshClient, sliceGw.Spec.SliceName)
	if err != nil {
		return false, err
	}

	sidecarGrpcAddress := podIP + ":5000"
	sliceRouterConnCtx := &router.GetRouteConfig{
		RemoteSliceGwNsmSubnet: sliceGw.Status.Config.SliceGatewayRemoteSubnet,
		NsmIp:                  gwNsmIP,
	}

	res, err := r.WorkerRouterClient.GetRouteInKernel(ctx, sidecarGrpcAddress, sliceRouterConnCtx)
	if err != nil {
		return false, err
	}

	if res.IsRoutePresent {
		return true, nil
	}

	return false, nil
}

func (r *Reconciler) TriggerGwDeploymentDeletion(ctx context.Context, sliceName, sliceGwName, depName, newDepName string) error {
	log := r.Log
	// Mark the deployment for deletion
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		deployments, err := slicegateway.GetDeployments(ctx, r.MeshClient, sliceName, sliceGwName)
		if err != nil {
			return nil
		}
		if deployments == nil {
			return nil
		}

		var depToDelete *appsv1.Deployment = nil
		for _, dep := range deployments.Items {
			if dep.Name == depName {
				depToDelete = &dep
				break
			}
		}

		if depToDelete == nil {
			return nil
		}

		depLabels := depToDelete.ObjectMeta.Labels
		if depLabels == nil {
			depLabels = make(map[string]string)
		}
		if _, ok := depLabels["kubeslice.io/marked-for-deletion"]; ok {
			log.Info("Deployment already marked for deletion", "deployment", depName)
			return nil
		}
		depLabels["kubeslice.io/marked-for-deletion"] = "true"

		depToDelete.ObjectMeta.Labels = depLabels

		return r.MeshClient.Update(ctx, depToDelete)
	})
	if err != nil {
		return err
	}

	// Update the slicegw status to remove the intermediate deployment.
	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		sliceGw := kubeslicev1beta1.SliceGateway{}
		err := r.MeshClient.Get(ctx, types.NamespacedName{Namespace: "kubeslice-system", Name: sliceGwName}, &sliceGw)
		if err != nil {
			return err
		}
		updateNeeded := false
		intermediateDeployments := sliceGw.Status.Config.SliceGatewayIntermediateDeployments
		for i, intrmDep := range sliceGw.Status.Config.SliceGatewayIntermediateDeployments {
			if intrmDep == newDepName {
				if i == len(sliceGw.Status.Config.SliceGatewayIntermediateDeployments)-1 {
					intermediateDeployments = intermediateDeployments[:i]
				} else {
					intermediateDeployments = append(intermediateDeployments[:i], intermediateDeployments[i+1:]...)
				}
				updateNeeded = true
				break
			}
		}
		if updateNeeded {
			sliceGw.Status.Config.SliceGatewayIntermediateDeployments = intermediateDeployments
			log.Info("Updating slicegw status:", "Deleted dep from interim list:", newDepName)
			return r.MeshClient.Status().Update(ctx, &sliceGw)
		}
		log.Info("Intermediate deployment update not needed for slicegw status", "deployment", depName)
		return nil
	})
	if err != nil {
		return err
	}

	return nil
}
