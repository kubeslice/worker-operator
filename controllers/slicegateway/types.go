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

package slicegateway

import (
	"context"

	monitoringEvents "github.com/kubeslice/kubeslice-monitoring/pkg/events"
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	"github.com/kubeslice/worker-operator/pkg/gwsidecar"
	hub "github.com/kubeslice/worker-operator/pkg/hub/hubclient"
	"github.com/kubeslice/worker-operator/pkg/netop"
	"github.com/kubeslice/worker-operator/pkg/router"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NetOpPod contains details of NetOp Pod running in the cluster
type NetOpPod struct {
	PodIP   string
	PodName string
	Node    string
}
type HubClientProvider interface {
	UpdateNodePortForSliceGwServer(ctx context.Context, sliceGwNodePort []int, sliceGwName string) error
	GetClusterNodeIP(ctx context.Context, clusterName, namespace string) ([]string, error)
	CreateWorkerSliceGwRecycler(ctx context.Context, gwRecyclerName, clientID, serverID, sliceGwServer, sliceGwClient, slice string) error
}
type WorkerGWSidecarClientProvider interface {
	GetSliceGwRemotePodName(ctx context.Context, gwRemoteVpnIP string, serverAddr string) (string, error)
	GetStatus(ctx context.Context, serverAddr string) (*gwsidecar.GwStatus, error)
	SendConnectionContext(ctx context.Context, serverAddr string, gwConnCtx *gwsidecar.GwConnectionContext) error
	UpdateSliceQosProfile(ctx context.Context, serverAddr string, slice *kubeslicev1beta1.Slice) error
}

type WorkerRouterClientProvider interface {
	GetClientConnectionInfo(ctx context.Context, addr string) ([]kubeslicev1beta1.AppPod, error)
	SendConnectionContext(ctx context.Context, serverAddr string, sliceRouterConnCtx *router.SliceRouterConnCtx) error
	UpdateEcmpRoutes(ctx context.Context, serverAddr string, ecmpUpdateInfo *router.UpdateEcmpInfo) error
}
type WorkerNetOpClientProvider interface {
	UpdateSliceQosProfile(ctx context.Context, addr string, slice *kubeslicev1beta1.Slice) error
	SendSliceLifeCycleEventToNetOp(ctx context.Context, addr string, sliceName string, eventType netop.EventType) error
	SendConnectionContext(ctx context.Context, serverAddr string, gw *kubeslicev1beta1.SliceGateway, sliceGwNodePorts []int) error
}

type WorkerRecyclerClientProvider interface {
	TriggerFSM(ctx context.Context, sliceGw *kubeslicev1beta1.SliceGateway, slice *kubeslicev1beta1.Slice, hubClient *hub.HubClientConfig, meshClient client.Client, gatewayPod *corev1.Pod,
		eventRecorder *monitoringEvents.EventRecorder, controllerName, gwRecyclerName string) (bool, error)
}
