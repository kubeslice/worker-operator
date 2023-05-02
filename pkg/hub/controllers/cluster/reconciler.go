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

package cluster

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"time"

	hubv1alpha1 "github.com/kubeslice/apis/pkg/controller/v1alpha1"
	"github.com/kubeslice/kubeslice-monitoring/pkg/events"
	"github.com/kubeslice/kubeslice-monitoring/pkg/metrics"
	"github.com/kubeslice/worker-operator/controllers"
	ossEvents "github.com/kubeslice/worker-operator/events"
	"github.com/kubeslice/worker-operator/pkg/cluster"
	"github.com/kubeslice/worker-operator/pkg/logger"
	"github.com/kubeslice/worker-operator/pkg/utils"
	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	GCP            string = "gcp"
	AWS            string = "aws"
	AZURE          string = "azure"
	controllerName string = "clusterReconciler"
)

type Reconciler struct {
	client.Client
	MeshClient    client.Client
	EventRecorder *events.EventRecorder

	// metrics
	gaugeClusterUp   *prometheus.GaugeVec
	gaugeComponentUp *prometheus.GaugeVec

	ReconcileInterval time.Duration
}

func NewReconciler(mc client.Client, er *events.EventRecorder, mf metrics.MetricsFactory) *Reconciler {
	gaugeClusterUp := mf.NewGauge("cluster_up", "Kubeslice cluster health status", []string{})
	gaugeComponentUp := mf.NewGauge("cluster_component_up", "Kubeslice cluster component health status", []string{"slice_cluster_component"})

	return &Reconciler{
		MeshClient:    mc,
		EventRecorder: er,

		gaugeClusterUp:   gaugeClusterUp,
		gaugeComponentUp: gaugeComponentUp,

		ReconcileInterval: 120 * time.Second,
	}
}

var retryAttempts = 0
var clusterDeregisterFinalizer = "networking.kubeslice.io/cluster-deregister-finalizer"

func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := logger.FromContext(ctx).WithName("cluster-reconciler")
	ctx = logger.WithLogger(ctx, log)
	cr, err := r.getCluster(ctx, req)
	if cr == nil {
		return reconcile.Result{}, err
	}
	log.Info("got cluster CR from hub", "cluster", cr)
	requeue, result, err := r.handleClusterDeletion(cr, ctx, req)
	if requeue {
		return result, err
	}

	cl := cluster.NewCluster(r.MeshClient, cr.Name)
	res, err, requeue := r.updateClusterCloudProviderInfo(ctx, cr, cl)
	if err != nil {
		utils.RecordEvent(ctx, r.EventRecorder, cr, nil, ossEvents.EventClusterProviderUpdateInfoFailed, controllerName)
		log.Error(err, "unable to update cloud provider info to cluster")
	}
	if requeue {
		return res, err
	}
	utils.RecordEvent(ctx, r.EventRecorder, cr, nil, ossEvents.EventClusterProviderUpdateInfoSuccesful, controllerName)
	res, err, requeue = r.updateCNISubnetConfig(ctx, cr, cl)
	if err != nil {
		utils.RecordEvent(ctx, r.EventRecorder, cr, nil, ossEvents.EventClusterCNISubnetUpdateFailed, controllerName)
		log.Error(err, "unable to update cni subnet config to cluster")
	}
	if requeue {
		return res, err
	}
	utils.RecordEvent(ctx, r.EventRecorder, cr, nil, ossEvents.EventClusterCNISubnetUpdateSuccessful, controllerName)
	res, err, requeue = r.updateNodeIps(ctx, cr)
	if err != nil {
		utils.RecordEvent(ctx, r.EventRecorder, cr, nil, ossEvents.EventClusterNodeIpUpdateFailed, controllerName)
		log.Error(err, "unable to update node ips to cluster")
	}
	if requeue {
		return res, err
	}
	utils.RecordEvent(ctx, r.EventRecorder, cr, nil, ossEvents.EventClusterNodeIpUpdateSuccessful, controllerName)
	// Update dashboard creds if it hasn't already (only one time event)
	if !r.isDashboardCredsUpdated(ctx, cr) {
		if err := r.updateDashboardCreds(ctx, cr); err != nil {
			utils.RecordEvent(ctx, r.EventRecorder, cr, nil, ossEvents.EventClusterDashboardCredsUpdateFailed, controllerName)
			log.Error(err, "unable to update dashboard creds")
			return reconcile.Result{}, err
		} else {
			utils.RecordEvent(ctx, r.EventRecorder, cr, nil, ossEvents.EventClusterDashboardCredsUpdated, controllerName)
			log.Info("Dashboard creds updated in hub")
			return reconcile.Result{Requeue: true}, nil
		}
	}
	if cr.Status.ClusterHealth == nil {
		cr.Status.ClusterHealth = &hubv1alpha1.ClusterHealth{}
	}

	if err := r.updateClusterHealthStatus(ctx, cr); err != nil {
		utils.RecordEvent(ctx, r.EventRecorder, cr, nil, ossEvents.EventClusterHealthStatusUpdateFailed, controllerName)
		log.Error(err, "unable to update cluster health status")
	}

	r.updateClusterMetrics(cr)

	if time.Since(cr.Status.ClusterHealth.LastUpdated.Time) > r.ReconcileInterval {
		cr.Status.ClusterHealth.LastUpdated = metav1.Now()
		if err := r.Status().Update(ctx, cr); err != nil {
			utils.RecordEvent(ctx, r.EventRecorder, cr, nil, ossEvents.EventClusterHealthStatusUpdateFailed, controllerName)
			log.Error(err, "unable to update cluster CR")
			return reconcile.Result{}, err
		}
		utils.RecordEvent(ctx, r.EventRecorder, cr, nil, ossEvents.EventClusterHealthStatusUpdated, controllerName)
	}
	utils.RecordEvent(ctx, r.EventRecorder, cr, nil, ossEvents.EventClusterHealthStatusUpdated, controllerName)
	return reconcile.Result{RequeueAfter: r.ReconcileInterval}, nil
}

