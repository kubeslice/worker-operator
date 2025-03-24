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
	"fmt"
	"os"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hubv1alpha1 "github.com/kubeslice/apis/pkg/controller/v1alpha1"
	spokev1alpha1 "github.com/kubeslice/apis/pkg/worker/v1alpha1"
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	hubutils "github.com/kubeslice/worker-operator/pkg/hub"
	"github.com/kubeslice/worker-operator/pkg/logger"
	"github.com/kubeslice/worker-operator/pkg/monitoring"
)

const (
	GCP   string = "gcp"
	AWS   string = "aws"
	AZURE string = "azure"
)

var scheme = runtime.NewScheme()
var log = logger.NewWrappedLogger().WithValues("type", "hub")
var toFilter = []string{"kubeslice-", "kubernetes.io"}

func init() {
	clientgoscheme.AddToScheme(scheme)
	utilruntime.Must(spokev1alpha1.AddToScheme(scheme))
	utilruntime.Must(hubv1alpha1.AddToScheme(scheme))
	utilruntime.Must(kubeslicev1beta1.AddToScheme(scheme))
}

type HubClientConfig struct {
	client.Client
	eventRecorder *monitoring.EventRecorder
}

type HubClientRpc interface {
	UpdateNodePortForSliceGwServer(ctx context.Context, sliceGwNodePort int32, sliceGwName string) error
	UpdateServiceExport(ctx context.Context, serviceexport *kubeslicev1beta1.ServiceExport) error
	UpdateServiceExportEndpointForIngressGw(ctx context.Context, serviceexport *kubeslicev1beta1.ServiceExport,
		ep *kubeslicev1beta1.ServicePod) error
	UpdateAppNamespaces(ctx context.Context, sliceConfigName string, onboardedNamespaces []string) error
	CreateWorkerSliceGwRecycler(ctx context.Context, gwRecyclerName, clientID, serverID, sliceGwServer, sliceGwClient, slice string) error
	DeleteWorkerSliceGwRecycler(ctx context.Context, gwRecyclerName string) error
	UpdateLBIPsForSliceGwServer(ctx context.Context, lbIP []string, sliceGwName string) error
}

func NewHubClientConfig(er *monitoring.EventRecorder) (*HubClientConfig, error) {
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
		Client:        hubClient,
		eventRecorder: er,
	}, err
}

func (hubClient *HubClientConfig) CreateWorkerSliceGwRecycler(ctx context.Context, gwRecyclerName, clientID, serverID, sliceGwServer, sliceGwClient, slice string) error {
	var workerslicegwrecycler spokev1alpha1.WorkerSliceGwRecycler
	err := hubClient.Get(ctx, types.NamespacedName{
		Name:      gwRecyclerName,
		Namespace: ProjectNamespace,
	}, &workerslicegwrecycler)
	if err == nil {
		// The object is already created. Return from here
		return nil
	}

	workerslicegwrecycler = spokev1alpha1.WorkerSliceGwRecycler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gwRecyclerName,
			Namespace: ProjectNamespace,
			Labels: map[string]string{
				"slice_name":   slice,
				"slicegw_name": sliceGwServer,
			},
		},
		Spec: spokev1alpha1.WorkerSliceGwRecyclerSpec{
			GwPair: spokev1alpha1.GwPair{
				ServerID: serverID,
				ClientID: clientID,
			},
			State:         "init",
			Request:       "verify_new_deployment_created",
			SliceGwServer: sliceGwServer,
			SliceGwClient: sliceGwClient,
			SliceName:     slice,
		},
	}
	return hubClient.Create(ctx, &workerslicegwrecycler)
}

