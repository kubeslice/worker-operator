/*
 *  Copyright (c) 2022 Avesha, Inc. All rights reserved.
 *
 *  SPDX-License-Identifier: Apache-2.0
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */

package slicegateway

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	"github.com/kubeslice/kubeslice-monitoring/pkg/events"
	ossEvents "github.com/kubeslice/worker-operator/events"
	"github.com/kubeslice/worker-operator/pkg/utils"
	webhook "github.com/kubeslice/worker-operator/pkg/webhook/pod"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	"github.com/kubeslice/worker-operator/controllers"
	"github.com/kubeslice/worker-operator/pkg/logger"
	nsmv1 "github.com/networkservicemesh/sdk-k8s/pkg/tools/k8s/apis/networkservicemesh.io/v1"
)

var sliceGwFinalizer = "networking.kubeslice.io/slicegw-finalizer"
var controllerName = "sliceGWController"

// SliceReconciler reconciles a Slice object
type SliceGwReconciler struct {
	client.Client
	Scheme                *runtime.Scheme
	Log                   logr.Logger
	HubClient             HubClientProvider
	WorkerRouterClient    WorkerRouterClientProvider
	WorkerNetOpClient     WorkerNetOpClientProvider
	WorkerGWSidecarClient WorkerGWSidecarClientProvider
	WorkerRecyclerClient  WorkerRecyclerClientProvider

	NetOpPods        []NetOpPod
	EventRecorder    *events.EventRecorder
	NodeIPs          []string
	NumberOfGateways int
}

// gwMap holds the mapping between gwPodName and NodePort number
var GwMap = make(map[string]int)

func readyToDeployGwClient(sliceGw *kubeslicev1beta1.SliceGateway) bool {
	return len(sliceGw.Status.Config.SliceGatewayRemoteNodeIPs) > 0 && len(sliceGw.Status.Config.SliceGatewayRemoteNodePorts) != 0 && sliceGw.Status.Config.SliceGatewayRemoteGatewayID != ""
}

//+kubebuilder:rbac:groups=networking.kubeslice.io,resources=slicegateways,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=networking.kubeslice.io,resources=slicegateways/finalizers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=networking.kubeslice.io,resources=slicegateways/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=networkservicemesh.io,resources=networkservices,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch
//+kubebuilder:rbac:groups=core,resources=endpoints,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch;

