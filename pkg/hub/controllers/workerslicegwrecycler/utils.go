package workerslicegwrecycler

import (
	"context"
	"errors"
	"time"

	retry "github.com/avast/retry-go"
	spokev1alpha1 "github.com/kubeslice/apis/pkg/worker/v1alpha1"
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	"github.com/kubeslice/worker-operator/controllers"
	"github.com/kubeslice/worker-operator/pkg/logger"
	"github.com/kubeslice/worker-operator/pkg/router"
	webhook "github.com/kubeslice/worker-operator/pkg/webhook/pod"
	"github.com/looplab/fsm"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *Reconciler) verify_new_deployment_created(e *fsm.Event) error {
	// we need to verify the number of deployments, shouold be 3 on both client and server and wait till the new pod is up and running and update this new pods name in workerslicegwrecycler object and move the fsm to new state = new_deployment_created
	ctx := context.Background()
	log := logger.FromContext(ctx).WithName("workerslicegwrecycler")
	// List all the available gw deployments
	// TODO: Move this redundant code to a func
	deployList := &appsv1.DeploymentList{}
	deployLabels := map[string]string{webhook.PodInjectLabelKey: "slicegateway"}
	listOpts := []client.ListOption{
		client.InNamespace(controllers.ControlPlaneNamespace),
		client.MatchingLabels(deployLabels),
	}
	err := r.MeshClient.List(ctx, deployList, listOpts...)
	if err != nil {
		r.Log.Error(err, "Failed to List gw deployments")
		return err
	}

	workerslicegwrecycler := e.Args[0].(*spokev1alpha1.WorkerSliceGwRecycler)
	isClient := e.Args[1].(bool)

	gwPod := corev1.Pod{}
	if isClient {
		if err := r.MeshClient.Get(context.Background(), types.NamespacedName{Namespace: "kubeslice-system", Name: workerslicegwrecycler.Spec.GwPair.ClientID}, &gwPod); err != nil {
			return err
		}
		// Check if Number of GW deployments should equal to 3
		if len(deployList.Items) != 3 {
			return errors.New("number of GW deployemt should equal to 3")
		}
		newGwDeploy, err := r.getNewestGwDeploy(ctx, gwPod.Labels["kubeslice.io/slice-gw"])
		if err != nil {
			return err
		}
		wait.Poll(1*time.Second, 60*time.Second, func() (done bool, err error) {
			return newGwDeploy.Status.Replicas == newGwDeploy.Status.AvailableReplicas, nil
		})
	} else {
		if err := r.MeshClient.Get(context.Background(), types.NamespacedName{Namespace: "kubeslice-system", Name: workerslicegwrecycler.Spec.GwPair.ServerID}, &gwPod); err != nil {
			return err
		}
		// Check if Number of GW deployments should equal to 3
		if len(deployList.Items) != 3 {
			return errors.New("number of deployemt should equal to 3")
		}
		// Select the newest deploy
		newGwDeploy, err := r.getNewestGwDeploy(ctx, gwPod.Labels["kubeslice.io/slice-gw"])
		if err != nil {
			return err
		}
		wait.Poll(1*time.Second, 60*time.Second, func() (done bool, err error) {
			return newGwDeploy.Status.Replicas == newGwDeploy.Status.AvailableReplicas, nil
		})
	}

	log.V(1).Info("removing label from pods", "pod", gwPod)
	delete(gwPod.Labels, "kubeslice.io/pod-type")
	gwPod.Labels["kubeslice.io/pod-type"] = "toBeDeleted"
	err = r.MeshClient.Update(context.Background(), &gwPod)
	if err != nil {
		return err
	}
	// wait till the replicas are back and get the latest spawned pod
	gwDeploy := appsv1.Deployment{}
	if isClient {
		wait.Poll(1*time.Second, 60*time.Second, func() (done bool, err error) {
			err = r.MeshClient.Get(context.Background(), types.NamespacedName{Namespace: "kubeslice-system", Name: workerslicegwrecycler.Spec.SliceGwClient}, &gwDeploy)
			if err != nil {
				return false, err
			}
			return gwDeploy.Status.Replicas == gwDeploy.Status.AvailableReplicas, nil
		})
	} else {
		wait.Poll(1*time.Second, 60*time.Second, func() (done bool, err error) {
			err = r.MeshClient.Get(context.Background(), types.NamespacedName{Namespace: "kubeslice-system", Name: workerslicegwrecycler.Spec.SliceGwServer}, &gwDeploy)
			if err != nil {
				return false, err
			}
			return gwDeploy.Status.Replicas == gwDeploy.Status.AvailableReplicas, nil
		})
	}
	podList := corev1.PodList{}
	labels := map[string]string{"kubeslice.io/pod-type": "slicegateway", "kubeslice.io/slice": workerslicegwrecycler.Spec.SliceName}
	listOptions := []client.ListOption{
		client.MatchingLabels(labels),
	}
	err = r.MeshClient.List(ctx, &podList, listOptions...)
	if err != nil {
		return err
	}
	newestPod := podList.Items[0]
	newestPodDuration := time.Since(podList.Items[0].CreationTimestamp.Time).Seconds()
	for _, pod := range podList.Items {
		duration := time.Since(pod.CreationTimestamp.Time).Seconds()
		if duration < newestPodDuration {
			newestPodDuration = duration
			newestPod = pod
		}
	}
	if isClient {
		workerslicegwrecycler.Status.Client.Response = "new_gw_spawned"
		workerslicegwrecycler.Status.Client.RecycledClient = newestPod.Name
		return r.Status().Update(ctx, workerslicegwrecycler)
	} else {
		// progress the FSM to the next state by updating the CR object with the next state: new_gw_spawned
		workerslicegwrecycler.Spec.GwPair.ClientID = workerslicegwrecycler.Status.Client.RecycledClient
		workerslicegwrecycler.Spec.GwPair.ServerID = newestPod.Name
		workerslicegwrecycler.Spec.State = new_gw_spawned
		workerslicegwrecycler.Spec.Request = update_routing_table
	}
	return r.Update(ctx, workerslicegwrecycler)
}

