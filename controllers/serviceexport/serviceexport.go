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

package serviceexport

import (
	"context"
	"fmt"
	"time"

	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	"github.com/kubeslice/worker-operator/controllers"
	ossEvents "github.com/kubeslice/worker-operator/events"
	"github.com/kubeslice/worker-operator/pkg/logger"
	"github.com/kubeslice/worker-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ReconcileAppPod reconciles serviceexport app pods
func (r *Reconciler) ReconcileAppPod(
	ctx context.Context, serviceexport *kubeslicev1beta1.ServiceExport) (ctrl.Result, error, bool) {
	log := logger.FromContext(ctx).WithValues("type", "app pod")
	debugLog := log.V(1)

	appPods, err := r.getAppPods(ctx, serviceexport)
	if err != nil {
		log.Error(err, "Failed to fetch app pods for serviceexport")
		return ctrl.Result{}, err, true
	}

	if !isServiceAppPodChanged(appPods, serviceexport.Status.Pods) {
		// No change in service pods, don't do anything and return
		return ctrl.Result{}, nil, false
	}
	serviceexport.Status.Pods = appPods
	serviceexport.Status.LastSync = 0                                        // Force sync to hub in next loop
	serviceexport.Status.ExportStatus = kubeslicev1beta1.ExportStatusPending // Set status to pending
	serviceexport.Status.AvailableEndpoints = len(appPods)

	log.Info("updating service app pods")
	debugLog.Info("updating service app pods", "app pods", appPods)
	err = r.Status().Update(ctx, serviceexport)
	if err != nil {
		log.Error(err, "Failed to update ServiceExport status for app pods")
		return ctrl.Result{}, err, true
	}
	log.Info("Service App pod status updated")
	return ctrl.Result{Requeue: true}, nil, true
}

func (r *Reconciler) getAppPods(ctx context.Context, serviceexport *kubeslicev1beta1.ServiceExport) ([]kubeslicev1beta1.ServicePod, error) {
	log := logger.FromContext(ctx).WithValues("type", "app pod")
	debugLog := log.V(1)

	podList := &corev1.PodList{}

	listOpts := []client.ListOption{
		client.MatchingLabels(serviceexport.Spec.Selector.MatchLabels),
		client.InNamespace(serviceexport.Namespace),
	}
	if err := r.List(ctx, podList, listOpts...); err != nil {
		log.Error(err, "Failed to list pods")
		return nil, err
	}

	debugLog.Info("pods matching labels", "count", len(podList.Items))

	appPods := []kubeslicev1beta1.ServicePod{}
	appPodsInSlice, err := getAppPodsInSlice(ctx, r.Client, serviceexport.Spec.Slice)
	if err != nil {
		log.Error(err, "Unable to fetch app pods in slice")
		return nil, err
	}

	debugLog.Info("app pods in slice", "pods", appPodsInSlice)
	for _, pod := range podList.Items {
		if pod.Status.Phase == corev1.PodRunning {
			dnsName := pod.Name + "." + getClusterName() + "." + serviceexport.Name + "." + serviceexport.Namespace + ".svc.slice.local"
			ip := getNsmIP(&pod, appPodsInSlice)
			// Avoid adding pods with no nsmip (not part of slice yet)
			if ip == "" {
				continue
			}
			appPods = append(appPods, kubeslicev1beta1.ServicePod{
				Name:    pod.Name,
				NsmIP:   ip,
				PodIp:   pod.Status.PodIP,
				DNSName: dnsName,
			})
		}
	}
	debugLog.Info("valid app pods in slice", "pods", appPods)
	return appPods, nil
}

func (r *Reconciler) ReconcileIngressGwPod(
	ctx context.Context, serviceexport *kubeslicev1beta1.ServiceExport) (ctrl.Result, error, bool) {
	log := logger.FromContext(ctx).WithValues("type", "ingress gw pod")
	debugLog := log.V(1)

	ingressEnabled, ingressGwPod, err := controllers.GetSliceIngressGwPod(ctx, r.Client, serviceexport.Spec.Slice)
	if err != nil {
		log.Error(err, "Failed to get ingress gw pod info")
		return ctrl.Result{}, err, true
	}

	if !ingressEnabled {
		return ctrl.Result{}, nil, false
	}

	if ingressGwPod == nil {
		debugLog.Info("Ingress gw pod not available yet, requeueing...")
		return ctrl.Result{
			RequeueAfter: 30 * time.Second,
		}, nil, true
	}

	if ingressGwPod.PodName != serviceexport.Status.IngressGwPod.Name ||
		ingressGwPod.NsmIP != serviceexport.Status.IngressGwPod.NsmIP {
		debugLog.Info("ingress gw info change", "old", serviceexport.Status.IngressGwPod, "new", ingressGwPod)
		serviceexport.Status.LastSync = 0
		serviceexport.Status.ExportStatus = kubeslicev1beta1.ExportStatusPending
		serviceexport.Status.IngressGwEnabled = true
		serviceexport.Status.IngressGwPod.Name = ingressGwPod.PodName
		serviceexport.Status.IngressGwPod.NsmIP = ingressGwPod.NsmIP
		err = r.Status().Update(ctx, serviceexport)
		if err != nil {
			log.Error(err, "Failed to update ServiceExport status for ingress gw")
			return ctrl.Result{}, err, true
		}
		log.Info("ingress gw status updated in serviceexport")
		return ctrl.Result{Requeue: true}, nil, true
	}

	return ctrl.Result{}, nil, false
}

func (r *Reconciler) DeleteServiceExportResources(ctx context.Context, serviceexport *kubeslicev1beta1.ServiceExport) error {
	log := logger.FromContext(ctx)
	slice, err := controllers.GetSlice(ctx, r.Client, serviceexport.Spec.Slice)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		log.Error(err, "Unable to fetch slice for serviceexport cleanup")
		return err
	}

	if slice.Status.SliceConfig == nil {
		return nil
	}

	return r.DeleteIstioResources(ctx, serviceexport, slice)
}