func (hubClient *HubClientConfig) DeleteWorkerSliceGwRecycler(ctx context.Context, gwRecyclerName string) error {
	var workerslicegwrecycler spokev1alpha1.WorkerSliceGwRecycler
	err := hubClient.Get(ctx, types.NamespacedName{
		Name:      gwRecyclerName,
		Namespace: ProjectNamespace,
	}, &workerslicegwrecycler)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		// The object is already created. Return from here
		return err
	}

	return hubClient.Delete(ctx, &workerslicegwrecycler)
}

func (hubClient *HubClientConfig) ListWorkerSliceGwRecycler(ctx context.Context, sliceGWName string) ([]spokev1alpha1.WorkerSliceGwRecycler, error) {
	workerslicegwrecycler := spokev1alpha1.WorkerSliceGwRecyclerList{}
	labels := map[string]string{"slicegw_name": sliceGWName}
	listOpts := []client.ListOption{
		client.MatchingLabels(labels),
		client.InNamespace(ProjectNamespace),
	}
	err := hubClient.List(ctx, &workerslicegwrecycler, listOpts...)
	if err != nil {
		return nil, err
	}
	return workerslicegwrecycler.Items, nil
}

func (hubClient *HubClientConfig) UpdateNodePortForSliceGwServer(ctx context.Context, sliceGwNodePorts []int, sliceGwName string) error {
	sliceGw := &spokev1alpha1.WorkerSliceGateway{}
	err := hubClient.Get(ctx, types.NamespacedName{
		Name:      sliceGwName,
		Namespace: ProjectNamespace,
	}, sliceGw)
	if err != nil {
		return err
	}

	// If the node ports on the controller cluster and the worker are the same, no need to update
	if hubutils.ListEqual(sliceGw.Spec.LocalGatewayConfig.NodePorts, sliceGwNodePorts) {
		return nil
	}

	sliceGw.Spec.LocalGatewayConfig.NodePorts = sliceGwNodePorts

	err = hubClient.Update(ctx, sliceGw)
	hubClient.eventRecorder.RecordEvent(ctx, &monitoring.Event{
		EventType:         monitoring.EventTypeNormal,
		Reason:            monitoring.EventReasonNodePortUpdate,
		Message:           fmt.Sprintf("NodePorts Updated to: %d", sliceGwNodePorts),
		ReportingInstance: "Controller Reconciler",
		Object:            sliceGw,
		Action:            "NodePortUpdated",
	})

	return err
}

func contains(i []string, o string) bool {
	for _, v := range i {
		if v == o {
			return true
		}
	}
	return false
}

func (hubClient *HubClientConfig) UpdateLBIPsForSliceGwServer(ctx context.Context, lbIPs []string, sliceGwName string) error {
	sliceGw := &spokev1alpha1.WorkerSliceGateway{}
	err := hubClient.Get(ctx, types.NamespacedName{
		Name:      sliceGwName,
		Namespace: ProjectNamespace,
	}, sliceGw)
	if err != nil {
		return err
	}

	updateNeeded := false
	if len(sliceGw.Spec.LocalGatewayConfig.LoadBalancerIps) == len(lbIPs) {
		for _, lbIP := range lbIPs {
			if !contains(sliceGw.Spec.LocalGatewayConfig.LoadBalancerIps, lbIP) {
				updateNeeded = true
				break
			}
		}
	} else {
		updateNeeded = true
	}

	if updateNeeded {
		sliceGw.Spec.LocalGatewayConfig.LoadBalancerIps = lbIPs
		return hubClient.Update(ctx, sliceGw)
	}

	return nil
}

func (hubClient *HubClientConfig) GetClusterNodeIP(ctx context.Context, clusterName, namespace string) ([]string, error) {
	cluster := &hubv1alpha1.Cluster{}
	err := hubClient.Get(ctx, types.NamespacedName{
		Name:      clusterName,
		Namespace: namespace,
	}, cluster)
	if err != nil {
		return []string{""}, err
	}
	return cluster.Status.NodeIPs, nil
}

