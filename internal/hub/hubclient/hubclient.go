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

package hub

import (
	"context"
	"os"

	"k8s.io/apimachinery/pkg/api/errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hubv1alpha1 "github.com/kubeslice/apis/pkg/controller/v1alpha1"
	spokev1alpha1 "github.com/kubeslice/apis/pkg/worker/v1alpha1"
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	"github.com/kubeslice/worker-operator/internal/cluster"
	"github.com/kubeslice/worker-operator/internal/logger"
)

var scheme = runtime.NewScheme()
var log = logger.NewLogger().WithValues("type", "hub")

func init() {
	clientgoscheme.AddToScheme(scheme)
	utilruntime.Must(spokev1alpha1.AddToScheme(scheme))
	utilruntime.Must(hubv1alpha1.AddToScheme(scheme))
	utilruntime.Must(kubeslicev1beta1.AddToScheme(scheme))
}

type HubClientConfig struct {
	client.Client
}

type HubClientRpc interface {
	UpdateNodePortForSliceGwServer(ctx context.Context, sliceGwNodePort int32, sliceGwName string) error
	UpdateServiceExport(ctx context.Context, serviceexport *kubeslicev1beta1.ServiceExport) error
	UpdateServiceExportEndpointForIngressGw(ctx context.Context, serviceexport *kubeslicev1beta1.ServiceExport,
		ep *kubeslicev1beta1.ServicePod) error
	UpdateAppNamespaces(ctx context.Context, sliceConfigName string, onboardedNamespaces []string) error
}

func NewHubClientConfig() (*HubClientConfig, error) {
	hubClient, err := client.New(&rest.Config{
		Host:            os.Getenv("HUB_HOST_ENDPOINT"),
		BearerTokenFile: HubTokenFile,
		TLSClientConfig: rest.TLSClientConfig{
			CAFile: HubCAFile,
		}},
		client.Options{
			Scheme: scheme,
		},
	)

	return &HubClientConfig{
		Client: hubClient,
	}, err
}

func (hubClient *HubClientConfig) UpdateNodePortForSliceGwServer(ctx context.Context, sliceGwNodePort int32, sliceGwName string) error {
	sliceGw := &spokev1alpha1.WorkerSliceGateway{}
	err := hubClient.Get(ctx, types.NamespacedName{
		Name:      sliceGwName,
		Namespace: ProjectNamespace,
	}, sliceGw)
	if err != nil {
		return err
	}

	if sliceGw.Spec.LocalGatewayConfig.NodePort == int(sliceGwNodePort) {
		// No update needed
		return nil
	}

	sliceGw.Spec.LocalGatewayConfig.NodePort = int(sliceGwNodePort)

	return hubClient.Update(ctx, sliceGw)
}

func PostClusterInfoToHub(ctx context.Context, spokeclient client.Client, hubClient client.Client, clusterName, nodeIP string, namespace string) error {
	err := updateClusterInfoToHub(ctx, spokeclient, hubClient, clusterName, nodeIP, namespace)
	if err != nil {
		log.Error(err, "Error Posting Cluster info to hub cluster")
		return err
	}
	log.Info("Posted cluster info to hub cluster")
	return nil
}

func updateClusterInfoToHub(ctx context.Context, spokeclient client.Client, hubClient client.Client, clusterName, nodeIP string, namespace string) error {
	hubCluster := &hubv1alpha1.Cluster{}
	err := hubClient.Get(ctx, types.NamespacedName{
		Name:      clusterName,
		Namespace: namespace,
	}, hubCluster)
	if err != nil {
		return err
	}

	c := cluster.NewCluster(spokeclient, clusterName)
	//get geographical info
	clusterInfo, err := c.GetClusterInfo(ctx)
	if err != nil {
		log.Error(err, "Error getting clusterInfo")
		return err
	}
	cniSubnet, err := c.GetNsmExcludedPrefix(ctx, "nsm-config", "kubeslice-system")
	if err != nil {
		log.Error(err, "Error getting cni Subnet")
		return err
	}
	log.Info("cniSubnet", "cniSubnet", cniSubnet)
	hubCluster.Spec.ClusterProperty.GeoLocation.CloudRegion = clusterInfo.ClusterProperty.GeoLocation.CloudRegion
	hubCluster.Spec.ClusterProperty.GeoLocation.CloudProvider = clusterInfo.ClusterProperty.GeoLocation.CloudProvider
	hubCluster.Spec.NodeIP = nodeIP
	hubCluster.Status.CniSubnet = cniSubnet
	if err := hubClient.Update(ctx, hubCluster); err != nil {
		log.Error(err, "Error updating to cluster spec on hub cluster")
		return err
	}
	hubCluster.Status.CniSubnet = cniSubnet
	if err := hubClient.Status().Update(ctx, hubCluster); err != nil {
		log.Error(err, "Error updating cniSubnet to cluster status on hub cluster")
		return err
	}
	return nil
}

