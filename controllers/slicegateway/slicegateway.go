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
	"math"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/kubeslice/worker-operator/controllers"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gwsidecarpb "github.com/kubeslice/gateway-sidecar/pkg/sidecar/sidecarpb"
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	ossEvents "github.com/kubeslice/worker-operator/events"
	"github.com/kubeslice/worker-operator/pkg/cluster"
	"github.com/kubeslice/worker-operator/pkg/gwsidecar"
	"github.com/kubeslice/worker-operator/pkg/logger"
	"github.com/kubeslice/worker-operator/pkg/router"
	"github.com/kubeslice/worker-operator/pkg/utils"
	webhook "github.com/kubeslice/worker-operator/pkg/webhook/pod"
)

var (
	vpnClientFileName        = "openvpn_client.ovpn"
	gwSidecarImage           = os.Getenv("AVESHA_GW_SIDECAR_IMAGE")
	gwSidecarImagePullPolicy = os.Getenv("AVESHA_GW_SIDECAR_IMAGE_PULLPOLICY")

	openVpnServerImage      = os.Getenv("AVESHA_OPENVPN_SERVER_IMAGE")
	openVpnClientImage      = os.Getenv("AVESHA_OPENVPN_CLIENT_IMAGE")
	openVpnServerPullPolicy = os.Getenv("AVESHA_OPENVPN_SERVER_PULLPOLICY")
	openVpnClientPullPolicy = os.Getenv("AVESHA_OPENVPN_CLIENT_PULLPOLICY")
)

// This is a thread-safe Map that contains the mapping of the gw client deployment to the remote node port.
var gwClientToRemotePortMap sync.Map

// labelsForSliceGwDeployment returns the labels for creating slice gw deployment
func labelsForSliceGwDeployment(name, slice, depName string) map[string]string {
	return map[string]string{
		"networkservicemesh.io/app":                      name,
		webhook.PodInjectLabelKey:                        "slicegateway",
		controllers.ApplicationNamespaceSelectorLabelKey: slice,
		controllers.SliceGatewaySelectorLabelKey:         name,
		"kubeslice.io/slice-gw-dep":                      depName,
	}
}

func labelsForSliceGwService(name, svcName, depName string) map[string]string {
	return map[string]string{
		controllers.SliceGatewaySelectorLabelKey: name,
		"kubeslice.io/slice-gw-dep":              depName,
	}
}

func labelsForSliceGwStatus(name string) map[string]string {
	return map[string]string{
		controllers.SliceGatewaySelectorLabelKey: name,
	}
}

// deploymentForGateway returns a gateway Deployment object
func (r *SliceGwReconciler) deploymentForGateway(g *kubeslicev1beta1.SliceGateway, depName string, gwConfigKey int) *appsv1.Deployment {
	if g.Status.Config.SliceGatewayHostType == "Server" {
		return r.deploymentForGatewayServer(g, depName, gwConfigKey)
	} else {
		return r.deploymentForGatewayClient(g, depName, gwConfigKey)
	}
}

func (r *SliceGwReconciler) deploymentForGatewayServer(g *kubeslicev1beta1.SliceGateway, depName string, gwConfigKey int) *appsv1.Deployment {
	ls := labelsForSliceGwDeployment(g.Name, g.Spec.SliceName, depName)

	var replicas int32 = 1

	var vpnSecretDefaultMode int32 = 420
	var vpnFilesRestrictedMode int32 = 0644

	var privileged = true

	sidecarImg := "nexus.dev.aveshalabs.io/kubeslice/gw-sidecar:1.0.0"
	sidecarPullPolicy := corev1.PullAlways
	vpnImg := "nexus.dev.aveshalabs.io/kubeslice/openvpn-server.ubuntu.18.04:1.0.0"
	vpnPullPolicy := corev1.PullAlways
	baseFileName := os.Getenv("CLUSTER_NAME") + "-" + g.Spec.SliceName + "-" + g.Status.Config.SliceGatewayName + ".vpn.aveshasystems.com"

	if len(gwSidecarImage) != 0 {
		sidecarImg = gwSidecarImage
	}

	if len(gwSidecarImagePullPolicy) != 0 {
		sidecarPullPolicy = corev1.PullPolicy(gwSidecarImagePullPolicy)
	}

	if len(openVpnServerImage) != 0 {
		vpnImg = openVpnServerImage
	}

	if len(openVpnServerPullPolicy) != 0 {
		vpnPullPolicy = corev1.PullPolicy(openVpnServerPullPolicy)
	}

	nsmAnnotation := fmt.Sprintf("kernel://vl3-service-%s/nsm0", g.Spec.SliceName)
	selectedNodeIP := ""
	nodeIps := cluster.GetNodeExternalIpList()
	if nodeIps != nil {
		selectedNodeIP = nodeIps[0]
	}
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      depName,
			Namespace: g.Namespace,
			Labels: map[string]string{
				controllers.ApplicationNamespaceSelectorLabelKey: g.Spec.SliceName,
				webhook.PodInjectLabelKey:                        "slicegateway",
				"kubeslice.io/slicegw":                           g.Name,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: ls,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: ls,
					Annotations: map[string]string{
						"prometheus.io/port":    "18080",
						"prometheus.io/scrape":  "true",
						"networkservicemesh.io": nsmAnnotation,
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "vpn-gateway-server",
					Affinity: &corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{{
									MatchExpressions: []corev1.NodeSelectorRequirement{{
										Key:      controllers.NodeTypeSelectorLabelKey,
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{"gateway"},
									}},
								}},
							},
						},
						PodAntiAffinity: getPodAntiAffinity(g.Spec.SliceName, g.Name),
					},
					Containers: []corev1.Container{{
						Name:            "kubeslice-sidecar",
						Image:           sidecarImg,
						ImagePullPolicy: sidecarPullPolicy,
						Env: []corev1.EnvVar{
							{
								Name:  "SLICE_NAME",
								Value: g.Spec.SliceName,
							},
							{
								Name:  "CLUSTER_ID",
								Value: controllers.ClusterName,
							},
							{
								Name:  "REMOTE_CLUSTER_ID",
								Value: g.Status.Config.SliceGatewayRemoteClusterID,
							},
							{
								Name:  "GATEWAY_ID",
								Value: g.Status.Config.SliceGatewayID,
							},
							{
								Name:  "REMOTE_GATEWAY_ID",
								Value: g.Status.Config.SliceGatewayRemoteGatewayID,
							},
							{
								Name:  "POD_TYPE",
								Value: "GATEWAY_POD",
							},
							{
								Name:  "NODE_IP",
								Value: selectedNodeIP,
							},
							{
								Name:  "OPEN_VPN_MODE",
								Value: "SERVER",
							},
							{
								Name:  "MOUNT_PATH",
								Value: "config",
							},
						},
						SecurityContext: &corev1.SecurityContext{
							Privileged:               &privileged,
							AllowPrivilegeEscalation: &privileged,
							Capabilities: &corev1.Capabilities{
								Add: []corev1.Capability{
									"NET_ADMIN",
								},
							},
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "shared-volume",
								MountPath: "/config",
							},
							{
								Name:      "vpn-certs",
								MountPath: "/var/run/vpn",
							},
						},
						Resources: corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								"memory": resource.MustParse("200Mi"),
								"cpu":    resource.MustParse("500m"),
							},
							Requests: corev1.ResourceList{
								"memory": resource.MustParse("50Mi"),
								"cpu":    resource.MustParse("50m"),
							},
						},
					}, {
						Name:            "kubeslice-openvpn-server",
						Image:           vpnImg,
						ImagePullPolicy: vpnPullPolicy,
						Command: []string{
							"/usr/local/bin/waitForConfigToRunCmd.sh",
						},
						Args: []string{
							"/etc/openvpn/openvpn.conf",
							"90",
							"ovpn_run",
						},
						SecurityContext: &corev1.SecurityContext{
							Privileged:               &privileged,
							AllowPrivilegeEscalation: &privileged,
							Capabilities: &corev1.Capabilities{
								Add: []corev1.Capability{
									"NET_ADMIN",
								},
							},
						},
						VolumeMounts: []corev1.VolumeMount{{
							Name:      "shared-volume",
							MountPath: "/etc/openvpn",
						}},
					}},
					Volumes: []corev1.Volume{
						{
							Name: "shared-volume",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
						{
							Name: "vpn-certs",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName:  g.Name + "-" + strconv.Itoa(gwConfigKey), // secret contains the vpn config
									DefaultMode: &vpnSecretDefaultMode,
									Items: []corev1.KeyToPath{
										{
											Key:  "ovpnConfigFile",
											Path: "openvpn.conf",
										}, {
											Key:  "pkiCACertFile",
											Path: "pki/" + "ca.crt",
										}, {
											Key:  "pkiDhPemFile",
											Path: "pki/" + "dh.pem",
										}, {
											Key:  "pkiTAKeyFile",
											Path: "pki/" + baseFileName + "-" + "ta.key",
										}, {
											Key:  "pkiIssuedCertFile",
											Path: "pki/issued/" + baseFileName + ".crt",
											Mode: &vpnFilesRestrictedMode,
										}, {
											Key:  "pkiPrivateKeyFile",
											Path: "pki/private/" + baseFileName + ".key",
											Mode: &vpnFilesRestrictedMode,
										}, {
											Key:  "ccdFile",
											Path: "ccd/" + g.Status.Config.SliceGatewayRemoteGatewayID,
										},
									},
								},
							},
						},
					},
					Tolerations: []corev1.Toleration{{
						Key:      controllers.NodeTypeSelectorLabelKey,
						Operator: "Equal",
						Effect:   "NoSchedule",
						Value:    "gateway",
					}, {
						Key:      controllers.NodeTypeSelectorLabelKey,
						Operator: "Equal",
						Effect:   "NoExecute",
						Value:    "gateway",
					}},
				},
			},
		},
	}

	if len(controllers.ImagePullSecretName) != 0 {
		dep.Spec.Template.Spec.ImagePullSecrets = []corev1.LocalObjectReference{{
			Name: controllers.ImagePullSecretName,
		}}
	}
	// Set SliceGateway instance as the owner and controller
	ctrl.SetControllerReference(g, dep, r.Scheme)
	return dep
}

