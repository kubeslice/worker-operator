package workerslicegwrecycler

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	spokev1alpha1 "github.com/kubeslice/apis/pkg/worker/v1alpha1"
	"github.com/kubeslice/kubeslice-monitoring/pkg/events"
	ossEvents "github.com/kubeslice/worker-operator/events"

	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"

	retry "github.com/avast/retry-go"
	"github.com/kubeslice/worker-operator/pkg/logger"
	"github.com/kubeslice/worker-operator/pkg/utils"
	"github.com/looplab/fsm"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// FSM State names
const (
	ST_init                   string = "init"
	ST_new_deployment_created string = "new_deployment_created_state"
	ST_slicerouter_updated    string = "slicerouter_updated_state"
	ST_old_deployment_deleted string = "old_deployment_deleted_state"
	ST_error                  string = "error_state"
	ST_end                    string = "end"
)

// FSM Event names
const (
	EV_verify_new_deployment_created string = "verify_new_deployment_created"
	EV_update_routing_table          string = "update_routing_table"
	EV_delete_old_gw_deployment      string = "delete_old_gw_deployment"
	EV_on_error                      string = "on_error"
	EV_end                           string = "end"
)

// Operation Request names
type Request int

const (
	REQ_invalid Request = -1
	REQ_none    Request = iota
	REQ_create_new_deployment
	REQ_update_routing_table
	REQ_delete_old_gw_deployment
)

// Operation Response names
type Response int

const (
	RESP_invalid Response = -1
	RESP_none    Response = iota
	RESP_new_deployment_created
	RESP_routing_table_updated
	RESP_old_deployment_deleted
)

const (
	controllerName string = "workerSliceGWRecyclerController"
)

type Reconciler struct {
	client.Client
	Log                   logr.Logger
	Scheme                *runtime.Scheme
	MeshClient            client.Client
	WorkerGWSidecarClient WorkerGWSidecarClientProvider
	WorkerRouterClient    WorkerRouterClientProvider
	EventRecorder         *events.EventRecorder
	FSM                   map[string]*fsm.FSM
}