func getHubServiceDiscoveryEps(serviceexport *kubeslicev1beta1.ServiceExport) []hubv1alpha1.ServiceDiscoveryEndpoint {
	epList := []hubv1alpha1.ServiceDiscoveryEndpoint{}

	for _, pod := range serviceexport.Status.Pods {
		ep := hubv1alpha1.ServiceDiscoveryEndpoint{
			PodName: pod.Name,
			Cluster: ClusterName,
			NsmIp:   pod.NsmIP,
			DnsName: pod.DNSName,
		}
		epList = append(epList, ep)
	}

	return epList
}

func getHubServiceDiscoveryPorts(serviceexport *kubeslicev1beta1.ServiceExport) []hubv1alpha1.ServiceDiscoveryPort {
	portList := []hubv1alpha1.ServiceDiscoveryPort{}
	for _, port := range serviceexport.Spec.Ports {
		portList = append(portList, hubv1alpha1.ServiceDiscoveryPort{
			Name:     port.Name,
			Port:     port.ContainerPort,
			Protocol: string(port.Protocol),
		})
	}

	return portList
}

func getHubServiceExportObjName(serviceexport *kubeslicev1beta1.ServiceExport) string {
	return serviceexport.Name + "-" + serviceexport.ObjectMeta.Namespace + "-" + ClusterName
}

func getHubServiceExportObj(serviceexport *kubeslicev1beta1.ServiceExport) *hubv1alpha1.ServiceExportConfig {
	return &hubv1alpha1.ServiceExportConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getHubServiceExportObjName(serviceexport),
			Namespace: ProjectNamespace,
		},
		Spec: hubv1alpha1.ServiceExportConfigSpec{
			ServiceName:               serviceexport.Name,
			ServiceNamespace:          serviceexport.ObjectMeta.Namespace,
			SourceCluster:             ClusterName,
			SliceName:                 serviceexport.Spec.Slice,
			ServiceDiscoveryEndpoints: getHubServiceDiscoveryEps(serviceexport),
			ServiceDiscoveryPorts:     getHubServiceDiscoveryPorts(serviceexport),
		},
	}
}

func getHubServiceDiscoveryEp(ep *kubeslicev1beta1.ServicePod) hubv1alpha1.ServiceDiscoveryEndpoint {
	return hubv1alpha1.ServiceDiscoveryEndpoint{
		PodName: ep.Name,
		Cluster: ClusterName,
		NsmIp:   ep.NsmIP,
		DnsName: ep.DNSName,
	}
}

func (hubClient *HubClientConfig) UpdateServiceExportEndpointForIngressGw(ctx context.Context, serviceexport *kubeslicev1beta1.ServiceExport,
	ep *kubeslicev1beta1.ServicePod) error {
	hubSvcEx := &hubv1alpha1.ServiceExportConfig{}
	err := hubClient.Get(ctx, types.NamespacedName{
		Name:      getHubServiceExportObjName(serviceexport),
		Namespace: ProjectNamespace,
	}, hubSvcEx)
	if err != nil {
		if errors.IsNotFound(err) {
			hubSvcExObj := &hubv1alpha1.ServiceExportConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      getHubServiceExportObjName(serviceexport),
					Namespace: ProjectNamespace,
				},
				Spec: hubv1alpha1.ServiceExportConfigSpec{
					ServiceName:               serviceexport.Name,
					ServiceNamespace:          serviceexport.ObjectMeta.Namespace,
					SourceCluster:             ClusterName,
					SliceName:                 serviceexport.Spec.Slice,
					ServiceDiscoveryEndpoints: []hubv1alpha1.ServiceDiscoveryEndpoint{getHubServiceDiscoveryEp(ep)},
					ServiceDiscoveryPorts:     getHubServiceDiscoveryPorts(serviceexport),
				},
			}
			err = hubClient.Create(ctx, hubSvcExObj)
			if err != nil {
				return err
			}
			return nil
		}
		return err
	}

	hubSvcEx.Spec.ServiceDiscoveryEndpoints = []hubv1alpha1.ServiceDiscoveryEndpoint{getHubServiceDiscoveryEp(ep)}
	hubSvcEx.Spec.ServiceDiscoveryPorts = getHubServiceDiscoveryPorts(serviceexport)

	err = hubClient.Update(ctx, hubSvcEx)
	if err != nil {
		return err
	}

	return nil
}

