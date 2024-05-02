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

package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/kubeslice/apis/pkg/controller/v1alpha1"
	spokev1alpha1 "github.com/kubeslice/apis/pkg/worker/v1alpha1"
	"github.com/kubeslice/kubeslice-monitoring/pkg/events"
	"github.com/kubeslice/kubeslice-monitoring/pkg/metrics"
	ossEvents "github.com/kubeslice/worker-operator/events"

	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	"github.com/kubeslice/worker-operator/pkg/gwsidecar"
	"github.com/kubeslice/worker-operator/pkg/logger"
	"github.com/kubeslice/worker-operator/pkg/utils"
	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type component struct {
	name          string
	labels        map[string]string
	ns            string
	ignoreMissing bool
}

func NewSliceReconciler(hubclient client.Client, mc client.Client, er *events.EventRecorder, mf metrics.MetricsFactory) *SliceReconciler {
	return &SliceReconciler{
		Client:            hubclient,
		MeshClient:        mc,
		EventRecorder:     er,
		Log:               ctrl.Log.WithName("hub").WithName("controllers").WithName("SliceConfig"),
		ReconcileInterval: 120 * time.Second,

		counterSliceCreated: mf.NewCounter("slice_created_total", "Slice created in worker", []string{"slice"}),
		counterSliceUpdated: mf.NewCounter("slice_updated_total", "Slice updated in worker", []string{"slice"}),
		counterSliceDeleted: mf.NewCounter("slice_deleted_total", "Slice deleted in worker", []string{"slice"}),

		counterSliceCreationFailed: mf.NewCounter("slice_creation_failed_total", "Slice creation failed in worker", []string{"slice"}),
		counterSliceUpdationFailed: mf.NewCounter("slice_updation_failed_total", "Slice updation failed in worker", []string{"slice"}),
		counterSliceDeletionFailed: mf.NewCounter("slice_deletion_failed_total", "Slice deletion failed in worker", []string{"slice"}),
		gaugeSliceUp:               mf.NewGauge("slice_up", "Kubeslice slice health status", []string{"slice"}),
		gaugeComponentUp:           mf.NewGauge("slice_component_up", "Kubeslice slice component health status", []string{"slice", "slice_component"}),
	}
}

var components = []component{
	{
		name: "dns",
		labels: map[string]string{
			"app": "kubeslice-dns",
		},
		ns: ControlPlaneNamespace,
	},
	{
		name: "slice-gateway",
		labels: map[string]string{
			"kubeslice.io/pod-type": "slicegateway",
		},
		ns: ControlPlaneNamespace,
	},
	{
		name: "slice-router",
		labels: map[string]string{
			"kubeslice.io/pod-type": "router",
		},
		ns: ControlPlaneNamespace,
	},
	{
		name: "egress",
		labels: map[string]string{
			"istio": "egressgateway",
		},
		ns:            ControlPlaneNamespace,
		ignoreMissing: true,
	},
	{
		name: "ingress",
		labels: map[string]string{
			"istio": "ingressgateway",
		},
		ns:            ControlPlaneNamespace,
		ignoreMissing: true,
	},
	{
		name: "gateway-tunnel",
		labels: map[string]string{
			"kubeslice.io/pod-type": "slicegateway",
		},
		ns: ControlPlaneNamespace,
	},
}

type SliceReconciler struct {
	client.Client
	Log               logr.Logger
	MeshClient        client.Client
	EventRecorder     *events.EventRecorder
	ReconcileInterval time.Duration

	// metrics
	counterSliceCreated        *prometheus.CounterVec
	counterSliceUpdated        *prometheus.CounterVec
	counterSliceDeleted        *prometheus.CounterVec
	counterSliceCreationFailed *prometheus.CounterVec
	counterSliceUpdationFailed *prometheus.CounterVec
	counterSliceDeletionFailed *prometheus.CounterVec
	gaugeSliceUp               *prometheus.GaugeVec
	gaugeComponentUp           *prometheus.GaugeVec
}

