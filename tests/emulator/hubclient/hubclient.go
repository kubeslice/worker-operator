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

package hubclient

import (
	"context"

	hubv1alpha1 "github.com/kubeslice/apis/pkg/controller/v1alpha1"
	spokev1alpha1 "github.com/kubeslice/apis/pkg/worker/v1alpha1"
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"

	_ "github.com/stretchr/testify/mock"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type HubClientEmulator struct {
	client.Client
}

func NewHubClientEmulator(client client.Client) (*HubClientEmulator, error) {
	return &HubClientEmulator{
		Client: client,
	}, nil
}

func (hubClientEmulator *HubClientEmulator) UpdateNodePortForSliceGwServer(
	ctx context.Context, sliceGwNodePort []int, sliceGwName string) error {
	return nil
}

func (hubClientEmulator *HubClientEmulator) UpdateServiceExport(
	ctx context.Context, serviceexport *kubeslicev1beta1.ServiceExport) error {
	return nil
}

func (hubClientEmulator *HubClientEmulator) UpdateServiceExportEndpointForIngressGw(ctx context.Context,
	serviceexport *kubeslicev1beta1.ServiceExport, ep *kubeslicev1beta1.ServicePod) error {
	return nil
}

func (hubClientEmulator *HubClientEmulator) DeleteServiceExport(
	ctx context.Context, serviceexport *kubeslicev1beta1.ServiceExport) error {
	return nil
}

func (hubClientEmulator *HubClientEmulator) UpdateAppNamespaces(ctx context.Context, sliceConfigName string, onboardedNamespaces []string) error {
	return nil
}

func (hubClientEmulator *HubClientEmulator) UpdateAppPodsList(
	ctx context.Context,
	sliceConfigName string,
	appPods []kubeslicev1beta1.AppPod,
) error {
	return nil
}
func (hubClientEmulator *HubClientEmulator) UpdateAppNamesapces(ctx context.Context, sliceConfigName string, onboardedNamespaces []string) error {
	return nil
}

func (hubClientEmulator *HubClientEmulator) GetClusterNodeIP(ctx context.Context, clusterName, namespace string) ([]string, error) {
	return []string{"35.235.10.1"}, nil
}

func (hubClientEmulator *HubClientEmulator) CreateWorkerSliceGwRecycler(ctx context.Context, gwRecyclerName, clientID, serverID, sliceGwServer, sliceGwClient, slice string) error {
	return nil
}

func (hubClientEmulator *HubClientEmulator) DeleteWorkerSliceGwRecycler(ctx context.Context, gwRecyclerName string) error {
	return nil
}

func (hubClientEmulator *HubClientEmulator) ListWorkerSliceGwRecycler(ctx context.Context, sliceGWName string) ([]spokev1alpha1.WorkerSliceGwRecycler, error) {
	return nil, nil
}

func (hubClientEmulator *HubClientEmulator) GetVPNKeyRotation(ctx context.Context, rotationName string) (*hubv1alpha1.VpnKeyRotation, error) {
	return nil, nil
}

func (hubClientEmulator *HubClientEmulator) UpdateLBIPsForSliceGwServer(
	ctx context.Context, lbIPs []string, sliceGwName string) error {
	return nil
}

func (hubClientEmulator *HubClientEmulator) GetClusterNamespaceConfig(ctx context.Context, clusterName string) (map[string]string, map[string]string, error) {
	return nil, nil, nil
}
