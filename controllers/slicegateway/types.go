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

	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	"github.com/kubeslice/worker-operator/pkg/gwsidecar"
	"github.com/kubeslice/worker-operator/pkg/netop"
	"github.com/kubeslice/worker-operator/pkg/router"
)

// NetOpPod contains details of NetOp Pod running in the cluster
type NetOpPod struct {
	PodIP   string
	PodName string
	Node    string
}

type HubClientProvider interface {
	UpdateNodePortForSliceGwServer(ctx context.Context, sliceGwNodePort int32, sliceGwName string) error
	UpdateNodeIpInCluster(ctx context.Context, clusterName, nodeIP, namespace string, slicegateway *kubeslicev1beta1.SliceGateway) error
	GetClusterNodeIP(ctx context.Context, clusterName, namespace string) (string, error)
}
type WorkerGWSidecarClientProvider interface {
	GetStatus(ctx context.Context, serverAddr string) (*gwsidecar.GwStatus, error)
	SendConnectionContext(ctx context.Context, serverAddr string, gwConnCtx *gwsidecar.GwConnectionContext) error
	UpdateSliceQosProfile(ctx context.Context, serverAddr string, slice *kubeslicev1beta1.Slice) error
}

type WorkerRouterClientProvider interface {
	GetClientConnectionInfo(ctx context.Context, addr string) ([]kubeslicev1beta1.AppPod, error)
	SendConnectionContext(ctx context.Context, serverAddr string, sliceRouterConnCtx *router.SliceRouterConnCtx) error
}
type WorkerNetOpClientProvider interface {
	UpdateSliceQosProfile(ctx context.Context, addr string, slice *kubeslicev1beta1.Slice) error
	SendSliceLifeCycleEventToNetOp(ctx context.Context, addr string, sliceName string, eventType netop.EventType) error
	SendConnectionContext(ctx context.Context, serverAddr string, gw *kubeslicev1beta1.SliceGateway, sliceGwNodePort int32) error
}