var sliceFinalizer = "controller.kubeslice.io/hubSpokeSlice-finalizer"
var sliceControllerName string = "workerSliceController"

func (r *SliceReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := r.Log.WithValues("sliceconfig", req.NamespacedName)
	ctx = logger.WithLogger(ctx, log)
	debuglog := log.V(1)
	slice := &spokev1alpha1.WorkerSliceConfig{}

	err := r.Get(ctx, req.NamespacedName, slice)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Return and don't requeue
			log.Info("Slice resource not found in hub. Ignoring since object must be deleted")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	log.Info("got slice from hub", "slice", slice.Name)
	debuglog.Info("got slice from hub", "slice", slice)
	sliceName := slice.Spec.SliceName
	*r.EventRecorder = (*r.EventRecorder).WithSlice(sliceName)
	requeue, result, err := r.handleSliceDeletion(slice, ctx, req)
	if requeue {
		return result, err
	}

	meshSlice := &kubeslicev1beta1.Slice{}
	sliceRef := client.ObjectKey{
		Name:      sliceName,
		Namespace: ControlPlaneNamespace,
	}

	err = r.MeshClient.Get(ctx, sliceRef, meshSlice)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, create it in the spoke cluster
			log.Info("Slice resource not found in spoke cluster, creating")

			// Fetch project namespace label
			var projectNs string
			if metav1.HasLabel(slice.ObjectMeta, "project-namespace") {
				projectNs = slice.ObjectMeta.GetLabels()["project-namespace"]
			}

			s := &kubeslicev1beta1.Slice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      sliceName,
					Namespace: ControlPlaneNamespace,
					Labels: map[string]string{
						"project-namespace": projectNs,
					},
				},
				Spec: kubeslicev1beta1.SliceSpec{},
			}

			err = r.MeshClient.Create(ctx, s)
			if err != nil {
				log.Error(err, "unable to create slice in spoke cluster", "slice", s)
				utils.RecordEvent(ctx, r.EventRecorder, slice, nil, ossEvents.EventSliceCreationFailed, sliceControllerName)
				r.counterSliceCreationFailed.WithLabelValues(s.Name).Add(1)
				return reconcile.Result{}, err
			}
			log.Info("slice created in spoke cluster")
			r.counterSliceCreated.WithLabelValues(s.Name).Add(1)
			utils.RecordEvent(ctx, r.EventRecorder, slice, nil, ossEvents.EventSliceCreated, sliceControllerName)
			err = r.updateSliceConfig(ctx, s, slice)
			if err != nil {
				log.Error(err, "unable to update slice status in spoke cluster", "slice", s)
				return reconcile.Result{}, err
			}
			log.Info("slice status updated in spoke cluster")
			utils.RecordEvent(ctx, r.EventRecorder, slice, nil, ossEvents.EventWorkerSliceConfigUpdated, sliceControllerName)
			return reconcile.Result{RequeueAfter: r.ReconcileInterval}, nil
		}
		r.counterSliceUpdationFailed.WithLabelValues(sliceName).Add(1)
		return reconcile.Result{}, err
	}

	err = r.updateSliceConfig(ctx, meshSlice, slice)
	if err != nil {
		r.counterSliceUpdationFailed.WithLabelValues(sliceName).Add(1)
		log.Error(err, "unable to update slice status in spoke cluster", "slice", meshSlice)
		return reconcile.Result{}, err
	}
	if slice.Status.SliceHealth == nil {
		slice.Status.SliceHealth = &spokev1alpha1.SliceHealth{}
	}
	err = r.updateSliceHealth(ctx, slice)
	if err != nil {
		log.Error(err, "unable to update slice health status in hub cluster", "workerSlice", slice)
		r.counterSliceUpdationFailed.WithLabelValues(sliceName).Add(1)
		return reconcile.Result{}, err
	}

	r.UpdateSliceHealthMetrics(slice)

	if time.Since(slice.Status.SliceHealth.LastUpdated.Time) > r.ReconcileInterval {
		slice.Status.SliceHealth.LastUpdated = metav1.Now()
		if err := r.Status().Update(ctx, slice); err != nil {
			log.Error(err, "unable to update slice CR")
			utils.RecordEvent(ctx, r.EventRecorder, slice, nil, ossEvents.EventWorkerSliceHealthUpdateFailed, sliceControllerName)
			r.counterSliceUpdationFailed.WithLabelValues(sliceName).Add(1)
			return reconcile.Result{}, err
		} else {
			utils.RecordEvent(ctx, r.EventRecorder, slice, nil, ossEvents.EventWorkerSliceHealthUpdated, sliceControllerName)
			log.Info("succesfully updated the slice CR ", "slice CR ", slice.Name)
		}
	}
	r.counterSliceUpdated.WithLabelValues(sliceName).Add(1)
	return reconcile.Result{RequeueAfter: r.ReconcileInterval}, nil
}

