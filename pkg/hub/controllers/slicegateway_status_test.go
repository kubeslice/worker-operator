/*
 *  Copyright (c) 2026 Avesha, Inc. All rights reserved.
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

package controllers

import (
	"context"
	"testing"

	spokev1alpha1 "github.com/kubeslice/apis/pkg/worker/v1alpha1"
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func pod(state string) *kubeslicev1beta1.GwPodInfo {
	return &kubeslicev1beta1.GwPodInfo{TunnelStatus: kubeslicev1beta1.TunnelStatus{TunnelState: state}}
}

func TestDeriveGatewayConnectionState(t *testing.T) {
	cases := []struct {
		name string
		pods []*kubeslicev1beta1.GwPodInfo
		want string
	}{
		{
			name: "no pod status is Pending",
			pods: nil,
			want: spokev1alpha1.GatewayConnectionStatePending,
		},
		{
			name: "all pods up is Connected",
			pods: []*kubeslicev1beta1.GwPodInfo{pod("UP"), pod("UP")},
			want: spokev1alpha1.GatewayConnectionStateConnected,
		},
		{
			name: "at least one pod up is Connected (HA)",
			pods: []*kubeslicev1beta1.GwPodInfo{pod("DOWN"), pod("UP")},
			want: spokev1alpha1.GatewayConnectionStateConnected,
		},
		{
			name: "all pods down is NotConnected",
			pods: []*kubeslicev1beta1.GwPodInfo{pod("DOWN"), pod("DOWN")},
			want: spokev1alpha1.GatewayConnectionStateNotConnected,
		},
		{
			name: "unknown/empty pod states are not up",
			pods: []*kubeslicev1beta1.GwPodInfo{pod("UNKNOWN"), pod("")},
			want: spokev1alpha1.GatewayConnectionStateNotConnected,
		},
		{
			name: "nil pod entries are ignored",
			pods: []*kubeslicev1beta1.GwPodInfo{nil, pod("UP")},
			want: spokev1alpha1.GatewayConnectionStateConnected,
		},
		{
			name: "all pod entries nil (no status reported) is Pending",
			pods: []*kubeslicev1beta1.GwPodInfo{nil, nil},
			want: spokev1alpha1.GatewayConnectionStatePending,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := deriveGatewayConnectionState(tc.pods)
			if got != tc.want {
				t.Fatalf("%s: got %q, want %q", tc.name, got, tc.want)
			}
		})
	}
}

func meshGwWithPods(states ...string) *kubeslicev1beta1.SliceGateway {
	mesh := &kubeslicev1beta1.SliceGateway{}
	for _, s := range states {
		mesh.Status.GatewayPodStatus = append(mesh.Status.GatewayPodStatus, pod(s))
	}
	return mesh
}

func TestReconcileGatewayConnectionStatus(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := spokev1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add scheme: %v", err)
	}
	key := types.NamespacedName{Name: "slice-hub-spoke1", Namespace: "kubeslice-project"}

	t.Run("writes Connected when a tunnel is up", func(t *testing.T) {
		gw := &spokev1alpha1.WorkerSliceGateway{}
		gw.Name, gw.Namespace = key.Name, key.Namespace
		c := fake.NewClientBuilder().WithScheme(scheme).
			WithObjects(gw).WithStatusSubresource(gw).Build()
		r := &SliceGwReconciler{Client: c}

		if err := r.reconcileGatewayConnectionStatus(context.Background(), gw, meshGwWithPods("UP")); err != nil {
			t.Fatalf("reconcile: %v", err)
		}
		got := &spokev1alpha1.WorkerSliceGateway{}
		if err := c.Get(context.Background(), key, got); err != nil {
			t.Fatalf("get: %v", err)
		}
		if got.Status.ConnectionState != spokev1alpha1.GatewayConnectionStateConnected {
			t.Fatalf("connectionState = %q, want Connected", got.Status.ConnectionState)
		}
		if got.Status.LastTransitionTime == nil {
			t.Fatal("expected LastTransitionTime to be set on transition")
		}
	})

	t.Run("no write when state is unchanged", func(t *testing.T) {
		gw := &spokev1alpha1.WorkerSliceGateway{}
		gw.Name, gw.Namespace = key.Name, key.Namespace
		gw.Status.ConnectionState = spokev1alpha1.GatewayConnectionStateNotConnected
		c := fake.NewClientBuilder().WithScheme(scheme).
			WithObjects(gw).WithStatusSubresource(gw).Build()
		r := &SliceGwReconciler{Client: c}

		// all pods down -> NotConnected, same as current -> no update, no timestamp.
		if err := r.reconcileGatewayConnectionStatus(context.Background(), gw, meshGwWithPods("DOWN")); err != nil {
			t.Fatalf("reconcile: %v", err)
		}
		got := &spokev1alpha1.WorkerSliceGateway{}
		if err := c.Get(context.Background(), key, got); err != nil {
			t.Fatalf("get: %v", err)
		}
		if got.Status.LastTransitionTime != nil {
			t.Fatal("expected no LastTransitionTime when state is unchanged")
		}
	})
}

func TestReasonMessageForState(t *testing.T) {
	cases := map[string]struct{ reason, msg string }{
		spokev1alpha1.GatewayConnectionStateConnected:    {"TunnelEstablished", "gateway tunnel is up"},
		spokev1alpha1.GatewayConnectionStateNotConnected: {"TunnelDown", "all gateway pods report their tunnel is down"},
		spokev1alpha1.GatewayConnectionStatePending:      {"Reconciling", "waiting for gateway tunnel connectivity to be reported"},
		"": {"Reconciling", "waiting for gateway tunnel connectivity to be reported"},
	}
	for state, want := range cases {
		r, m := reasonMessageForState(state)
		if r != want.reason || m != want.msg {
			t.Errorf("state %q: got (%q,%q), want (%q,%q)", state, r, m, want.reason, want.msg)
		}
	}
}
