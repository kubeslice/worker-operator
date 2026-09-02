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

package cluster

import (
	"context"
	"testing"

	hubv1alpha1 "github.com/kubeslice/apis/pkg/controller/v1alpha1"
	mevents "github.com/kubeslice/kubeslice-monitoring/pkg/events"
	"github.com/kubeslice/kubeslice-monitoring/pkg/metrics"
	ossEvents "github.com/kubeslice/worker-operator/events"
	hub "github.com/kubeslice/worker-operator/pkg/hub/hubclient"
	utilmock "github.com/kubeslice/worker-operator/pkg/mocks"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/scheme"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// newConditionsReconciler builds a Reconciler wired to a mock client that
// accepts any Event Create, so tests can focus on cr.Status.Conditions and on
// how many events fired rather than on mock plumbing.
func newConditionsReconciler(t *testing.T, connInfo ConnectionInfo) (*Reconciler, *utilmock.MockClient) {
	t.Helper()
	client := utilmock.NewClient()
	client.On("Create", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	testScheme := runtime.NewScheme()
	if err := scheme.AddToScheme(testScheme); err != nil {
		t.Fatalf("adding core scheme: %v", err)
	}
	testScheme.AddKnownTypeWithName(hubv1alpha1.GroupVersion.WithKind("Cluster"), &hubv1alpha1.Cluster{})

	er := mevents.NewEventRecorder(client, testScheme, ossEvents.EventsMap, mevents.EventRecorderOptions{
		Cluster:   "worker-1",
		Project:   "avesha",
		Component: "worker-operator",
	})
	mf, err := metrics.NewMetricsFactory(prometheus.NewRegistry(), metrics.MetricsFactoryOptions{})
	if err != nil {
		t.Fatalf("NewMetricsFactory: %v", err)
	}
	return NewReconciler(client, client, &er, mf, connInfo), client
}

func newTestCluster() *hubv1alpha1.Cluster {
	return &hubv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "worker-1", Namespace: "avesha"},
	}
}

// TestSetConnectionConditions_NonHA covers #469's EndpointNotConfigured case:
// every existing non-HA deployment, which must see an explicit Unknown state
// rather than either a misleading True/False or a silently absent condition.
func TestSetConnectionConditions_NonHA(t *testing.T) {
	r, client := newConditionsReconciler(t, ConnectionInfo{Enabled: false})
	cr := newTestCluster()

	r.setConnectionConditions(context.Background(), cr)

	connected := meta.FindStatusCondition(cr.Status.Conditions, ConditionControllerConnected)
	assert.NotNil(t, connected)
	assert.Equal(t, metav1.ConditionUnknown, connected.Status)
	assert.Equal(t, ReasonEndpointNotConfigured, connected.Reason)

	synced := meta.FindStatusCondition(cr.Status.Conditions, ConditionControllerEndpointSynced)
	assert.NotNil(t, synced)
	assert.Equal(t, metav1.ConditionUnknown, synced.Status)

	client.AssertNotCalled(t, "Create", mock.Anything, mock.Anything, mock.Anything)
}

// TestSetConnectionConditions_ReportsReconnectOnceThenSteadyState is #468
// scenario 1 (clean failover/reconnect) at the reconciler level: the first
// reconcile after a resolved switch must say so, and every reconcile after
// that must report plain Connected, not repeat ReconnectedAfterFailover.
func TestSetConnectionConditions_ReportsReconnectOnceThenSteadyState(t *testing.T) {
	r, client := newConditionsReconciler(t, ConnectionInfo{Enabled: true, Reconnected: true})
	cr := newTestCluster()

	r.setConnectionConditions(context.Background(), cr)
	first := meta.FindStatusCondition(cr.Status.Conditions, ConditionControllerConnected)
	assert.Equal(t, metav1.ConditionTrue, first.Status)
	assert.Equal(t, ReasonReconnectedAfterFailover, first.Reason)

	r.setConnectionConditions(context.Background(), cr)
	second := meta.FindStatusCondition(cr.Status.Conditions, ConditionControllerConnected)
	assert.Equal(t, ReasonConnected, second.Reason, "every reconcile after the first must report plain Connected")

	// ControllerEndpointChanged fires once on the first call (announcing the
	// failover), then ControllerConnected fires once on the second call (the
	// transition into steady Connected) — not on the first, since its reason
	// there is ReconnectedAfterFailover, not Connected.
	client.AssertNumberOfCalls(t, "Create", 2)
}

// TestSetConnectionConditions_Idempotent is #469's explicit acceptance
// criterion: calling this with the same outcome twice must not bump
// LastTransitionTime, or every steady-state reconcile would look like a
// fresh transition to anyone reading the CR.
func TestSetConnectionConditions_Idempotent(t *testing.T) {
	r, client := newConditionsReconciler(t, ConnectionInfo{Enabled: true})
	cr := newTestCluster()

	r.setConnectionConditions(context.Background(), cr)
	firstTransition := meta.FindStatusCondition(cr.Status.Conditions, ConditionControllerConnected).LastTransitionTime

	r.setConnectionConditions(context.Background(), cr)
	secondTransition := meta.FindStatusCondition(cr.Status.Conditions, ConditionControllerConnected).LastTransitionTime

	assert.Equal(t, firstTransition, secondTransition, "an unchanged outcome must not bump LastTransitionTime")
	// The second no-op reconcile must not fire another event.
	client.AssertNumberOfCalls(t, "Create", 1)
}

// TestSetConnectionConditions_NeverReportsLiveFailure documents the
// architectural limit explained in conditions.go: this method only ever
// leaves DialFailed/CertVerificationFailed absent, because it only runs at
// all on a hub this worker could just reach.
func TestSetConnectionConditions_NeverReportsLiveFailure(t *testing.T) {
	r, _ := newConditionsReconciler(t, ConnectionInfo{Enabled: true})
	cr := newTestCluster()

	r.setConnectionConditions(context.Background(), cr)

	connected := meta.FindStatusCondition(cr.Status.Conditions, ConditionControllerConnected)
	assert.NotEqual(t, hub.ReasonDialFailed, connected.Reason)
	assert.NotEqual(t, hub.ReasonCertVerificationFailed, connected.Reason)
}
