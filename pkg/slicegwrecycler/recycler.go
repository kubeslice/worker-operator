package slicegwrecycler

import (
	"context"

	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"

	"github.com/kubeslice/kubeslice-monitoring/pkg/events"
	ossEvents "github.com/kubeslice/worker-operator/events"

	hub "github.com/kubeslice/worker-operator/pkg/hub/hubclient"
	"github.com/kubeslice/worker-operator/pkg/logger"
	"github.com/kubeslice/worker-operator/pkg/utils"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"k8s.io/apimachinery/pkg/runtime"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type recyclerClient struct {
	controllerClient client.Client
	workerClient     client.Client
	ctx              context.Context
	eventRecorder    *events.EventRecorder
	scheme           *runtime.Scheme
}

func NewRecyclerClient(ctx context.Context, workerClient client.Client, controllerClient client.Client, er *events.EventRecorder, scheme *runtime.Scheme) (*recyclerClient, error) {
	return &recyclerClient{
		controllerClient: controllerClient,
		workerClient:     workerClient,
		ctx:              ctx,
		eventRecorder:    er,
		scheme:           scheme,
	}, nil
}

func (r recyclerClient) TriggerFSM(sliceGw *kubeslicev1beta1.SliceGateway, slice *kubeslicev1beta1.Slice,
	serverID, clientID, controllerName string) error {
	// start FSM for graceful termination of gateway pods
	// create workerslicegwrecycler on controller
	log := logger.FromContext(r.ctx).WithName("fsm-recycler")

	log.Info("creating workerslicegwrecycler", "gwRecyclerName", serverID, "slicegateway", sliceGw.Name)
	err := r.controllerClient.(*hub.HubClientConfig).CreateWorkerSliceGwRecycler(r.ctx,
		serverID,           // recycler name
		clientID, serverID, // gateway pod pairs to recycle
		sliceGw.Name, sliceGw.Status.Config.SliceGatewayRemoteGatewayID, // slice gateway server and client name
		sliceGw.Spec.SliceName) // slice name
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			log.Info("workerslicegwrecycler already exists", "gwRecyclerName", serverID)
			return nil
		}
		return err
	}
	utils.RecordEvent(r.ctx, r.eventRecorder, sliceGw, slice, ossEvents.EventSliceGWRebalancingSuccess, controllerName)
	return nil
}