func (r *SliceGwReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var sliceGwNodePorts []int
	var noOfGwServices int
	log := r.Log.WithValues("slicegateway", req.NamespacedName)
	sliceGw := &kubeslicev1beta1.SliceGateway{}
	err := r.Get(ctx, req.NamespacedName, sliceGw)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Return and don't requeue
			log.Info("SliceGateway resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get SliceGateway")
		return ctrl.Result{}, err
	}
	*r.EventRecorder = (*r.EventRecorder).WithSlice(sliceGw.Spec.SliceName)
	// Examine DeletionTimestamp to determine if object is under deletion
	// The object is not being deleted, so if it does not have our finalizer,
	// then lets add the finalizer and update the object. This is equivalent
	// registering our finalizer.
	// The object is being deleted
	// cheanup Gateway related resources
	// Stop reconciliation as the item is being deleted
	reconcile, result, err := r.handleSliceGwDeletion(sliceGw, ctx)
	if reconcile {
		return result, err
	}

	sliceName := sliceGw.Spec.SliceName
	sliceGwName := sliceGw.Name

	log = log.WithValues("slice", sliceGw.Spec.SliceName)
	ctx = logger.WithLogger(ctx, log)

	log.Info("reconciling", "slicegateway", sliceGw.Name, "NumberOfGateways", r.NumberOfGateways)

	// Check if the slice to which this gateway belongs is created
	slice, err := controllers.GetSlice(ctx, r.Client, sliceName)
	if err != nil {
		log.Error(err, "Failed to get Slice", "slice", sliceName)
		return ctrl.Result{}, err
	}
	if slice == nil {
		log.Info("Slice object not created yet. Waiting...")
		return ctrl.Result{
			RequeueAfter: 10 * time.Second,
		}, nil
	}

	// Check if slice router network service endpoint (NSE) is present before spawning slice gateway pod.
	// Gateways connect to vL3 slice router at startup, hence it is necessary to check if the
	// NSE present before creating the gateway pods.
	foundSliceRouterService, err := r.FindSliceRouterService(ctx, r.Client, sliceName)
	if !foundSliceRouterService {
		if err != nil {
			log.Error(err, "Failed to get Network Service EP list for vL3", "Name", "vl3-service-"+sliceName)
			return ctrl.Result{}, err
		}
		log.Info("No endpoints found for vL3 NSE yet. Waiting...")
		return ctrl.Result{
			RequeueAfter: 10 * time.Second,
		}, nil
	}

	// true if the gateway is openvpn server
	// Check if the Gw service already exists, if not create a new one if it is a server
	if isServer(sliceGw) {
		// during the rebalancing phase noOfGwServices becomes 3,
		// we cannot directly increment r.NumberOfGateways directly
		noOfGwServices, err = r.getNumberOfGatewayNodePortServices(ctx, sliceGw)
		log.Info("Number of gw services present", "noOfGwServices", noOfGwServices)
		if err != nil {
			return ctrl.Result{}, err
		}
		// This condition will be true during rebalancing
		if noOfGwServices > r.NumberOfGateways {
			reconcile, result, sliceGwNodePorts, err = r.handleSliceGwSvcCreation(ctx, sliceGw, noOfGwServices)
			if reconcile {
				utils.RecordEvent(ctx, r.EventRecorder, sliceGw, slice, ossEvents.EventSliceGWServiceCreationFailed, controllerName)
				return result, err
			}
		} else {
			reconcile, result, sliceGwNodePorts, err = r.handleSliceGwSvcCreation(ctx, sliceGw, r.NumberOfGateways)
			if reconcile {
				utils.RecordEvent(ctx, r.EventRecorder, sliceGw, slice, ossEvents.EventSliceGWServiceCreationFailed, controllerName)
				return result, err
			}
		}
	}
	// client can be deployed only if remoteNodeIp,SliceGatewayRemoteNodePort abd SliceGatewayRemoteGatewayID is present
	if !canDeployGw(sliceGw) {
		// no need to deploy gateway deployment or service
		log.Info("Unable to deploy slicegateway client, remote info not available, requeuing")
		return ctrl.Result{
			RequeueAfter: 10 * time.Second,
		}, nil
	}
	if isClient(sliceGw) {

		//reconcile headless service and endpoint for DNS Query by OpenVPN Client
		if err := r.reconcileGatewayHeadlessService(ctx, sliceGw); err != nil {
			return ctrl.Result{}, err
		}
		//create an endpoint if not exists
		requeue, res, err := r.reconcileGatewayEndpoint(ctx, sliceGw)
		if requeue {
			return res, err
		}
		noOfGwServices = len(sliceGw.Status.Config.SliceGatewayRemoteNodePorts)
		sliceGwNodePorts = sliceGw.Status.Config.SliceGatewayRemoteNodePorts
	}
	// Check if the deployment already exists, if not create a new one
	// spin up 2 gw deployments
	log.Info("Number of gw services present", "noOfGwServices", noOfGwServices)
	if err := r.reconcileGwMap(ctx, sliceGw); err != nil {
		return ctrl.Result{}, err
	}

	deployments, err := r.getDeployments(ctx, sliceGw)
	if err != nil {
		return ctrl.Result{}, err
	}
	if isClient(sliceGw) {
		for _, deploy := range deployments.Items {
			_, ok := GwMap[deploy.Name]
			if !ok {
				for _, c := range deploy.Spec.Template.Spec.Containers {
					if c.Name == "kubeslice-sidecar" {
						for _, env := range c.Env {
							if env.Name == "NODE_PORT" {
								nodePort, _ := strconv.Atoi(env.Value)
								GwMap[deploy.Name] = nodePort
							}
						}
					}
				}
			}
		}
	}
	if isServer(sliceGw) {
		noOfGwServices, _ = r.getNumberOfGatewayNodePortServices(ctx, sliceGw)
	} else {
		// fetch the latest slicegw
		//TODO: check if this is required?
		_ = r.Get(ctx, req.NamespacedName, sliceGw)
		noOfGwServices = len(sliceGw.Status.Config.SliceGatewayRemoteNodePorts)
		log.Info("number of nodeports", "nodeports", sliceGw.Status.Config.SliceGatewayRemoteNodePorts, "len", len(sliceGw.Status.Config.SliceGatewayRemoteNodePorts))
	}
	log.Info("gw deployments", "deployments", len(deployments.Items))

	if len(deployments.Items) < noOfGwServices {
		vpnKeyRotation, err := r.HubClient.GetVPNKeyRotation(ctx, sliceName)
		if err != nil {
			return ctrl.Result{}, err
		}
		secretVersion := 1
		if vpnKeyRotation != nil {
			secretVersion = vpnKeyRotation.Spec.RotationCount
		}
		for i := 0; i < noOfGwServices; i++ {
			found := &appsv1.Deployment{}
			err = r.Get(ctx, types.NamespacedName{Name: sliceGwName + "-" + fmt.Sprint(i), Namespace: controllers.ControlPlaneNamespace}, found)
			if err != nil {
				if errors.IsNotFound(err) {
					// Define a new deployment
					dep := r.deploymentForGateway(sliceGw, i, secretVersion)
					log.Info("Creating a new Deployment", "Namespace", dep.Namespace, "Name", dep.Name)
					err = r.Create(ctx, dep)
					if err != nil {
						log.Error(err, "Failed to create new Deployment", "Namespace", dep.Namespace, "Name", dep.Name)
						return ctrl.Result{}, err
					}
					return ctrl.Result{Requeue: true}, nil
				}
				log.Error(err, "Failed to get Deployment")
				return ctrl.Result{}, err
			}

		}
	}

	// reconcile map in case operator pod restarted or if entry is not present

	//fetch netop pods
	err = r.getNetOpPods(ctx, sliceGw)
	if err != nil {
		log.Error(err, "Unable to fetch netop pods")
		return ctrl.Result{}, err
	}

	res, err, requeue := r.ReconcileGwPodStatus(ctx, sliceGw)
	if err != nil {
		log.Error(err, "Failed to reconcile slice gw pod status")
		//post event to slicegw
		utils.RecordEvent(ctx, r.EventRecorder, sliceGw, slice, ossEvents.EventSliceGWPodReconcileFailed, controllerName)
		return ctrl.Result{}, err
	}
	if requeue {
		return res, nil
	}

	res, err, requeue = r.SendConnectionContextAndQosToGwPod(ctx, slice, sliceGw, req)
	if err != nil {
		log.Error(err, "Failed to send connection context to gw pod")
		//post event to slicegw
		utils.RecordEvent(ctx, r.EventRecorder, sliceGw, slice, ossEvents.EventSliceGWConnectionContextFailed, controllerName)
		return ctrl.Result{}, err
	}
	if requeue {
		return res, nil
	}

	res, err, requeue = r.SendConnectionContextToSliceRouter(ctx, sliceGw)
	if err != nil {
		log.Error(err, "Failed to send connection context to slice router pod")
		//post event to slicegw
		utils.RecordEvent(ctx, r.EventRecorder, sliceGw, slice, ossEvents.EventSliceRouterConnectionContextFailed, controllerName)
		return ctrl.Result{}, err
	}
	if requeue {
		return res, nil
	}

	log.Info("sync QoS with netop pods from slicegw")
	err = r.SyncNetOpConnectionContextAndQos(ctx, slice, sliceGw, sliceGwNodePorts)
	if err != nil {
		log.Error(err, "Error sending QOS Profile to netop pod")
		//post event to slicegw
		utils.RecordEvent(ctx, r.EventRecorder, sliceGw, slice, ossEvents.EventSliceNetopQoSSyncFailed, controllerName)
		return ctrl.Result{}, err
	}
	if isServer(sliceGw) {
		toRebalance, err := r.isRebalancingRequired(ctx, sliceGw)
		if err != nil {
			log.Error(err, "Unable to rebalace gw pods")
			utils.RecordEvent(ctx, r.EventRecorder, sliceGw, slice, ossEvents.EventSliceGWRebalancingFailed, controllerName)
			return ctrl.Result{}, err
		}
		log.Info("Rebalancing required?", "toRebalance", toRebalance)
		if toRebalance {
			// start FSM for graceful termination of gateway pods
			// create workerslicegwrecycler on controller
			newestPod, err := r.getNewestPod(sliceGw)
			if err != nil {
				return ctrl.Result{}, err
			}
			err = r.WorkerRecyclerClient.TriggerFSM(sliceGw, slice, newestPod, controllerName, sliceGwName+"-"+fmt.Sprint(0), 1)
			// to maintain consistency with recycling we are creating this CR with zero as suffix in the name
			if err != nil {
				// TODO:add an event and log
				log.Error(err, "Error while recycling gateway pods")
				return ctrl.Result{}, err
			}
		}
	}
	return ctrl.Result{Requeue: true}, nil
}