func (r *SliceGwReconciler) serviceForGateway(g *kubeslicev1beta1.SliceGateway, svcName, depName string) *corev1.Service {
	proto := corev1.ProtocolUDP
	if g.Status.Config.SliceGatewayProtocol == "TCP" {
		proto = corev1.ProtocolTCP
	}
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      svcName,
			Namespace: g.Namespace,
			Labels: map[string]string{
				controllers.ApplicationNamespaceSelectorLabelKey: g.Spec.SliceName,
				"kubeslice.io/slicegw":                           g.Name,
			},
		},
		Spec: corev1.ServiceSpec{
			Type:     "NodePort",
			Selector: labelsForSliceGwService(g.Name, svcName, depName),
			Ports: []corev1.ServicePort{{
				Port:       11194,
				Protocol:   proto,
				TargetPort: intstr.FromInt(11194),
			}},
		},
	}
	//TODO:rework here
	// if len(g.Status.Config.SliceGatewayNodePorts) != 0 {
	// 	svc.Spec.Ports[0].NodePort = int32(g.Status.Config.SliceGatewayNodePort)
	// }
	ctrl.SetControllerReference(g, svc, r.Scheme)
	return svc
}

func (r *SliceGwReconciler) deploymentForGatewayClient(g *kubeslicev1beta1.SliceGateway, depName string, gwConfigKey int) *appsv1.Deployment {
	log := logger.FromContext(context.Background()).WithValues("type", "slicegateway")
	var replicas int32 = 1
	var privileged = true

	var vpnSecretDefaultMode int32 = 0644

	sidecarImg := "nexus.dev.aveshalabs.io/kubeslice/gw-sidecar:1.0.0"
	sidecarPullPolicy := corev1.PullAlways

	vpnImg := "nexus.dev.aveshalabs.io/kubeslice/openvpn-client.alpine.amd64:1.0.0"
	vpnPullPolicy := corev1.PullAlways

	ls := labelsForSliceGwDeployment(g.Name, g.Spec.SliceName, depName)

	if len(gwSidecarImage) != 0 {
		sidecarImg = gwSidecarImage
	}

	if len(gwSidecarImagePullPolicy) != 0 {
		sidecarPullPolicy = corev1.PullPolicy(gwSidecarImagePullPolicy)
	}

	if len(openVpnClientImage) != 0 {
		vpnImg = openVpnClientImage
	}

	if len(openVpnClientPullPolicy) != 0 {
		vpnPullPolicy = corev1.PullPolicy(openVpnClientPullPolicy)
	}

	nsmAnnotation := fmt.Sprintf("kernel://vl3-service-%s/nsm0", g.Spec.SliceName)

	remotePortNumber := 0
	remotePortVal, found := gwClientToRemotePortMap.Load(depName)
	if found {
		remotePortNumber = remotePortVal.(int)
		// Check if cache is valid
		if !checkIfNodePortIsValid(g.Status.Config.SliceGatewayRemoteNodePorts, remotePortNumber) {
			log.Info("Port number mapping is invalid", "depName", depName, "port", remotePortNumber)
			gwClientToRemotePortMap.Delete(depName)
			remotePortNumber = 0
		}
	}

	if !found || remotePortNumber == 0 {
		for _, nodePort := range g.Status.Config.SliceGatewayRemoteNodePorts {
			if !checkIfNodePortIsAlreadyUsed(nodePort) {
				gwClientToRemotePortMap.Store(depName, nodePort)
				log.Info("Storing in map", "depName", depName, "port", nodePort)
				break
			}
		}
		remotePortVal, found = gwClientToRemotePortMap.Load(depName)
		if !found {
			log.Error(errors.New("NodePort Unavailable"), "NodePort Unavailable for deployment", depName)
			return nil
		}
		remotePortNumber = remotePortVal.(int)
	}

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      depName,
			Namespace: g.Namespace,
			Labels: map[string]string{
				controllers.ApplicationNamespaceSelectorLabelKey: g.Spec.SliceName,
				webhook.PodInjectLabelKey:                        "slicegateway",
				"kubeslice.io/slicegw":                           g.Name,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: ls,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: ls,
					Annotations: map[string]string{
						"prometheus.io/port":    "18080",
						"prometheus.io/scrape":  "true",
						"networkservicemesh.io": nsmAnnotation,
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "vpn-gateway-client",
					Affinity: &corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{{
									MatchExpressions: []corev1.NodeSelectorRequirement{{
										Key:      controllers.NodeTypeSelectorLabelKey,
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{"gateway"},
									}},
								}},
							},
						},
						PodAntiAffinity: getPodAntiAffinity(g.Spec.SliceName, g.Name),
					},
					Containers: []corev1.Container{{
						Name:            "kubeslice-sidecar",
						Image:           sidecarImg,
						ImagePullPolicy: sidecarPullPolicy,
						Env: []corev1.EnvVar{
							{
								Name:  "SLICE_NAME",
								Value: g.Spec.SliceName,
							},
							{
								Name:  "REMOTE_CLUSTER_ID",
								Value: g.Status.Config.SliceGatewayRemoteClusterID,
							},
							{
								Name:  "GATEWAY_ID",
								Value: g.Status.Config.SliceGatewayID,
							},
							{
								Name:  "REMOTE_GATEWAY_ID",
								Value: g.Status.Config.SliceGatewayRemoteGatewayID,
							},
							{
								Name:  "POD_TYPE",
								Value: "GATEWAY_POD",
							},
							{
								Name:  "OPEN_VPN_MODE",
								Value: "CLIENT",
							},
							{
								Name:  "GW_LOG_LEVEL",
								Value: os.Getenv("GW_LOG_LEVEL"),
							},
							{
								Name:  "NODE_PORT",
								Value: strconv.Itoa(remotePortNumber),
							},
						},
						SecurityContext: &corev1.SecurityContext{
							Privileged:               &privileged,
							AllowPrivilegeEscalation: &privileged,
							Capabilities: &corev1.Capabilities{
								Add: []corev1.Capability{
									"NET_ADMIN",
								},
							},
						},
						VolumeMounts: []corev1.VolumeMount{{
							Name:      "shared-volume",
							MountPath: "/config",
						}},
						Resources: corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								"memory": resource.MustParse("200Mi"),
								"cpu":    resource.MustParse("500m"),
							},
							Requests: corev1.ResourceList{
								"memory": resource.MustParse("50Mi"),
								"cpu":    resource.MustParse("50m"),
							},
						},
					}, {
						Name:            "kubeslice-openvpn-client",
						Image:           vpnImg,
						ImagePullPolicy: vpnPullPolicy,
						Command: []string{
							"/usr/local/bin/waitForConfigToRunCmd.sh",
						},
						Args: getOVPNClientContainerArgs(remotePortNumber, g),
						SecurityContext: &corev1.SecurityContext{
							Privileged:               &privileged,
							AllowPrivilegeEscalation: &privileged,
							Capabilities: &corev1.Capabilities{
								Add: []corev1.Capability{
									"NET_ADMIN",
								},
							},
						},
						VolumeMounts: []corev1.VolumeMount{{
							Name:      "shared-volume",
							MountPath: "/vpnclient",
						}},
					}},
					Volumes: []corev1.Volume{{
						Name: "shared-volume",
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName:  g.Name + "-" + strconv.Itoa(gwConfigKey),
								DefaultMode: &vpnSecretDefaultMode,
								Items: []corev1.KeyToPath{
									{
										Key:  "ovpnConfigFile",
										Path: "openvpn_client.ovpn",
									},
								},
							},
						},
					}},
					Tolerations: []corev1.Toleration{{
						Key:      controllers.NodeTypeSelectorLabelKey,
						Operator: "Equal",
						Effect:   "NoSchedule",
						Value:    "gateway",
					}, {
						Key:      controllers.NodeTypeSelectorLabelKey,
						Operator: "Equal",
						Effect:   "NoExecute",
						Value:    "gateway",
					}},
				},
			},
		},
	}
	if len(controllers.ImagePullSecretName) != 0 {
		dep.Spec.Template.Spec.ImagePullSecrets = []corev1.LocalObjectReference{{
			Name: controllers.ImagePullSecretName,
		}}
	}

	// Set SliceGateway instance as the owner and controller
	ctrl.SetControllerReference(g, dep, r.Scheme)
	return dep
}

