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

	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	"github.com/stretchr/testify/mock"
)

type HubClientEmulator struct {
	mock.Mock
}

func NewHubClientEmulator() (*HubClientEmulator, error) {
	return new(HubClientEmulator), nil
}

func (hubClientEmulator *HubClientEmulator) UpdateNodeIpInCluster(ctx context.Context, clusterName, nodeIP, namespace string) error {
	return nil
}

func (hubClientEmulator *HubClientEmulator) UpdateNodePortForSliceGwServer(
	ctx context.Context, sliceGwNodePort int32, sliceGwName string) error {
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

func (hubClientEmulator *HubClientEmulator) GetClusterNodeIP(ctx context.Context, clusterName, namespace string) (string, error) {
	return "", nil
}