func (r *Reconciler) SyncSvcExportStatus(ctx context.Context, serviceexport *kubeslicev1beta1.ServiceExport) (ctrl.Result, error, bool) {
	log := logger.FromContext(ctx)

	if serviceexport.Status.LastSync != 0 {
		return ctrl.Result{}, nil, false
	}

	var err error
	if serviceexport.Status.IngressGwEnabled {
		ep := &kubeslicev1beta1.ServicePod{
			Name:  fmt.Sprintf("%s-%s-ingress", serviceexport.Name, serviceexport.ObjectMeta.Namespace),
			NsmIP: serviceexport.Status.IngressGwPod.NsmIP,
			DNSName: fmt.Sprintf("%s-ingress.%s.%s.svc.slice.local",
				serviceexport.Name, controllers.ClusterName, serviceexport.Namespace),
		}
		err = r.HubClient.UpdateServiceExportEndpointForIngressGw(ctx, serviceexport, ep)
	} else {
		err = r.HubClient.UpdateServiceExport(ctx, serviceexport)
	}

	if err != nil {
		log.Error(err, "Failed to post serviceexport")
		serviceexport.Status.ExportStatus = kubeslicev1beta1.ExportStatusError
		errN := r.Status().Update(ctx, serviceexport)
		if errN != nil {
			log.Error(errN, "Failed to update ServiceExport status")
			return ctrl.Result{}, errN, true
		}
		//post event to service export
		utils.RecordEvent(ctx, r.EventRecorder, serviceexport, nil, ossEvents.EventSyncServiceExportStatusFailed, controllerName)
		return ctrl.Result{}, err, true
	}

	log.Info("serviceexport sync success")
	//post event to service export
	utils.RecordEvent(ctx, r.EventRecorder, serviceexport, nil, ossEvents.EventSyncServiceExportStatusSuccessfully, controllerName)

	currentTime := time.Now().Unix()
	serviceexport.Status.LastSync = currentTime
	err = r.Status().Update(ctx, serviceexport)
	if err != nil {
		log.Error(err, "Failed to update serviceexport sync time")
		return ctrl.Result{}, err, true
	}

	return ctrl.Result{Requeue: true}, nil, true
}

// ReconcileAliases reconciles serviceexport aliases
func (r *Reconciler) ReconcileAliases(
	ctx context.Context, serviceexport *kubeslicev1beta1.ServiceExport) (ctrl.Result, error, bool) {
	log := logger.FromContext(ctx).WithValues("type", "aliases")

	if len(serviceexport.Spec.Aliases) == 0 && len(serviceexport.Status.Aliases) == 0 {
		return ctrl.Result{}, nil, false
	}

	// Check if an update is needed to the alias info in the status
	if !isAliasStatusInSync(serviceexport.Status.Aliases, serviceexport.Spec.Aliases) {
		serviceexport.Status.Aliases = serviceexport.Spec.Aliases
		serviceexport.Status.LastSync = 0
		err := r.Status().Update(ctx, serviceexport)
		if err != nil {
			log.Error(err, "Failed to update serviceexport ports")
			return ctrl.Result{}, err, true
		}
		log.Info("serviceexport status updated with aliases", ":", serviceexport.Status.Aliases)
		return ctrl.Result{Requeue: true}, nil, true
	}

	return ctrl.Result{}, nil, false
}

func isAliasStatusInSync(aliasListInStatus []string, aliasListInSpec []string) bool {
	if len(aliasListInStatus) != len(aliasListInSpec) {
		return false
	}
	// Comparing the two lists by first creating a map from the Status list and doing a lookup
	// against every element in the Spec list. Not using reflect.DeepEqual() as it requires the
	// lists to be in the same order to be deeply equal, and also the DeepEqual() func is not
	// performant.
	// Rob Pike in the official Go blog:
	// "It's a powerful tool that should be used with care and avoided unless strictly necessary"
	// So using it to just compare two lists/slices might not be a good usecase.
	aliasInStatusMap := make(map[string]struct{})
	for _, aliasInStatus := range aliasListInStatus {
		aliasInStatusMap[aliasInStatus] = struct{}{}
	}
	for _, aliasInSpec := range aliasListInSpec {
		_, ok := aliasInStatusMap[aliasInSpec]
		if !ok {
			return false
		}
	}

	return true
}
