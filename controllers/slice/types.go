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

package slice

import (
	"context"

	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	"github.com/kubeslice/worker-operator/pkg/gatewayedge"
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
	UpdateAppPodsList(ctx context.Context, sliceConfigName string, appPods []kubeslicev1beta1.AppPod) error
	UpdateAppNamespaces(ctx context.Context, sliceConfigName string, onboardedNamespaces []string) error
	GetClusterNamespaceConfig(ctx context.Context, clusterName string) (map[string]string, map[string]string, error)
}

type WorkerRouterClientProvider interface {
	GetClientConnectionInfo(ctx context.Context, addr string) ([]kubeslicev1beta1.AppPod, error)
	SendConnectionContext(ctx context.Context, serverAddr string, sliceRouterConnCtx *router.SliceRouterConnCtx) error
}

type WorkerNetOpClientProvider interface {
	UpdateSliceQosProfile(ctx context.Context, addr string, slice *kubeslicev1beta1.Slice) error
	SendSliceLifeCycleEventToNetOp(ctx context.Context, addr string, sliceName string, eventType netop.EventType) error
	SendConnectionContext(ctx context.Context, serverAddr string, gw *kubeslicev1beta1.SliceGateway, sliceGwNodePorts []int) error
}

type WorkerGatewayEdgeClientProvider interface {
	UpdateSliceGwServiceMap(ctx context.Context, serverAddr string, gwSvcMap *gatewayedge.SliceGwServiceMap) (*gatewayedge.GwEdgeResponse, error)
}