func (r *SliceReconciler) updateSliceConfig(ctx context.Context, meshSlice *kubeslicev1beta1.Slice, spokeSlice *spokev1alpha1.WorkerSliceConfig) error {
	if meshSlice.Status.SliceConfig == nil {
		meshSlice.Status.SliceConfig = &kubeslicev1beta1.SliceConfig{
			SliceDisplayName: spokeSlice.Spec.SliceName,
			SliceSubnet:      spokeSlice.Spec.SliceSubnet,
			SliceIpam: kubeslicev1beta1.SliceIpamConfig{
				SliceIpamType:    spokeSlice.Spec.SliceIpamType,
				IpamClusterOctet: spokeSlice.Spec.IpamClusterOctet,
			},
			ClusterSubnetCIDR: spokeSlice.Spec.ClusterSubnetCIDR,
			SliceType:         spokeSlice.Spec.SliceType,
		}
	}
	if meshSlice.Status.SliceConfig.SliceSubnet == "" {
		meshSlice.Status.SliceConfig.SliceSubnet = spokeSlice.Spec.SliceSubnet
	}
	if meshSlice.ObjectMeta.Labels == nil {
		meshSlice.ObjectMeta.Labels = make(map[string]string)
		if spokeSlice.ObjectMeta.Labels != nil {
			meshSlice.ObjectMeta.Labels = spokeSlice.ObjectMeta.Labels
		}
	}

	if meshSlice.Status.SliceConfig.SliceIpam.IpamClusterOctet == 0 {
		meshSlice.Status.SliceConfig.SliceIpam.IpamClusterOctet = spokeSlice.Spec.IpamClusterOctet
	}

	if meshSlice.Status.SliceConfig.ClusterSubnetCIDR == "" || meshSlice.Status.SliceConfig.ClusterSubnetCIDR != spokeSlice.Spec.ClusterSubnetCIDR {
		meshSlice.Status.SliceConfig.ClusterSubnetCIDR = spokeSlice.Spec.ClusterSubnetCIDR
	}

	meshSlice.Status.SliceConfig.SliceGatewayServiceType = spokeSlice.Spec.SliceGatewayProvider.SliceGatewayServiceType
	meshSlice.Status.SliceConfig.SliceGatewayProtocol = spokeSlice.Spec.SliceGatewayProvider.SliceGatewayProtocol

	meshSlice.Status.SliceConfig.SliceOverlayNetworkDeploymentMode = spokeSlice.Spec.OverlayNetworkDeploymentMode

	meshSlice.Status.SliceConfig.QosProfileDetails = kubeslicev1beta1.QosProfileDetails{
		QueueType:               spokeSlice.Spec.QosProfileDetails.QueueType,
		BandwidthCeilingKbps:    spokeSlice.Spec.QosProfileDetails.BandwidthCeilingKbps,
		BandwidthGuaranteedKbps: spokeSlice.Spec.QosProfileDetails.BandwidthGuaranteedKbps,
		DscpClass:               spokeSlice.Spec.QosProfileDetails.DscpClass,
		TcType:                  spokeSlice.Spec.QosProfileDetails.TcType,
		Priority:                spokeSlice.Spec.QosProfileDetails.Priority,
	}

	meshSlice.Status.SliceConfig.NamespaceIsolationProfile = &kubeslicev1beta1.NamespaceIsolationProfile{
		IsolationEnabled:      spokeSlice.Spec.NamespaceIsolationProfile.IsolationEnabled,
		AllowedNamespaces:     spokeSlice.Spec.NamespaceIsolationProfile.AllowedNamespaces,
		ApplicationNamespaces: spokeSlice.Spec.NamespaceIsolationProfile.ApplicationNamespaces,
	}

	extGwCfg := spokeSlice.Spec.ExternalGatewayConfig
	meshSlice.Status.SliceConfig.ExternalGatewayConfig = &kubeslicev1beta1.ExternalGatewayConfig{
		GatewayType: extGwCfg.GatewayType,
		Egress: &kubeslicev1beta1.ExternalGatewayConfigOptions{
			Enabled: extGwCfg.Egress.Enabled,
		},
		Ingress: &kubeslicev1beta1.ExternalGatewayConfigOptions{
			Enabled: extGwCfg.Ingress.Enabled,
		},
		NsIngress: &kubeslicev1beta1.ExternalGatewayConfigOptions{
			Enabled: extGwCfg.NsIngress.Enabled,
		},
	}

	return r.MeshClient.Status().Update(ctx, meshSlice)
}

