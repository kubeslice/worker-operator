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
	_ "errors"
	"fmt"
	"math"
	"os"
	"reflect"
	"strconv"
	"time"

	"github.com/kubeslice/worker-operator/controllers"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gwsidecarpb "github.com/kubeslice/gateway-sidecar/pkg/sidecar/sidecarpb"
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	"github.com/kubeslice/worker-operator/pkg/cluster"
	"github.com/kubeslice/worker-operator/pkg/gwsidecar"
	"github.com/kubeslice/worker-operator/pkg/logger"
	"github.com/kubeslice/worker-operator/pkg/router"
	webhook "github.com/kubeslice/worker-operator/pkg/webhook/pod"
)

var (
	gwSidecarImage           = os.Getenv("AVESHA_GW_SIDECAR_IMAGE")
	gwSidecarImagePullPolicy = os.Getenv("AVESHA_GW_SIDECAR_IMAGE_PULLPOLICY")

	openVpnServerImage      = os.Getenv("AVESHA_OPENVPN_SERVER_IMAGE")
	openVpnClientImage      = os.Getenv("AVESHA_OPENVPN_CLIENT_IMAGE")
	openVpnServerPullPolicy = os.Getenv("AVESHA_OPENVPN_SERVER_PULLPOLICY")
	openVpnClientPullPolicy = os.Getenv("AVESHA_OPENVPN_CLIENT_PULLPOLICY")
)

// labelsForSliceGwDeployment returns the labels for creating slice gw deployment
func labelsForSliceGwDeployment(name string, slice string, i int) map[string]string {
	return map[string]string{
		"networkservicemesh.io/app":                      name,
		webhook.PodInjectLabelKey:                        "slicegateway",
		controllers.ApplicationNamespaceSelectorLabelKey: slice,
		"kubeslice.io/slice-gw":                          name,
		"kubeslice.io/slicegateway-pod":                  fmt.Sprint(i),
	}
}

func labelsForSliceGwService(name string, i int) map[string]string {
	return map[string]string{
		"kubeslice.io/slice-gw":         name,
		"kubeslice.io/slicegateway-pod": fmt.Sprint(i),
	}
}

func labelsForSliceGwStatus(name string) map[string]string {
	return map[string]string{
		"kubeslice.io/slice-gw": name,
	}
}

// deploymentForGateway returns a gateway Deployment object
func (r *SliceGwReconciler) deploymentForGateway(g *kubeslicev1beta1.SliceGateway, i int) *appsv1.Deployment {
	if g.Status.Config.SliceGatewayHostType == "Server" {
		return r.deploymentForGatewayServer(g, i)
	} else {
		return r.deploymentForGatewayClient(g, i)
	}
}