func (r *SliceGwReconciler) GetGwPodInfo(ctx context.Context, sliceGw *kubeslicev1beta1.SliceGateway) ([]*kubeslicev1beta1.GwPodInfo, error) {
	log := logger.FromContext(ctx).WithValues("type", "slicegateway")
	podList := &corev1.PodList{}
	gwPodList := make([]*kubeslicev1beta1.GwPodInfo, 0)
	listOpts := []client.ListOption{
		client.InNamespace(sliceGw.Namespace),
		client.MatchingLabels(labelsForSliceGwStatus(sliceGw.Name)),
	}
	if err := r.List(ctx, podList, listOpts...); err != nil {
		log.Error(err, "Failed to list pods")
		return gwPodList, err
	}
	for _, pod := range podList.Items {
		if pod.Status.Phase == corev1.PodRunning && pod.ObjectMeta.DeletionTimestamp == nil {
			gwPod := &kubeslicev1beta1.GwPodInfo{PodName: pod.Name, PodIP: pod.Status.PodIP}
			gwPod.PodCreationTS = &pod.CreationTimestamp
			if existingPodInfo := findGwPodInfo(sliceGw.Status.GatewayPodStatus, pod.Name); existingPodInfo != nil {
				gwPod.OriginalPodCreationTS = existingPodInfo.OriginalPodCreationTS
			} else {
				// should only be called once!!
				gwPod.OriginalPodCreationTS = &pod.CreationTimestamp
			}
			// OriginalPodCreationTS should only be updated once at the start
			gwPodList = append(gwPodList, gwPod)
		}
	}

	return gwPodList, nil
}

func isGatewayStatusChanged(slicegateway *kubeslicev1beta1.SliceGateway, gwPod *kubeslicev1beta1.GwPodInfo) bool {
	return !contains(getPodNames(slicegateway), gwPod.PodName) ||
		!contains(getPodIPs(slicegateway), gwPod.PodIP) ||
		!contains(getLocalNSMIPs(slicegateway), gwPod.LocalNsmIP) ||
		!isGWPodStatusChanged(slicegateway, gwPod)
}

func (r *SliceGwReconciler) ReconcileGwPodStatus(ctx context.Context, slicegateway *kubeslicev1beta1.SliceGateway) (ctrl.Result, error, bool) {
	log := logger.FromContext(ctx).WithValues("type", "SliceGw")
	debugLog := log.V(1)

	gwPodsInfo, err := r.GetGwPodInfo(ctx, slicegateway)
	if err != nil {
		log.Error(err, "Error while fetching the pods", "Failed to fetch podIps and podNames")
		return ctrl.Result{RequeueAfter: 5 * time.Second}, err, true
	}
	if len(gwPodsInfo) == 0 {
		log.Info("Gw pods not available yet, requeuing")
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil, true
	}
	toUpdate, toReconcile := false, false
	for _, gwPod := range gwPodsInfo {
		sidecarGrpcAddress := gwPod.PodIP + ":5000"

		status, err := r.WorkerGWSidecarClient.GetStatus(ctx, sidecarGrpcAddress)
		if err != nil {
			log.Error(err, "Unable to fetch gw status")
			return ctrl.Result{}, err, true
		}
		gwPod.LocalNsmIP = status.NsmStatus.LocalIP
		gwPod.TunnelStatus = kubeslicev1beta1.TunnelStatus(status.TunnelStatus)
		// this grpc call fails untill the openvpn tunnel connection is not established, so its better to do not reconcile in case of errors, hence the reconciler does not proceedes further
		gwPod.PeerPodName, err = r.getRemoteGwPodName(ctx, slicegateway.Status.Config.SliceGatewayRemoteVpnIP, gwPod.PodIP)
		if err != nil {
			log.Error(err, "Error getting peer pod name", "PodName", gwPod.PodName, "PodIP", gwPod.PodIP)
		}
		debugLog.Info("Got gw status", "result", status)
		if isGatewayStatusChanged(slicegateway, gwPod) {
			toUpdate = true
		}
		if len(slicegateway.Status.GatewayPodStatus) != len(gwPodsInfo) {
			toUpdate = true
		}
	}
	if toUpdate {
		log.Info("gwPodsInfo", "gwPodsInfo", gwPodsInfo)
		slicegateway.Status.GatewayPodStatus = gwPodsInfo
		slicegateway.Status.ConnectionContextUpdatedOn = 0
		err := r.Status().Update(ctx, slicegateway)
		if err != nil {
			debugLog.Error(err, "error while update", "Failed to update SliceGateway status for gateway status")
			return ctrl.Result{}, err, true
		}
		toReconcile = true
	}
	if toReconcile {
		return ctrl.Result{}, err, true
	}
	return ctrl.Result{}, nil, false
}