func (r *Reconciler) updateClusterHealthStatus(ctx context.Context, cr *hubv1alpha1.Cluster) error {
	log := logger.FromContext(ctx)
	csList := []hubv1alpha1.ComponentStatus{}
	var chs hubv1alpha1.ClusterHealthStatus = hubv1alpha1.ClusterHealthStatusNormal

	for _, c := range components {
		cs, err := r.getComponentStatus(ctx, &c)
		if err != nil {
			log.Error(err, "unable to fetch component status")
		}
		if cs != nil {
			csList = append(csList, *cs)
			if cs.ComponentHealthStatus != hubv1alpha1.ComponentHealthStatusNormal {
				chs = hubv1alpha1.ClusterHealthStatusWarning
				log.Info("Component unhealthy", "component", c.name)
			}
		}
	}

	// Cluster health changed, raise event
	if cr.Status.ClusterHealth.ClusterHealthStatus != chs {
		if chs == hubv1alpha1.ClusterHealthStatusNormal {
			log.Info("cluster health is back to normal")
			utils.RecordEvent(ctx, r.EventRecorder, cr, nil, ossEvents.EventClusterHealthy, controllerName)
		} else if chs == hubv1alpha1.ClusterHealthStatusWarning {
			log.Info("cluster health is in warning state")
			utils.RecordEvent(ctx, r.EventRecorder, cr, nil, ossEvents.EventClusterUnhealthy, controllerName)
		}
	}

	cr.Status.ClusterHealth.ComponentStatuses = csList
	cr.Status.ClusterHealth.ClusterHealthStatus = chs

	return nil
}