func (r *Reconciler) update_routing_table(e *fsm.Event) error {
	ctx := context.Background()
	log := logger.FromContext(ctx).WithName("workerslicegwrecycler")

	log.Info("in update_routing_table", "current_state", r.FSM.Current())

	workerslicegwrecycler := e.Args[0].(*spokev1alpha1.WorkerSliceGwRecycler)
	isClient := e.Args[1].(bool)
	slicegateway := e.Args[2].(kubeslicev1beta1.SliceGateway)
	var nsmIPOfNewGwPod string

	// fsm library used does not has the error handling mechanism for callback currently,
	// hence we need to retry in case of errors
	retry.Do(func() error {
		var gwPod string
		if isClient {
			gwPod = workerslicegwrecycler.Status.Client.RecycledClient
		} else {
			gwPod = workerslicegwrecycler.Spec.GwPair.ServerID
		}
		// fetch the latest slicegw object
		if isClient {
			if err := r.MeshClient.Get(ctx, types.NamespacedName{Namespace: "kubeslice-system", Name: workerslicegwrecycler.Spec.SliceGwClient}, &slicegateway); err != nil {
				log.Error(err, "error fetching slicegw")
				return err
			}
		} else {
			if err := r.MeshClient.Get(ctx, types.NamespacedName{Namespace: "kubeslice-system", Name: workerslicegwrecycler.Spec.SliceGwServer}, &slicegateway); err != nil {
				log.Error(err, "error fetching slicegw")
				return err
			}
		}

		log.Info("recycled pod", "podname", gwPod)

		nsmIPOfNewGwPod = getNsmIp(&slicegateway, gwPod)
		if nsmIPOfNewGwPod == "" {
			log.Info("nsmIPOfNewGwPod not populated yet..empty")
			return errors.New("nsmIPOfNewGwPod not populated yet..empty")
		}

		log.Info("nsmIPOfNewGwPod", "nsmIPOfNewGwPod", nsmIPOfNewGwPod)

		// call router func to verify if route was added
		_, podIP, err := controllers.GetSliceRouterPodNameAndIP(ctx, r.MeshClient, slicegateway.Spec.SliceName)
		if err != nil {
			log.Error(err, "Unable to get slice router pod info")
			return err
		}

		sidecarGrpcAddress := podIP + ":5000"
		sliceRouterConnCtx := &router.GetRouteConfig{
			RemoteSliceGwNsmSubnet: slicegateway.Status.Config.SliceGatewayRemoteSubnet,
			NsmIp:                  nsmIPOfNewGwPod,
		}
		res, err := r.WorkerRouterClient.GetRouteInKernel(ctx, sidecarGrpcAddress, sliceRouterConnCtx)
		if err != nil {
			log.Error(err, "error in GetRouteInKernel")
			return err
		}
		log.Info("is route injected", "res", res)
		if !res.IsRoutePresent {
			return errors.New("route not yet present")
		}
		return nil
	}, retry.Attempts(5000), retry.Delay(1*time.Second))

	// use retry.RetryOnConflict
	if isClient {
		workerslicegwrecycler.Status.Client.Response = slicerouter_updated
		return r.Status().Update(ctx, workerslicegwrecycler)
	}

	workerslicegwrecycler.Spec.State = slicerouter_updated
	workerslicegwrecycler.Spec.Request = delete_old_gw_pods

	return r.Update(ctx, workerslicegwrecycler)
}