func (r *SliceGwReconciler) deploymentForGatewayServer(g *kubeslicev1beta1.SliceGateway, i int) *appsv1.Deployment {
	ls := labelsForSliceGwDeployment(g.Name, g.Spec.SliceName, i)

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

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      g.Name + "-" + fmt.Sprint(i),
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
						PodAntiAffinity: getPodAntiAffinity(g.Spec.SliceName),
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
								Value: controllers.NodeIP,
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
									SecretName:  g.Name,
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

func (r *SliceGwReconciler) serviceForGateway(g *kubeslicev1beta1.SliceGateway, i int) *corev1.Service {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "svc-" + g.Name + "-" + fmt.Sprint(i),
			Namespace: g.Namespace,
			Labels: map[string]string{
				controllers.ApplicationNamespaceSelectorLabelKey: g.Spec.SliceName,
				"kubeslice.io/slicegw":                           g.Name,
			},
		},
		Spec: corev1.ServiceSpec{
			Type:     "NodePort",
			Selector: labelsForSliceGwService(g.Name, i),
			Ports: []corev1.ServicePort{{
				Port:       11194,
				Protocol:   corev1.ProtocolUDP,
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

func (r *SliceGwReconciler) deploymentForGatewayClient(g *kubeslicev1beta1.SliceGateway, i int) *appsv1.Deployment {
	var replicas int32 = 1
	var privileged = true

	var vpnSecretDefaultMode int32 = 0644

	certFileName := "openvpn_client.ovpn"
	sidecarImg := "nexus.dev.aveshalabs.io/kubeslice/gw-sidecar:1.0.0"
	sidecarPullPolicy := corev1.PullAlways

	vpnImg := "nexus.dev.aveshalabs.io/kubeslice/openvpn-client.alpine.amd64:1.0.0"
	vpnPullPolicy := corev1.PullAlways

	ls := labelsForSliceGwDeployment(g.Name, g.Spec.SliceName, i)

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

	// If val is present in the map loop through all the nodePorts and select a unique one
	if !checkIfNodePortIsAlreadyUsed(g.Status.Config.SliceGatewayRemoteNodePorts[i]) {
		GwMap[g.Name+"-"+fmt.Sprint(i)] = g.Status.Config.SliceGatewayRemoteNodePorts[i]
	} else {
		// select nodePort that is not used
		for _, nodePort := range g.Status.Config.SliceGatewayRemoteNodePorts {
			if selectNodePort(nodePort) {
				GwMap[g.Name+"-"+fmt.Sprint(i)] = nodePort
			}
		}
	}
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      g.Name + "-" + fmt.Sprint(i),
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
						PodAntiAffinity: getPodAntiAffinity(g.Spec.SliceName),
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
								Value: strconv.Itoa(GwMap[g.Name+"-"+fmt.Sprint(i)]),
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
						Args: []string{
							"/vpnclient/" + certFileName,
							"90",
							"openvpn",
							"--remote",
							g.Status.Config.SliceGatewayRemoteGatewayID,
							"--port",
							strconv.Itoa(GwMap[g.Name+"-"+fmt.Sprint(i)]),
							"--ping-restart",
							"15",
							"--proto",
							"udp",
							"--txqueuelen",
							"5000",
							"--config",
							"/vpnclient/" + certFileName,
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
							MountPath: "/vpnclient",
						}},
					}},
					Volumes: []corev1.Volume{{
						Name: "shared-volume",
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName:  g.Name,
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
			log.Error(err, "Error getting peer pod name %v,pod ip %v", gwPod.PodName, gwPod.PodIP)
		}
		debugLog.Info("Got gw status", "result", status)
		if r.isRouteRemoved(slicegateway, gwPod.PodName) {
			gwPod.RouteRemoved = 1
		}
		if isGatewayStatusChanged(slicegateway, gwPod) {
			toUpdate = true
		}
		if len(slicegateway.Status.GatewayPodStatus) != len(gwPodsInfo) {
			toUpdate = true
		}
		if status.TunnelStatus.Status == int32(gwsidecarpb.TunnelStatusType_GW_TUNNEL_STATE_DOWN) {
			if !r.isRouteRemoved(slicegateway, gwPod.PodName) {
				log.Info("slicegw status down", "route not removed,removing route", gwPod.PodName)
				err := r.UpdateRoutesInRouter(ctx, slicegateway, gwPod.LocalNsmIP)
				if err != nil {
					toReconcile = true
				} else {
					gwPod.RouteRemoved = 1
					toUpdate = true
				}
			}
		} else {
			if r.isRouteRemoved(slicegateway, gwPod.PodName) {
				log.Info("updating gw pod remove route field ", "--->", gwPod)
				gwPod.RouteRemoved = 0
				toUpdate = true
			}
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
func (r *SliceGwReconciler) UpdateRoutesInRouter(ctx context.Context, slicegateway *kubeslicev1beta1.SliceGateway, NsmIP string) error {
	log := logger.FromContext(ctx).WithValues("type", "SliceGw")

	_, podIP, err := controllers.GetSliceRouterPodNameAndIP(ctx, r.Client, slicegateway.Spec.SliceName)
	if err != nil {
		log.Error(err, "Unable to get slice router pod info")
		return err
	}
	if podIP == "" {
		log.Info("Slice router podIP not available yet, requeuing")
		return err
	}

	if slicegateway.Status.Config.SliceGatewayRemoteSubnet == "" ||
		len(slicegateway.Status.GatewayPodStatus) == 0 {
		log.Info("Waiting for remote subnet and local nsm IPs. Delaying conn ctx update to router")
		return err
	}

	sidecarGrpcAddress := podIP + ":5000"
	routeInfo := &router.UpdateEcmpInfo{
		RemoteSliceGwNsmSubnet: slicegateway.Status.Config.SliceGatewayRemoteSubnet,
		NsmIpToDelete:          NsmIP,
	}
	err = r.WorkerRouterClient.UpdateEcmpRoutes(ctx, sidecarGrpcAddress, routeInfo)
	if err != nil {
		log.Error(err, "Unable to update ecmp routes in the slice router")
		return err
	}
	return nil
}
func (r *SliceGwReconciler) SendConnectionContextAndQosToGwPod(ctx context.Context, slice *kubeslicev1beta1.Slice, slicegateway *kubeslicev1beta1.SliceGateway) (ctrl.Result, error, bool) {
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
	for i, _ := range gwPodsInfo {
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

		slicegateway.Status.ConnectionContextUpdatedOn = time.Now().Unix()
		err = r.Status().Update(ctx, slicegateway)
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

	sidecarGrpcAddress := podIP + ":5000"
	LocalNsmIPs := getLocalNSMIPsForRouter(slicegateway)
	connCtx := &router.SliceRouterConnCtx{
		RemoteSliceGwNsmSubnet: slicegateway.Status.Config.SliceGatewayRemoteSubnet,
		LocalNsmGwPeerIPs:      LocalNsmIPs,
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
	r.Log.Info("slicegw remote pod name", "slicegw", remoteGwPodName)
	if err != nil {
		r.Log.Error(err, "Failed to get slicegw remote pod name. PodIp: %v", podIP)
		return "", err
	}
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
	e := &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      slicegateway.Status.Config.SliceGatewayRemoteGatewayID,
			Namespace: slicegateway.Namespace,
		},
		Subsets: []corev1.EndpointSubset{
			{
				Addresses: getAddrSlice(slicegateway.Status.Config.SliceGatewayRemoteNodeIPs),
			},
		},
	}
	ctrl.SetControllerReference(slicegateway, e, r.Scheme)
	return e
}

// reconcileNodes gets the current nodeIP in use from controller cluster CR and compares it with
// the current nodeIpList (nodeIpList contains list of externalIPs or a single nodeIP if provided by user)
// if the nodeIP is no longer available , we update the cluster CR on controller cluster
func (r *SliceGwReconciler) reconcileNodes(ctx context.Context, slicegateway *kubeslicev1beta1.SliceGateway) error {
	log := r.Log
	//currentNodeIP and nodeIpList would be same in case of operator restart because it is set at the start of operator in main.go, hence it is better to fetch the nodeIP in use from controller cluster CR!
	//TODO: can we store nodeIP in slicegw?
	currentNodeIP, err := r.HubClient.GetClusterNodeIP(ctx, os.Getenv("CLUSTER_NAME"), os.Getenv("HUB_PROJECT_NAMESPACE"))
	if err != nil {
		return err
	}
	nodeIpList := cluster.GetNodeExternalIpList()
	if len(nodeIpList) == 0 {
		//err := errors.New("node IP list is empty")
		return nil
	}
	if !validatenodeipcount(nodeIpList, currentNodeIP) {
		//nodeIP updated , update the cluster CR
		log.Info("Mismatch in node IP", "IP in use", currentNodeIP, "IP to be used", nodeIpList)
		err := r.HubClient.UpdateNodeIpInCluster(ctx, os.Getenv("CLUSTER_NAME"), nodeIpList, os.Getenv("HUB_PROJECT_NAMESPACE"), slicegateway)
		if err != nil {
			return err
		}
		r.NodeIPs = nodeIpList
	}
	return nil
}

func (r *SliceGwReconciler) reconcileGatewayHeadlessService(ctx context.Context, sliceGw *kubeslicev1beta1.SliceGateway) error {
	log := r.Log
	serviceFound := corev1.Service{}
	err := r.Get(ctx, types.NamespacedName{Namespace: sliceGw.Namespace, Name: sliceGw.Status.Config.SliceGatewayRemoteGatewayID}, &serviceFound)
	if err != nil {
		if errors.IsNotFound(err) {
			// Create a new service with same name as SliceGatewayRemoteGatewayID , because --remote flag of openvpn client is populated with same name. So it would call this svc to get a server IP(through endpoint)
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
		if errors.IsNotFound(err) {
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
	// endpoint already exists , check if sliceGatewayRemoteNodeIp is changed then update the endpoint
	toUpdate := false
	debugLog.Info("SliceGatewayRemoteNodeIP", "SliceGatewayRemoteNodeIP", sliceGw.Status.Config.SliceGatewayRemoteNodeIPs)
	if !validateEndpointAddresses(endpointFound.Subsets[0], sliceGw.Status.Config.SliceGatewayRemoteNodeIPs) {
		debugLog.Info("Updating the Endpoint, since sliceGatewayRemoteNodeIp has changed", "from endpointFound", endpointFound.Subsets[0].Addresses[0].IP)
		endpointFound.Subsets[0].Addresses = getAddrSlice(sliceGw.Status.Config.SliceGatewayRemoteNodeIPs)
		toUpdate = true
	}
	if toUpdate {
		err := r.Update(ctx, &endpointFound)
		if err != nil {
			log.Error(err, "Error updating Endpoint")
			return true, ctrl.Result{}, err
		}
	}
	return false, ctrl.Result{}, nil
}

func (r *SliceGwReconciler) isRouteRemoved(slicegw *kubeslicev1beta1.SliceGateway, podName string) bool {
	for _, gwPod := range slicegw.Status.GatewayPodStatus {
		if gwPod.PodName == podName {
			return gwPod.RouteRemoved == 1
		}
	}
	return false
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
func getAddrSlice(nodeIPS []string) []corev1.EndpointAddress {
	endpointSlice := make([]corev1.EndpointAddress, 0)
	for _, ip := range nodeIPS {
		endpointSlice = append(endpointSlice, corev1.EndpointAddress{IP: ip})
	}
	return endpointSlice
}
func validateEndpointAddresses(subset corev1.EndpointSubset, remoteNodeIPS []string) bool {
	addrSlice := subset.Addresses
	for _, address := range addrSlice {
		if !contains(remoteNodeIPS, address.IP) {
			return false
		}
	}
	return true
}

// total -> external ip list of nodes in the k8s cluster
// current -> ip list present in nodeIPs of cluster cr
func validatenodeipcount(total, current []string) bool {
	return reflect.DeepEqual(total, current)
}
func UpdateGWPodStatus(gwPodStatus []*kubeslicev1beta1.GwPodInfo, podName string) []*kubeslicev1beta1.GwPodInfo {
	index := -1
	for i, _ := range gwPodStatus {
		if gwPodStatus[i].PodName == podName {
			index = i
			break
		}
	}
	return append(gwPodStatus[:index], gwPodStatus[index+1:]...)

}
func getLocalNSMIPs(slicegateway *kubeslicev1beta1.SliceGateway) []string {
	nsmIPs := make([]string, 0)
	for i, _ := range slicegateway.Status.GatewayPodStatus {
		nsmIPs = append(nsmIPs, slicegateway.Status.GatewayPodStatus[i].LocalNsmIP)
	}
	return nsmIPs
}
func getLocalNSMIPsForRouter(slicegateway *kubeslicev1beta1.SliceGateway) []string {
	nsmIPs := make([]string, 0)
	for _, gwPod := range slicegateway.Status.GatewayPodStatus {
		if gwPod.RouteRemoved == 1 || gwPod.LocalNsmIP == "" {
			continue
		}
		nsmIPs = append(nsmIPs, gwPod.LocalNsmIP)
	}
	return nsmIPs
}
func getPodIPs(slicegateway *kubeslicev1beta1.SliceGateway) []string {
	podIPs := make([]string, 0)
	for i, _ := range slicegateway.Status.GatewayPodStatus {
		podIPs = append(podIPs, slicegateway.Status.GatewayPodStatus[i].PodIP)
	}
	return podIPs
}
func getPodNames(slicegateway *kubeslicev1beta1.SliceGateway) []string {
	podNames := make([]string, 0)
	for i, _ := range slicegateway.Status.GatewayPodStatus {
		podNames = append(podNames, slicegateway.Status.GatewayPodStatus[i].PodName)
	}
	return podNames
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
func getPodAntiAffinity(slice string) *corev1.PodAntiAffinity {
	return &corev1.PodAntiAffinity{
		PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{{
			Weight: 100,
			PodAffinityTerm: corev1.PodAffinityTerm{
				TopologyKey: "kubernetes.io/hostname",
				LabelSelector: &metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{{
						Key:      controllers.PodTypeSelectorLabelKey,
						Operator: metav1.LabelSelectorOpIn,
						Values:   []string{"slicegateway"},
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

func (r *SliceGwReconciler) getNewestPod(slicegw *kubeslicev1beta1.SliceGateway) (*corev1.Pod, error) {
	PodList := corev1.PodList{}
	labels := map[string]string{"kubeslice.io/pod-type": "slicegateway", controllers.ApplicationNamespaceSelectorLabelKey: slicegw.Spec.SliceName,
		"kubeslice.io/slice-gw": slicegw.Name}
	listOptions := []client.ListOption{
		client.MatchingLabels(labels),
	}
	ctx := context.Background()
	err := r.List(ctx, &PodList, listOptions...)
	if err != nil {
		return &corev1.Pod{}, err
	}
	newestPod := PodList.Items[0]
	newestPodDuration := time.Since(PodList.Items[0].CreationTimestamp.Time).Seconds()
	for _, pod := range PodList.Items {
		duration := time.Since(pod.CreationTimestamp.Time).Seconds()
		if duration < newestPodDuration {
			newestPodDuration = duration
			newestPod = pod
		}
	}
	return &newestPod, nil
}

// isRebalancingRequired checks if rebalancing needs to be performed
func (r *SliceGwReconciler) isRebalancingRequired(ctx context.Context, sliceGw *kubeslicev1beta1.SliceGateway) (bool, error) {
	log := r.Log
	//fetch the slicegateway deployment
	var readyReplica int
	deployments, err := r.getDeployments(ctx, sliceGw)
	if err != nil {
		log.Error(err, "Error while fetching deployments")
		return false, err
	}
	for _, foundDep := range deployments.Items {
		replicas := foundDep.Status.ReadyReplicas
		readyReplica += int(replicas)
	}

	//get the minimum number of pods that have to be associated with a node
	nodeCount := len(cluster.GetNodeExternalIpList())
	MinNumberOfPodsReq := math.Ceil(float64(readyReplica) / float64(nodeCount))

	log.Info("rebalancing reqd?", "nodeCount", nodeCount, "replicas", readyReplica, "MinNumberOfPodsReq", MinNumberOfPodsReq)

	//check if rebalancing is required
	nodeToPodMap := make(map[string]int32)
	PodList := corev1.PodList{}
	labels := map[string]string{controllers.PodTypeSelectorLabelKey: "slicegateway","kubeslice.io/slice-gw":sliceGw.Name}
	listOptions := []client.ListOption{
		client.MatchingLabels(labels),
	}
	err = r.Client.List(ctx, &PodList, listOptions...)
	if err != nil {
		log.Error(err, "can't fetch pod list:")
		return false, err
	}

	if len(PodList.Items) == 0 {
		log.Error(err, "the pods list is empty")
		return false, err
	}
	nodeList := corev1.NodeList{}
	nodeLabels := map[string]string{controllers.NodeTypeSelectorLabelKey: "gateway"}
	listOpts := []client.ListOption{
		client.MatchingLabels(nodeLabels),
	}
	if err := r.List(ctx, &nodeList, listOpts...); err != nil {
		log.Error(err, "Error getting kubeslice nodeList")
		return false, err
	}
	if len(nodeList.Items) == 0 {
		// no gateway nodes found
		return false, fmt.Errorf("no gateway nodes available")
	}
	//populate the map with key as node name and the value with the number of pods that particular node holds
	for _, pod := range PodList.Items {
		if _, ok := nodeToPodMap[pod.Spec.NodeName]; !ok {
			nodeToPodMap[pod.Spec.NodeName] = 1
		} else {
			nodeToPodMap[pod.Spec.NodeName] += 1
		}
	}
	//update the map with new nodes
	newNodeAdded := false
	for _, node := range nodeList.Items {
		if _, ok := nodeToPodMap[node.Name]; !ok {
			nodeToPodMap[node.Name] = 0
			newNodeAdded = true
		}
	}

	r.Log.Info("nodeToPodMap", "nodeToPodMap", nodeToPodMap)
	for _, pods := range nodeToPodMap {
		if (pods > int32(MinNumberOfPodsReq)) && newNodeAdded {
			return true, nil
		}
	}
	return false, nil
}
