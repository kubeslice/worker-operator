/*
 *  Copyright (c) 2022 Avesha, Inc. All rights reserved.
 *
 *  SPDX-License-Identifier: Apache-2.0
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */

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
	Object              runtime.Object
	EventType           EventType
	Reason              string
	Message             string
	Action              string
	ReportingController string
	Note                string
}

func (recorder *EventRecorder) Record(event *Event) {
	recorder.Recorder.Event(event.Object, string(event.EventType), event.Reason, event.Message)
}