func (r *SliceGwReconciler) reconcileGwMap(ctx context.Context, sliceGw *kubeslicev1beta1.SliceGateway) error {
	listOpts := []client.ListOption{
		client.MatchingLabels(map[string]string{
			controllers.ApplicationNamespaceSelectorLabelKey: sliceGw.Spec.SliceName,
			"kubeslice.io/slicegw":                           sliceGw.Name,
		}),
	}
	deployList := appsv1.DeploymentList{}
	if err := r.List(ctx, &deployList, listOpts...); err != nil {
		return err
	}
	for d, _ := range GwMap {
		found := false
		for _, deploy := range deployList.Items {
			if d == deploy.Name {
				found = true
			}
		}
		if !found {
			delete(GwMap, d)
		}
	}
	return nil
}
func (r *SliceGwReconciler) getDeployments(ctx context.Context, sliceGw *kubeslicev1beta1.SliceGateway) (*appsv1.DeploymentList, error) {
	listOpts := []client.ListOption{
		client.MatchingLabels(map[string]string{
			controllers.ApplicationNamespaceSelectorLabelKey: sliceGw.Spec.SliceName,
			webhook.PodInjectLabelKey:                        "slicegateway",
			"kubeslice.io/slicegw":                           sliceGw.Name,
		}),
		client.InNamespace(controllers.ControlPlaneNamespace),
	}
	deployList := appsv1.DeploymentList{}
	if err := r.List(ctx, &deployList, listOpts...); err != nil {
		return nil, err
	}
	return &deployList, nil
}