func getUniqueIdentifier(req ctrl.Request) string {
	return req.String()
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logger.FromContext(ctx).WithName("workerslicegwrecycler").WithName(req.Name)
	ctx = logger.WithLogger(ctx, log)

	// Get the unique identifier for the current CR
	// This is used as a key while maintaining the map
	crIdentifier := getUniqueIdentifier(req)

	workerslicegwrecycler := &spokev1alpha1.WorkerSliceGwRecycler{}

	err := r.Get(ctx, req.NamespacedName, workerslicegwrecycler)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Return and don't requeue
			log.Info("workerslicegwrecycler resource not found. Ignoring since object must be deleted")
			// delete the FSM from map
			delete(r.FSM, crIdentifier)
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get workerslicegwrecycler")
		return ctrl.Result{}, err
	}

	slicegw := kubeslicev1beta1.SliceGateway{}

	if err := r.MeshClient.Get(ctx, types.NamespacedName{Namespace: "kubeslice-system", Name: workerslicegwrecycler.Spec.SliceGwServer}, &slicegw); err != nil {
		if errors.IsNotFound(err) {
			if err := r.MeshClient.Get(ctx, types.NamespacedName{Namespace: "kubeslice-system", Name: workerslicegwrecycler.Spec.SliceGwClient}, &slicegw); err != nil {
				// workergwrecycler not meant for this cluster, return and dont requeue
				log.Error(err, "workergwrecycler not meant for this cluster, return and dont requeue")
				return ctrl.Result{}, nil

			}
		}
	}
	// Retrieve or create the FSM for the current CR
	f, exists := r.FSM[crIdentifier]
	if !exists {
		// Create a new FSM for the CR
		f = fsm.NewFSM(
			ST_init,
			fsm.Events{
				{Name: EV_verify_new_deployment_created, Src: []string{ST_init, ST_new_deployment_created}, Dst: ST_new_deployment_created},
				{Name: EV_update_routing_table, Src: []string{ST_init, ST_new_deployment_created, ST_slicerouter_updated}, Dst: ST_slicerouter_updated},
				{Name: EV_delete_old_gw_deployment, Src: []string{ST_init, ST_slicerouter_updated, ST_old_deployment_deleted}, Dst: ST_old_deployment_deleted},
				{Name: EV_on_error, Src: []string{ST_init, ST_new_deployment_created, ST_slicerouter_updated, ST_old_deployment_deleted}, Dst: ST_error},
				{Name: EV_end, Src: []string{ST_init, ST_new_deployment_created, ST_slicerouter_updated, ST_old_deployment_deleted, ST_error}, Dst: ST_end},
			},
			fsm.Callbacks{},
		)
		r.FSM[crIdentifier] = f
	}

	log.Info("reconciling workerslicegwrecycler ", "workerslicegwrecycler", workerslicegwrecycler.Name, "identifier", crIdentifier)
	log.Info("current state", "FSM", f.Current())

	*r.EventRecorder = (*r.EventRecorder).WithSlice(slicegw.Spec.SliceName)
	isClient := slicegw.Status.Config.SliceGatewayHostType == "Client"
	isServer := slicegw.Status.Config.SliceGatewayHostType == "Server"

	// To handle operator restart
	if isServer {
		f.SetState(workerslicegwrecycler.Spec.State)
	}

	if isServer {
		switch f.Current() {
		case ST_init:
			// In init state. Create new deployment if not already created.
			depName := getNewDeploymentName(workerslicegwrecycler.Spec.GwPair.ServerID)
			// Trigger new deployment creation if not already created.
			_, err, _ := r.CreateNewDeployment(ctx, depName, slicegw.Spec.SliceName, slicegw.Name)
			if err != nil {
				return ctrl.Result{}, err
			}

			err = retry.Do(func() error {
				workerslicegwrecycler := &spokev1alpha1.WorkerSliceGwRecycler{}
				err := r.Get(ctx, req.NamespacedName, workerslicegwrecycler)
				if err != nil {
					log.Error(err, "Failed to get workerslicegwrecycler")
					return err
				}
				workerslicegwrecycler.Spec.State = ST_new_deployment_created
				workerslicegwrecycler.Spec.Request = getRequestString(REQ_create_new_deployment)
				return r.Update(ctx, workerslicegwrecycler)
			}, retry.Attempts(10))
			if err != nil {
				return ctrl.Result{}, err
			}

			err = f.Event(EV_verify_new_deployment_created)
			if err != nil {
				return ctrl.Result{}, err
			}

			utils.RecordEvent(ctx, r.EventRecorder, workerslicegwrecycler, nil, ossEvents.EventFSMNewGWSpawned, controllerName)

			return ctrl.Result{}, nil

		case ST_new_deployment_created:
			// Check if the new deployment was created on the server side
			if !r.CheckIfDeploymentIsPresent(ctx, getNewDeploymentName(workerslicegwrecycler.Spec.GwPair.ServerID), slicegw.Spec.SliceName, slicegw.Name) {
				return ctrl.Result{
					RequeueAfter: 10 * time.Second,
				}, nil
			}

			// Check if the deployment was created on the client side
			if getResponseIndex(workerslicegwrecycler.Status.Client.Response) < RESP_new_deployment_created {
				log.Info("Waiting for new deployment to be created on the client side", "currentResponse", workerslicegwrecycler.Status.Client.Response, "respCode", getResponseIndex(workerslicegwrecycler.Status.Client.Response))
				return ctrl.Result{
					RequeueAfter: 10 * time.Second,
				}, nil
			}

			nsmIP, err := r.GetNsmIPForNewDeployment(ctx, &slicegw, getNewDeploymentName(workerslicegwrecycler.Spec.GwPair.ServerID))
			if nsmIP == "" {
				log.Info("Waiting for new deployment to get nsm IP")
				return ctrl.Result{
					RequeueAfter: 10 * time.Second,
				}, nil
			}
			log.Info("Nsm IP obtained on new server deployment", "nsmIP", nsmIP)

			// Check if the slice router installed a route to the remote cluster with the new gw pod as next hop
			routeInstalled, err := r.CheckRouteInSliceRouter(ctx, &slicegw, nsmIP)
			if !routeInstalled {
				log.Info("Waiting for new route to be installed in the router")
				return ctrl.Result{
					RequeueAfter: 10 * time.Second,
				}, nil
			}
			log.Info("Route installed in the slice router for the new server deployment")

			err = retry.Do(func() error {
				workerslicegwrecycler := &spokev1alpha1.WorkerSliceGwRecycler{}
				err := r.Get(ctx, req.NamespacedName, workerslicegwrecycler)
				if err != nil {
					log.Error(err, "Failed to get workerslicegwrecycler")
					return err
				}
				workerslicegwrecycler.Spec.State = ST_slicerouter_updated
				workerslicegwrecycler.Spec.Request = getRequestString(REQ_update_routing_table)
				return r.Update(ctx, workerslicegwrecycler)
			}, retry.Attempts(10))
			if err != nil {
				return ctrl.Result{}, err
			}

			err = f.Event(EV_update_routing_table)
			if err != nil {
				return ctrl.Result{}, err
			}

			return ctrl.Result{}, nil

		case ST_slicerouter_updated:
			log.Info("In router update state")
			// Deployments are created on both the clusters. Update the routing table in the slice
			// router to include the new deployment and remove the old.
			err = r.MarkGwRouteForDeletion(ctx, &slicegw, workerslicegwrecycler.Spec.GwPair.ServerID)
			if err != nil {
				log.Error(err, "Failed to mark gw route for deletion")
				return ctrl.Result{
					RequeueAfter: 10 * time.Second,
				}, err
			}

			nsmIP, err := r.GetNsmIPForNewDeployment(ctx, &slicegw, workerslicegwrecycler.Spec.GwPair.ServerID)
			if nsmIP != "" {
				routeExists, _ := r.CheckRouteInSliceRouter(ctx, &slicegw, nsmIP)
				if routeExists {
					log.Info("Waiting for the route to be removed", "depName", workerslicegwrecycler.Spec.GwPair.ServerID, "nsmIP", nsmIP)
					return ctrl.Result{
						RequeueAfter: 10 * time.Second,
					}, nil
				}
			}

			// Check if the deployment was created on the client side
			if getResponseIndex(workerslicegwrecycler.Status.Client.Response) < RESP_routing_table_updated {
				log.Info("Waiting for the routing table to be updated on the client side")
				return ctrl.Result{
					RequeueAfter: 10 * time.Second,
				}, nil
			}

			err = retry.Do(func() error {
				workerslicegwrecycler := &spokev1alpha1.WorkerSliceGwRecycler{}
				err := r.Get(ctx, req.NamespacedName, workerslicegwrecycler)
				if err != nil {
					log.Error(err, "Failed to get workerslicegwrecycler")
					return err
				}
				workerslicegwrecycler.Spec.State = ST_old_deployment_deleted
				workerslicegwrecycler.Spec.Request = getRequestString(REQ_delete_old_gw_deployment)
				return r.Update(ctx, workerslicegwrecycler)
			}, retry.Attempts(10))
			if err != nil {
				return ctrl.Result{}, err
			}

			err = f.Event(EV_delete_old_gw_deployment)
			if err != nil {
				return ctrl.Result{}, err
			}

			utils.RecordEvent(ctx, r.EventRecorder, workerslicegwrecycler, nil, ossEvents.EventFSMRoutingTableUpdated, controllerName)

			// Route has been removed on both sides now.
			// Wait for a few seconds before deleting the deployment to make sure there are no
			// packets in transit or in the txqueue of the gw pod being removed.
			return ctrl.Result{
				RequeueAfter: 5 * time.Second,
			}, nil

		case ST_old_deployment_deleted:
			err := r.TriggerGwDeploymentDeletion(ctx, slicegw.Spec.SliceName, slicegw.Name, workerslicegwrecycler.Spec.GwPair.ServerID,
				getNewDeploymentName(workerslicegwrecycler.Spec.GwPair.ServerID))
			if err != nil {
				return ctrl.Result{
					RequeueAfter: 10 * time.Second,
				}, nil
			}

			// Check if the deployment was deleted
			if r.CheckIfDeploymentIsPresent(ctx, workerslicegwrecycler.Spec.GwPair.ServerID, slicegw.Spec.SliceName, slicegw.Name) {
				return ctrl.Result{
					RequeueAfter: 10 * time.Second,
				}, nil
			}

			// Check if the deployment was deleted on the client side
			if getResponseIndex(workerslicegwrecycler.Status.Client.Response) < RESP_old_deployment_deleted {
				log.Info("Waiting for the deployment to be deleted on the client side")
				return ctrl.Result{
					RequeueAfter: 10 * time.Second,
				}, nil
			}

			err = retry.Do(func() error {
				workerslicegwrecycler := &spokev1alpha1.WorkerSliceGwRecycler{}
				err := r.Get(ctx, req.NamespacedName, workerslicegwrecycler)
				if err != nil {
					log.Error(err, "Failed to get workerslicegwrecycler")
					return err
				}
				workerslicegwrecycler.Spec.State = ST_end
				workerslicegwrecycler.Spec.Request = getRequestString(REQ_none)
				return r.Update(ctx, workerslicegwrecycler)
			}, retry.Attempts(10))
			if err != nil {
				return ctrl.Result{}, err
			}

			err = f.Event(EV_end)
			if err != nil {
				return ctrl.Result{}, err
			}

			utils.RecordEvent(ctx, r.EventRecorder, workerslicegwrecycler, nil, ossEvents.EventFSMDeleteOldGW, controllerName)

			return ctrl.Result{}, nil

		case ST_error:
			log.Info("In Error state")
			return ctrl.Result{
				RequeueAfter: 30 * time.Second,
			}, nil

		case ST_end:
			log.Info("In end state")
			// Wait for the slicegw recycler to clean the CR
			return ctrl.Result{
				RequeueAfter: 60 * time.Second,
			}, nil

		default:
			log.Info("In default state")
			return ctrl.Result{
				RequeueAfter: 30 * time.Second,
			}, nil

		}
	}

	if isClient {
		switch getRequestIndex(workerslicegwrecycler.Spec.Request) {
		case REQ_create_new_deployment:
			if getResponseIndex(workerslicegwrecycler.Status.Client.Response) >= RESP_new_deployment_created {
				return ctrl.Result{}, nil
			}
			depName := getNewDeploymentName(workerslicegwrecycler.Spec.GwPair.ClientID)
			_, err, _ := r.CreateNewDeployment(ctx, depName, slicegw.Spec.SliceName, slicegw.Name)
			if err != nil {
				return ctrl.Result{}, err
			}

			nsmIP, err := r.GetNsmIPForNewDeployment(ctx, &slicegw, getNewDeploymentName(workerslicegwrecycler.Spec.GwPair.ClientID))
			if nsmIP == "" {
				log.Info("Waiting for new deployment to get nsm IP")
				return ctrl.Result{
					RequeueAfter: 10 * time.Second,
				}, nil
			}
			log.Info("Nsm IP obtained on new client deployment", "nsmIP", nsmIP)

			// Check if the slice router installed a route to the remote cluster with the new gw pod as next hop
			routeInstalled, err := r.CheckRouteInSliceRouter(ctx, &slicegw, nsmIP)
			if !routeInstalled {
				log.Info("Waiting for new route to be installed in the router")
				return ctrl.Result{
					RequeueAfter: 10 * time.Second,
				}, nil
			}
			log.Info("Route installed in the slice router for the new client deployment")

			err = retry.Do(func() error {
				workerslicegwrecycler := &spokev1alpha1.WorkerSliceGwRecycler{}
				err := r.Get(ctx, req.NamespacedName, workerslicegwrecycler)
				if err != nil {
					log.Error(err, "Failed to get workerslicegwrecycler")
					return err
				}
				workerslicegwrecycler.Status.Client.Response = getResponseString(RESP_new_deployment_created)
				return r.Status().Update(ctx, workerslicegwrecycler)
			}, retry.Attempts(10))
			if err != nil {
				return ctrl.Result{}, err
			}

			utils.RecordEvent(ctx, r.EventRecorder, workerslicegwrecycler, nil, ossEvents.EventFSMNewGWSpawned, controllerName)

			return ctrl.Result{}, nil

		case REQ_update_routing_table:
			if getResponseIndex(workerslicegwrecycler.Status.Client.Response) >= RESP_routing_table_updated {
				return ctrl.Result{}, nil
			}
			// Deployments are created on both the clusters. Update the routing table in the slice
			// router to include the new deployment and remove the old.
			err = r.MarkGwRouteForDeletion(ctx, &slicegw, workerslicegwrecycler.Spec.GwPair.ClientID)
			if err != nil {
				log.Error(err, "Failed to mark gw route for deletion")
				return ctrl.Result{
					RequeueAfter: 10 * time.Second,
				}, err
			}

			nsmIP, err := r.GetNsmIPForNewDeployment(ctx, &slicegw, workerslicegwrecycler.Spec.GwPair.ClientID)
			if nsmIP != "" {
				routeExists, _ := r.CheckRouteInSliceRouter(ctx, &slicegw, nsmIP)
				if routeExists {
					log.Info("Waiting for the route to be removed", "depName", workerslicegwrecycler.Spec.GwPair.ClientID, "nsmIP", nsmIP)
					return ctrl.Result{
						RequeueAfter: 10 * time.Second,
					}, nil
				}
			}

			err = retry.Do(func() error {
				workerslicegwrecycler := &spokev1alpha1.WorkerSliceGwRecycler{}
				err := r.Get(ctx, req.NamespacedName, workerslicegwrecycler)
				if err != nil {
					log.Error(err, "Failed to get workerslicegwrecycler")
					return err
				}
				workerslicegwrecycler.Status.Client.Response = getResponseString(RESP_routing_table_updated)
				return r.Status().Update(ctx, workerslicegwrecycler)
			}, retry.Attempts(10))
			if err != nil {
				return ctrl.Result{}, err
			}

			utils.RecordEvent(ctx, r.EventRecorder, workerslicegwrecycler, nil, ossEvents.EventFSMRoutingTableUpdated, controllerName)

			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil

		case REQ_delete_old_gw_deployment:
			if getResponseIndex(workerslicegwrecycler.Status.Client.Response) >= RESP_old_deployment_deleted {
				return ctrl.Result{}, nil
			}
			err := r.TriggerGwDeploymentDeletion(ctx, slicegw.Spec.SliceName, slicegw.Name, workerslicegwrecycler.Spec.GwPair.ClientID,
				getNewDeploymentName(workerslicegwrecycler.Spec.GwPair.ClientID))
			if err != nil {
				return ctrl.Result{
					RequeueAfter: 10 * time.Second,
				}, nil
			}

			// Check if the deployment was deleted
			if r.CheckIfDeploymentIsPresent(ctx, workerslicegwrecycler.Spec.GwPair.ClientID, slicegw.Spec.SliceName, slicegw.Name) {
				return ctrl.Result{
					RequeueAfter: 10 * time.Second,
				}, nil
			}

			err = retry.Do(func() error {
				workerslicegwrecycler := &spokev1alpha1.WorkerSliceGwRecycler{}
				err := r.Get(ctx, req.NamespacedName, workerslicegwrecycler)
				if err != nil {
					log.Error(err, "Failed to get workerslicegwrecycler")
					return err
				}
				workerslicegwrecycler.Status.Client.Response = getResponseString(RESP_old_deployment_deleted)
				return r.Status().Update(ctx, workerslicegwrecycler)
			}, retry.Attempts(10))
			if err != nil {
				return ctrl.Result{}, err
			}

			utils.RecordEvent(ctx, r.EventRecorder, workerslicegwrecycler, nil, ossEvents.EventFSMDeleteOldGW, controllerName)

			return ctrl.Result{}, nil

		default:
			log.Info("In default state")
			return ctrl.Result{
				RequeueAfter: 10 * time.Second,
			}, nil
		}
	}

	return ctrl.Result{}, nil
}

func (a *Reconciler) InjectClient(c client.Client) error {
	a.Client = c
	return nil
}

// SetupWithManager sets up reconciler with manager
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.FSM = make(map[string]*fsm.FSM) // Initialize the map of FSMs
	return ctrl.NewControllerManagedBy(mgr).
		For(&spokev1alpha1.WorkerSliceGwRecycler{}).
		Complete(r)
}