func (r *Reconciler) updateClusterMetrics(cr *hubv1alpha1.Cluster) {
	if cr.Status.ClusterHealth.ClusterHealthStatus == hubv1alpha1.ClusterHealthStatusNormal {
		r.gaugeClusterUp.WithLabelValues().Set(1)
	} else {
		r.gaugeClusterUp.WithLabelValues().Set(0)
	}

	for _, cs := range cr.Status.ClusterHealth.ComponentStatuses {
		if cs.ComponentHealthStatus == hubv1alpha1.ComponentHealthStatusNormal {
			r.gaugeComponentUp.WithLabelValues(cs.Component).Set(1)
		} else {
			r.gaugeComponentUp.WithLabelValues(cs.Component).Set(0)
		}
	}
}

func (r *Reconciler) getComponentStatus(ctx context.Context, c *component) (*hubv1alpha1.ComponentStatus, error) {
	log := logger.FromContext(ctx)
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
	cs := &hubv1alpha1.ComponentStatus{
		Component: c.name,
	}
	if len(pods) == 0 && c.ignoreMissing {
		return nil, nil
	}
	if len(pods) == 0 {
		log.Error(fmt.Errorf("no pods running"), "unhealthy", "pod", c.name)
		cs.ComponentHealthStatus = hubv1alpha1.ComponentHealthStatusError
		return cs, nil
	}

	for _, pod := range pods {
		if pod.Status.Phase != corev1.PodRunning {
			log.Info("pod is not healthy", "component", c.name)
			cs.ComponentHealthStatus = hubv1alpha1.ComponentHealthStatusError
			return cs, nil
		}
	}

	cs.ComponentHealthStatus = hubv1alpha1.ComponentHealthStatusNormal
	return cs, nil
}

func (r *Reconciler) updateClusterCloudProviderInfo(ctx context.Context, cr *hubv1alpha1.Cluster, cl cluster.ClusterInterface) (ctrl.Result, error, bool) {
	log := logger.FromContext(ctx)
	clusterInfo, err := cl.GetClusterInfo(ctx)
	if err != nil {
		log.Error(err, "Error getting clusterInfo")
		return ctrl.Result{}, err, true
	}
	log.Info("got clusterinfo", "ci", clusterInfo)
	toUpdate := false
	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		err := r.Get(ctx, types.NamespacedName{
			Name:      cr.Name,
			Namespace: cr.Namespace,
		}, cr)
		if err != nil {
			return err
		}
		cloudProvider := clusterInfo.ClusterProperty.GeoLocation.CloudProvider
		cloudRegion := clusterInfo.ClusterProperty.GeoLocation.CloudRegion
		if cloudProvider == GCP || cloudProvider == AWS || cloudProvider == AZURE {
			// compare the current cloud region and provider with values stored in cluster spec, if not same then update
			if cloudRegion != cr.Spec.ClusterProperty.GeoLocation.CloudRegion || cloudProvider != cr.Spec.ClusterProperty.GeoLocation.CloudProvider {
				log.Info("updating Cluster's cloud info", "cloudProvider", cloudProvider, "cloudRegion", cloudRegion)
				cr.Spec.ClusterProperty.GeoLocation.CloudProvider = cloudProvider
				cr.Spec.ClusterProperty.GeoLocation.CloudRegion = cloudRegion
				toUpdate = true
				return r.Update(ctx, cr)
			}
		}
		return nil
	})
	if err != nil {
		log.Error(err, "Failed to update Cloud Provider Info on hub cluster")
		return ctrl.Result{}, err, true
	}
	if toUpdate {
		return ctrl.Result{Requeue: true}, nil, true
	}
	return ctrl.Result{}, nil, false
}

func (r *Reconciler) updateCNISubnetConfig(ctx context.Context, cr *hubv1alpha1.Cluster, cl cluster.ClusterInterface) (ctrl.Result, error, bool) {
	log := logger.FromContext(ctx)
	toUpdate := false
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		err := r.Get(ctx, types.NamespacedName{
			Name:      cr.Name,
			Namespace: cr.Namespace,
		}, cr)
		if err != nil {
			return err
		}
		cniSubnet, err := cl.GetNsmExcludedPrefix(ctx, "nsm-config", "kubeslice-system")
		if err != nil {
			log.Error(err, "Failed to get nsm config")
			return err
		}
		if !reflect.DeepEqual(cr.Status.CniSubnet, cniSubnet) {
			cr.Status.CniSubnet = cniSubnet
			cr.Status.RegistrationStatus = hubv1alpha1.RegistrationStatusRegistered
			toUpdate = true
			return r.Status().Update(ctx, cr)
		}
		return nil
	})
	if err != nil {
		log.Error(err, "Error updating cniSubnet to cluster status on hub cluster")
		return ctrl.Result{}, err, true
	}
	if toUpdate {
		return ctrl.Result{Requeue: true}, nil, true
	}
	return ctrl.Result{}, nil, false
}

