package workerslicegwrecycler

import (
	//"context"
	//"errors"
	"strings"
	/*
		"os"
		"strconv"
		"time"

		retry "github.com/avast/retry-go"
		spokev1alpha1 "github.com/kubeslice/apis/pkg/worker/v1alpha1"
		kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
		"github.com/kubeslice/worker-operator/controllers"
		//slicegwpkg "github.com/kubeslice/worker-operator/controllers/slicegateway"
		"github.com/kubeslice/worker-operator/pkg/logger"
		"github.com/kubeslice/worker-operator/pkg/router"
		"github.com/looplab/fsm"
		appsv1 "k8s.io/api/apps/v1"
		corev1 "k8s.io/api/core/v1"
		apierrors "k8s.io/apimachinery/pkg/api/errors"
		"k8s.io/apimachinery/pkg/types"
		"sigs.k8s.io/controller-runtime/pkg/client"
		"sigs.k8s.io/controller-runtime/pkg/reconcile"
	*/)

func getNewDeploymentName(gwID string) string {
	l := strings.Split(gwID, "-")
	depInstance := l[len(l)-1]

	newDepInstance := "1"

	if depInstance == "0" {
		newDepInstance = "1"
	} else {
		newDepInstance = "0"
	}

	l[len(l)-1] = newDepInstance

	return strings.Join(l, "-")
}

func getRequestString(req Request) string {
	switch req {
	case REQ_none:
		return "none"
	case REQ_create_new_deployment:
		return "create_new_deployment"
	case REQ_update_routing_table:
		return "update_routing_table"
	case REQ_delete_old_gw_deployment:
		return "delete_old_gw_deployment"
	default:
		return ""
	}
}

func getRequestIndex(req string) Request {
	switch req {
	case "none":
		return REQ_none
	case "create_new_deployment":
	    return REQ_create_new_deployment
	case "update_routing_table":
		return REQ_update_routing_table
	case "delete_old_gw_deployment": 
	    return REQ_delete_old_gw_deployment
	default:
		return REQ_none
	}
}

func getResponseString(resp Response) string {
	switch resp {
	case RESP_none:
		return "none"
	case RESP_new_deployment_created:
		return "new_deployment_created"
	case RESP_routing_table_updated:
		return "routing_table_updated"
	case RESP_old_deployment_deleted:
		return "old_gw_deployment_deleted"
	default:
		return ""
	}
}

func getResponseIndex(resp string) Response {
	switch resp {
	case "none":
		return RESP_none
	case "new_deployment_created":
		return RESP_new_deployment_created
	case "routing_table_updated":
		return RESP_routing_table_updated
	case "old_gw_deployment_deleted":
		return RESP_old_deployment_deleted
	default:
		return RESP_none
	}
}