func isClient(sliceGw *kubeslicev1beta1.SliceGateway) bool {
	return sliceGw.Status.Config.SliceGatewayHostType == "Client"
}
func isServer(sliceGw *kubeslicev1beta1.SliceGateway) bool {
	return sliceGw.Status.Config.SliceGatewayHostType == "Server"
}
func canDeployGw(sliceGw *kubeslicev1beta1.SliceGateway) bool {
	return sliceGw.Status.Config.SliceGatewayHostType == "Server" || readyToDeployGwClient(sliceGw)
}

func (r *SliceGwReconciler) getNumberOfGatewayNodePortServices(ctx context.Context, sliceGw *kubeslicev1beta1.SliceGateway) (int, error) {
	listOpts := []client.ListOption{
		client.MatchingLabels(map[string]string{
			controllers.ApplicationNamespaceSelectorLabelKey: sliceGw.Spec.SliceName,
			"kubeslice.io/slicegw":                           sliceGw.Name,
		}),
		client.InNamespace(controllers.ControlPlaneNamespace),
	}
	services := corev1.ServiceList{}
	if err := r.List(ctx, &services, listOpts...); err != nil {
		return 0, err
	}
	return len(services.Items), nil
}
func (r *SliceGwReconciler) getNodePorts(ctx context.Context, sliceGw *kubeslicev1beta1.SliceGateway) ([]int, error) {
	var nodePorts []int
	listOpts := []client.ListOption{
		client.MatchingLabels(map[string]string{
			controllers.ApplicationNamespaceSelectorLabelKey: sliceGw.Spec.SliceName,
			"kubeslice.io/slicegw":                           sliceGw.Name,
		}),
	}
	services := corev1.ServiceList{}
	if err := r.List(ctx, &services, listOpts...); err != nil {
		return nil, err
	}
	for _, service := range services.Items {
		nodePorts = append(nodePorts, int(service.Spec.Ports[0].NodePort))
	}
	return nodePorts, nil
}

