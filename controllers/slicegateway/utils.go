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
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	gwsidecarpb "github.com/kubeslice/gateway-sidecar/pkg/sidecar/sidecarpb"
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	"github.com/kubeslice/worker-operator/controllers"
	ossEvents "github.com/kubeslice/worker-operator/events"
	"github.com/kubeslice/worker-operator/pkg/utils"
	webhook "github.com/kubeslice/worker-operator/pkg/webhook/pod"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func isClient(sliceGw *kubeslicev1beta1.SliceGateway) bool {
	return sliceGw.Status.Config.SliceGatewayHostType == "Client"
}
func isServer(sliceGw *kubeslicev1beta1.SliceGateway) bool {
	return sliceGw.Status.Config.SliceGatewayHostType == "Server"
}
func canDeployGw(sliceGw *kubeslicev1beta1.SliceGateway) bool {
	return sliceGw.Status.Config.SliceGatewayHostType == "Server" || readyToDeployGwClient(sliceGw)
}
func readyToDeployGwClient(sliceGw *kubeslicev1beta1.SliceGateway) bool {
	if sliceGw.Status.Config.SliceGatewayConnectivityType == "LoadBalancer" || os.Getenv("ENABLE_GW_LB_EDGE") != "" {
		return len(sliceGw.Status.Config.SliceGatewayServerLBIPs) > 0 || os.Getenv("GW_LB_IP") != ""
	}

	return len(sliceGw.Status.Config.SliceGatewayRemoteNodeIPs) > 0 && len(sliceGw.Status.Config.SliceGatewayRemoteNodePorts) != 0 && sliceGw.Status.Config.SliceGatewayRemoteGatewayID != ""
}

func getPodType(labels map[string]string) string {
	podType, found := labels[webhook.PodInjectLabelKey]
	if found {
		return podType
	}

	nsmLabel, found := labels["app"]
	if found {
		if nsmLabel == "nsmgr-daemonset" || nsmLabel == "nsm-kernel-plane" {
			return "nsm"
		}
	}

	return ""
}

func getGwSvcNameFromDepName(depName string) string {
	return "svc-" + depName
}

func contains[T comparable](s []T, e T) bool {
	for _, element := range s {
		if element == e {
			return true
		}
	}
	return false
}

func containsWithIndex[T comparable](s []T, e T) (bool, int) {
	for index, element := range s {
		if element == e {
			return true, index
		}
	}
	return false, 0
}

func getPodIPs(slicegateway *kubeslicev1beta1.SliceGateway) []string {
	podIPs := make([]string, 0)
	for i := range slicegateway.Status.GatewayPodStatus {
		podIPs = append(podIPs, slicegateway.Status.GatewayPodStatus[i].PodIP)
	}
	return podIPs
}

func getPodNames(slicegateway *kubeslicev1beta1.SliceGateway) []string {
	podNames := make([]string, 0)
	for i := range slicegateway.Status.GatewayPodStatus {
		podNames = append(podNames, slicegateway.Status.GatewayPodStatus[i].PodName)
	}
	return podNames
}

func GetDepNameFromPodName(sliceGwID, podName string) string {
	after, found := strings.CutPrefix(podName, sliceGwID)
	if !found {
		return ""
	}
	l := strings.Split(after, "-")
	if len(l) < len([]string{"emptyString", "gwInstance", "depInstance"}) {
		return ""
	}

	return sliceGwID + "-" + l[1] + "-" + l[2]
}

func isGWPodStatusChanged(slicegateway *kubeslicev1beta1.SliceGateway, gwPod *kubeslicev1beta1.GwPodInfo) bool {
	gwPodStatus := slicegateway.Status.GatewayPodStatus
	for _, gw := range gwPodStatus {
		if gw.PodName == gwPod.PodName {
			return gw.TunnelStatus.Status == gwPod.TunnelStatus.Status && gw.PeerPodName == gwPod.PeerPodName
		}
	}
	return false
}

func getPodAntiAffinity(slice, slicegateway string) *corev1.PodAntiAffinity {
	return &corev1.PodAntiAffinity{
		PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{{
			Weight: 100,
			PodAffinityTerm: corev1.PodAffinityTerm{
				TopologyKey: "kubernetes.io/hostname",
				LabelSelector: &metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{{
						Key:      controllers.SliceGatewaySelectorLabelKey,
						Operator: metav1.LabelSelectorOpIn,
						Values:   []string{slicegateway},
					}, {
						Key:      controllers.ApplicationNamespaceSelectorLabelKey,
						Operator: metav1.LabelSelectorOpIn,
						Values:   []string{slice},
					}},
				},
			},
		}},
	}
}

