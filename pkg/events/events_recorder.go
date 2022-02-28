package events

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"strings"
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
	Object      runtime.Object
	EventType   EventType
	Reason      string
	Message     string
	ClusterName string
}

func (event *Event) NewEvent(eventRecorder *EventRecorder) {
	eventRecorder.Recorder.Event(event.Object, string(event.EventType), event.Reason, event.Message)
}

func getSeverity(eventType EventType) string {
	switch strings.ToLower(string(eventType)) {
	case "normal":
		return "SEVERITY_NORMAL"
	case "warning":
		return "SEVERITY_WARNING"
	}
	return "SEVERITY_NORMAL"
}