func (r *SliceGwReconciler) handleSliceGwSvcCreation(ctx context.Context, sliceGw *kubeslicev1beta1.SliceGateway, n int) (bool, reconcile.Result, []int, error) {
	log := logger.FromContext(ctx).WithName("slicegw")
	sliceGwName := sliceGw.Name
	foundsvc := &corev1.Service{}
	var sliceGwNodePorts []int
	no, _ := r.getNumberOfGatewayNodePortServices(ctx, sliceGw)
	if no != n {
		// capping the number of services to be 2 for now,later i will be equal to number of gateway nodes,eg: i = len(node.GetExternalIpList())
		for i := 0; i < n; i++ {
			err := r.Get(ctx, types.NamespacedName{Name: "svc-" + sliceGwName + "-" + fmt.Sprint(i), Namespace: controllers.ControlPlaneNamespace}, foundsvc)
			if err != nil {
				if errors.IsNotFound(err) {
					svc := r.serviceForGateway(sliceGw, i)
					log.Info("Creating a new Service", "Namespace", svc.Namespace, "Name", svc.Name)
					err = r.Create(ctx, svc)
					if err != nil {
						log.Error(err, "Failed to create new Service", "Namespace", svc.Namespace, "Name", svc.Name)
						return true, ctrl.Result{}, nil, err
					}
					return true, ctrl.Result{Requeue: true}, nil, nil
				}
				log.Error(err, "Failed to get Service")
				return true, ctrl.Result{}, nil, err
			}
		}
	}
	sliceGwNodePorts, _ = r.getNodePorts(ctx, sliceGw)
	err := r.HubClient.UpdateNodePortForSliceGwServer(ctx, sliceGwNodePorts, sliceGwName)
	if err != nil {
		log.Error(err, "Failed to update NodePort for sliceGw in the hub")
		utils.RecordEvent(ctx, r.EventRecorder, sliceGw, nil, ossEvents.EventSliceGWNodePortUpdateFailed, controllerName)
		return true, ctrl.Result{}, sliceGwNodePorts, err
	}
	return false, reconcile.Result{}, sliceGwNodePorts, nil
}
func (r *SliceGwReconciler) handleSliceGwDeletion(sliceGw *kubeslicev1beta1.SliceGateway, ctx context.Context) (bool, reconcile.Result, error) {
	// Examine DeletionTimestamp to determine if object is under deletion
	log := logger.FromContext(ctx).WithName("slicegw-deletion")
	if sliceGw.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !controllerutil.ContainsFinalizer(sliceGw, sliceGwFinalizer) {
			controllerutil.AddFinalizer(sliceGw, sliceGwFinalizer)
			if err := r.Update(ctx, sliceGw); err != nil {
				return true, ctrl.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(sliceGw, sliceGwFinalizer) {
			log.Info("Deleting sliceGW", "sliceGw", sliceGw.Name)
			//cheanup Gateway related resources

			if err := r.cleanupSliceGwResources(ctx, sliceGw); err != nil {
				log.Error(err, "error while deleting sliceGW")
				utils.RecordEvent(ctx, r.EventRecorder, sliceGw, nil, ossEvents.EventSliceGWDeleteFailed, controllerName)
			}
			controllerutil.RemoveFinalizer(sliceGw, sliceGwFinalizer)
			if err := r.Update(ctx, sliceGw); err != nil {
				return true, ctrl.Result{}, err
			}
		}
		utils.RecordEvent(ctx, r.EventRecorder, sliceGw, nil, ossEvents.EventSliceGWDeleted, controllerName)
		// Stop reconciliation as the item is being deleted
		return true, ctrl.Result{}, nil
	}
	return false, reconcile.Result{}, nil
}

func getPodType(labels map[string]string) string {
	podType, found := labels[webhook.PodInjectLabelKey]
	if found {
		return podType
	}

	nsmLabel, found := labels["app"]
	if found {
		if nsmLabel == "nsmgr-daemonset" || nsmLabel == "nsm-kernel-plane" {
			return "nsm"
		}
	}

	return ""
}

func (r *SliceGwReconciler) findSliceGwObjectsToReconcile(pod client.Object) []reconcile.Request {
	podLabels := pod.GetLabels()
	if podLabels == nil {
		return []reconcile.Request{}
	}

	podType := getPodType(podLabels)

	sliceGwList := &kubeslicev1beta1.SliceGatewayList{}
	var err error

	switch podType {
	case "router":
		sliceName, found := podLabels[controllers.ApplicationNamespaceSelectorLabelKey]
		if !found {
			return []reconcile.Request{}
		}

		sliceGwList, err = r.findObjectsForSliceRouterUpdate(sliceName)
		if err != nil {
			return []reconcile.Request{}
		}
	case "netop":
		sliceGwList, err = r.findObjectsForNetopUpdate()
		if err != nil {
			return []reconcile.Request{}
		}
	case "nsm":
		sliceGwList, err = r.findObjectsForNsmUpdate()
		if err != nil {
			return []reconcile.Request{}
		}
	default:
		return []reconcile.Request{}
	}

	requests := make([]reconcile.Request, len(sliceGwList.Items))
	for i, item := range sliceGwList.Items {
		requests[i] = reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      item.GetName(),
				Namespace: item.GetNamespace(),
			},
		}
	}
	return requests
}

