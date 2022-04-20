package events

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
)

type EventType string

var EventTypeWarning EventType = "Warning"
var EventTypeNormal EventType = "Normal"

type EventRecorder struct {
	// recorder is an event recorder for recording Event resources to the Kubernetes API.
	Recorder record.EventRecorder
}

func NewEventRecorder(recorder record.EventRecorder) *EventRecorder {
	c := &EventRecorder{
		Recorder: recorder,
	}
	return c
}

type Event struct {
	Object    runtime.Object
	EventType EventType
	Reason    string
	Message   string
}

func (recorder *EventRecorder) Record(event *Event) {
	recorder.Recorder.Event(event.Object, string(event.EventType), event.Reason, event.Message)
}
