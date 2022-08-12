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
	"time"

	"github.com/go-logr/logr"
	webhook "github.com/kubeslice/worker-operator/pkg/webhook/deploy"
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
	"github.com/kubeslice/worker-operator/pkg/events"
	"github.com/kubeslice/worker-operator/pkg/logger"
	nsmv1alpha1 "github.com/networkservicemesh/networkservicemesh/k8s/pkg/apis/networkservice/v1alpha1"
)

var sliceGwFinalizer = "networking.kubeslice.io/slicegw-finalizer"

// SliceReconciler reconciles a Slice object
type SliceGwReconciler struct {
	client.Client
	Scheme                *runtime.Scheme
	Log                   logr.Logger
	HubClient             HubClientProvider
	WorkerRouterClient    WorkerRouterClientProvider
	WorkerNetOpClient     WorkerNetOpClientProvider
	WorkerGWSidecarClient WorkerGWSidecarClientProvider
	NetOpPods             []NetOpPod
	EventRecorder         *events.EventRecorder
	NodeIP                []string
}

func readyToDeployGwClient(sliceGw *kubeslicev1beta1.SliceGateway) bool {
	return sliceGw.Status.Config.SliceGatewayRemoteNodeIP != "" && sliceGw.Status.Config.SliceGatewayRemoteNodePort != 0 && sliceGw.Status.Config.SliceGatewayRemoteGatewayID != ""
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
	var sliceGwNodePort int32
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
	// Examine DeletionTimestamp to determine if object is under deletion
	// The object is not being deleted, so if it does not have our finalizer,
	// then lets add the finalizer and update the object. This is equivalent
	// registering our finalizer.
	// The object is being deleted
	//cheanup Gateway related resources
	// Stop reconciliation as the item is being deleted
	reconcile, result, err := r.handleSliceGwDeletion(sliceGw, ctx)
	if reconcile {
		return result, err
	}

	sliceName := sliceGw.Spec.SliceName
	sliceGwName := sliceGw.Name

	log = log.WithValues("slice", sliceGw.Spec.SliceName)
	ctx = logger.WithLogger(ctx, log)

	log.Info("reconciling", "slicegateway", sliceGw.Name)

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
	foundSliceRouterService, err := FindSliceRouterService(ctx, r.Client, sliceName)
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
		reconcile, result, sliceGwNodePort, err = r.handleSliceGwSvcCreation(ctx, sliceGw)
		if reconcile {
			return result, err
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
	}
	// Check if the deployment already exists, if not create a new one
	found := &appsv1.Deployment{}
	err = r.Get(ctx, types.NamespacedName{Name: sliceGwName, Namespace: controllers.ControlPlaneNamespace}, found)
	if err != nil {
		if errors.IsNotFound(err) {
			// Define a new deployment
			dep := r.deploymentForGateway(sliceGw)
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
		r.EventRecorder.Record(
			&events.Event{
				Object:    sliceGw,
				EventType: events.EventTypeWarning,
				Reason:    "Error",
				Message:   "Failed to reconcile slice gw pod status",
			},
		)
		return ctrl.Result{}, err
	}
	if requeue {
		return res, nil
	}

	res, err, requeue = r.SendConnectionContextAndQosToGwPod(ctx, slice, sliceGw)
	if err != nil {
		log.Error(err, "Failed to send connection context to gw pod")
		//post event to slicegw
		r.EventRecorder.Record(
			&events.Event{
				Object:    sliceGw,
				EventType: events.EventTypeWarning,
				Reason:    "Error",
				Message:   "Failed to send connection context to gw pod",
			},
		)
		return ctrl.Result{}, err
	}
	if requeue {
		return res, nil
	}

	res, err, requeue = r.SendConnectionContextToSliceRouter(ctx, sliceGw)
	if err != nil {
		log.Error(err, "Failed to send connection context to slice router pod")
		//post event to slicegw
		r.EventRecorder.Record(
			&events.Event{
				Object:    sliceGw,
				EventType: events.EventTypeWarning,
				Reason:    "Error",
				Message:   "Failed to send connection context to slice router pod",
			},
		)
		return ctrl.Result{}, err
	}
	if requeue {
		return res, nil
	}
	log.Info("sync QoS with netop pods from slicegw")
	err = r.SyncNetOpConnectionContextAndQos(ctx, slice, sliceGw, sliceGwNodePort)
	if err != nil {
		log.Error(err, "Error sending QOS Profile to netop pod")
		//post event to slicegw
		r.EventRecorder.Record(
			&events.Event{
				Object:    sliceGw,
				EventType: events.EventTypeWarning,
				Reason:    "Error",
				Message:   "Failed to send QOS Profile to netop pod",
			},
		)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
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
func (r *SliceGwReconciler) handleSliceGwSvcCreation(ctx context.Context, sliceGw *kubeslicev1beta1.SliceGateway) (bool, reconcile.Result, int32, error) {
	log := logger.FromContext(ctx).WithName("slicegw-create-service")
	sliceGwName := sliceGw.Name

	foundsvc := &corev1.Service{}
	err := r.Get(ctx, types.NamespacedName{Name: "svc-" + sliceGwName, Namespace: controllers.ControlPlaneNamespace}, foundsvc)
	if err != nil {
		if errors.IsNotFound(err) {

			svc := r.serviceForGateway(sliceGw)
			log.Info("Creating a new Service", "Namespace", svc.Namespace, "Name", svc.Name)
			err = r.Create(ctx, svc)
			if err != nil {
				log.Error(err, "Failed to create new Service", "Namespace", svc.Namespace, "Name", svc.Name)
				return true, ctrl.Result{}, 0, err
			}
			return true, ctrl.Result{Requeue: true}, 0, nil
		}
		log.Error(err, "Failed to get Service")
		return true, ctrl.Result{}, 0, err
	}

	sliceGwNodePort := foundsvc.Spec.Ports[0].NodePort
	err = r.HubClient.UpdateNodePortForSliceGwServer(ctx, sliceGwNodePort, sliceGwName)
	if err != nil {
		log.Error(err, "Failed to update NodePort for sliceGw in the hub")

		r.EventRecorder.Record(
			&events.Event{
				Object:    sliceGw,
				EventType: events.EventTypeWarning,
				Reason:    "Error",
				Message:   "Unable to post NodePort to kubeslice-controller cluster",
			},
		)
		return true, ctrl.Result{}, sliceGwNodePort, err
	}

	if err := r.reconcileNodes(ctx, sliceGw); err != nil {
		return true, ctrl.Result{}, sliceGwNodePort, err
	}
	return false, reconcile.Result{}, sliceGwNodePort, nil
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

			r.cleanupSliceGwResources(ctx, sliceGw)
			controllerutil.RemoveFinalizer(sliceGw, sliceGwFinalizer)
			if err := r.Update(ctx, sliceGw); err != nil {
				return true, ctrl.Result{}, err
			}
		}
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

func FindSliceRouterService(ctx context.Context, c client.Client, sliceName string) (bool, error) {
	vl3NseEpList := &nsmv1alpha1.NetworkServiceEndpointList{}
	opts := []client.ListOption{
		client.InNamespace(controllers.ControlPlaneNamespace),
		client.MatchingLabels{"app": "vl3-nse-" + sliceName,
			"networkservicename": "vl3-service-" + sliceName},
	}
	err := c.List(ctx, vl3NseEpList, opts...)
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	if len(vl3NseEpList.Items) == 0 {
		return false, nil
	}
	return true, nil
}