func (hubClient *HubClientConfig) GetVPNKeyRotation(ctx context.Context, rotationName string) (*hubv1alpha1.VpnKeyRotation, error) {
	vpnKeyRotation := &hubv1alpha1.VpnKeyRotation{}
	err := hubClient.Get(ctx, types.NamespacedName{
		Name:      rotationName,
		Namespace: ProjectNamespace,
	}, vpnKeyRotation)
	if err != nil {
		return nil, err
	}
	return vpnKeyRotation, nil
}

func UpdateNamespaceInfoToHub(ctx context.Context, hubClient client.Client, onboardNamespace, sliceName string) error {
	hubCluster := &hubv1alpha1.Cluster{}
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		err := hubClient.Get(ctx, types.NamespacedName{
			Name:      os.Getenv("CLUSTER_NAME"),
			Namespace: os.Getenv("HUB_PROJECT_NAMESPACE"),
		}, hubCluster)
		if err != nil {
			return err
		}
		nsIndex, nsInfo := getNamespaceInfo(onboardNamespace, hubCluster.Status.Namespaces)
		if nsInfo != nil && nsInfo.SliceName != sliceName {
			// update the existing namespace
			hubCluster.Status.Namespaces[nsIndex] = hubv1alpha1.NamespacesConfig{
				Name:      onboardNamespace,
				SliceName: sliceName,
			}
			log.Info("Updating namespace on hub cluster", "cluster", ClusterName, "namespace", onboardNamespace)
		} else if nsIndex == -1 {
			hubCluster.Status.Namespaces = append(hubCluster.Status.Namespaces, hubv1alpha1.NamespacesConfig{
				Name:      onboardNamespace,
				SliceName: sliceName,
			})
			log.Info("Adding namespace on hub cluster", "cluster", ClusterName, "namespace", onboardNamespace)
		}
		return hubClient.Status().Update(ctx, hubCluster)
	})
	if err != nil {
		return err
	}
	log.Info("Successfully update cluster namespace", "namespace", onboardNamespace)
	return nil
}

// gets the index of worker namespace from hub cluster CR array
func indexOf(onboardNamespace string, ns []hubv1alpha1.NamespacesConfig) int {
	for k, v := range ns {
		if onboardNamespace == v.Name {
			return k
		}
	}
	return -1
}

func DeleteNamespaceInfoFromHub(ctx context.Context, hubClient client.Client, onboardNamespace string) error {
	hubCluster := &hubv1alpha1.Cluster{}
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		err := hubClient.Get(ctx, types.NamespacedName{
			Name:      os.Getenv("CLUSTER_NAME"),
			Namespace: os.Getenv("HUB_PROJECT_NAMESPACE"),
		}, hubCluster)
		if err != nil {
			return err
		}
		toDeleteNs := indexOf(onboardNamespace, hubCluster.Status.Namespaces)
		if toDeleteNs == -1 {
			return nil
		}
		log.Info("Deleting namespace on hub cluster", "cluster", ClusterName, "namespace", onboardNamespace)
		hubCluster.Status.Namespaces = append(hubCluster.Status.Namespaces[:toDeleteNs],
			hubCluster.Status.Namespaces[toDeleteNs+1:]...)
		return hubClient.Status().Update(ctx, hubCluster)
	})
	if err != nil {
		return err
	}
	log.Info("Successfully update cluster namespace", "namespace", onboardNamespace)
	return nil
}

// gets the namespace info along with the index of worker namespace from hub cluster CR array
// needs when we need to update the ns info if slice changes
func getNamespaceInfo(onboardNamespace string, ns []hubv1alpha1.NamespacesConfig) (int, *hubv1alpha1.NamespacesConfig) {
	for k, v := range ns {
		if onboardNamespace == v.Name {
			return k, &v
		}
	}
	return -1, nil
}

