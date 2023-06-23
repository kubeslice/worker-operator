package vpnkeyrotation

import (
	"context"

	"github.com/kubeslice/kubeslice-monitoring/pkg/events"
	monitoringEvents "github.com/kubeslice/kubeslice-monitoring/pkg/events"
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	hub "github.com/kubeslice/worker-operator/pkg/hub/hubclient"
	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type WorkerRecyclerClientProvider interface {
	TriggerFSM(ctx context.Context, sliceGw *kubeslicev1beta1.SliceGateway, slice *kubeslicev1beta1.Slice, hubClient *hub.HubClientConfig, meshClient client.Client, gatewayPod *corev1.Pod,
		eventrecorder *monitoringEvents.EventRecorder, controllerName, gwRecyclerName string) (bool, error)
}

type Reconciler struct {
	client.Client
	MeshClient    client.Client
	HubClient     client.Client
	EventRecorder *events.EventRecorder

	// metrics
	gaugeClusterUp       *prometheus.GaugeVec
	gaugeComponentUp     *prometheus.GaugeVec
	WorkerRecyclerClient WorkerRecyclerClientProvider
}