func (r *SliceGwReconciler) sliceGwObjectsToReconcileForNodeRestart(node client.Object) []reconcile.Request {
	sliceGwList, err := r.findAllSliceGwObjects()
	if err != nil {
		return []reconcile.Request{}
	}
	requests := make([]reconcile.Request, len(sliceGwList.Items))
	for i, item := range sliceGwList.Items {
		requests[i] = reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      item.GetName(),
				Namespace: item.GetNamespace(),
			},
		}
	}
	return requests
}

func (r *SliceGwReconciler) findObjectsForSliceRouterUpdate(sliceName string) (*kubeslicev1beta1.SliceGatewayList, error) {
	sliceGwList := &kubeslicev1beta1.SliceGatewayList{}
	listOpts := []client.ListOption{
		client.MatchingLabels(map[string]string{controllers.ApplicationNamespaceSelectorLabelKey: sliceName}),
	}
	err := r.List(context.Background(), sliceGwList, listOpts...)
	if err != nil {
		return nil, err
	}
	return sliceGwList, nil
}

func (r *SliceGwReconciler) findAllSliceGwObjects() (*kubeslicev1beta1.SliceGatewayList, error) {
	sliceGwList := &kubeslicev1beta1.SliceGatewayList{}
	listOpts := []client.ListOption{
		client.InNamespace(controllers.ControlPlaneNamespace),
	}
	err := r.List(context.Background(), sliceGwList, listOpts...)
	if err != nil {
		return nil, err
	}

	return sliceGwList, nil
}

