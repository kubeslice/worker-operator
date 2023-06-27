package slicegwrecycler

import (
	"context"

	monitoringEvents "github.com/kubeslice/kubeslice-monitoring/pkg/events"
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	hub "github.com/kubeslice/worker-operator/pkg/hub/hubclient"
	_ "github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type VPNClientEmulator struct {
	client.Client
}

func NewVPNClientEmulator(client client.Client) (*VPNClientEmulator, error) {
	return &VPNClientEmulator{
		Client: client,
	}, nil
}

func (c VPNClientEmulator) TriggerFSM(ctx context.Context, sliceGw *kubeslicev1beta1.SliceGateway, slice *kubeslicev1beta1.Slice, hubClient *hub.HubClientConfig, meshClient client.Client, gatewayPod *corev1.Pod,
	eventRecorder *monitoringEvents.EventRecorder, controllerName, gwRecyclerName string) (bool, error) {
	return true, nil
}
