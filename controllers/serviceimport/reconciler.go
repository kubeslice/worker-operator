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

package serviceimport

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"github.com/kubeslice/worker-operator/internal/logger"
	"github.com/kubeslice/worker-operator/pkg/events"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// Reconciler reconciles a ServiceImport object
type Reconciler struct {
	client.Client
	Log           logr.Logger
	Scheme        *runtime.Scheme
	ClusterID     string
	EventRecorder *events.EventRecorder
}

var finalizerName = "networking.kubeslice.io/serviceimport-finalizer"

// NewReconciler creates a new reconciler for serviceimport
func NewReconciler(c client.Client, s *runtime.Scheme, clusterId string) Reconciler {
	return Reconciler{
		Client:    c,
		Log:       ctrl.Log.WithName("controllers").WithName("ServiceImport"),
		Scheme:    s,
		ClusterID: clusterId,
	}
}

// +kubebuilder:rbac:groups=networking.kubeslice.io,resources=serviceimports,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.kubeslice.io,resources=serviceimports/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;create;update

// Reconcile reconciles serviceimport CR
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("serviceimport", req.NamespacedName)

	serviceimport := &kubeslicev1beta1.ServiceImport{}
	err := r.Get(ctx, req.NamespacedName, serviceimport)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Return and don't requeue
			log.Info("serviceimport resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get serviceimport")
		return ctrl.Result{}, err
	}

	log = log.WithValues("slice", serviceimport.Spec.Slice)
	debugLog := log.V(1)
	ctx = logger.WithLogger(ctx, log)

	log.Info("reconciling", "serviceimport", serviceimport.Name)

	// examine DeletionTimestamp to determine if object is under deletion
	if serviceimport.ObjectMeta.DeletionTimestamp.IsZero() {
		// register our finalizer
		if !containsString(serviceimport.GetFinalizers(), finalizerName) {
			log.Info("adding finalizer")
			controllerutil.AddFinalizer(serviceimport, finalizerName)
			if err := r.Update(ctx, serviceimport); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{Requeue: true}, nil
		}
	} else {
		// The object is being deleted
		if containsString(serviceimport.GetFinalizers(), finalizerName) {
			log.Info("deleting serviceimport")
			if err := r.DeleteServiceImportResources(ctx, serviceimport); err != nil {
				log.Error(err, "unable to delete service import resources")
				return ctrl.Result{}, err
			}

			log.Info("removing finalizer")
			controllerutil.RemoveFinalizer(serviceimport, finalizerName)
			if err := r.Update(ctx, serviceimport); err != nil {
				return ctrl.Result{}, err
			}
		}

		return ctrl.Result{}, nil
	}

	if serviceimport.Status.ExposedPorts != portListToDisplayString(serviceimport.Spec.Ports) {
		serviceimport.Status.ExposedPorts = portListToDisplayString(serviceimport.Spec.Ports)
		if serviceimport.Status.ImportStatus == kubeslicev1beta1.ImportStatusInitial {
			serviceimport.Status.ImportStatus = kubeslicev1beta1.ImportStatusPending
		}
		err = r.Status().Update(ctx, serviceimport)
		if err != nil {
			log.Error(err, "Failed to update serviceimport ports")
			//post event to service import
			r.EventRecorder.Record(
				&events.Event{
					Object:    serviceimport,
					EventType: events.EventTypeWarning,
					Reason:    "Error",
					Message:   "Failed to update serviceimport ports",
				},
			)
			return ctrl.Result{}, err
		}
		log.Info("serviceimport updated with ports")
		return ctrl.Result{Requeue: true}, nil
	}

	if serviceimport.Status.AvailableEndpoints != len(serviceimport.Status.Endpoints) {
		serviceimport.Status.AvailableEndpoints = len(serviceimport.Status.Endpoints)
		err = r.Status().Update(ctx, serviceimport)
		if err != nil {
			log.Error(err, "Failed to update availableendpoints")
			//post event to service import
			r.EventRecorder.Record(
				&events.Event{
					Object:    serviceimport,
					EventType: events.EventTypeWarning,
					Reason:    "Error",
					Message:   "Failed to update available endpoints in service import",
				},
			)
			return ctrl.Result{}, err
		}
		log.Info("serviceimport updated with availableendpoints")
	}

	res, err, requeue := r.reconcileDNSEntries(ctx, serviceimport)
	if requeue {
		log.Info("DNS entries reconciled")
		debugLog.Info("requeuing after DNS reconcile", "res", res, "er", err)
		return res, err
	}

	res, err, requeue = r.reconcileIstio(ctx, serviceimport)
	if requeue {
		log.Info("reconciled istio resources")
		debugLog.Info("requeuing after Istio reconcile", "res", res, "er", err)
		return res, err
	}

	// Set import status to ready when reconciliation is complete
	if serviceimport.Status.ImportStatus != kubeslicev1beta1.ImportStatusReady {
		serviceimport.Status.ImportStatus = kubeslicev1beta1.ImportStatusReady
		err = r.Status().Update(ctx, serviceimport)
		if err != nil {
			log.Error(err, "Failed to update serviceimport import status")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{
		RequeueAfter: 10 * time.Second,
	}, nil
}

// SetupWithManager sets up reconciler with manager
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kubeslicev1beta1.ServiceImport{}).
		Complete(r)
}
