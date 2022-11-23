package workerslicegwrecycler

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/kubeslice/worker-operator/pkg/events"
	"github.com/looplab/fsm"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	spokev1alpha1 "github.com/kubeslice/apis/pkg/worker/v1alpha1"
)

const (
	INIT string = "init"
	new_gw_spawned string  = "new_gw_spawned"
	slicerouter_updated  string = "slicerouter_updated"
	old_gw_deleted string = "old_gw_deleted"
	ERROR string = "error"
	END string = "end"
)

const (
	spawn_new_gw_pod  string = "spawn_new_gw_pod"
	update_routing_table string = "update_routing_table"
	delete_old_gw_pods string = "delete_old_gw_pods"
)

type HubClientProvider interface{}

type Reconciler struct {
	client.Client
	Log           logr.Logger
	Scheme        *runtime.Scheme
	HubClient     HubClientProvider
	EventRecorder *events.EventRecorder
	FSM *fsm.FSM
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return ctrl.Result{}, nil
}

// SetupWithManager sets up reconciler with manager
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	//TODO: add more states and events 
	r.FSM = fsm.NewFSM(
		INIT,
		fsm.Events{
			{Name: spawn_new_gw_pod, Src: []string{INIT}, Dst: new_gw_spawned},
			{Name: update_routing_table, Src: []string{new_gw_spawned}, Dst: slicerouter_updated},
			{Name: delete_old_gw_pods, Src: []string{slicerouter_updated}, Dst: old_gw_deleted},
		},
		fsm.Callbacks{
			"enter_new_gw_spawned": func(e *fsm.Event) {r.spawn_new_gw_pod(e)},
			"enter_slicerouter_updated": func(e *fsm.Event) { r.update_routing_table(e) },
		},
	)
	return ctrl.NewControllerManagedBy(mgr).
		For(&spokev1alpha1.WorkerSliceGwRecycler{}).
		Complete(r)
}