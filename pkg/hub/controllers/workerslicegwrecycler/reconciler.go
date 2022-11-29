package workerslicegwrecycler

import (
	"context"

	"github.com/go-logr/logr"
	spokev1alpha1 "github.com/kubeslice/apis/pkg/worker/v1alpha1"
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	"github.com/kubeslice/worker-operator/pkg/events"
	"github.com/kubeslice/worker-operator/pkg/logger"
	"github.com/looplab/fsm"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	INIT                string = "init"
	new_gw_spawned      string = "new_gw_spawned"
	slicerouter_updated string = "slicerouter_updated"
	old_gw_deleted      string = "old_gw_deleted"
	ERROR               string = "error"
	END                 string = "end"
)

const (
	spawn_new_gw_pod     string = "spawn_new_gw_pod"
	update_routing_table string = "update_routing_table"
	delete_old_gw_pods   string = "delete_old_gw_pods"
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
	log := r.Log.WithValues("workerslicegwrecycler", req.NamespacedName)
	ctx = logger.WithLogger(ctx, log)

	workerslicegwrecycler := &spokev1alpha1.WorkerSliceGwRecycler{}

	err := r.Get(ctx, req.NamespacedName, workerslicegwrecycler)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Return and don't requeue
			log.Info("workerslicegwrecycler resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get workerslicegwrecycler")
		return ctrl.Result{}, err
	}

	log.Info("reconciling", "workerslicegwrecycler", workerslicegwrecycler.Name)

	slicegw := kubeslicev1beta1.SliceGateway{}

	if err := r.MeshClient.Get(ctx, types.NamespacedName{Namespace: "kubeslice-system", Name: workerslicegwrecycler.Spec.SliceGwServer}, &slicegw); err != nil {
		if errors.IsNotFound(err) {
			if err := r.MeshClient.Get(ctx, types.NamespacedName{Namespace: "kubeslice-system", Name: workerslicegwrecycler.Spec.SliceGwClient}, &slicegw); err != nil {
				return ctrl.Result{}, err
			}
		}
	}
	isClient := slicegw.Status.Config.SliceGatewayHostType == "Client"

	if isClient {
		switch workerslicegwrecycler.Spec.Request {
		case spawn_new_gw_pod:
			err := r.FSM.Event(spawn_new_gw_pod, workerslicegwrecycler, isClient)
			if err != nil {
				return ctrl.Result{}, err
			}
		case update_routing_table:
			err := r.FSM.Event(update_routing_table, workerslicegwrecycler, isClient, slicegw)
			if err != nil {
				return ctrl.Result{}, err
			}
		case delete_old_gw_pods:
			err := r.FSM.Event(delete_old_gw_pods,workerslicegwrecycler, isClient)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		switch workerslicegwrecycler.Status.Client.Response {
		case new_gw_spawned:
			err := r.FSM.Event(spawn_new_gw_pod, workerslicegwrecycler, isClient)
			if err != nil {
				return ctrl.Result{}, err
			}
		case slicerouter_updated:
			err := r.FSM.Event(update_routing_table, workerslicegwrecycler, isClient, slicegw)
			if err != nil {
				return ctrl.Result{}, err
			}
		case old_gw_deleted:
			err := r.FSM.Event(delete_old_gw_pods, workerslicegwrecycler, isClient)
			if err != nil {
				return ctrl.Result{}, err
			}
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
			{Name: spawn_new_gw_pod, Src: []string{INIT}, Dst: new_gw_spawned},
			{Name: update_routing_table, Src: []string{INIT,new_gw_spawned}, Dst: slicerouter_updated},
			{Name: delete_old_gw_pods, Src: []string{INIT,slicerouter_updated}, Dst: old_gw_deleted},
		},
		fsm.Callbacks{
			"enter_new_gw_spawned":      func(e *fsm.Event) { r.spawn_new_gw_pod(e) },
			"enter_slicerouter_updated": func(e *fsm.Event) { r.update_routing_table(e) },
			"enter_old_gw_deleted":  func(e *fsm.Event) { r.delete_old_gw_pods(e) },
		},
	)
	return ctrl.NewControllerManagedBy(mgr).
		For(&spokev1alpha1.WorkerSliceGwRecycler{}).
		Complete(r)
}