func (r *Reconciler) delete_old_gw_pods(e *fsm.Event) error {
	ctx := context.Background()
	log := logger.FromContext(ctx).WithName("workerslicegwrecycler")

	log.Info("Deleteing Old gw pods")

	workerslicegwrecycler := e.Args[0].(*spokev1alpha1.WorkerSliceGwRecycler)
	isClient := e.Args[1].(bool)
	slicegateway := e.Args[2].(kubeslicev1beta1.SliceGateway)
	// before deleting old gw_pods delete the route from vl3 router

	retry.Do(func() error {
		podList := corev1.PodList{}
		labels := map[string]string{"kubeslice.io/pod-type": "toBeDeleted", "kubeslice.io/slice": workerslicegwrecycler.Spec.SliceName}
		listOptions := []client.ListOption{
			client.MatchingLabels(labels),
		}
		err := r.MeshClient.List(ctx, &podList, listOptions...)
		if err != nil {
			return err
		}
		grpcAdd := podList.Items[0].Status.PodIP + ":5000"

		status, err := r.WorkerGWSidecarClient.GetStatus(ctx, grpcAdd)
		if err != nil {
			r.Log.Error(err, "Unable to fetch gw status")
			return err
		}
		nsmIP := status.NsmStatus.LocalIP
		_, podIP, err := controllers.GetSliceRouterPodNameAndIP(ctx, r.MeshClient, slicegateway.Spec.SliceName)
		if err != nil {
			r.Log.Error(err, "Unable to get slice router pod info")
			return err
		}
		if podIP == "" {
			r.Log.Info("Slice router podIP not available yet, requeuing")
			return err
		}

		if slicegateway.Status.Config.SliceGatewayRemoteSubnet == "" ||
			len(slicegateway.Status.GatewayPodStatus) == 0 {
			r.Log.Info("Waiting for remote subnet and local nsm IPs. Delaying conn ctx update to router")
			return err
		}

		sidecarGrpcAddress := podIP + ":5000"
		routeInfo := &router.UpdateEcmpInfo{
			RemoteSliceGwNsmSubnet: slicegateway.Status.Config.SliceGatewayRemoteSubnet,
			NsmIpToDelete:          nsmIP,
		}
		err = r.WorkerRouterClient.UpdateEcmpRoutes(ctx, sidecarGrpcAddress, routeInfo)
		if err != nil {
			r.Log.Error(err, "Unable to update ecmp routes in the slice router")
			return err
		}
		return nil
	}, retry.Attempts(5000), retry.Delay(1*time.Second))

	podList := corev1.PodList{}
	labels := map[string]string{"kubeslice.io/pod-type": "toBeDeleted", "kubeslice.io/slice": workerslicegwrecycler.Spec.SliceName}
	listOptions := []client.ListOption{
		client.MatchingLabels(labels),
	}
	err := r.MeshClient.List(ctx, &podList, listOptions...)
	if err != nil {
		return err
	}
	r.Log.Info("old gw pods to be deleted", "podList", podList)
	//TODO:add rbac in charts to include delete verb
	err = r.MeshClient.Delete(ctx, &podList.Items[0])
	if err != nil {
		return err
	}
	if isClient {
		workerslicegwrecycler.Status.Client.Response = old_gw_deleted
		return r.Status().Update(ctx, workerslicegwrecycler)
	}
	workerslicegwrecycler.Spec.State = old_gw_deleted
	workerslicegwrecycler.Spec.Request = "end"
	err = r.Update(ctx, workerslicegwrecycler)
	if err != nil {
		return err
	}
	// FSM complete - delete workergwrecycler
	return r.Delete(ctx, workerslicegwrecycler)
}

func getNsmIp(slicegw *kubeslicev1beta1.SliceGateway, podName string) string {
	for _, gwPod := range slicegw.Status.GatewayPodStatus {
		if gwPod.PodName == podName {
			return gwPod.LocalNsmIP
		}
	}
	return ""
}

func (r *Reconciler) getNewestGwDeploy(ctx context.Context, sliceGwName string) (*appsv1.Deployment, error) {
	deployList := &appsv1.DeploymentList{}
	labels := map[string]string{"kubeslice.io/slice-gw": sliceGwName}
	listOpts := []client.ListOption{
		client.InNamespace(controllers.ControlPlaneNamespace),
		client.MatchingLabels(labels),
	}
	err := r.MeshClient.List(ctx, deployList, listOpts...)
	if err != nil {
		r.Log.Error(err, "Failed to List gw deployments")
		return &appsv1.Deployment{}, err
	}
	newestDeploy := deployList.Items[0]
	newestDeployDuration := time.Since(deployList.Items[0].CreationTimestamp.Time).Seconds()
	for _, deploy := range deployList.Items {
		duration := time.Since(deploy.CreationTimestamp.Time).Seconds()
		if duration < newestDeployDuration {
			newestDeployDuration = duration
			newestDeploy = deploy
		}
	}
	return &newestDeploy, nil
}