func (r *SliceGwReconciler) SendConnectionContextAndQosToGwPod(ctx context.Context, slice *kubeslicev1beta1.Slice, slicegateway *kubeslicev1beta1.SliceGateway, req reconcile.Request) (ctrl.Result, error, bool) {
	log := logger.FromContext(ctx).WithValues("type", "SliceGw")

	gwPodsInfo, err := r.GetGwPodInfo(ctx, slicegateway)
	if err != nil {
		log.Error(err, "Error while fetching the pods", "Failed to fetch podIps and podNames")
		return ctrl.Result{RequeueAfter: 5 * time.Second}, err, true
	}
	if len(gwPodsInfo) == 0 {
		log.Info("Gw podIPs not available yet, requeuing")
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil, true
	}
	connCtx := &gwsidecar.GwConnectionContext{
		RemoteSliceGwVpnIP:     slicegateway.Status.Config.SliceGatewayRemoteVpnIP,
		RemoteSliceGwNsmSubnet: slicegateway.Status.Config.SliceGatewayRemoteSubnet,
	}
	for i := range gwPodsInfo {
		sidecarGrpcAddress := gwPodsInfo[i].PodIP + ":5000"

		err := r.WorkerGWSidecarClient.SendConnectionContext(ctx, sidecarGrpcAddress, connCtx)
		if err != nil {
			log.Error(err, "Unable to send conn ctx to gw")
			return ctrl.Result{RequeueAfter: 10 * time.Second}, err, true
		}

		err = r.WorkerGWSidecarClient.UpdateSliceQosProfile(ctx, sidecarGrpcAddress, slice)
		if err != nil {
			log.Error(err, "Failed to send qos to gateway")
			return ctrl.Result{RequeueAfter: 10 * time.Second}, err, true
		}

		err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
			err := r.Get(ctx, req.NamespacedName, slicegateway)
			slicegateway.Status.ConnectionContextUpdatedOn = time.Now().Unix()
			err = r.Status().Update(ctx, slicegateway)
			if err != nil {
				log.Error(err, "Failed to update SliceGateway status for conn ctx update in retry loop")
				return err
			}
			return nil
		})
		if err != nil {
			log.Error(err, "Failed to update SliceGateway status for conn ctx update")
			return ctrl.Result{}, err, true
		}
	}
	return ctrl.Result{}, nil, false
}

// In the event of slice router deletion as well this function needs to be called so that the routes can be injected into the router sidecar
func (r *SliceGwReconciler) SendConnectionContextToSliceRouter(ctx context.Context, slicegateway *kubeslicev1beta1.SliceGateway) (ctrl.Result, error, bool) {
	log := logger.FromContext(ctx).WithValues("type", "SliceGw")

	_, podIP, err := controllers.GetSliceRouterPodNameAndIP(ctx, r.Client, slicegateway.Spec.SliceName)
	if err != nil {
		log.Error(err, "Unable to get slice router pod info")
		return ctrl.Result{}, err, true
	}
	if podIP == "" {
		log.Info("Slice router podIP not available yet, requeuing")
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil, true
	}

	if slicegateway.Status.Config.SliceGatewayRemoteSubnet == "" ||
		len(slicegateway.Status.GatewayPodStatus) == 0 {
		log.Info("Waiting for remote subnet and local nsm IPs. Delaying conn ctx update to router")
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil, true
	}

	excludeRouteGwPodList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(slicegateway.Namespace),
		client.MatchingLabels(map[string]string{
			"kubeslice.io/exclude-gw-route": "true",
		}),
	}
	if err := r.List(ctx, excludeRouteGwPodList, listOpts...); err != nil {
		return ctrl.Result{}, err, true
	}

	gwNsmIPs := []string{}
	for _, gwPod := range slicegateway.Status.GatewayPodStatus {
		if isPodPresentInPodList(excludeRouteGwPodList, gwPod.PodName) {
			continue
		}
		if gwPod.LocalNsmIP == "" {
			continue
		}
		if gwPod.PeerPodName == "" {
			continue
		}
		if gwPod.TunnelStatus.Status != int32(gwsidecarpb.TunnelStatusType_GW_TUNNEL_STATE_UP) {
			continue
		}
		gwNsmIPs = append(gwNsmIPs, gwPod.LocalNsmIP)
	}

	sidecarGrpcAddress := podIP + ":5000"
	connCtx := &router.SliceRouterConnCtx{
		RemoteSliceGwNsmSubnet: slicegateway.Status.Config.SliceGatewayRemoteSubnet,
		LocalNsmGwPeerIPs:      gwNsmIPs,
	}
	log.Info("Conn ctx to send to slice router ", "connCtx", connCtx)
	err = r.WorkerRouterClient.SendConnectionContext(ctx, sidecarGrpcAddress, connCtx)
	if err != nil {
		log.Error(err, "Unable to send conn ctx to slice router")
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil, true
	}

	return ctrl.Result{}, nil, false
}

func (r *SliceGwReconciler) SyncNetOpConnectionContextAndQos(ctx context.Context, slice *kubeslicev1beta1.Slice, slicegw *kubeslicev1beta1.SliceGateway, sliceGwNodePorts []int) error {
	log := logger.FromContext(ctx).WithValues("type", "SliceGw")
	debugLog := log.V(1)

	for i := range r.NetOpPods {
		n := &r.NetOpPods[i]
		debugLog.Info("syncing netop pod", "podName", n.PodName)
		sidecarGrpcAddress := n.PodIP + ":5000"

		err := r.WorkerNetOpClient.SendConnectionContext(ctx, sidecarGrpcAddress, slicegw, sliceGwNodePorts)
		if err != nil {
			log.Error(err, "Failed to send conn ctx to netop. PodIp: %v, PodName: %v", n.PodIP, n.PodName)
			return err
		}
		err = r.WorkerNetOpClient.UpdateSliceQosProfile(ctx, sidecarGrpcAddress, slice)
		if err != nil {
			log.Error(err, "Failed to send qos to netop. PodIp: %v, PodName: %v", n.PodIP, n.PodName)
			return err
		}
	}
	debugLog.Info("netop pods sync complete", "pods", r.NetOpPods)
	return nil
}

// getRemoteGwPodName returns the remote gw PodName.
func (r *SliceGwReconciler) getRemoteGwPodName(ctx context.Context, gwRemoteVpnIP string, podIP string) (string, error) {
	r.Log.Info("calling gw sidecar to get PodName", "type", "slicegw")
	sidecarGrpcAddress := podIP + ":5000"
	remoteGwPodName, err := r.WorkerGWSidecarClient.GetSliceGwRemotePodName(ctx, gwRemoteVpnIP, sidecarGrpcAddress)
	if err != nil {
		r.Log.Error(err, "Failed to get slicegw remote pod name", "PodIp", podIP)
		return "", err
	}
	r.Log.Info("slicegw remote pod name", "RemotePodName", remoteGwPodName)
	return remoteGwPodName, nil
}

func (r *SliceGwReconciler) createHeadlessServiceForGwServer(slicegateway *kubeslicev1beta1.SliceGateway) *corev1.Service {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      slicegateway.Status.Config.SliceGatewayRemoteGatewayID,
			Namespace: slicegateway.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Ports:     nil,
			Selector:  nil,
			ClusterIP: "None",
		},
	}
	ctrl.SetControllerReference(slicegateway, svc, r.Scheme)
	return svc
}

func (r *SliceGwReconciler) createEndpointForGatewayServer(slicegateway *kubeslicev1beta1.SliceGateway) *corev1.Endpoints {
	endpointIPs := slicegateway.Status.Config.SliceGatewayRemoteNodeIPs
	// TODO: Remove the env var that overrides the slicegateway config coming from the controller
	if slicegateway.Status.Config.SliceGatewayConnectivityType == "LoadBalancer" || os.Getenv("ENABLE_GW_LB_EDGE") != "" {
		endpointIPs = slicegateway.Status.Config.SliceGatewayServerLBIPs
		if os.Getenv("GW_LB_IP") != "" {
			endpointIPs = []string{os.Getenv("GW_LB_IP")}
		}
	}
	e := &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      slicegateway.Status.Config.SliceGatewayRemoteGatewayID,
			Namespace: slicegateway.Namespace,
		},
		Subsets: []corev1.EndpointSubset{
			{
				Addresses: getAddrSlice(endpointIPs),
			},
		},
	}
	ctrl.SetControllerReference(slicegateway, e, r.Scheme)
	return e
}