func (hubClient *HubClientConfig) UpdateServiceExport(ctx context.Context, serviceexport *kubeslicev1beta1.ServiceExport) error {
	hubSvcEx := &hubv1alpha1.ServiceExportConfig{}
	err := hubClient.Get(ctx, types.NamespacedName{
		Name:      getHubServiceExportObjName(serviceexport),
		Namespace: ProjectNamespace,
	}, hubSvcEx)
	if err != nil {
		if errors.IsNotFound(err) {
			err = hubClient.Create(ctx, getHubServiceExportObj(serviceexport))
			if err != nil {
				return err
			}
			return nil
		}
		return err
	}

	hubSvcEx.Spec = getHubServiceExportObj(serviceexport).Spec

	err = hubClient.Update(ctx, hubSvcEx)
	if err != nil {
		return err
	}

	return nil
}

func (hubClient *HubClientConfig) DeleteServiceExport(ctx context.Context, serviceexport *kubeslicev1beta1.ServiceExport) error {
	hubSvcEx := &hubv1alpha1.ServiceExportConfig{}
	err := hubClient.Get(ctx, types.NamespacedName{
		Name:      getHubServiceExportObjName(serviceexport),
		Namespace: ProjectNamespace,
	}, hubSvcEx)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	err = hubClient.Delete(ctx, hubSvcEx)
	if err != nil {
		return err
	}

	return nil
}

func (hubClient *HubClientConfig) UpdateAppPodsList(ctx context.Context, sliceConfigName string, appPods []kubeslicev1beta1.AppPod) error {
	sliceConfig := &spokev1alpha1.WorkerSliceConfig{}
	err := hubClient.Get(ctx, types.NamespacedName{
		Name:      sliceConfigName,
		Namespace: ProjectNamespace,
	}, sliceConfig)
	if err != nil {
		return err
	}

	sliceConfig.Status.ConnectedAppPods = []spokev1alpha1.AppPod{}
	for _, pod := range appPods {
		sliceConfig.Status.ConnectedAppPods = append(sliceConfig.Status.ConnectedAppPods, spokev1alpha1.AppPod{
			PodName:      pod.PodName,
			PodNamespace: pod.PodNamespace,
			PodIP:        pod.PodIP,
			NsmIP:        pod.NsmIP,
			NsmInterface: pod.NsmInterface,
			NsmPeerIP:    pod.NsmPeerIP,
		})
	}

	return hubClient.Status().Update(ctx, sliceConfig)
}
func (hubClient *HubClientConfig) UpdateAppNamespaces(ctx context.Context, sliceConfigName string, onboardedNamespaces []string) error {
	log.Info("updating onboardedNamespaces to workersliceconfig", "onboardedNamespaces", onboardedNamespaces)
	workerSliceConfig := &spokev1alpha1.WorkerSliceConfig{}
	err := hubClient.Get(ctx, types.NamespacedName{
		Name:      sliceConfigName,
		Namespace: ProjectNamespace,
	}, workerSliceConfig)
	if err != nil {
		return err
	}
	workerSliceConfig.Status.OnboardedAppNamespaces = []spokev1alpha1.NamespaceConfig{}
	o := make([]spokev1alpha1.NamespaceConfig, len(onboardedNamespaces))
	for i, ns := range onboardedNamespaces {
		o[i].Name = ns
	}
	workerSliceConfig.Status.OnboardedAppNamespaces = o
	return hubClient.Status().Update(ctx, workerSliceConfig)
}