func getLocalNSMIPs(slicegateway *kubeslicev1beta1.SliceGateway) []string {
	nsmIPs := make([]string, 0)
	for i := range slicegateway.Status.GatewayPodStatus {
		nsmIPs = append(nsmIPs, slicegateway.Status.GatewayPodStatus[i].LocalNsmIP)
	}
	return nsmIPs
}

func isPodPresentInPodList(podList *corev1.PodList, podName string) bool {
	for _, pod := range podList.Items {
		if pod.Name == podName {
			return true
		}
	}

	return false
}

// findGwPodInfo finds existing GwPodInfo by pod name in the gateway status.
func findGwPodInfo(gwPodStatus []*kubeslicev1beta1.GwPodInfo, podName string) *kubeslicev1beta1.GwPodInfo {
	for _, gwPod := range gwPodStatus {
		if gwPod != nil && gwPod.PodName == podName {
			return gwPod
		}
	}
	return nil
}

// ValidateGatewayPodReadiness checks if a gateway pod is ready for FSM trigger.
// This includes validating:
// 1. Pod exists in gateway status
// 2. Tunnel is UP and operational
// 3. TUN interface is created and configured
// 4. Peer pod information is available
//
// Returns an error with specific message patterns that callers can check
// to determine if the error is transient (should retry).
func ValidateGatewayPodReadiness(sliceGw *kubeslicev1beta1.SliceGateway, podName string) error {
	if sliceGw == nil {
		return fmt.Errorf("sliceGw is nil")
	}

	// Check 1: Find pod info in status
	podInfo := findGwPodInfo(sliceGw.Status.GatewayPodStatus, podName)
	if podInfo == nil {
		return fmt.Errorf("pod %s not found in gateway status", podName)
	}

	// Check 2: Validate tunnel status is UP
	if podInfo.TunnelStatus.Status != int32(gwsidecarpb.TunnelStatusType_GW_TUNNEL_STATE_UP) {
		return fmt.Errorf("tunnel not up for pod %s, current status: %d", podName, podInfo.TunnelStatus.Status)
	}

	// Check 3: Validate TUN interface is configured
	if podInfo.TunnelStatus.IntfName == "" {
		return fmt.Errorf("tunnel interface name not set for pod %s", podName)
	}

	// Check 4: Validate tunnel has local and remote IPs
	if podInfo.TunnelStatus.LocalIP == "" || podInfo.TunnelStatus.RemoteIP == "" {
		return fmt.Errorf("tunnel IPs not configured for pod %s (local: %s, remote: %s)",
			podName, podInfo.TunnelStatus.LocalIP, podInfo.TunnelStatus.RemoteIP)
	}

	// Check 5: Validate peer pod information is available
	if podInfo.PeerPodName == "" {
		return fmt.Errorf("peer pod name not available for pod %s", podName)
	}

	// All checks passed - pod is ready
	return nil
}

func getPodPairToRebalance(podsOnNode []corev1.Pod, sliceGw *kubeslicev1beta1.SliceGateway) (string, string) {
	for _, pod := range podsOnNode {
		podInfo := findGwPodInfo(sliceGw.Status.GatewayPodStatus, pod.Name)
		if podInfo == nil {
			continue
		}
		if podInfo.TunnelStatus.Status != int32(gwsidecarpb.TunnelStatusType_GW_TUNNEL_STATE_UP) {
			continue
		}
		if podInfo.PeerPodName != "" {
			return podInfo.PodName, podInfo.PeerPodName
		}
	}

	return "", ""
}

func GetPeerGwPodName(gwPodName string, sliceGw *kubeslicev1beta1.SliceGateway) (string, error) {
	podInfo := findGwPodInfo(sliceGw.Status.GatewayPodStatus, gwPodName)
	if podInfo == nil {
		return "", fmt.Errorf("gw pod %s not found in status", gwPodName)
	}
	if podInfo.TunnelStatus.Status != int32(gwsidecarpb.TunnelStatusType_GW_TUNNEL_STATE_UP) {
		return "", fmt.Errorf("gw tunnel is down for pod %s, current status: %d", gwPodName, podInfo.TunnelStatus.Status)
	}
	if podInfo.PeerPodName == "" {
		return "", fmt.Errorf("gw peer pod info unavailable for pod %s", gwPodName)
	}

	return podInfo.PeerPodName, nil
}

