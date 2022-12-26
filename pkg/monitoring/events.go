package monitoring

import (
	"fmt"
	"os"

	zap "go.uber.org/zap"
	"golang.org/x/net/context"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/reference"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type EventType string

var (
	ClusterName = os.Getenv("CLUSTER_NAME")

	EventTypeWarning EventType = "Warning"
	EventTypeNormal  EventType = "Normal"
)

type EventRecorder struct {
	client         client.Client
	logger         *zap.SugaredLogger
	scheme         *runtime.Scheme
	releaseVersion string
}

type Event struct {
	Object        runtime.Object
	RelatedObject runtime.Object
	EventType     EventType
	Reason        string
	Slice         string
	Tenant        string
	Message       string
	Action        string
}

func NewEventRecorder(client client.Client, scheme *runtime.Scheme, logger *zap.SugaredLogger, version string) *EventRecorder {
	return &EventRecorder{
		client:         client,
		logger:         logger,
		scheme:         scheme,
		releaseVersion: version,
	}
}

func (er *EventRecorder) RecordEvent(ctx context.Context, e *Event) error {
	ref, err := reference.GetReference(er.scheme, e.Object)
	if err != nil {
		er.logger.With("error", err).Error("Unable to parse event obj reference")
		return err
	}

	t := metav1.Now()
	ev := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%v.%x", ref.Name, t.UnixNano()),
			Namespace: ref.Namespace,
			Labels: map[string]string{
				"sliceCluster":   ClusterName,
				"sliceNamespace": ref.Namespace,
				"sliceName":      e.Slice,
				"sliceTenant":    e.Tenant,
			},
			Annotations: map[string]string{
				"release": er.releaseVersion,
			},
		},
		InvolvedObject:      *ref,
		EventTime:           metav1.NowMicro(),
		Reason:              e.Reason,
		Message:             e.Message,
		FirstTimestamp:      t,
		LastTimestamp:       t,
		Count:               1,
		ReportingController: "ks",
		ReportingInstance:   "ks",
		Source: corev1.EventSource{
			Component: "kubeslice-monitoring",
		},
		Action: e.Action,
		Type:   string(e.EventType),
	}

	if e.RelatedObject != nil {
		related, err := reference.GetReference(er.scheme, e.RelatedObject)
		if err != nil {
			er.logger.With("error", err).Error("Unable to parse event related obj reference")
			return err
		}
		ev.Related = related
	}

	if err := er.client.Create(ctx, ev); err != nil {
		er.logger.With("error", err, "event", ev).Error("Unable to create event")
	}
	return nil
}