func (r *SliceGwReconciler) reconcileGatewayHeadlessService(ctx context.Context, sliceGw *kubeslicev1beta1.SliceGateway) error {
	log := r.Log
	serviceFound := corev1.Service{}
	err := r.Get(ctx, types.NamespacedName{Namespace: sliceGw.Namespace, Name: sliceGw.Status.Config.SliceGatewayRemoteGatewayID}, &serviceFound)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Create a new service with same name as SliceGatewayRemoteGatewayID, because --remote flag of openvpn client is populated with same name. So it would call this svc to get a server IP(through endpoint)
			svc := r.createHeadlessServiceForGwServer(sliceGw)
			if err := r.Create(ctx, svc); err != nil {
				log.Error(err, "Failed to create headless service", "Name", svc.Name)
				return err
			}
			log.Info("Created a headless service for slicegw", "slicegw", sliceGw.Name)
		} else {
			log.Error(err, "Failed to Get Headless service")
			return err
		}
	}
	return nil
}

func (r *SliceGwReconciler) reconcileGatewayEndpoint(ctx context.Context, sliceGw *kubeslicev1beta1.SliceGateway) (bool, ctrl.Result, error) {
	log := r.Log
	debugLog := log.V(1)
	endpointFound := corev1.Endpoints{}
	err := r.Get(ctx, types.NamespacedName{
		Namespace: sliceGw.Namespace,
		Name:      sliceGw.Status.Config.SliceGatewayRemoteGatewayID}, &endpointFound)
	if err != nil {
		if apierrors.IsNotFound(err) {
			//Create a new endpoint with same name as RemoteGatewayID
			endpoint := r.createEndpointForGatewayServer(sliceGw)
			if err := r.Create(ctx, endpoint); err != nil {
				log.Error(err, "Failed to create an Endpoint", "Name", endpoint.Name)
				return true, ctrl.Result{}, err
			}
			return true, ctrl.Result{Requeue: true}, nil
		}
		return true, ctrl.Result{}, err
	}
	// endpoint already exists, check if sliceGatewayRemoteNodeIp is changed then update the endpoint
	toUpdate := false
	//remoteNodeIPs := sliceGw.Status.Config.SliceGatewayRemoteNodeIPs
	endpointIPs := sliceGw.Status.Config.SliceGatewayRemoteNodeIPs
	if sliceGw.Status.Config.SliceGatewayConnectivityType == "LoadBalancer" || os.Getenv("ENABLE_GW_LB_EDGE") != "" {
		endpointIPs = sliceGw.Status.Config.SliceGatewayServerLBIPs
		if os.Getenv("GW_LB_IP") != "" {
			endpointIPs = []string{os.Getenv("GW_LB_IP")}
		}
	}
	currentEndpointFound := endpointFound.Subsets[0]
	debugLog.Info("SliceGatewayRemoteNodeIP", "SliceGatewayRemoteNodeIP", endpointIPs)

	if !checkEndpointSubset(currentEndpointFound, endpointIPs, true) {
		log.Info("Updating the Endpoint, since sliceGatewayRemoteNodeIp has changed", "from endpointFound", currentEndpointFound.Addresses[0].IP)
		endpointFound.Subsets[0].Addresses = getAddrSlice(endpointIPs)
		toUpdate = true
	}
	// When "toUpdate" is set to true we update the endpoints addresses
	if toUpdate {
		err := r.Update(ctx, &endpointFound)
		if err != nil {
			log.Error(err, "Error updating Endpoint")
			return true, ctrl.Result{}, err
		}
		if checkEndpointSubset(currentEndpointFound, endpointIPs, false) {
			// refresh the connections by restarting the gateway pods when the new node IPs are completely distinct
			log.Info("mismatch in node ips so restarting gateway pods")
			if r.restartGatewayPods(ctx, sliceGw.Name) != nil {
				return true, ctrl.Result{}, err
			} else {
				return true, ctrl.Result{Requeue: true}, nil
			}
		}
	}
	return false, ctrl.Result{}, nil
}

func checkEndpointSubset(subset corev1.EndpointSubset, remoteNodeIPs []string, requireMatch bool) bool {
	addrSlice := subset.Addresses

	if requireMatch {
		if len(addrSlice) != len(remoteNodeIPs) {
			return false
		}
	}

	exists := make(map[string]bool)
	for _, value := range addrSlice {
		exists[value.IP] = true
	}

	for _, value := range remoteNodeIPs {
		if requireMatch && !exists[value] {
			// endpoints gettings used should match with SliceGatewayRemoteNodeIPs
			return false
		} else if !requireMatch && exists[value] {
			// when node IPs are totally different
			return false
		}
	}
	return true
}

func (r *SliceGwReconciler) restartGatewayPods(ctx context.Context, sliceGWName string) error {
	log := r.Log
	podsList := corev1.PodList{}
	labels := map[string]string{"kubeslice.io/pod-type": "slicegateway",
		"kubeslice.io/slice-gw": sliceGWName,
	}
	listOptions := []client.ListOption{
		client.MatchingLabels(labels),
	}
	err := r.List(ctx, &podsList, listOptions...)
	if err != nil {
		return err
	}

	for _, pod := range podsList.Items {
		log.Info("restarting gateway pods", pod.Name)
		err = r.Delete(ctx, &pod)
		if err != nil {
			return err
		}
	}
	return nil

}

func getAddrSlice(endpointIPs []string) []corev1.EndpointAddress {
	endpointSlice := make([]corev1.EndpointAddress, 0)
	for _, ip := range endpointIPs {
		endpointSlice = append(endpointSlice, corev1.EndpointAddress{IP: ip})
	}
	return endpointSlice
}

func UpdateGWPodStatus(gwPodStatus []*kubeslicev1beta1.GwPodInfo, podName string) []*kubeslicev1beta1.GwPodInfo {
	index := -1
	for i := range gwPodStatus {
		if gwPodStatus[i].PodName == podName {
			index = i
			break
		}
	}
	return append(gwPodStatus[:index], gwPodStatus[index+1:]...)

}

func (r *SliceGwReconciler) gwPodPlacementIsSkewed(ctx context.Context, sliceGw *kubeslicev1beta1.SliceGateway) (bool, string, string, error) {
	log := r.Log

	podList := corev1.PodList{}
	labels := map[string]string{controllers.PodTypeSelectorLabelKey: "slicegateway", "kubeslice.io/slice-gw": sliceGw.Name}
	listOptions := []client.ListOption{
		client.MatchingLabels(labels),
	}
	err := r.Client.List(ctx, &podList, listOptions...)
	if err != nil {
		log.Error(err, "can't fetch pod list:")
		return false, "", "", err
	}

	podCount := len(podList.Items)

	numGwInstances := r.NumberOfGateways
	if isClient(sliceGw) {
		if len(sliceGw.Status.Config.SliceGatewayRemoteNodePorts) < r.NumberOfGateways {
			numGwInstances = len(sliceGw.Status.Config.SliceGatewayRemoteNodePorts)
		}
	}

	// Do not run any recycler before all the gw instances are running.
	if podCount < numGwInstances {
		return false, "", "", nil
	}

	nodeList := corev1.NodeList{}
	nodeLabels := map[string]string{controllers.NodeTypeSelectorLabelKey: "gateway"}
	listOpts := []client.ListOption{
		client.MatchingLabels(nodeLabels),
	}
	if err := r.List(ctx, &nodeList, listOpts...); err != nil {
		log.Error(err, "Error getting kubeslice nodeList")
		return false, "", "", err
	}
	nodeCount := len(nodeList.Items)
	if nodeCount <= 1 {
		return false, "", "", nil
	}

	desiredNumPodsPerNode := int(math.Ceil(float64(podCount) / float64(nodeCount)))
	nodeToPodMap := make(map[string][]corev1.Pod)

	for _, pod := range podList.Items {
		nodeToPodMap[pod.Spec.NodeName] = append(nodeToPodMap[pod.Spec.NodeName], pod)
	}

	for _, podsOnNode := range nodeToPodMap {
		if len(podsOnNode) > desiredNumPodsPerNode {
			podToRebalance, peerPodToRebalance := getPodPairToRebalance(podsOnNode, sliceGw)
			if podToRebalance == "" || peerPodToRebalance == "" {
				continue
			}
			return true, podToRebalance, peerPodToRebalance, nil
		}
	}

	return false, "", "", nil
}