func (r *SliceGwReconciler) findObjectsForNetopUpdate() (*kubeslicev1beta1.SliceGatewayList, error) {
	return r.findAllSliceGwObjects()
}

func (r *SliceGwReconciler) findObjectsForNsmUpdate() (*kubeslicev1beta1.SliceGatewayList, error) {
	return r.findAllSliceGwObjects()
}

// SetupWithManager sets up the controller with the Manager.
func (r *SliceGwReconciler) SetupWithManager(mgr ctrl.Manager) error {
	var labelSelector metav1.LabelSelector

	// The slice gateway reconciler needs to be invoked whenever there is an update to the
	// slice router or the netop pods. This is needed to re-send connection context to the
	// restarted slice router or netop pods.
	// We will add a watch on those pods with appropriate label selectors for filtering.
	labelSelector.MatchLabels = map[string]string{webhook.PodInjectLabelKey: "router"}
	slicerouterPredicate, err := predicate.LabelSelectorPredicate(labelSelector)
	if err != nil {
		return err
	}

	labelSelector.MatchLabels = map[string]string{webhook.PodInjectLabelKey: "netop"}
	netopPredicate, err := predicate.LabelSelectorPredicate(labelSelector)
	if err != nil {
		return err
	}

	labelSelector.MatchLabels = map[string]string{"app": "nsmgr-daemonset"}
	nsmgrPredicate, err := predicate.LabelSelectorPredicate(labelSelector)
	if err != nil {
		return err
	}

	labelSelector.MatchLabels = map[string]string{"app": "nsm-kernel-plane"}
	nsmfwdPredicate, err := predicate.LabelSelectorPredicate(labelSelector)
	if err != nil {
		return err
	}

	sliceGwUpdPredicate := predicate.Or(
		slicerouterPredicate, netopPredicate, nsmgrPredicate, nsmfwdPredicate,
	)

	// The slice gateway reconciler needs to be invoked whenever there is an update to the
	// kubeslice gateway nodes
	labelSelector.MatchLabels = map[string]string{controllers.NodeTypeSelectorLabelKey: "gateway"}
	nodePredicate, err := predicate.LabelSelectorPredicate(labelSelector)
	if err != nil {
		return err
	}
	// The mapping function for the slice router pod update should only invoke the reconciler
	// of the slice gateway objects that belong to the same slice as the restarted slice router.
	// The netop pods are slice agnostic. Hence, all slice gateway objects belonging to every slice
	// should be invoked if a netop pod restarts. Its mapping function will select all the slice
	// gateway objects in the control plane namespace.
	return ctrl.NewControllerManagedBy(mgr).
		For(&kubeslicev1beta1.SliceGateway{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Watches(&source.Kind{Type: &corev1.Pod{}},
			handler.EnqueueRequestsFromMapFunc(r.findSliceGwObjectsToReconcile),
			builder.WithPredicates(sliceGwUpdPredicate),
		).
		Watches(&source.Kind{Type: &corev1.Node{}},
			handler.EnqueueRequestsFromMapFunc(r.sliceGwObjectsToReconcileForNodeRestart),
			builder.WithPredicates(nodePredicate),
		).
		Complete(r)
}

func (r *SliceGwReconciler) FindSliceRouterService(ctx context.Context, c client.Client, sliceName string) (bool, error) {
	vl3NseEp := &nsmv1.NetworkServiceEndpoint{}
	err := r.Get(ctx, types.NamespacedName{Name: "vl3-nse-" + sliceName, Namespace: controllers.ControlPlaneNamespace}, vl3NseEp)
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}
