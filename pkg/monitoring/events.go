package monitoring

import (
	"fmt"

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
	EventTypeWarning EventType = "Warning"
	EventTypeNormal  EventType = "Normal"
)

type EventRecorder struct {
	Client    client.Client
	Logger    *zap.SugaredLogger
	Scheme    *runtime.Scheme
	Version   string
	Cluster   string
	Tenant    string
	Slice     string
	Namespace string
	Component string
}

type Event struct {
	Object            runtime.Object
	RelatedObject     runtime.Object
	EventType         EventType
	Reason            string
	Message           string
	Action            string
	ReportingInstance string
}

func (er *EventRecorder) Copy() *EventRecorder {
	return &EventRecorder{
		Client:    er.Client,
		Logger:    er.Logger,
		Scheme:    er.Scheme,
		Version:   er.Version,
		Cluster:   er.Cluster,
		Tenant:    er.Tenant,
		Slice:     er.Slice,
		Namespace: er.Namespace,
		Component: er.Component,
	}
}

// WithSlice returns a new recorder with added slice name for raising events
func (er *EventRecorder) WithSlice(slice string) *EventRecorder {
	e := er.Copy()
	e.Slice = slice
	return e
}

// WithNamespace returns a new recorder with added namespace name
// If namespace is not provided, recorder will use the object namespace
func (er *EventRecorder) WithNamespace(ns string) *EventRecorder {
	e := er.Copy()
	e.Namespace = ns
	return e
}

// RecordEvent raises a new event with the given fields
// TODO: events caching and aggregation
func (er *EventRecorder) RecordEvent(ctx context.Context, e *Event) error {
	ref, err := reference.GetReference(er.Scheme, e.Object)
	if err != nil {
		er.Logger.With("error", err).Error("Unable to parse event obj reference")
		return err
	}

	ns := er.Namespace
	if ns == "" {
		ns = ref.Namespace
	}

	t := metav1.Now()
	ev := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%v.%x", ref.Name, t.UnixNano()),
			Namespace: ns,
			Labels: map[string]string{
				"sliceCluster":   er.Cluster,
				"sliceNamespace": ns,
				"sliceName":      er.Slice,
				"sliceTenant":    er.Tenant,
			},
			Annotations: map[string]string{
				"release": er.Version,
			},
		},
		InvolvedObject:      *ref,
		EventTime:           metav1.NowMicro(),
		Reason:              e.Reason,
		Message:             e.Message,
		FirstTimestamp:      t,
		LastTimestamp:       t,
		Count:               1,
		ReportingController: er.Component,
		ReportingInstance:   e.ReportingInstance,
		Source: corev1.EventSource{
			Component: er.Component,
		},
		Action: e.Action,
		Type:   string(e.EventType),
	}

	if e.RelatedObject != nil {
		related, err := reference.GetReference(er.Scheme, e.RelatedObject)
		if err != nil {
			er.Logger.With("error", err).Error("Unable to parse event related obj reference")
			return err
		}
		ev.Related = related
	}

	er.Logger.Info("raised event", "event", ev)

	if err := er.Client.Create(ctx, ev); err != nil {
		er.Logger.With("error", err, "eventReason", ev.Reason).Error("Unable to create event")
	}
	return nil
}