func (r *SliceGwReconciler) ReconcileGwPodPlacement(ctx context.Context, sliceGw *kubeslicev1beta1.SliceGateway) error {
	log := r.Log
	// The gw pod rebalancing is always performed on a deployment. We expect the gw pods belonging to a slicegateway
	// object between any two clusters to placed on different nodes marked as kubeslice gateway nodes. If they are
	// initially placed on the same node due to lack of kubeslice-gateway nodes, the rebalancing algorithim is expected
	// to run when a new kubeslice-gateway node is added to the cluster.
	// Check if any recycler is already running for this slice gw. We will rebalance only one gw pair (out of the
	// r.NumberOfGateways) at a time because we could achieve the desired pod placement without having to recycle all the
	// gw pairs of the slicegateway object.
	recyclers, err := r.HubClient.ListWorkerSliceGwRecycler(ctx, sliceGw.Name)
	if err != nil {
		return err
	}
	if len(recyclers) > 0 {
		// Check if any of the recyclers are in the end state and clean them up
		for _, recycler := range recyclers {
			// TODO: Need a better way to get the name of the end state. Using "end" for now
			if recycler.Spec.State == "end" {
				log.Info("Deleting gw recycler", "recycler", recycler.Name)
				err := r.HubClient.DeleteWorkerSliceGwRecycler(ctx, recycler.Name)
				if err != nil {
					log.Error(err, "Failed to delete recycler", "recycler", recycler.Name)
				}
			}
		}
		return nil
	}

	rebalanceNeeded, podToRebalance, peerPodToRebalance, err := r.gwPodPlacementIsSkewed(ctx, sliceGw)
	if err != nil {
		return err
	}
	if !rebalanceNeeded {
		return nil
	}
	if podToRebalance == "" || peerPodToRebalance == "" {
		return nil
	}

	log.Info("Rebalancing pods", "Local Pod", podToRebalance, "Remote Pod", peerPodToRebalance)

	// Rebalancing is done at the deployment level. We should get the names of the deployment from the pod names
	// chosen to be recycled.
	// TODO: We should get the name of the client deployment from the in-band communication of the gw pair.
	serverID := GetDepNameFromPodName(sliceGw.Status.Config.SliceGatewayID, podToRebalance)
	clientID := GetDepNameFromPodName(sliceGw.Status.Config.SliceGatewayRemoteGatewayID, peerPodToRebalance)
	if serverID == "" || clientID == "" {
		return nil
	}

	slice, err := controllers.GetSlice(ctx, r.Client, sliceGw.Spec.SliceName)
	if err != nil {
		log.Error(err, "Failed to get Slice", "slice", sliceGw.Spec.SliceName)
		return err
	}
	err = r.WorkerRecyclerClient.TriggerFSM(sliceGw, slice, serverID, clientID, controllerName)
	if err != nil {
		log.Error(err, "Error while recycling gateway pods")
		return err
	}

	return nil
}

func (r *SliceGwReconciler) handleSliceGwSvcCreation(ctx context.Context, sliceGw *kubeslicev1beta1.SliceGateway, svcName, depName string) (reconcile.Result, error, bool) {
	log := logger.FromContext(ctx).WithName("slicegw")

	svc := r.serviceForGateway(sliceGw, svcName, depName)
	log.Info("Creating a new Service", "Namespace", svc.Namespace, "Name", svc.Name)
	err := r.Create(ctx, svc)
	if err != nil {
		log.Error(err, "Failed to create new Service", "Namespace", svc.Namespace, "Name", svc.Name)
		return ctrl.Result{}, err, true
	}

	return ctrl.Result{Requeue: true}, nil, true
}

func (r *SliceGwReconciler) handleSliceGwSvcDeletion(ctx context.Context, sliceGw *kubeslicev1beta1.SliceGateway, svcName, depName string) error {
	log := logger.FromContext(ctx).WithName("slicegw")
	serviceFound := corev1.Service{}
	err := r.Get(ctx, types.NamespacedName{Namespace: sliceGw.Namespace, Name: svcName}, &serviceFound)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}

		return err
	}

	err = r.Delete(ctx, &serviceFound)
	if err != nil {
		log.Error(err, "Failed to delete gw svc", "Name", svcName)
		return err
	}

	log.Info("Deleted gw svc", "svcName", svcName)

	return nil
}

func (r *SliceGwReconciler) getNodePorts(ctx context.Context, sliceGw *kubeslicev1beta1.SliceGateway) ([]int, error) {
	var nodePorts []int
	listOpts := []client.ListOption{
		client.MatchingLabels(map[string]string{
			controllers.ApplicationNamespaceSelectorLabelKey: sliceGw.Spec.SliceName,
			"kubeslice.io/slicegw":                           sliceGw.Name,
		}),
	}
	services := corev1.ServiceList{}
	if err := r.List(ctx, &services, listOpts...); err != nil {
		return nil, err
	}
	for _, service := range services.Items {
		nodePorts = append(nodePorts, int(service.Spec.Ports[0].NodePort))
	}
	return nodePorts, nil
}

func (r *SliceGwReconciler) getGatewayServices(ctx context.Context, sliceGw *kubeslicev1beta1.SliceGateway) (*corev1.ServiceList, error) {
	listOpts := []client.ListOption{
		client.MatchingLabels(map[string]string{
			controllers.ApplicationNamespaceSelectorLabelKey: sliceGw.Spec.SliceName,
			"kubeslice.io/slicegw":                           sliceGw.Name,
		}),
		client.InNamespace(controllers.ControlPlaneNamespace),
	}
	services := corev1.ServiceList{}
	if err := r.List(ctx, &services, listOpts...); err != nil {
		return nil, err
	}
	return &services, nil
}

func (r *SliceGwReconciler) ReconcileGatewayServices(ctx context.Context, sliceGw *kubeslicev1beta1.SliceGateway) (ctrl.Result, error, bool) {
	log := r.Log
	gwServices, err := r.getGatewayServices(ctx, sliceGw)
	if err != nil {
		return ctrl.Result{}, err, true
	}

	sliceGwName := sliceGw.Name

	for gwInstance := 0; gwInstance < r.NumberOfGateways; gwInstance++ {
		if !gwServiceIsPresent(sliceGwName, gwInstance, gwServices) {
			svcName := "svc-" + sliceGwName + "-" + fmt.Sprint(gwInstance) + "-" + "0"
			depName := sliceGwName + "-" + fmt.Sprint(gwInstance) + "-" + "0"
			_, err, _ := r.handleSliceGwSvcCreation(ctx, sliceGw, svcName, depName)
			if err != nil {
				return ctrl.Result{}, err, true
			}
			return ctrl.Result{Requeue: true}, nil, true
		}
	}

	nodePorts, err := r.getNodePorts(ctx, sliceGw)
	if err != nil {
		return ctrl.Result{}, err, true
	}

	err = r.HubClient.UpdateNodePortForSliceGwServer(ctx, nodePorts, sliceGwName)
	if err != nil {
		log.Error(err, "Failed to update NodePort for sliceGw in the hub")
		utils.RecordEvent(ctx, r.EventRecorder, sliceGw, nil, ossEvents.EventSliceGWNodePortUpdateFailed, controllerName)
		return ctrl.Result{}, err, true
	}

	// Update LB IP if service type is LoadBalancer
	if sliceGw.Status.Config.SliceGatewayConnectivityType == "LoadBalancer" {
		lbSvcList, err := controllers.GetSliceGatewayEdgeServices(ctx, r.Client, sliceGw.Spec.SliceName)
		if err != nil {
			log.Error(err, "Failed to get LB IP service info")
			return ctrl.Result{}, err, true
		}
		lbIPs := []string{}
		for _, lbSvc := range lbSvcList.Items {
			for _, lbIngress := range lbSvc.Status.LoadBalancer.Ingress {
				if lbIngress.IP != "" {
					lbIPs = append(lbIPs, lbIngress.IP)
					continue
				}
				if lbIngress.Hostname != "" {
					resolver := net.Resolver{}
					resolvedIps, err := resolver.LookupHost(context.Background(), lbIngress.Hostname)
					if err != nil {
						log.Error(err, "Failed to resolve name", "hostname", lbIngress.Hostname)
						return ctrl.Result{}, err, true
					}
					log.Info("Resolved hostname", "IPs", resolvedIps)
					lbIPs = append(lbIPs, resolvedIps...)
					continue
				}
			}
		}
		err = r.HubClient.UpdateLBIPsForSliceGwServer(ctx, lbIPs, sliceGwName)
		if err != nil {
			log.Error(err, "Failed to update LB IP for gw server")
			return ctrl.Result{}, err, true
		}
	}

	return ctrl.Result{}, nil, false
}