func (a *SliceReconciler) InjectClient(c client.Client) error {
	a.Client = c
	return nil
}

func (r *SliceReconciler) deleteSliceResourceOnSpoke(ctx context.Context, slice *spokev1alpha1.WorkerSliceConfig) error {
	log := logger.FromContext(ctx)
	sliceOnSpoke := &kubeslicev1beta1.Slice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      slice.Spec.SliceName,
			Namespace: ControlPlaneNamespace,
		},
	}
	if err := r.MeshClient.Delete(ctx, sliceOnSpoke); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}
	log.Info("Deleted Slice CR on spoke cluster", "slice", sliceOnSpoke.Name)
	return nil
}

func (r *SliceReconciler) handleSliceDeletion(slice *spokev1alpha1.WorkerSliceConfig, ctx context.Context, req reconcile.Request) (bool, reconcile.Result, error) {
	sliceName := slice.Spec.SliceName
	if slice.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !controllerutil.ContainsFinalizer(slice, sliceFinalizer) {
			controllerutil.AddFinalizer(slice, sliceFinalizer)
			if err := r.Update(ctx, slice); err != nil {
				return true, reconcile.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(slice, sliceFinalizer) {
			// our finalizer is present, so lets handle any external dependency
			if err := r.deleteSliceResourceOnSpoke(ctx, slice); err != nil {
				// if fail to delete the external dependency here, return with error
				// so that it can be retried
				r.counterSliceDeletionFailed.WithLabelValues(sliceName).Add(1)
				return true, reconcile.Result{}, err
			}
			// remove our finalizer from the spokeslice and update it.
			// retry on conflict
			err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				//fetch the latest spokeslice from hub
				if err := r.Get(ctx, req.NamespacedName, slice); err != nil {
					r.counterSliceDeletionFailed.WithLabelValues(sliceName).Add(1)
					return err
				}
				//remove the finalizer
				controllerutil.RemoveFinalizer(slice, sliceFinalizer)
				if err := r.Update(ctx, slice); err != nil {
					r.counterSliceDeletionFailed.WithLabelValues(sliceName).Add(1)
					return err
				}
				r.counterSliceDeleted.WithLabelValues(sliceName).Add(1)
				return nil
			})
			if err != nil {
				return true, reconcile.Result{}, err
			}
		}
		// Stop reconciliation as the item is being deleted
		return true, reconcile.Result{}, nil
	}
	return false, reconcile.Result{}, nil
}