func (r *Reconciler) updateNodeIps(ctx context.Context, cr *hubv1alpha1.Cluster) (ctrl.Result, error, bool) {
	log := logger.FromContext(ctx)
	toUpdate := false
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		err := r.Get(ctx, types.NamespacedName{
			Name:      cr.Name,
			Namespace: cr.Namespace,
		}, cr)
		if err != nil {
			return err
		}
		nodeIPs, err := cluster.GetNodeIP(r.MeshClient)
		if err != nil {
			log.Error(err, "Error Getting nodeIP")
			return err
		}
		// Populate NodeIPs if not already updated
		// OR gets the current nodeIP in use from controller cluster CR and compares it with
		// the current nodeIpList (nodeIpList contains list of externalIPs or internalIPs)
		// if the nodeIP is no longer available, we update the cluster CR on controller cluster
		if cr.Status.NodeIPs == nil || len(cr.Status.NodeIPs) == 0 ||
			!validatenodeips(nodeIPs, cr.Status.NodeIPs) {
			log.Info("Mismatch in node IP", "IP in use", cr.Status.NodeIPs, "IP to be used", nodeIPs)
			cr.Status.NodeIPs = nodeIPs
			toUpdate = true
			return r.Status().Update(ctx, cr)
		}
		return nil
	})
	if err != nil {
		log.Error(err, "Error updating to node ip's on hub cluster")
		utils.RecordEvent(ctx, r.EventRecorder, cr, nil, ossEvents.EventClusterNodeIpAutoDetectionFailed, controllerName)
		return ctrl.Result{}, err, true
	}
	if toUpdate {
		utils.RecordEvent(ctx, r.EventRecorder, cr, nil, ossEvents.EventClusterNodeIpAutoDetected, controllerName)
		return ctrl.Result{Requeue: true}, nil, true
	}
	return ctrl.Result{}, nil, false
}

// total -> external ip list of nodes in the k8s cluster
// current -> ip list present in nodeIPs of cluster cr
func validatenodeips(total, current []string) bool {
	if len(total) != len(current) {
		return false
	}
	for i := range total {
		if total[i] != current[i] {
			return false
		}
	}
	return true
}

func (r *Reconciler) isDashboardCredsUpdated(ctx context.Context, cr *hubv1alpha1.Cluster) bool {
	return cr.Spec.ClusterProperty.Monitoring.KubernetesDashboard.Enabled
}