func (r *SliceGwReconciler) ReconcileGatewayDeployments(ctx context.Context, sliceGw *kubeslicev1beta1.SliceGateway) (ctrl.Result, error, bool) {
	// The slice gateway deployments follow a naming format. The format is the following:
	// <sliceGatewayName>-<gwInstance>-<deploymentInstance>
	// sliceGatewayName is the name of the slicegateway object.
	//     It is usually in the form of <worker-cluster-1-name>-<worker-cluster-2-name>.
	// gwInstance is the instance number of the slicegateway deployments. It is derived from r.NumberOfGateways.
	//     It ranges from 0 to r.NumberOfGateways - 1
	// deploymentInstance is the instance number within the gwInstance. It is needed when the gateways are being updated due to
	//     vpn key rotation or gw pod re-placement. Currently, it will only have two values: 0 or 1.
	// Example deployment name: slicered-worker-1-worker-2-0-0, where slicered-worker-1-worker-2 is the name of the slicegateway
	// between two worker clusters worker-1 and worker-2, the 0-0 suffix holds the gwInstance and the deploymentInstance. If the numer
	// of gateways (r.NumberOfGateways) is 2, the names of the deployments would be:
	// slicered-worker-1-worker-2-0-0, slicered-worker-1-worker-2-1-0
	// If the deployments are being recycled in a make before break fashion, we would have the following deployments:
	// slicered-worker-1-worker-2-0-0, slicered-worker-1-worker-2-0-1
	// slicered-worker-1-worker-2-1-0, slicered-worker-1-worker-2-1-1
	// Note the deployment instance numbers in the names of the deployments.
	// After the recycling concludes we would be left with the following deployments:
	// slicered-worker-1-worker-2-0-1
	// slicered-worker-1-worker-2-1-1
	// The deploymentInstance cycles between 0 and 1. So when the next recycling occurs, the names of the deployments would go back to:
	// slicered-worker-1-worker-2-0-0
	// slicered-worker-1-worker-2-1-0
	log := r.Log
	deployments, err := GetDeployments(ctx, r.Client, sliceGw.Spec.SliceName, sliceGw.Name)
	if err != nil {
		return ctrl.Result{}, err, true
	}

	sliceName := sliceGw.Spec.SliceName
	sliceGwName := sliceGw.Name

	vpnKeyRotation, err := r.HubClient.GetVPNKeyRotation(ctx, sliceName)
	if err != nil {
		return ctrl.Result{}, err, true
	}

	gwConfigKey := 1
	if vpnKeyRotation != nil {
		gwConfigKey = vpnKeyRotation.Spec.RotationCount
	}

	numGwInstances := r.NumberOfGateways
	if isClient(sliceGw) {
		if len(sliceGw.Status.Config.SliceGatewayRemoteNodePorts) < r.NumberOfGateways {
			numGwInstances = len(sliceGw.Status.Config.SliceGatewayRemoteNodePorts)
		}
	}

	for gwInstance := 0; gwInstance < numGwInstances; gwInstance++ {
		if !gwDeploymentIsPresent(sliceGwName, gwInstance, deployments) {
			dep := r.deploymentForGateway(sliceGw, sliceGwName+"-"+fmt.Sprint(gwInstance)+"-"+"0", gwConfigKey)
			log.Info("Creating a new Deployment", "Namespace", dep.Namespace, "Name", dep.Name)
			err = r.Create(ctx, dep)
			if err != nil {
				log.Error(err, "Failed to create new Deployment", "Namespace", dep.Namespace, "Name", dep.Name)
				return ctrl.Result{}, err, true
			}
			return ctrl.Result{Requeue: true}, nil, true
		}
	}

	// Create PodDisruptionBudget for slice gateway's pod to at least have 1 instance of pods on each worker
	// when disruption has occurred.
	//
	// Note: This should run an attempt to create PDB regardless of whether current reconciliation creating deployments
	// as the request could've been requeued due to failure at the creation of PDB.
	if err = r.createPodDisruptionBudgetForSliceGatewayPods(ctx, sliceName, sliceGw); err != nil {
		log.Error(err, "Failed to create PodDisruptionBudget for SliceGW deployments",
			"SliceName", sliceName, "SliceGwName", sliceGwName)

		return ctrl.Result{}, err, true
	}

	// Reconcile deployment to node port mapping for gw client deployments
	if isClient(sliceGw) {
		for _, deployment := range deployments.Items {
			found, nodePortInUse := getClientGwRemotePortInUse(ctx, r.Client, sliceGw, deployment.Name)
			if found {
				// Check if the portInUse is valid.
				// It is valid only if the list of remoteNodePorts in the slicegw object contains the portInUse.
				if !checkIfNodePortIsValid(sliceGw.Status.Config.SliceGatewayRemoteNodePorts, nodePortInUse) {
					// Get a valid port number for this deployment
					portNumToUpdate, err := allocateNodePortToClient(sliceGw.Status.Config.SliceGatewayRemoteNodePorts, deployment.Name, &gwClientToRemotePortMap)
					if err != nil {
						return ctrl.Result{}, err, true
					}
					// Update the port map
					gwClientToRemotePortMap.Store(deployment.Name, portNumToUpdate)
					err = r.updateGatewayDeploymentNodePort(ctx, r.Client, sliceGw, &deployment, portNumToUpdate)
					if err != nil {
						return ctrl.Result{}, err, true
					}
					// Requeue if the update was successful
					return ctrl.Result{}, nil, true
				}
				// At this point, the node port in use is valid. Check if the port map is populated. Only case
				// where it is not populated is if the operator restarts. The populated value must match the
				// port in use. If not, the deploy needs to be updated to match the state stored in the operator.
				portInMap, foundInMap := gwClientToRemotePortMap.Load(deployment.Name)
				if foundInMap {
					if portInMap != nodePortInUse {
						// Update the deployment since the port numbers do not match
						err := r.updateGatewayDeploymentNodePort(ctx, r.Client, sliceGw, &deployment, portInMap.(int))
						if err != nil {
							return ctrl.Result{}, err, true
						}
						// Requeue if the update was successful
						return ctrl.Result{}, nil, true
					}
				} else {
					gwClientToRemotePortMap.Store(deployment.Name, nodePortInUse)
				}
			}
		}

	}

	// Delete any deployments marked for deletion. We could have an external orchestrator (like the workerslicegatewayrecycler) request
	// for deletion of a gateway deployment by adding a label to the deployment object. The slicegateway reconciler should be the sole
	// manager of gateway services and deployments. External entities can only request for creation or deletion of deployments.
	deploymentsToDelete, err := getDeploymentsMarkedForDeletion(ctx, r.Client, sliceGw.Spec.SliceName, sliceGw.Name)
	if err != nil {
		return ctrl.Result{}, err, true
	}

	if deploymentsToDelete != nil {
		for _, depToDelete := range deploymentsToDelete.Items {
			// Delete the gw svc associated with the deployment
			err := r.handleSliceGwSvcDeletion(ctx, sliceGw, getGwSvcNameFromDepName(depToDelete.Name), depToDelete.Name)
			if err != nil {
				log.Error(err, "Failed to delete gw svc", "svcName", depToDelete.Name)
				return ctrl.Result{}, err, true
			}
			err = r.Delete(ctx, &depToDelete)
			if err != nil {
				log.Error(err, "Failed to delete deployment", "depName", depToDelete.Name)
				return ctrl.Result{}, err, true
			}

			if isClient(sliceGw) {
				value, loaded := gwClientToRemotePortMap.LoadAndDelete(depToDelete.Name)
				if loaded {
					log.Info("Deleted port num map for gw deployment", "depName", depToDelete.Name, "port", value.(int))
				}
			}

		}
	}

	return ctrl.Result{}, nil, false
}