func (r *SliceReconciler) updateSliceHealth(ctx context.Context, slice *spokev1alpha1.WorkerSliceConfig) error {
	log := logger.FromContext(ctx)
	debuglog := log.V(1)
	slice.Status.SliceHealth.ComponentStatuses = []spokev1alpha1.ComponentStatus{}
	slice.Status.SliceHealth.SliceHealthStatus = spokev1alpha1.SliceHealthStatusNormal
	if slice.Spec.OverlayNetworkDeploymentMode == v1alpha1.NONET {
		debuglog.Info("skipping workerslice health check for no-network deployment")
		return nil
	}
	// check for components since slice network is enabled
	for _, c := range components {
		cs, err := r.getComponentStatus(ctx, &c, slice.Spec.SliceName)
		if err != nil {
			log.Error(err, "unable to fetch component status")
		}
		if cs != nil {
			debuglog.Info("adding component status in workerslice obj", "component", cs)
			slice.Status.SliceHealth.ComponentStatuses = append(slice.Status.SliceHealth.ComponentStatuses, *cs)
			if cs.ComponentHealthStatus != spokev1alpha1.ComponentHealthStatusNormal {
				slice.Status.SliceHealth.SliceHealthStatus = spokev1alpha1.SliceHealthStatusWarning
			}
			debuglog.Info("updated slice health", "SliceHealth", slice.Status.SliceHealth)
		}
	}
	return nil
}

func (r *SliceReconciler) getComponentStatus(ctx context.Context, c *component, sliceName string) (*spokev1alpha1.ComponentStatus, error) {
	log := logger.FromContext(ctx)
	debuglog := log.V(1)
	debuglog.Info("component health check started", "component", c.name)
	if c.name != "dns" {
		c.labels["kubeslice.io/slice"] = sliceName
	}
	if c.name == "slice-gateway" || c.name == "gateway-tunnel" {
		sliceGwList := &kubeslicev1beta1.SliceGatewayList{}
		listOpts := []client.ListOption{
			client.MatchingLabels(map[string]string{
				"kubeslice.io/slice": sliceName,
			}),
			client.InNamespace(c.ns),
		}
		debuglog.Info("gw obj label for health check", "kubeslice.io/slice", c.labels["kubeslice.io/slice"])
		if err := r.MeshClient.List(ctx, sliceGwList, listOpts...); err != nil {
			if errors.IsNotFound(err) {
				debuglog.Info("Object Not found. Skipping health check", "component", c.name)
				return nil, nil
			} else {
				log.Error(err, "Failed to list slice gateway objects", "component", c.name)
				return nil, err
			}
		}
		if len(sliceGwList.Items) == 0 {
			debuglog.Info("No GateWay objects found. Skipping health check", "component", c.name)
			return nil, nil
		}
		switch c.name {
		case "slice-gateway":
			cs, err := r.fetchSliceGatewayHealth(ctx, c, sliceGwList)
			return cs, err
		case "gateway-tunnel":
			cs, err := r.fetchTunnelStatus(ctx, c, sliceGwList)
			return cs, err
		}
	}
	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.MatchingLabels(c.labels),
		client.InNamespace(c.ns),
	}
	if err := r.MeshClient.List(ctx, podList, listOpts...); err != nil {
		log.Error(err, "Failed to list pods", "pod", c.name)
		return nil, err
	}
	pods := podList.Items
	cs := &spokev1alpha1.ComponentStatus{
		Component: c.name,
	}
	if len(pods) == 0 {
		if c.ignoreMissing {
			debuglog.Info("ignore missing pod for ", "component", c.name)
			return nil, nil
		}
		log.Error(fmt.Errorf("no pods running"), "unhealthy", "pod", c.name)
		cs.ComponentHealthStatus = spokev1alpha1.ComponentHealthStatusError
		return cs, nil
	}
	// TODO: verify "PodConditionType == ContainersReady" when
	// readiness-probe for kubeslice components are implemented
	for _, pod := range pods {
		if pod.Status.Phase != corev1.PodRunning {
			debuglog.Info("pod is not in running phase", "component", c.name)
			cs.ComponentHealthStatus = spokev1alpha1.ComponentHealthStatusError
			return cs, nil
		} else {
			for _, containerStatus := range pod.Status.ContainerStatuses {
				terminatedState := containerStatus.State.Terminated
				if terminatedState != nil && terminatedState.ExitCode != 0 {
					log.Error(fmt.Errorf("container terminated with non-zero exitcode"), "component unhealthy",
						"component", c.name,
						"container", containerStatus.Name,
						"exitcode", terminatedState.ExitCode)
					cs.ComponentHealthStatus = spokev1alpha1.ComponentHealthStatusError
					return cs, nil
				}
			}
		}
	}
	debuglog.Info("health status normal", "component", c.name)
	cs.ComponentHealthStatus = spokev1alpha1.ComponentHealthStatusNormal
	return cs, nil
}

