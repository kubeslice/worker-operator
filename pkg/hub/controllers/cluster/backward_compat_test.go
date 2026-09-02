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

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestSetConnectionConditions_NonHATakesPrecedenceOverReconnected is issue
// #468 scenario 4 (non-HA backward compat), defensively: main.go can only
// ever produce Reconnected=true alongside Enabled=true in practice (it comes
// from StartupConnection, which only runs inside the Enabled guard), but if
// that ever changed, a non-HA worker must still report EndpointNotConfigured
// rather than a misleading ReconnectedAfterFailover.
func TestSetConnectionConditions_NonHATakesPrecedenceOverReconnected(t *testing.T) {
	r, _ := newConditionsReconciler(t, ConnectionInfo{Enabled: false, Reconnected: true})
	cr := newTestCluster()

	r.setConnectionConditions(context.Background(), cr)

	connected := meta.FindStatusCondition(cr.Status.Conditions, ConditionControllerConnected)
	assert.Equal(t, metav1.ConditionUnknown, connected.Status)
	assert.Equal(t, ReasonEndpointNotConfigured, connected.Reason,
		"Enabled=false must win regardless of Reconnected, since this repo's own callers never expect the combination")
}