func (r *SliceGwReconciler) createIntermediateGatewayDeployment(ctx context.Context, sliceGw *kubeslicev1beta1.SliceGateway, depToCreate string) error {
	log := r.Log

	sliceName := sliceGw.Spec.SliceName

	vpnKeyRotation, err := r.HubClient.GetVPNKeyRotation(ctx, sliceName)
	if err != nil {
		return err
	}

	gwConfigKey := 1
	if vpnKeyRotation != nil {
		gwConfigKey = vpnKeyRotation.Spec.RotationCount
	}

	dep := r.deploymentForGateway(sliceGw, depToCreate, gwConfigKey)
	log.Info("Creating a new Deployment", "Namespace", dep.Namespace, "Name", dep.Name)
	err = r.Create(ctx, dep)
	if err != nil {
		log.Error(err, "Failed to create new Deployment", "Namespace", dep.Namespace, "Name", dep.Name)
		return err
	}

	return nil
}

func (r *SliceGwReconciler) createIntermediateGatewayService(ctx context.Context, sliceGw *kubeslicev1beta1.SliceGateway) (bool, error) {
	gwServices, err := r.getGatewayServices(ctx, sliceGw)
	if err != nil {
		return false, err
	}

	svcToCreate := ""
	depName := ""
	for _, intermediateDep := range sliceGw.Status.Config.SliceGatewayIntermediateDeployments {
		if getGwService(gwServices, "svc"+"-"+intermediateDep) == nil {
			svcToCreate = "svc" + "-" + intermediateDep
			depName = intermediateDep
			break
		}
	}

	if svcToCreate == "" {
		return false, nil
	}

	_, err, _ = r.handleSliceGwSvcCreation(ctx, sliceGw, svcToCreate, depName)
	if err != nil {
		return false, err
	}

	return true, nil
}

func (r *SliceGwReconciler) newIntermediateDeploymentNeeded(ctx context.Context, sliceGw *kubeslicev1beta1.SliceGateway) string {
	depToCreate := ""
	for _, intermediateDep := range sliceGw.Status.Config.SliceGatewayIntermediateDeployments {
		if getGwDeployment(ctx, r.Client, sliceGw, intermediateDep) == nil {
			depToCreate = intermediateDep
			break
		}
	}

	return depToCreate
}

func (r *SliceGwReconciler) ReconcileIntermediateGatewayDeployments(ctx context.Context, sliceGw *kubeslicev1beta1.SliceGateway) (ctrl.Result, error, bool) {
	log := r.Log
	// Check if there is a request to have an intermediate deployment created
	depToCreate := r.newIntermediateDeploymentNeeded(ctx, sliceGw)
	if depToCreate != "" {
		// If client, wait for the new nodeport number to be available before creating a new deployment
		if isClient(sliceGw) {
			nodePortAvailable := false
			for _, nodePort := range sliceGw.Status.Config.SliceGatewayRemoteNodePorts {
				if !checkIfNodePortIsAlreadyUsed(nodePort) {
					nodePortAvailable = true
					break
				}
			}

			if !nodePortAvailable {
				log.Info("NodePort not available. Waiting to create new intermediate deployment")
				return ctrl.Result{
					RequeueAfter: 10 * time.Second,
				}, nil, true
			}

		}

		err := r.createIntermediateGatewayDeployment(ctx, sliceGw, depToCreate)
		if err != nil {
			return ctrl.Result{}, err, true
		}

		return ctrl.Result{Requeue: true}, nil, true
	}

	if isServer(sliceGw) {
		newSvcCreated, err := r.createIntermediateGatewayService(ctx, sliceGw)
		if err != nil {
			return ctrl.Result{}, err, true
		}
		if newSvcCreated {
			return ctrl.Result{Requeue: true}, nil, true
		}
	}

	return ctrl.Result{}, nil, false
}

// createPodDisruptionBudgetForSliceGatewayPods checks for PodDisruptionBudget objects in the cluster that match the
// slice gateway pods, and if missing, it creates a PDB with minimum availability of 1 so at least one pod remains in
// case of a disruption.
func (r *SliceGwReconciler) createPodDisruptionBudgetForSliceGatewayPods(ctx context.Context,
	sliceName string, sliceGateway *kubeslicev1beta1.SliceGateway) error {
	log := r.Log.WithValues("sliceName", sliceName, "sliceGwName", sliceGateway.Name)

	// List PDBs in cluster that match the slice gateway pods
	pdbs, err := listPodDisruptionBudgetForSliceGateway(ctx, r.Client, sliceName, sliceGateway.Name)
	if err != nil && !apierrors.IsNotFound(err) {
		log.Error(err, "failed to list PodDisruptionBudgets that match the slice gateway")

		// When some unexpected error occurred, return the error for requeuing the request
		return err
	}

	// Check if PDB already exists that matches the current slice gateway
	if len(pdbs) > 0 {
		// PodDisruptionBudget matching the slice gateway already exists. Skipping creation.
		return nil
	}

	// Create PDB manifest with minimum availability of 1 pod
	pdb := constructPodDisruptionBudget(sliceName, sliceGateway.Name, DefaultMinAvailablePodsInPDB)

	// Set SliceGateway instance as the owner and controller for PDB
	if err = ctrl.SetControllerReference(sliceGateway, pdb, r.Scheme); err != nil {
		log.Error(err, "Failed to set slice gateway as owner to PodDisruptionBudget",
			"pdb", pdb.Name)

		return fmt.Errorf("failed to set slice gateway %q as owner to PodDisruptionBudget %q: %v",
			sliceGateway.Name, pdb.Name, err)
	}

	// Create PDB for slice gateway's pod to have at least 1 pod on each worker when disruption occurs
	if err = r.Create(ctx, pdb); err != nil {
		if apierrors.IsAlreadyExists(err) {
			// PDB is already exists. So, ignoring the current request.
			return nil
		}

		log.Error(err, "PodDisruptionBudget creation failed", "pdb", pdb.Name)

		// When any other unexpected error occurred when attempting to create PDB, fail the request
		return fmt.Errorf("failed to create PodDisruptionBudget for SliceGW pods: %v", err)
	}

	// PDB created successfully
	log.Info("PodDisruptionBudget for slice gateway pods created successfully")

	return nil
}

// updateGatewayDeploymentNodePort updates the gateway client deployments with the relevant updated ports
// from the workersliceconfig
func (r *SliceGwReconciler) updateGatewayDeploymentNodePort(ctx context.Context, c client.Client, g *kubeslicev1beta1.SliceGateway, deployment *appsv1.Deployment, nodePort int) error {
	containers := deployment.Spec.Template.Spec.Containers
	for contIndex, cont := range containers {
		if cont.Name == "kubeslice-sidecar" {
			for index, key := range cont.Env {
				if key.Name == "NODE_PORT" {
					updatedEnvVar := corev1.EnvVar{
						Name:  "NODE_PORT",
						Value: strconv.Itoa(nodePort),
					}
					cont.Env[index] = updatedEnvVar
				}
			}
		} else if cont.Name == "kubeslice-openvpn-client" {
			containers[contIndex].Args = getOVPNClientContainerArgs(nodePort, g)
		}
	}
	deployment.Spec.Template.Spec.Containers = containers
	err := r.Update(ctx, deployment)
	if err != nil {
		return err
	}
	return nil
}