func (r *SliceReconciler) fetchTunnelStatus(ctx context.Context, c *component, sliceGwList *kubeslicev1beta1.SliceGatewayList) (*spokev1alpha1.ComponentStatus, error) {
	log := logger.FromContext(ctx)
	debuglog := log.V(1)
	debuglog.Info("health check for gateway tunnel")
	tunnel := &spokev1alpha1.ComponentStatus{
		Component: c.name,
	}
	listOpts := []client.ListOption{
		client.MatchingLabels(c.labels),
		client.InNamespace(c.ns),
	}

	podList := &corev1.PodList{}
	if err := r.MeshClient.List(ctx, podList, listOpts...); err != nil {
		log.Error(err, "Failed to list pods", "component", c.name)
		tunnel.ComponentHealthStatus = spokev1alpha1.ComponentHealthStatusError
		return tunnel, nil
	}
	pods := podList.Items
	if len(pods) == 0 {
		log.Error(fmt.Errorf("no gw pods running"), "error state", "component", c.name)
		tunnel.ComponentHealthStatus = spokev1alpha1.ComponentHealthStatusError
		return tunnel, nil
	}
	expectedTunnelCount := len(sliceGwList.Items) * 2 // gw redundancy
	tunnelHealthy := 0

	gwSideCarClient, err := gwsidecar.NewWorkerGWSidecarClientProvider()
	if err != nil {
		log.Error(err, "Failed to get GwSideCarClient", "component", c.name)
		return nil, err
	}
	for _, pod := range podList.Items {
		debuglog.Info("gateway pod found", "pod name", pod.Name)
		if pod.Status.Phase == corev1.PodRunning && pod.ObjectMeta.DeletionTimestamp == nil {
			sidecarGrpcAddress := pod.Status.PodIP + ":5000"
			debuglog.Info("side car", "grpc addr", sidecarGrpcAddress)
			gs, err := gwSideCarClient.GetStatus(ctx, sidecarGrpcAddress)
			if err != nil {
				log.Error(err, "failed to get tunnel status")
				continue
			}
			debuglog.Info("tunnel status", "values", gs.TunnelStatus)
			// atleast one gw tunnel is up
			if gs.TunnelStatus.Status == 0 {
				tunnelHealthy += 1
			}
		}
	}
	debuglog.Info("tunnel health flag", "tunnelHealthy", tunnelHealthy)
	if expectedTunnelCount == tunnelHealthy {
		// all gw tunnels are healthy
		tunnel.ComponentHealthStatus = spokev1alpha1.ComponentHealthStatusNormal
	} else if tunnelHealthy > 0 {
		// atleast one gw tunnel is healthy
		tunnel.ComponentHealthStatus = spokev1alpha1.ComponentHealthStatusWarning
	} else {
		// no gw tunnel is healthy
		tunnel.ComponentHealthStatus = spokev1alpha1.ComponentHealthStatusError
	}
	return tunnel, nil
}

