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

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubeslicev1beta1 "github.com/kubeslice/operator/api/v1beta1"
	"github.com/kubeslice/operator/controllers"
	"github.com/kubeslice/operator/internal/logger"
	"github.com/kubeslice/operator/pkg/events"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// Reconciler reconciles serviceexport resource
type Reconciler struct {
	client.Client
	Log           logr.Logger
	Scheme        *runtime.Scheme
	HubClient     HubClientProvider
	EventRecorder *events.EventRecorder
}

type HubClientProvider interface {
	UpdateServiceExport(ctx context.Context, serviceexport *kubeslicev1beta1.ServiceExport) error
	UpdateServiceExportEndpointForIngressGw(ctx context.Context, serviceexport *kubeslicev1beta1.ServiceExport, ep *kubeslicev1beta1.ServicePod) error
	DeleteServiceExport(ctx context.Context, serviceexport *kubeslicev1beta1.ServiceExport) error
}

var finalizerName = "servicediscovery.kubeslice.io/serviceexport-finalizer"

// +kubebuilder:rbac:groups=servicediscovery.kubeslice.io,resources=serviceexports,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=servicediscovery.kubeslice.io,resources=serviceexports/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=servicediscovery.kubeslice.io,resources=serviceexports/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch

// Reconcile reconciles serviceexport
func (r Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("serviceexport", req.NamespacedName)

	serviceexport := &kubeslicev1beta1.ServiceExport{}
	err := r.Get(ctx, req.NamespacedName, serviceexport)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Return and don't requeue
			log.Info("serviceexport resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get serviceexport")
		return ctrl.Result{}, err
	}

	log = log.WithValues("slice", serviceexport.Spec.Slice)
	debugLog := log.V(1)
	ctx = logger.WithLogger(ctx, log)

	log.Info("reconciling", "serviceexport", serviceexport.Name)

	// examine DeletionTimestamp to determine if object is under deletion
	if serviceexport.ObjectMeta.DeletionTimestamp.IsZero() {
		// register our finalizer
		if !containsString(serviceexport.GetFinalizers(), finalizerName) {
			log.Info("adding finalizer")
			controllerutil.AddFinalizer(serviceexport, finalizerName)
			if err := r.Update(ctx, serviceexport); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{Requeue: true}, nil
		}
	} else {
		// The object is being deleted
		if containsString(serviceexport.GetFinalizers(), finalizerName) {
			log.Info("deleting serviceexport")
			if err := r.HubClient.DeleteServiceExport(ctx, serviceexport); err != nil {
				log.Error(err, "unable to delete service export on the hub from the spoke")
				return ctrl.Result{}, err
			}

			if err := r.DeleteServiceExportResources(ctx, serviceexport); err != nil {
				log.Error(err, "unable to delete service export resources")
				return ctrl.Result{}, err
			}

			log.Info("removing finalizer")
			controllerutil.RemoveFinalizer(serviceexport, finalizerName)
			if err := r.Update(ctx, serviceexport); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{Requeue: true}, nil
		}

		return ctrl.Result{}, nil
	}
	// update service export with slice Label if not present
	labels := serviceexport.GetLabels()
	if value, exists := labels["kubeslice.io/slice"]; !exists || value != serviceexport.Spec.Slice {
		// the label does not exists or the sliceName is incorrect
		if labels == nil {
			labels = make(map[string]string)
		}
		labels["kubeslice.io/slice"] = serviceexport.Spec.Slice
		serviceexport.SetLabels(labels)

		if err := r.Update(ctx, serviceexport); err != nil {
			return ctrl.Result{}, err
		}
		debugLog.Info("Added Label for serviceexport", "serviceexport", serviceexport.Name)
		return ctrl.Result{Requeue: true}, nil
	}

	// Reconciler running for the first time. Set the initial status here
	if serviceexport.Status.ExportStatus == kubeslicev1beta1.ExportStatusInitial {
		serviceexport.Status.DNSName = serviceexport.Name + "." + serviceexport.Namespace + ".svc.slice.local"
		serviceexport.Status.ExportStatus = kubeslicev1beta1.ExportStatusPending
		serviceexport.Status.ExposedPorts = portListToDisplayString(serviceexport.Spec.Ports)
		err = r.Status().Update(ctx, serviceexport)
		if err != nil {
			log.Error(err, "Failed to update serviceexport initial status")
			return ctrl.Result{}, err
		}

		log.Info("serviceexport updated with initial status")

		return ctrl.Result{Requeue: true}, nil
	}

	if serviceexport.Status.ExposedPorts != portListToDisplayString(serviceexport.Spec.Ports) {
		serviceexport.Status.ExposedPorts = portListToDisplayString(serviceexport.Spec.Ports)
		serviceexport.Status.LastSync = 0
		err = r.Status().Update(ctx, serviceexport)
		if err != nil {
			log.Error(err, "Failed to update serviceexport ports")
			//post event to service export
			r.EventRecorder.Record(
				&events.Event{
					Object:    serviceexport,
					EventType: events.EventTypeWarning,
					Reason:    "Error",
					Message:   "Failed to update serviceexport ports",
				},
			)
			return ctrl.Result{}, err
		}

		log.Info("serviceexport updated with ports")

		return ctrl.Result{Requeue: true}, nil
	}

	res, err, requeue := r.ReconcileAppPod(ctx, serviceexport)
	if requeue {
		log.Info("app pods reconciled")
		debugLog.Info("requeuing after app pod reconcile", "res", res, "er", err)
		return res, err
	}

	if serviceexport.Status.LastSync == 0 {
		ingressGwPod, err := controllers.GetSliceIngressGwPod(ctx, r.Client, serviceexport.Spec.Slice)
		if err != nil {
			log.Error(err, "Failed to get ingress gw pod info")
			return ctrl.Result{}, err
		}

		if ingressGwPod != nil {
			ep := &kubeslicev1beta1.ServicePod{
				Name:  fmt.Sprintf("%s-%s-ingress", serviceexport.Name, serviceexport.ObjectMeta.Namespace),
				NsmIP: ingressGwPod.NsmIP,
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
			r.Status().Update(ctx, serviceexport)
			//post event to service export
			r.EventRecorder.Record(
				&events.Event{
					Object:    serviceexport,
					EventType: events.EventTypeWarning,
					Reason:    "Error",
					Message:   "Failed to post serviceexport to kubeslice-controller cluster",
				})
			return ctrl.Result{}, err
		}
		log.Info("serviceexport sync success")
		//post event to service export
		r.EventRecorder.Record(
			&events.Event{
				Object:    serviceexport,
				EventType: events.EventTypeNormal,
				Reason:    "Success",
				Message:   "Successfully posted serviceexport to kubeslice-controller cluster",
			})
		currentTime := time.Now().Unix()
		serviceexport.Status.LastSync = currentTime
		err = r.Status().Update(ctx, serviceexport)
		if err != nil {
			log.Error(err, "Failed to update serviceexport sync time")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	res, err, requeue = r.ReconcileIstio(ctx, serviceexport)
	if requeue {
		log.Info("istio reconciled")
		debugLog.Info("requeuing after Istio reconcile", "res", res, "er", err)
		return res, err
	}

	// Set export status to ready when reconciliation is complete
	if serviceexport.Status.ExportStatus != kubeslicev1beta1.ExportStatusReady {
		serviceexport.Status.ExportStatus = kubeslicev1beta1.ExportStatusReady
		err = r.Status().Update(ctx, serviceexport)
		if err != nil {
			log.Error(err, "Failed to update serviceexport export status")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{
		RequeueAfter: 30 * time.Second,
	}, nil
}

// SetupWithManager setus up reconciler with manager
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kubeslicev1beta1.ServiceExport{}).
		Complete(r)
}
