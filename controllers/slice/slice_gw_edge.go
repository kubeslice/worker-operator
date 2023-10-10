/*
 *  Copyright (c) 2023 Avesha, Inc. All rights reserved.
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
	"os"

	"github.com/kubeslice/slicegw-edge/pkg/edgeservice"
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	"github.com/kubeslice/worker-operator/controllers"
	"github.com/kubeslice/worker-operator/pkg/gatewayedge"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func labelsForSliceGatewayEdgeDeployment(sliceName string) map[string]string {
	return map[string]string{
		controllers.SliceGatewaySelectorLabelKey: sliceName,
		"kubeslice.io/app":                       "slice-gw-edge",
	}
}

func labelsForSliceGatewayEdgeSvc(sliceName string) map[string]string {
	return map[string]string{
		controllers.SliceGatewaySelectorLabelKey: sliceName,
		controllers.SliceGatewayEdgeTypeLabelKey: "LoadBalancer",
	}
}

func (r *SliceReconciler) getSliceGatewayEdgeServices(ctx context.Context, slice *kubeslicev1beta1.Slice) (*corev1.ServiceList, error) {
	listOpts := []client.ListOption{
		client.MatchingLabels(labelsForSliceGatewayEdgeSvc(slice.Name)),
		client.InNamespace(controllers.ControlPlaneNamespace),
	}
	services := corev1.ServiceList{}
	if err := r.List(ctx, &services, listOpts...); err != nil {
		return nil, err
	}
	return &services, nil
}

func (r *SliceReconciler) getSliceGatewayEdgeDeployments(ctx context.Context, slice *kubeslicev1beta1.Slice) (*appsv1.DeploymentList, error) {
	listOpts := []client.ListOption{
		client.MatchingLabels(labelsForSliceGatewayEdgeDeployment(slice.Name)),
		client.InNamespace(controllers.ControlPlaneNamespace),
	}
	deployments := appsv1.DeploymentList{}
	if err := r.List(ctx, &deployments, listOpts...); err != nil {
		return nil, err
	}

	return &deployments, nil
}

func getPortListForEdgeSvc(portmap *map[string]int32) *[]corev1.ServicePort {
	ports := []corev1.ServicePort{}
	for sliceGwSvcName, sliceGwSvcPort := range *portmap {
		ports = append(ports, corev1.ServicePort{
			Name:     sliceGwSvcName,
			Protocol: corev1.ProtocolUDP,
			Port:     int32(sliceGwSvcPort),
		})
	}

	return &ports
}

func serviceForSliceGatewayEdge(sliceName, svcName string, portmap *map[string]int32) *corev1.Service {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      svcName,
			Namespace: controllers.ControlPlaneNamespace,
			Labels:    labelsForSliceGatewayEdgeSvc(sliceName),
		},
		Spec: corev1.ServiceSpec{
			Type:     "LoadBalancer",
			Selector: labelsForSliceGatewayEdgeDeployment(sliceName),
			Ports:    *getPortListForEdgeSvc(portmap),
		},
	}

	return svc
}

func (r *SliceReconciler) createSliceGatewayEdgeService(ctx context.Context, slice *kubeslicev1beta1.Slice, portmap *map[string]int32) error {
	log := r.Log.WithValues("slice", slice.Name)
	svc := serviceForSliceGatewayEdge(slice.Name, "svc-"+slice.Name+"-gw-edge", portmap)
	ctrl.SetControllerReference(slice, svc, r.Scheme)

	err := r.Create(ctx, svc)
	if err != nil {
		log.Error(err, "Failed to create slice gateway edge service", "Name", svc.Name)
		return err
	}

	return nil
}

func allPortsAccountedInEdgeSvc(gwEdgeSvc *corev1.Service, portmap *map[string]int32) bool {
	svcPortMap := make(map[string]int32)

	for _, svcPort := range gwEdgeSvc.Spec.Ports {
		svcPortMap[svcPort.Name] = svcPort.Port
	}

	if len(svcPortMap) != len(*portmap) {
		return false
	}

	for sliceGwSvcName, sliceGwPort := range *portmap {
		svcPort, found := svcPortMap[sliceGwSvcName]
		if !found || svcPort != sliceGwPort {
			return false
		}
	}

	return true
}

func (r *SliceReconciler) reconcileSliceGatewayEdgeService(ctx context.Context, slice *kubeslicev1beta1.Slice) (ctrl.Result, error, bool) {
	log := r.Log.WithValues("slice", slice.Name)
	debugLog := log.V(1)
	sliceGwSvcList, err := controllers.GetSliceGwServices(ctx, r.Client, slice.Name)
	if err != nil {
		return ctrl.Result{}, err, true
	}

	if len(sliceGwSvcList.Items) == 0 {
		return ctrl.Result{}, nil, false
	}

	// Build portmap
	portmap := make(map[string]int32)
	for _, sliceGwSvc := range sliceGwSvcList.Items {
		portmap[sliceGwSvc.Name] = sliceGwSvc.Spec.Ports[0].NodePort
	}

	debugLog.Info("portmap from slicegw svcs", "portmap", portmap)

	gwEdgeSvc, err := r.getSliceGatewayEdgeServices(ctx, slice)
	if err != nil {
		if apierrors.IsNotFound(err) {
			err := r.createSliceGatewayEdgeService(ctx, slice, &portmap)
			if err != nil {
				return ctrl.Result{}, err, true
			}
			return ctrl.Result{Requeue: true}, nil, true
		}

		return ctrl.Result{}, err, true
	}

	if gwEdgeSvc == nil || len(gwEdgeSvc.Items) == 0 {
		err := r.createSliceGatewayEdgeService(ctx, slice, &portmap)
		if err != nil {
			return ctrl.Result{}, err, true
		}

		return ctrl.Result{Requeue: true}, nil, true
	}

	// Check if an update is needed.
	// An update is needed if there is a new slice gw pair added or an old one deleted.
	// The port list in the LB service must include all the NodePorts of the slice gw servers.
	if !allPortsAccountedInEdgeSvc(&gwEdgeSvc.Items[0], &portmap) {
		gwEdgeSvc.Items[0].Spec.Ports = *getPortListForEdgeSvc(&portmap)
		log.Info("Updating edge svc", "updated port list", gwEdgeSvc.Items[0].Spec.Ports)
		err := r.Update(ctx, &gwEdgeSvc.Items[0])
		if err != nil {
			return ctrl.Result{}, err, true
		}
	}

	return ctrl.Result{}, nil, false
}

func deploymentForSliceGatewayEdge(sliceName, depName string) *appsv1.Deployment {
	var replicas int32 = 1
	var privileged = true

	gwEdgeImg := os.Getenv("AVESHA_SLICE_GW_EDGE_IMAGE")
	if gwEdgeImg == "" {
		// TODO: Push the default image to aveshalabs nexus
		gwEdgeImg = "aveshatest/kubeslice-gateway-edge:1.0.0"
	}

	imgPullPolicy := corev1.PullAlways

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      depName,
			Namespace: controllers.ControlPlaneNamespace,
			Labels:    labelsForSliceGatewayEdgeDeployment(sliceName),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labelsForSliceGatewayEdgeDeployment(sliceName),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labelsForSliceGatewayEdgeDeployment(sliceName),
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "kubeslice-gateway-edge",
					Containers: []corev1.Container{{
						Name:            "kubeslice-gateway-edge",
						Image:           gwEdgeImg,
						ImagePullPolicy: imgPullPolicy,
						Env: []corev1.EnvVar{
							{
								Name:  "SLICE_NAME",
								Value: sliceName,
							},
							{
								Name:  "POD_TYPE",
								Value: "GATEWAY_EDGE_POD",
							},
						},
						SecurityContext: &corev1.SecurityContext{
							Privileged: &privileged,
							Capabilities: &corev1.Capabilities{
								Add: []corev1.Capability{
									"NET_ADMIN",
								},
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

	return dep
}

func (r *SliceReconciler) createSliceGatewayEdgeDeployment(ctx context.Context, slice *kubeslicev1beta1.Slice) error {
	log := r.Log.WithValues("slice", slice.Name)
	deployment := deploymentForSliceGatewayEdge(slice.Name, slice.Name+"-gw-edge")
	ctrl.SetControllerReference(slice, deployment, r.Scheme)

	err := r.Create(ctx, deployment)
	if err != nil {
		log.Error(err, "Failed to create slice gateway edge deployment")
		return err
	}

	return nil
}

func (r *SliceReconciler) getSliceGatewayEdgePods(ctx context.Context, sliceName string) (*[]corev1.Pod, error) {
	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(controllers.ControlPlaneNamespace),
		client.MatchingLabels(labelsForSliceGatewayEdgeDeployment(sliceName)),
	}
	if err := r.List(ctx, podList, listOpts...); err != nil {
		return nil, err
	}

	healthyGwEdgePods := []corev1.Pod{}
	for _, pod := range podList.Items {
		if pod.Status.Phase == corev1.PodRunning && pod.ObjectMeta.DeletionTimestamp == nil {
			healthyGwEdgePods = append(healthyGwEdgePods, pod)
		}
	}

	return &healthyGwEdgePods, nil
}

// Send the mapping between the NodePort number of the VPN service and the clusterIP of that service.
// The inter-cluster traffic coming into the slice passes through the LB and goes to the edge pod. The edge
// has to decide which VPN server pod the traffic needs to be forwarded to. The distinction is made using the
// destination port number of the traffic. It would be the NodePort number of the recipient VPN server.
func (r *SliceReconciler) syncSliceGwServiceMap(ctx context.Context, slice *kubeslicev1beta1.Slice) error {
	log := r.Log.WithValues("slice", slice.Name)
	gwEdgePodList, err := r.getSliceGatewayEdgePods(ctx, slice.Name)
	if err != nil {
		return err
	}

	if len(*gwEdgePodList) == 0 {
		log.Info("Slice gateway edge pods not ready yet...")
		return nil
	}

	sliceGwSvcList, err := controllers.GetSliceGwServices(ctx, r.Client, slice.Name)
	if err != nil {
		return err
	}

	// Construct the grpc message
	svcmap := gatewayedge.SliceGwServiceMap{}
	svcmap.SliceName = slice.Name
	for _, svc := range sliceGwSvcList.Items {
		svcLabels := svc.ObjectMeta.Labels
		if svcLabels == nil {
			continue
		}
		if svcLabels != nil {
			_, found := svcLabels["kubeslice.io/slicegw"]
			if !found {
				continue
			}
		}
		sliceGwSvcInstance := &gatewayedge.SliceGwServiceInfo{
			SliceGwServiceInfo: edgeservice.SliceGwServiceInfo{
				GwSvcName:       svc.Name,
				GwSvcClusterIP:  svc.Spec.ClusterIP,
				GwSvcNodePort:   uint32(svc.Spec.Ports[0].NodePort),
				GwSvcTargetPort: uint32(svc.Spec.Ports[0].TargetPort.IntVal),
			},
		}

		svcmap.SliceGwServiceList = append(svcmap.SliceGwServiceList, &sliceGwSvcInstance.SliceGwServiceInfo)
	}

	for _, edgePod := range *gwEdgePodList {
		grpcAddress := edgePod.Status.PodIP + ":5000"
		_, err := r.WorkerGatewayEdgeClient.UpdateSliceGwServiceMap(ctx, grpcAddress, &svcmap)
		if err != nil {
			log.Error(err, "Failed to send slice gw service info to edge pod", "addr", grpcAddress)
			return err
		}
	}

	return nil
}

func (r *SliceReconciler) reconcileSliceGatewayEdgeDeployment(ctx context.Context, slice *kubeslicev1beta1.Slice) (ctrl.Result, error, bool) {
	gwEdgeDeployments, err := r.getSliceGatewayEdgeDeployments(ctx, slice)
	if err != nil {
		if apierrors.IsNotFound(err) {
			err := r.createSliceGatewayEdgeDeployment(ctx, slice)
			if err != nil {
				return ctrl.Result{}, err, true
			}
			return ctrl.Result{Requeue: true}, nil, true
		}

		return ctrl.Result{}, err, true
	}

	if gwEdgeDeployments == nil || len(gwEdgeDeployments.Items) == 0 {
		err := r.createSliceGatewayEdgeDeployment(ctx, slice)
		if err != nil {
			return ctrl.Result{}, err, true
		}

		return ctrl.Result{Requeue: true}, nil, true
	}

	return ctrl.Result{}, nil, false
}

func (r *SliceReconciler) ReconcileSliceGwEdge(ctx context.Context, slice *kubeslicev1beta1.Slice) (ctrl.Result, error, bool) {
	if slice.Status.SliceConfig.SliceGatewayServiceType != "LoadBalancer" && os.Getenv("ENABLE_GW_LB_EDGE") == "" {
		return ctrl.Result{}, nil, false
	}

	// There would be one slice gateway edge deployment that would handle traffic for all the
	// cluster pairs of a slice. It is only created on clusters that are marked to be VPN servers.
	// The edge is a simple passthrough proxy that does not terminate any
	// network connections, it merely applies NAT rules to redirect traffic to the right slice gateway
	// server pods.
	res, err, requeue := r.reconcileSliceGatewayEdgeDeployment(ctx, slice)
	if err != nil {
		return ctrl.Result{}, err, true
	}
	if requeue {
		return res, nil, true
	}

	// The edge needs to know the mapping of port numbers to the clusterIP of the VPN services. It needs this
	// info to set up the NATing rules.
	err = r.syncSliceGwServiceMap(ctx, slice)
	if err != nil {
		return ctrl.Result{}, err, true
	}

	return r.reconcileSliceGatewayEdgeService(ctx, slice)
}