func getHubServiceDiscoveryEps(serviceexport *kubeslicev1beta1.ServiceExport) []hubv1alpha1.ServiceDiscoveryEndpoint {
	epList := []hubv1alpha1.ServiceDiscoveryEndpoint{}

	port := serviceexport.Spec.Ports[0].ContainerPort

	for _, pod := range serviceexport.Status.Pods {
		ep := hubv1alpha1.ServiceDiscoveryEndpoint{
			PodName: pod.Name,
			Cluster: ClusterName,
			NsmIp:   pod.NsmIP,
			DnsName: pod.DNSName,
			Port:    port,
		}
		epList = append(epList, ep)
	}

	return epList
}

func getHubServiceDiscoveryPorts(serviceexport *kubeslicev1beta1.ServiceExport) []hubv1alpha1.ServiceDiscoveryPort {
	portList := []hubv1alpha1.ServiceDiscoveryPort{}
	for _, port := range serviceexport.Spec.Ports {
		portList = append(portList, hubv1alpha1.ServiceDiscoveryPort{
			Name:            port.Name,
			Port:            port.ContainerPort,
			Protocol:        string(port.Protocol),
			ServicePort:     port.ServicePort,
			ServiceProtocol: string(port.ServiceProtocol),
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
			Aliases:                   serviceexport.Spec.Aliases,
		},
	}
}

func getHubServiceDiscoveryEpForIngressGw(ep *kubeslicev1beta1.ServicePod) hubv1alpha1.ServiceDiscoveryEndpoint {
	return hubv1alpha1.ServiceDiscoveryEndpoint{
		PodName: ep.Name,
		Cluster: ClusterName,
		NsmIp:   ep.NsmIP,
		DnsName: ep.DNSName,
		Port:    8080,
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
		if apierrors.IsNotFound(err) {
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
					ServiceDiscoveryEndpoints: []hubv1alpha1.ServiceDiscoveryEndpoint{getHubServiceDiscoveryEpForIngressGw(ep)},
					ServiceDiscoveryPorts:     getHubServiceDiscoveryPorts(serviceexport),
					Aliases:                   serviceexport.Spec.Aliases,
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

	hubSvcEx.Spec.ServiceDiscoveryEndpoints = []hubv1alpha1.ServiceDiscoveryEndpoint{getHubServiceDiscoveryEpForIngressGw(ep)}
	hubSvcEx.Spec.ServiceDiscoveryPorts = getHubServiceDiscoveryPorts(serviceexport)
	hubSvcEx.Spec.Aliases = serviceexport.Spec.Aliases

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
		if apierrors.IsNotFound(err) {
			err = hubClient.Create(ctx, getHubServiceExportObj(serviceexport))
			if err != nil {
				return err
			}
			return nil
		}
		return err
	}

	hubSvcEx.Spec = getHubServiceExportObj(serviceexport).Spec

	log.WithValues("serviceexport", serviceexport.Name).Info("Updated serviceexport on hub", "spec", hubSvcEx.Spec)

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
		if apierrors.IsNotFound(err) {
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

func partialContains(slice []string, str string) bool {
	for _, s := range slice {
		if strings.Contains(str, s) {
			return true
		}
	}
	return false
}

func filterLabelsAndAnnotations(data map[string]string) map[string]string {
	filtered := make(map[string]string)
	for key, value := range data {
		// Skip if `key` contains any substring in `toFilter`
		if !partialContains(toFilter, key) {
			filtered[key] = value
		}
	}
	return filtered
}

func (hubclient *HubClientConfig) GetClusterNamespaceConfig(ctx context.Context, clusterName string) (map[string]string, map[string]string, error) {
	cluster := &hubv1alpha1.Cluster{}
	err := hubclient.Get(ctx, types.NamespacedName{Name: clusterName, Namespace: ProjectNamespace}, cluster)
	if err != nil {
		return nil, nil, err
	}
	return filterLabelsAndAnnotations(cluster.GetLabels()), filterLabelsAndAnnotations(cluster.GetAnnotations()), nil
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
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
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
	})
	return err
}