/*

func (r *Reconciler) verify_new_deployment_created(e *fsm.Event) error {
	// we need to verify the number of deployments presnt
	// should be 2 (one old and newly created) on both client and server.
	// wait till the new deployment is up and running and update this new pods name in workerslicegwrecycler object and move the fsm to new state = new_deployment_created
	ctx := context.Background()

	workerslicegwrecycler := e.Args[0].(*spokev1alpha1.WorkerSliceGwRecycler)
	log := logger.FromContext(ctx).WithName("workerslicegwrecycler").WithName(workerslicegwrecycler.Name)
	log.Info("In verify new dep created")
	return nil


	isClient := e.Args[1].(bool)
	req := e.Args[2].(reconcile.Request)
	var slicegateway string
	// List all the available gw deployments
	deployList := &appsv1.DeploymentList{}
	gwPod := corev1.Pod{}
	if isClient {
		err := retry.Do(func() error {
			// update the ClientRedundancyNumber in workerslicegwrecycler
			if err := r.MeshClient.Get(context.Background(), types.NamespacedName{Namespace: "kubeslice-system", Name: workerslicegwrecycler.Spec.GwPair.ClientID}, &gwPod); err != nil {
				return err
			}
			clientRedundancyNumber, _ := strconv.Atoi(gwPod.Labels["kubeslice.io/slicegatewayRedundancyNumber"])
			workerslicegwrecycler.Spec.ClientRedundancyNumber = clientRedundancyNumber
			err := r.Update(ctx, workerslicegwrecycler)
			if err != nil {
				return err
			}

			slicegateway = workerslicegwrecycler.Spec.SliceGwClient
			deployLabels := getDeployLabels(workerslicegwrecycler, isClient)
			listOpts := []client.ListOption{
				client.InNamespace(controllers.ControlPlaneNamespace),
				client.MatchingLabels(deployLabels),
			}
			err = r.MeshClient.List(ctx, deployList, listOpts...)
			if err != nil {
				log.Error(err, "failed to list gw deployments")
				return err
			}
			// Check if Number of GW deployments should equal to 2
			if len(deployList.Items) != 2 {
				return errors.New("number of gateway deployments should be equal to 2")
			}
			newGwDeploy, err := r.getNewestGwDeploy(ctx, workerslicegwrecycler.Spec.SliceGwClient)
			if err != nil {
				log.Error(err, "error in getNewestGwDeploy")
				return err
			}

			if newGwDeploy.Status.Replicas != newGwDeploy.Status.AvailableReplicas {
				return errors.New("waiting for the new gw pod to be up and running")
			}
			log.Info("new deployment is up for client", "workerslicegwrecycler", workerslicegwrecycler.Name)

			return nil
		}, retry.Attempts(200))

		if err != nil {
			log.Error(err, "error in verify_new_deployment_created event")
			r.moveFSMToErrorState(req, err)
			workerslicegwrecycler.Status.Client.Response = ERROR
			_ = r.Status().Update(ctx, workerslicegwrecycler)
			return err
		}

	} else {
		err := retry.Do(func() error {
			slicegateway = workerslicegwrecycler.Spec.SliceGwServer
			deployLabels := getDeployLabels(workerslicegwrecycler, isClient)
			listOpts := []client.ListOption{
				client.InNamespace(controllers.ControlPlaneNamespace),
				client.MatchingLabels(deployLabels),
			}
			err := r.MeshClient.List(ctx, deployList, listOpts...)
			if err != nil {
				log.Error(err, "failed to list gateway deployments")
				return err
			}
			// Check if Number of GW deployments should equal to 2
			if len(deployList.Items) != 2 {
				return errors.New("number of GW deployemt should be equal to 2")
			}
			newGwDeploy, err := r.getNewestGwDeploy(ctx, workerslicegwrecycler.Spec.SliceGwServer)
			if err != nil {
				log.Error(err, "Error:", err.Error())
				return err
			}
			if newGwDeploy.Status.Replicas != newGwDeploy.Status.AvailableReplicas {
				return errors.New("waiting for gw pod to be up and running")
			}
			log.Info("new deployment is up for server", "workerslicegwrecycler", workerslicegwrecycler.Name)
			return nil
		}, retry.Attempts(100))

		if err != nil {
			log.Error(err, "error in verify_new_deployment_created")
			r.moveFSMToErrorState(req, err)
			workerslicegwrecycler.Spec.State = ERROR
			_ = r.Status().Update(ctx, workerslicegwrecycler)
			return err
		}

		if err := r.MeshClient.Get(context.Background(), types.NamespacedName{Namespace: "kubeslice-system", Name: workerslicegwrecycler.Spec.GwPair.ServerID}, &gwPod); err != nil {
			return err
		}
	}

	err := retry.Do(func() error {
		// add this label for future reference
		gwPod.Labels["kubeslice.io/gw-pod-type"] = "toBeDeleted"
		err := r.MeshClient.Update(context.Background(), &gwPod)
		if err != nil {
			return err
		}
		log.Info("labelled gateway pod to be deleted")

		podList := corev1.PodList{}
		labels := getPodLabels(workerslicegwrecycler, slicegateway, isClient)
		listOptions := []client.ListOption{
			client.MatchingLabels(labels),
		}
		err = r.MeshClient.List(ctx, &podList, listOptions...)
		if err != nil {
			log.Error(err, "Error:", err.Error())
			return err
		}

		newestPod := podList.Items[0]
		newestPodTime := podList.Items[0].CreationTimestamp.Time

		for _, pod := range podList.Items {
			if pod.Labels["kubeslice.io/gw-pod-type"] == "toBeDeleted" {
				continue
			}
			if pod.CreationTimestamp.Time.After(newestPodTime) {
				newestPodTime = pod.CreationTimestamp.Time
				newestPod = pod
			}
		}

		if isClient {
			workerslicegwrecycler.Status.Client.Response = new_deployment_created
			workerslicegwrecycler.Status.Client.RecycledClient = newestPod.Name
			return r.Status().Update(ctx, workerslicegwrecycler)
		} else {
			// progress the FSM to the next state by updating the CR object with the next state: new_gw_spawned
			workerslicegwrecycler.Spec.GwPair.ServerID = newestPod.Name
			workerslicegwrecycler.Spec.State = new_deployment_created
			workerslicegwrecycler.Spec.Request = update_routing_table
		}
		return r.Update(ctx, workerslicegwrecycler)
	}, retry.Attempts(200))

	if err != nil {
		r.moveFSMToErrorState(req, err)
		if isClient {
			workerslicegwrecycler.Status.Client.Response = ERROR
			_ = r.Status().Update(ctx, workerslicegwrecycler)
		} else {
			workerslicegwrecycler.Spec.State = ERROR
			_ = r.Status().Update(ctx, workerslicegwrecycler)
		}
		return err
	}
	return nil

}

func (r *Reconciler) update_routing_table(e *fsm.Event) error {
	ctx := context.Background()
	workerslicegwrecycler := e.Args[0].(*spokev1alpha1.WorkerSliceGwRecycler)
	log := logger.FromContext(ctx).WithName("workerslicegwrecycler").WithName(workerslicegwrecycler.Name)
	log.Info("In update routing table")
	return nil


	isClient := e.Args[1].(bool)
	slicegateway := e.Args[2].(kubeslicev1beta1.SliceGateway)
	req := e.Args[3].(reconcile.Request)
	var nsmIPOfNewGwPod string

	log.Info("checking for routes update in vl3 pod")
	// fsm library used does not has the error handling mechanism for callback currently,
	// hence we need to retry in case of errors
	err := retry.Do(func() error {
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
	}, retry.Attempts(100))

	if err != nil {
		log.Error(err, "Err():", err.Error())
		r.moveFSMToErrorState(req, err)
		if isClient {
			workerslicegwrecycler.Status.Client.Response = ERROR
			_ = r.Status().Update(ctx, workerslicegwrecycler)
		} else {
			workerslicegwrecycler.Spec.State = ERROR
			_ = r.Status().Update(ctx, workerslicegwrecycler)
		}
		return err
	}

	if isClient {
		err = retry.Do(func() error {
			if err := r.Get(ctx, types.NamespacedName{Namespace: req.Namespace, Name: req.Name}, workerslicegwrecycler); err != nil {
				log.Error(err, "error getting workerslicegwrecycler")
				return err
			}
			workerslicegwrecycler.Status.Client.Response = slicerouter_updated
			return r.Status().Update(ctx, workerslicegwrecycler)
		}, retry.Attempts(20))
		return err
	}
	// once the routes are updated its time for server cluster to delete the old routes from vl3 pod , delete the old deployment and svc
	// step1: delete the routes from vl3 router
	// once the routes are updated its time for server cluster to delete the old routes from vl3 pod , delete the old deployment and svc
	// step1: delete the routes from vl3 router
	err = retry.Do(func() error {
		podList := corev1.PodList{}
		labels := map[string]string{
			"kubeslice.io/gw-pod-type":                  "toBeDeleted",
			"kubeslice.io/slice":                        workerslicegwrecycler.Spec.SliceName,
			"kubeslice.io/slicegatewayRedundancyNumber": fmt.Sprint(workerslicegwrecycler.Spec.ServerRedundancyNumber),
			"kubeslice.io/slice-gw":                     slicegateway.Name,
		}
		listOptions := []client.ListOption{
			client.MatchingLabels(labels),
		}
		err := r.MeshClient.List(ctx, &podList, listOptions...)
		if err != nil {
			return err
		}

		// if the control comes here,(after returning from error at next steps) and if pod is already deleted, move ahead
		if len(podList.Items) == 0 {
			log.V(1).Info("No pods with label kubeslice.io/gw-pod-type:toBeDeleted found.")
			return nil
		} else if len(podList.Items) == 1 {
			var (
				grpcAdd       string
				nsmIP         string
				nodePortValue int32
				deployName    string
			)

			grpcAdd = podList.Items[0].Status.PodIP + ":5000"

			status, err := r.WorkerGWSidecarClient.GetStatus(ctx, grpcAdd)
			if err != nil {
				log.Error(err, "unable to fetch gw status")
				return err
			}
			nsmIP = status.NsmStatus.LocalIP

			_, podIP, err := controllers.GetSliceRouterPodNameAndIP(ctx, r.MeshClient, slicegateway.Spec.SliceName)
			if err != nil {
				log.Error(err, "unable to get slice router pod info")
				return err
			}
			if podIP == "" {
				log.Info("slice router podIP not available yet, requeuing")
				return err
			}

			if slicegateway.Status.Config.SliceGatewayRemoteSubnet == "" ||
				len(slicegateway.Status.GatewayPodStatus) == 0 {
				log.Info("waiting for remote subnet and local nsm IPs. Delaying conn ctx update to router")
				return err
			}

			sidecarGrpcAddress := podIP + ":5000"
			routeInfo := &router.UpdateEcmpInfo{
				RemoteSliceGwNsmSubnet: slicegateway.Status.Config.SliceGatewayRemoteSubnet,
				NsmIpToDelete:          nsmIP,
			}
			err = r.WorkerRouterClient.UpdateEcmpRoutes(ctx, sidecarGrpcAddress, routeInfo)
			if err != nil {
				log.Error(err, "unable to update ecmp routes in the slice router")
				return err
			}

			// fetch the replicaset and deployment from podName
			rsName := podList.Items[0].ObjectMeta.OwnerReferences[0].Name
			rs := appsv1.ReplicaSet{}
			if err := r.MeshClient.Get(ctx, types.NamespacedName{Namespace: controllers.ControlPlaneNamespace, Name: rsName}, &rs); err != nil {
				log.Error(err, "error getting replicaset")
				return err
			}
			deployName = rs.ObjectMeta.OwnerReferences[0].Name
			// delete nodePort service, name = svc-deployName
			nodePortService := corev1.Service{}
			if err := r.MeshClient.Get(ctx, types.NamespacedName{Namespace: controllers.ControlPlaneNamespace, Name: "svc-" + deployName}, &nodePortService); err != nil {
				return err
			}
			// if service is present, then delete it and wait till nodeport is removed from WorkerSliceGateway
			if err == nil {
				// service exists, so delete it
				nodePortValue = nodePortService.Spec.Ports[0].NodePort
				err = r.MeshClient.Delete(ctx, &nodePortService)
				if err != nil {
					return err
				}

				err = retry.Do(func() error {
					sliceGw := &spokev1alpha1.WorkerSliceGateway{}
					err = r.Get(ctx, types.NamespacedName{
						Name:      slicegateway.Name,
						Namespace: os.Getenv("HUB_PROJECT_NAMESPACE"),
					}, sliceGw)

					if err != nil {
						return err
					}

					log.V(1).Info("number of nodeports", "nodeports", len(sliceGw.Spec.LocalGatewayConfig.NodePorts))
					if isPresent(sliceGw.Spec.LocalGatewayConfig.NodePorts, int(nodePortValue)) {
						// node port not present, break
						return fmt.Errorf("nodeport still present in WorkerSliceGateway")
					}
					return nil
				}, retry.Attempts(20))

			} else if apierrors.IsNotFound(err) {
				// Service is not found, move ahead without returning an error
				log.V(1).Info("Service not found. Continuing without deletion.")
			} else {
				// Other errors occurred during the Get operation
				return err
			}
			// step3: delete the deployment
			deployToBeDeleted := appsv1.Deployment{}
			if err = r.MeshClient.Get(ctx, types.NamespacedName{Namespace: controllers.ControlPlaneNamespace, Name: deployName}, &deployToBeDeleted); err != nil {
				log.Error(err, "error getting deployment")
				return err
			}
			err = r.MeshClient.Delete(ctx, &deployToBeDeleted)
			if err != nil {
				return err
			}
			return nil
		} else {
			log.V(1).Info("pod list", "pod list", podList.Items)
			log.Error(fmt.Errorf("more than 1 pod with label kubeslice.io/gw-pod-type:toBeDeleted, something went wrong"), "Err:()")
			return errors.New("error deleting old gw pods after removing routes")
		}
	}, retry.Attempts(500))
	if err != nil {
		log.Error(err, "Error:()", err.Error())
		r.moveFSMToErrorState(req, err)
		workerslicegwrecycler.Spec.State = ERROR
		_ = r.Status().Update(ctx, workerslicegwrecycler)
		return err
	}
	workerslicegwrecycler.Spec.State = slicerouter_updated
	workerslicegwrecycler.Spec.Request = delete_old_gw_pods

	err = retry.Do(func() error {
		return r.Update(ctx, workerslicegwrecycler)
	})
	return nil

}

func isPresent(nodePorts []int, v int) bool {
	for _, j := range nodePorts {
		if j == v {
			return true
		}
	}
	return false
}

func (r *Reconciler) delete_old_gw_pods(e *fsm.Event) error {
	ctx := context.Background()

	workerslicegwrecycler := e.Args[0].(*spokev1alpha1.WorkerSliceGwRecycler)
	log := logger.FromContext(ctx).WithName("workerslicegwrecycler").WithName(workerslicegwrecycler.Name)

	log.Info("deleteing old gateway deployment")
	isClient := e.Args[1].(bool)
	slicegateway := e.Args[2].(kubeslicev1beta1.SliceGateway)
	req := e.Args[3].(reconcile.Request)
	deployToBeDeleted := appsv1.Deployment{}
	// before deleting old gw deployment delete the routes from vl3 router
	if isClient {
		err := retry.Do(func() error {
			// step1: update the routes in vl3
			podList := corev1.PodList{}
			labels := map[string]string{
				"kubeslice.io/gw-pod-type":                  "toBeDeleted",
				"kubeslice.io/slice":                        workerslicegwrecycler.Spec.SliceName,
				"kubeslice.io/slicegatewayRedundancyNumber": fmt.Sprint(workerslicegwrecycler.Spec.ClientRedundancyNumber),
				"kubeslice.io/slice-gw":                     slicegateway.Name,
			}
			listOptions := []client.ListOption{
				client.MatchingLabels(labels),
			}
			err := r.MeshClient.List(ctx, &podList, listOptions...)
			if err != nil {
				return err
			}
			// there should be only 1 pod with label "kubeslice.io/gw-pod-type":"toBeDeleted" per gw pair.
			if len(podList.Items) == 0 {
				log.V(1).Info("No pods with label kubeslice.io/gw-pod-type:toBeDeleted found.")
				return nil
			} else if len(podList.Items) == 1 {
				grpcAdd := podList.Items[0].Status.PodIP + ":5000"

				status, err := r.WorkerGWSidecarClient.GetStatus(ctx, grpcAdd)
				if err != nil {
					log.Error(err, "Unable to fetch gw status")
					return err
				}
				nsmIP := status.NsmStatus.LocalIP
				_, podIP, err := controllers.GetSliceRouterPodNameAndIP(ctx, r.MeshClient, slicegateway.Spec.SliceName)
				if err != nil {
					log.Error(err, "unable to get slice router pod info")
					return err
				}
				if podIP == "" {
					log.V(1).Info("slice router podIP not available yet, requeuing")
					return err
				}

				if slicegateway.Status.Config.SliceGatewayRemoteSubnet == "" ||
					len(slicegateway.Status.GatewayPodStatus) == 0 {
					log.V(1).Info("Waiting for remote subnet and local nsm IPs. Delaying conn ctx update to router")
					return err
				}

				sidecarGrpcAddress := podIP + ":5000"
				routeInfo := &router.UpdateEcmpInfo{
					RemoteSliceGwNsmSubnet: slicegateway.Status.Config.SliceGatewayRemoteSubnet,
					NsmIpToDelete:          nsmIP,
				}
				err = r.WorkerRouterClient.UpdateEcmpRoutes(ctx, sidecarGrpcAddress, routeInfo)
				if err != nil {
					log.Error(err, "unable to update ecmp routes in the slice router")
					return err
				}
				// step3: delete the old deployment
				rsName := podList.Items[0].ObjectMeta.OwnerReferences[0].Name
				rs := appsv1.ReplicaSet{}
				if err := r.MeshClient.Get(ctx, types.NamespacedName{Namespace: controllers.ControlPlaneNamespace, Name: rsName}, &rs); err != nil {
					log.Error(err, "error getting replicaset")
					return err
				}
				deployName := rs.ObjectMeta.OwnerReferences[0].Name
				deployToBeDeleted = appsv1.Deployment{}
				if err := r.MeshClient.Get(ctx, types.NamespacedName{Namespace: controllers.ControlPlaneNamespace, Name: deployName}, &deployToBeDeleted); err != nil {
					log.Error(err, "error getting deployment")
					return err
				}
				err = r.MeshClient.Delete(ctx, &deployToBeDeleted)
				if err != nil {
					return err
				}
				return nil
			} else {
				log.V(1).Info("pod list", "pod list", podList.Items)
				log.Error(fmt.Errorf("more than 1 pod with label kubeslice.io/gw-pod-type:toBeDeleted, something went wrong"), "Err:()")
				return errors.New("error deleting old gw pods after removing routes")
			}
		}, retry.Attempts(200))

		if err != nil {
			log.Error(err, "Err():", err.Error())
			r.moveFSMToErrorState(req, err)
			workerslicegwrecycler.Status.Client.Response = ERROR
			_ = r.Status().Update(ctx, workerslicegwrecycler)
			return err
		}
		// once the deployment is deleted, update the global map GwMap to delete the deploy info.
		//delete(slicegwpkg.GwMap, deployToBeDeleted.Name)

		err = retry.Do(func() error {
			if err := r.Get(ctx, types.NamespacedName{Namespace: req.Namespace, Name: req.Name}, workerslicegwrecycler); err != nil {
				log.Error(err, "error getting workerslicegwrecycler")
				return err
			}
			workerslicegwrecycler.Status.Client.Response = old_gw_deleted
			return r.Status().Update(ctx, workerslicegwrecycler)
		})
		return nil
	}

	// end the FSM
	err := retry.Do(func() error {
		workerslicegwrecycler.Spec.State = old_gw_deleted
		workerslicegwrecycler.Spec.Request = "end"
		err := r.Update(ctx, workerslicegwrecycler)
		if err != nil {
			return err
		}
		// FSM complete - delete workergwrecycler
		return r.Delete(ctx, workerslicegwrecycler)
	})
	if err != nil {
		log.Error(err, "Error:()", err.Error())
		r.moveFSMToErrorState(req, err)
		workerslicegwrecycler.Spec.State = ERROR
		_ = r.Status().Update(ctx, workerslicegwrecycler)
		return err
	}
	return nil
}

func (r *Reconciler) errorEntryFunction(e *fsm.Event) error {
	// raise events
	// move the FSM to INIT
	return nil
}

func (r *Reconciler) moveFSMToErrorState(req reconcile.Request, err error) {
	// move the FSM to ERROR state
	r.FSM[req.String()].Event(onError, err)
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
	labels := map[string]string{"kubeslice.io/slicegw": sliceGwName}
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
*/
