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
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	_ "github.com/stretchr/testify/mock"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
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

func (hubClientEmulator *HubClientEmulator) UpdateNodeIpInCluster(ctx context.Context, clusterName, namespace string, nodeIPs []string) error {
	cluster := &hubv1alpha1.Cluster{}
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		err := hubClientEmulator.Get(ctx, types.NamespacedName{
			Name:      clusterName,
			Namespace: namespace,
		}, cluster)
		if err != nil {
			return err
		}
		cluster.Spec.NodeIPs = nodeIPs
		if err := hubClientEmulator.Update(ctx, cluster); err != nil {
			//log.Error(err, "Error updating to cluster spec on controller cluster")
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
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

func (hubClientEmulator *HubClientEmulator) GetClusterNodeIPs(ctx context.Context, clusterName, namespace string) ([]string, error) {
	return []string{"35.235.10.1"}, nil
}