func gwDeploymentIsPresent(sliceGwName string, gwInstance int, deployments *appsv1.DeploymentList) bool {
	for _, deployment := range deployments.Items {
		if deployment.Name == sliceGwName+"-"+fmt.Sprint(gwInstance)+"-"+"0" ||
			deployment.Name == sliceGwName+"-"+fmt.Sprint(gwInstance)+"-"+"1" {
			return true
		}
	}

	return false
}

func getGwDeployment(ctx context.Context, c client.Client, sliceGw *kubeslicev1beta1.SliceGateway, depName string) *appsv1.Deployment {
	deployments, err := GetDeployments(ctx, c, sliceGw.Spec.SliceName, sliceGw.Name)
	if err != nil {
		return nil
	}

	for _, deployment := range deployments.Items {
		if deployment.Name == depName {
			return &deployment
		}
	}

	return nil
}

func gwServiceIsPresent(sliceGwName string, gwInstance int, services *corev1.ServiceList) bool {
	for _, service := range services.Items {
		if service.Name == "svc"+"-"+sliceGwName+"-"+fmt.Sprint(gwInstance)+"-"+"0" ||
			service.Name == "svc"+"-"+sliceGwName+"-"+fmt.Sprint(gwInstance)+"-"+"1" {
			return true
		}
	}

	return false
}

func getGwService(services *corev1.ServiceList, svcName string) *corev1.Service {
	for _, svc := range services.Items {
		if svc.Name == svcName {
			return &svc
		}
	}

	return nil
}

func checkIfNodePortIsAlreadyUsed(nodePort int) bool {
	found := false
	gwClientToRemotePortMap.Range(func(key, value interface{}) bool {
		if value.(int) == nodePort {
			found = true
			return false
		}

		return true
	})

	return found
}

func checkIfNodePortIsValid(nodePortList []int, nodePort int) bool {
	for _, nodePortVal := range nodePortList {
		if nodePortVal == nodePort {
			return true
		}
	}

	return false
}

func GetDeployments(ctx context.Context, c client.Client, sliceName, sliceGwName string) (*appsv1.DeploymentList, error) {
	listOpts := []client.ListOption{
		client.MatchingLabels(map[string]string{
			controllers.ApplicationNamespaceSelectorLabelKey: sliceName,
			webhook.PodInjectLabelKey:                        "slicegateway",
			"kubeslice.io/slicegw":                           sliceGwName,
		}),
		client.InNamespace(controllers.ControlPlaneNamespace),
	}
	deployList := appsv1.DeploymentList{}
	if err := c.List(ctx, &deployList, listOpts...); err != nil {
		return nil, err
	}
	return &deployList, nil
}

func getDeploymentsMarkedForDeletion(ctx context.Context, c client.Client, sliceName, sliceGwName string) (*appsv1.DeploymentList, error) {
	listOpts := []client.ListOption{
		client.MatchingLabels(map[string]string{
			controllers.ApplicationNamespaceSelectorLabelKey: sliceName,
			webhook.PodInjectLabelKey:                        "slicegateway",
			"kubeslice.io/slicegw":                           sliceGwName,
			"kubeslice.io/marked-for-deletion":               "true",
		}),
		client.InNamespace(controllers.ControlPlaneNamespace),
	}
	deployList := appsv1.DeploymentList{}
	if err := c.List(ctx, &deployList, listOpts...); err != nil {
		return nil, err
	}
	return &deployList, nil
}

func getClientGwRemotePortInUse(ctx context.Context, c client.Client, sliceGw *kubeslicev1beta1.SliceGateway, depName string) (bool, int) {
	deployment := getGwDeployment(ctx, c, sliceGw, depName)
	if deployment == nil {
		return false, -1
	}
	for _, container := range deployment.Spec.Template.Spec.Containers {
		if container.Name == "kubeslice-sidecar" {
			for _, envVar := range container.Env {
				if envVar.Name == "NODE_PORT" {
					nodePort, err := strconv.Atoi(envVar.Value)
					if err == nil {
						return true, nodePort
					}
				}
			}
		}
	}

	return false, -1
}

func GetPodForGwDeployment(ctx context.Context, c client.Client, depName string) (*corev1.Pod, error) {
	listOpts := []client.ListOption{
		client.MatchingLabels(map[string]string{
			"kubeslice.io/slice-gw-dep": depName,
		}),
		client.InNamespace(controllers.ControlPlaneNamespace),
	}
	podList := corev1.PodList{}
	if err := c.List(ctx, &podList, listOpts...); err != nil {
		return nil, err
	}

	return &podList.Items[0], nil
}