func (r *Reconciler) updateDashboardCreds(ctx context.Context, cr *hubv1alpha1.Cluster) error {
	log := logger.FromContext(ctx)
	log.Info("Updating kubernetes dashboard creds")

	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      KubeSliceDashboardSA,
			Namespace: controllers.ControlPlaneNamespace,
		},
	}
	if err := r.MeshClient.Get(ctx, types.NamespacedName{Name: sa.Name, Namespace: controllers.ControlPlaneNamespace}, sa); err != nil {
		log.Error(err, "Error getting service account")
		return err
	}

	if len(sa.Secrets) == 0 {
		err := fmt.Errorf("ServiceAccount has no secret")
		log.Error(err, "Error getting service account secret")
		return err
	}

	secret := &corev1.Secret{}
	err := r.MeshClient.Get(ctx, types.NamespacedName{Name: sa.Secrets[0].Name, Namespace: controllers.ControlPlaneNamespace}, secret)
	if err != nil {
		log.Error(err, "Error getting service account's secret")
		return err
	}

	secretName := os.Getenv("CLUSTER_NAME") + HubSecretSuffix
	if secret.Data == nil {
		return fmt.Errorf("dashboard secret data is nil")
	}
	token, ok := secret.Data["token"]
	if !ok {
		return fmt.Errorf("token not present in dashboard secret")
	}
	cacrt, ok := secret.Data["ca.crt"]
	if !ok {
		return fmt.Errorf("ca.crt not present in dashboard secret")
	}

	secretData := map[string][]byte{
		"token":  token,
		"ca.crt": cacrt,
	}
	hubSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: os.Getenv("HUB_PROJECT_NAMESPACE"),
		},
		Data: secretData,
	}
	log.Info("creating secret on hub", "hubSecret", hubSecret.Name)
	err = r.Create(ctx, &hubSecret)
	if apierrors.IsAlreadyExists(err) {
		err = r.Update(ctx, &hubSecret)
	}

	if err != nil {
		return err
	}
	cr.Spec.ClusterProperty.Monitoring.KubernetesDashboard.Endpoint = os.Getenv("CLUSTER_ENDPOINT")
	cr.Spec.ClusterProperty.Monitoring.KubernetesDashboard.AccessToken = secretName
	cr.Spec.ClusterProperty.Monitoring.KubernetesDashboard.Enabled = true
	log.Info("Posting cluster creds to hub cluster", "cluster", os.Getenv("CLUSTER_NAME"))
	return r.Update(ctx, cr)
}

func (r *Reconciler) getCluster(ctx context.Context, req reconcile.Request) (*hubv1alpha1.Cluster, error) {
	hubCluster := &hubv1alpha1.Cluster{}
	log := logger.FromContext(ctx)
	err := r.Get(ctx, req.NamespacedName, hubCluster)
	// Request object not found, could have been deleted after reconcile request.
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Return and don't requeue
			log.Info("cluster resource not found in hub. Ignoring since object must be deleted")
			return nil, nil
		}
		return nil, err
	}
	return hubCluster, nil
}

func (r *Reconciler) InjectClient(c client.Client) error {
	r.Client = c
	return nil
}

func (r *Reconciler) handleClusterDeletion(cluster *hubv1alpha1.Cluster, ctx context.Context, req reconcile.Request) (bool, reconcile.Result, error) {
	log := logger.FromContext(ctx)

	if cluster.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !controllerutil.ContainsFinalizer(cluster, clusterDeregisterFinalizer) {
			controllerutil.AddFinalizer(cluster, clusterDeregisterFinalizer)
			if err := r.Update(ctx, cluster); err != nil {
				return true, reconcile.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(cluster, clusterDeregisterFinalizer) && !cluster.Status.IsDeregisterInProgress && retryAttempts <= 3 {
			// our finalizer is present, so lets handle any external dependency
			if err := r.createDeregisterJob(ctx, cluster); err != nil {
				// unable to deregister the worker operator, return with an error and raise event
				log.Error(err, "unable to deregister the worker operator")
				// increment count for retryAttempts
				retryAttempts++
				// resetting isDeregisterInProgress to false
				isDeregisterInProgress := false
				statusUpdateErr := r.updateRegistrationStatusAndDeregisterInProgress(ctx, cluster, hubv1alpha1.RegistrationStatusDeregisterFailed, &isDeregisterInProgress)
				if statusUpdateErr != nil {
					log.Error(statusUpdateErr, "unable to update registration status")
				}
				recordEventErr := r.EventRecorder.RecordEvent(ctx, &events.Event{
					Object:            cluster,
					Name:              ossEvents.EventDeregistrationJobFailed,
					ReportingInstance: "cluster_reconciler",
				})
				if recordEventErr != nil {
					log.Error(recordEventErr, "unable to record event for cluster deregistration")
				}
				return true, reconcile.Result{}, err
			}
		}
		// Stop reconciliation as the item is being deleted
		return true, reconcile.Result{}, nil
	}
	return false, reconcile.Result{}, nil
}
