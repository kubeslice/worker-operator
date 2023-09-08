package vpnkeyrotation

import (
	"github.com/kubeslice/kubeslice-monitoring/pkg/events"
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type WorkerRecyclerClientProvider interface {
	// triggers FSM to recycle gateway pair by passing server gateway pod
	// numberOfGwSvc should be equal to number of new deploy that should come up
	TriggerFSM(sliceGw *kubeslicev1beta1.SliceGateway, slice *kubeslicev1beta1.Slice, serverID, clientID, controllerName string) error
}

type Reconciler struct {
	client.Client
	WorkerClient         client.Client
	ControllerClient     client.Client
	EventRecorder        *events.EventRecorder
	WorkerRecyclerClient WorkerRecyclerClientProvider
}