func GetNsmIPsForGwDeployment(ctx context.Context, c client.Client, sliceGwName, depName string) ([]string, error) {
	sliceGw := kubeslicev1beta1.SliceGateway{}
	if err := c.Get(ctx, types.NamespacedName{Namespace: controllers.ControlPlaneNamespace, Name: sliceGwName}, &sliceGw); err != nil {
		return nil, err
	}

	listOpts := []client.ListOption{
		client.MatchingLabels(map[string]string{
			"kubeslice.io/slice-gw-dep": depName,
		}),
		client.InNamespace(controllers.ControlPlaneNamespace),
	}
	podList := corev1.PodList{}
	if err := c.List(ctx, &podList, listOpts...); err != nil {
		return nil, err
	}

	nsmIPs := []string{}

	for _, pod := range podList.Items {
		for _, podStatus := range sliceGw.Status.GatewayPodStatus {
			if podStatus.PodName == pod.Name {
				if podStatus.LocalNsmIP != "" {
					nsmIPs = append(nsmIPs, podStatus.LocalNsmIP)
				}
				break
			}
		}
	}

	return nsmIPs, nil
}

func GetRemoteDepName(remoteGwID, localDepName string) string {
	l := strings.Split(localDepName, "-")
	return remoteGwID + "-" + l[len(l)-2] + "-" + l[len(l)-1]
}

func (r *SliceGwReconciler) cleanupSliceGwResources(ctx context.Context, slicegw *kubeslicev1beta1.SliceGateway) error {
	//delete gateway secret
	listOpts := []client.ListOption{
		client.InNamespace(slicegw.Namespace),
		client.MatchingLabels{"kubeslice.io/slice-gw": slicegw.Name},
	}
	secretList := &corev1.SecretList{}
	if err := r.List(ctx, secretList, listOpts...); err != nil {
		r.Log.Error(err, "Failed to list gateway secrets")
		return err
	}
	for _, v := range secretList.Items {
		meshSliceGwCerts := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      v.Name,
				Namespace: controllers.ControlPlaneNamespace,
			},
		}
		if err := r.Delete(ctx, meshSliceGwCerts); err != nil {
			utils.RecordEvent(ctx, r.EventRecorder, slicegw, nil, ossEvents.EventSliceGWSecretDeletionFailed, controllerName)
			r.Log.Error(err, "Error Deleting Gateway Secret while cleaning up.. Please Delete it before installing slice again")
			return err
		}
	}

	// Delete deployment to node port map
	deployments, err := GetDeployments(ctx, r.Client, slicegw.Spec.SliceName, slicegw.Name)
	if err != nil {
		r.Log.Error(err, "SliceGw Deletion: Failed to list deployments")
		return err
	}

	for _, deployment := range deployments.Items {
		port, found := gwClientToRemotePortMap.LoadAndDelete(deployment.Name)
		if found {
			r.Log.Info("SliceGw Deletion: Invalidated deployment to node port mapping", "depName", deployment.Name, "port", port)
		}
	}

	return nil
}

// getOVPNClientContainerArgs returns the args needed for the ovpn client deployment container
func getOVPNClientContainerArgs(remotePortNumber int, g *kubeslicev1beta1.SliceGateway) []string {
	args := []string{
		"/vpnclient/" + vpnClientFileName,
		"90",
		"openvpn",
		"--remote",
		g.Status.Config.SliceGatewayRemoteGatewayID,
		"--port",
		strconv.Itoa(remotePortNumber),
		"--ping-restart",
		"15",
		"--proto",
		strings.ToLower(g.Status.Config.SliceGatewayProtocol),
		"--txqueuelen",
		"5000",
		"--config",
		"/vpnclient/" + vpnClientFileName,
	}
	return args
}

// a helper to assign distinct port to each client deployment
func allocateNodePortToClient(correctNodePorts []int, depName string, nodePortsMap *sync.Map) (int, error) {
	nodePortsMap.Range(func(k, v interface{}) bool {
		if ok, index := containsWithIndex(correctNodePorts, v.(int)); ok {
			correctNodePorts = append(correctNodePorts[:index], correctNodePorts[index+1:]...)
		}
		return true
	})
	if len(correctNodePorts) > 0 {
		return correctNodePorts[0], nil
	} else {
		port, ok := nodePortsMap.Load(depName)
		if ok {
			return port.(int), nil
		}
		return 0, errors.New("could not allocate a port")
	}
}