func (r *SliceReconciler) fetchSliceGatewayHealth(ctx context.Context, c *component, sliceGwList *kubeslicev1beta1.SliceGatewayList) (*spokev1alpha1.ComponentStatus, error) {
	log := logger.FromContext(ctx)
	debuglog := log.V(1)
	//fetch number of deployments
	cs := &spokev1alpha1.ComponentStatus{
		Component: c.name,
	}
	expectedGwPodCount := len(sliceGwList.Items) * 2 // gw redundancy
	listOpts := []client.ListOption{
		client.MatchingLabels(c.labels),
		client.InNamespace(c.ns),
	}

	podList := &corev1.PodList{}
	if err := r.MeshClient.List(ctx, podList, listOpts...); err != nil {
		log.Error(err, "Failed to list pods", "pod", c.name)
		cs.ComponentHealthStatus = spokev1alpha1.ComponentHealthStatusError
		return cs, err
	}
	pods := podList.Items
	if len(pods) == 0 {
		log.Error(fmt.Errorf("no pods running"), "unhealthy", "pod", c.name)
		cs.ComponentHealthStatus = spokev1alpha1.ComponentHealthStatusError
		return cs, nil
	}

	// TODO: verify "PodConditionType == ContainersReady" when
	// readiness-probe for kubeslice components are implemented
	healthyCount := 0
	for _, pod := range pods {
		if pod.Status.Phase == corev1.PodRunning && pod.ObjectMeta.DeletionTimestamp == nil {
			debuglog.Info("pod is in running phase, check container status", "component", c.name, "pod", pod.Name)
			healthy := true
			for _, containerStatus := range pod.Status.ContainerStatuses {
				terminatedState := containerStatus.State.Terminated
				if terminatedState != nil && terminatedState.ExitCode != 0 {
					healthy = false
					debuglog.Info("container terminated with non-zero exitcode",
						"component", c.name,
						"pod", pod.Name,
						"container", containerStatus.Name,
						"exitcode", terminatedState.ExitCode)
					break
				}
			}
			if healthy {
				healthyCount += 1
			}
		}
	}
	debuglog.Info("slice gw health check flag", "expected pod count", expectedGwPodCount)
	debuglog.Info("slice gw health check flag", "healthyCount", healthyCount)
	if expectedGwPodCount == healthyCount {
		// all gw pods are healthy
		cs.ComponentHealthStatus = spokev1alpha1.ComponentHealthStatusNormal
	} else if healthyCount > 0 {
		// atleast one gw is healthy
		cs.ComponentHealthStatus = spokev1alpha1.ComponentHealthStatusWarning
	} else {
		// no gw is unhealthy
		cs.ComponentHealthStatus = spokev1alpha1.ComponentHealthStatusError
	}
	debuglog.Info("report gw health", "status", cs.ComponentHealthStatus)
	return cs, nil
}

func (r *SliceReconciler) UpdateSliceHealthMetrics(slice *spokev1alpha1.WorkerSliceConfig) {
	sliceName := slice.Spec.SliceName
	if slice.Status.SliceHealth.SliceHealthStatus == spokev1alpha1.SliceHealthStatusNormal {
		r.gaugeSliceUp.WithLabelValues(sliceName).Set(1)
	} else {
		r.gaugeSliceUp.WithLabelValues(sliceName).Set(0)
	}

	for _, cs := range slice.Status.SliceHealth.ComponentStatuses {
		if cs.ComponentHealthStatus == spokev1alpha1.ComponentHealthStatusNormal {
			r.gaugeComponentUp.WithLabelValues(sliceName, cs.Component).Set(1)
		} else {
			r.gaugeComponentUp.WithLabelValues(sliceName, cs.Component).Set(0)
		}
	}
}
