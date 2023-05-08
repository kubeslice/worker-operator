package workerslicegwrecycler

import (
	"context"

	"github.com/go-logr/logr"
	spokev1alpha1 "github.com/kubeslice/apis/pkg/worker/v1alpha1"
	"github.com/kubeslice/kubeslice-monitoring/pkg/events"
	ossEvents "github.com/kubeslice/worker-operator/events"

	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"

	"github.com/kubeslice/worker-operator/pkg/logger"
	"github.com/kubeslice/worker-operator/pkg/utils"
	"github.com/looplab/fsm"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	INIT                   string = "init"
	new_deployment_created string = "new_deployment_created"
	slicerouter_updated    string = "slicerouter_updated"
	old_gw_deleted         string = "old_gw_deleted"
	ERROR                  string = "error"
	END                    string = "end"
)

const (
	verify_new_deployment_created string = "verify_new_deployment_created"
	update_routing_table          string = "update_routing_table"
	delete_old_gw_pods            string = "delete_old_gw_pods"
	controllerName                string = "workerSliceGWRecyclerController"
)

type Reconciler struct {
	client.Client
	Log                   logr.Logger
	Scheme                *runtime.Scheme
	MeshClient            client.Client
	WorkerGWSidecarClient WorkerGWSidecarClientProvider
	WorkerRouterClient    WorkerRouterClientProvider
	EventRecorder         *events.EventRecorder
	FSM                   *fsm.FSM
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logger.FromContext(ctx).WithName("workerslicegwrecycler")
	ctx = logger.WithLogger(ctx, log)

	workerslicegwrecycler := &spokev1alpha1.WorkerSliceGwRecycler{}

	err := r.Get(ctx, req.NamespacedName, workerslicegwrecycler)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Return and don't requeue
			log.Info("workerslicegwrecycler resource not found. Ignoring since object must be deleted")
			// move FSM to INIT state
			r.FSM.SetState(INIT)
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get workerslicegwrecycler")
		return ctrl.Result{}, err
	}

	log.Info("reconciling", "workerslicegwrecycler", workerslicegwrecycler.Name)
	log.V(1).Info("current state", "FSM", r.FSM.Current())
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
	*r.EventRecorder = (*r.EventRecorder).WithSlice(slicegw.Spec.SliceName)
	isClient := slicegw.Status.Config.SliceGatewayHostType == "Client"

	if isClient {
		switch workerslicegwrecycler.Spec.Request {
		case verify_new_deployment_created:
			err := r.FSM.Event(verify_new_deployment_created, workerslicegwrecycler, isClient)
			if err != nil {
				utils.RecordEvent(ctx, r.EventRecorder, workerslicegwrecycler, nil, ossEvents.EventFSMNewGWSpawnFailed, controllerName)
				return ctrl.Result{}, err
			}
			utils.RecordEvent(ctx, r.EventRecorder, workerslicegwrecycler, nil, ossEvents.EventFSMNewGWSpawned, controllerName)
		case update_routing_table:
			err := r.FSM.Event(update_routing_table, workerslicegwrecycler, isClient, slicegw)
			if err != nil {
				utils.RecordEvent(ctx, r.EventRecorder, workerslicegwrecycler, nil, ossEvents.EventFSMRoutingTableUpdateFailed, controllerName)
				return ctrl.Result{}, err
			}
			utils.RecordEvent(ctx, r.EventRecorder, workerslicegwrecycler, nil, ossEvents.EventFSMRoutingTableUpdated, controllerName)
		case delete_old_gw_pods:
			err := r.FSM.Event(delete_old_gw_pods, workerslicegwrecycler, isClient, slicegw)
			if err != nil {
				utils.RecordEvent(ctx, r.EventRecorder, workerslicegwrecycler, nil, ossEvents.EventFSMDeleteOldGWFailed, controllerName)
				return ctrl.Result{}, err
			}
			utils.RecordEvent(ctx, r.EventRecorder, workerslicegwrecycler, nil, ossEvents.EventFSMDeleteOldGW, controllerName)
		}
	} else {
		switch workerslicegwrecycler.Status.Client.Response {
		case new_deployment_created:
			err := r.FSM.Event(verify_new_deployment_created, workerslicegwrecycler, isClient)
			if err != nil {
				utils.RecordEvent(ctx, r.EventRecorder, workerslicegwrecycler, nil, ossEvents.EventFSMNewGWSpawnFailed, controllerName)
				return ctrl.Result{}, err
			}
			utils.RecordEvent(ctx, r.EventRecorder, workerslicegwrecycler, nil, ossEvents.EventFSMNewGWSpawned, controllerName)
		case slicerouter_updated:
			err := r.FSM.Event(update_routing_table, workerslicegwrecycler, isClient, slicegw)
			if err != nil {
				utils.RecordEvent(ctx, r.EventRecorder, workerslicegwrecycler, nil, ossEvents.EventFSMRoutingTableUpdateFailed, controllerName)
				return ctrl.Result{}, err
			}
			utils.RecordEvent(ctx, r.EventRecorder, workerslicegwrecycler, nil, ossEvents.EventFSMRoutingTableUpdated, controllerName)
		case old_gw_deleted:
			err := r.FSM.Event(delete_old_gw_pods, workerslicegwrecycler, isClient, slicegw)
			if err != nil {
				utils.RecordEvent(ctx, r.EventRecorder, workerslicegwrecycler, nil, ossEvents.EventFSMDeleteOldGWFailed, controllerName)
				return ctrl.Result{}, err
			}
			utils.RecordEvent(ctx, r.EventRecorder, workerslicegwrecycler, nil, ossEvents.EventFSMDeleteOldGW, controllerName)
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
	//TODO: add more states and events
	r.FSM = fsm.NewFSM(
		INIT,
		fsm.Events{
			{Name: verify_new_deployment_created, Src: []string{INIT, new_deployment_created}, Dst: new_deployment_created},
			{Name: update_routing_table, Src: []string{INIT, new_deployment_created, slicerouter_updated}, Dst: slicerouter_updated},
			{Name: delete_old_gw_pods, Src: []string{INIT, slicerouter_updated}, Dst: old_gw_deleted},
		},
		fsm.Callbacks{
			"enter_new_deployment_created": func(e *fsm.Event) { r.verify_new_deployment_created(e) },
			"enter_slicerouter_updated":    func(e *fsm.Event) { r.update_routing_table(e) },
			"enter_old_gw_deleted":         func(e *fsm.Event) { r.delete_old_gw_pods(e) },
		},
	)
	return ctrl.NewControllerManagedBy(mgr).
		For(&spokev1alpha1.WorkerSliceGwRecycler{}).
		Complete(r)
}
