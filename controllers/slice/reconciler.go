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

package slice

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	"github.com/kubeslice/worker-operator/controllers"
	"github.com/kubeslice/worker-operator/internal/logger"
	"github.com/kubeslice/worker-operator/internal/manifest"
	"github.com/kubeslice/worker-operator/pkg/events"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var sliceFinalizer = "mesh.kubeslice.io/slice-finalizer"

// SliceReconciler reconciles a Slice object
type SliceReconciler struct {
	client.Client
	EventRecorder *events.EventRecorder
	Scheme        *runtime.Scheme
	Log           logr.Logger
	NetOpPods     []NetOpPod
	HubClient     HubClientProvider
}

//+kubebuilder:rbac:groups=networking.kubeslice.io,resources=slice,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=networking.kubeslice.io,resources=slice/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=networking.kubeslice.io,resources=slice/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;update
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=networking.istio.io,resources=gateways,verbs=get;list;create;update;watch;delete
//+kubebuilder:rbac:groups=networking.istio.io,resources=serviceentries,verbs=get;list;create;update;watch;delete
//+kubebuilder:rbac:groups=networking.istio.io,resources=virtualservices,verbs=get;list;create;update;watch;delete
//+kubebuilder:webhook:path=/mutate-appsv1-deploy,mutating=true,failurePolicy=fail,groups="apps",resources=deployments,verbs=create;update,versions=v1,name=mdeploy.avesha.io,admissionReviewVersions=v1,sideEffects=NoneOnDryRun
//+kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch

func (r *SliceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("slice", req.NamespacedName)
	debugLog := log.V(1)
	ctx = logger.WithLogger(ctx, log)

	slice := &kubeslicev1beta1.Slice{}

	err := r.Get(ctx, req.NamespacedName, slice)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Return and don't requeue
			log.Info("Slice resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get Slice")
		return ctrl.Result{}, err
	}

	log.Info("reconciling", "slice", slice.Name)

	// Examine DeletionTimestamp to determine if object is under deletion
	if slice.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !controllerutil.ContainsFinalizer(slice, sliceFinalizer) {
			controllerutil.AddFinalizer(slice, sliceFinalizer)
			if err := r.Update(ctx, slice); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(slice, sliceFinalizer) {
			log.Info("Deleting slice", "slice", slice.Name)
			// send slice deletion event to netops
			err = r.SendSliceDeletionEventToNetOp(ctx, req.NamespacedName.Name, req.NamespacedName.Namespace)
			if err != nil {
				log.Error(err, "Failed to send slice deletetion event to netop")
			}
			//cleanup slice resources
			r.cleanupSliceResources(ctx, slice)
			controllerutil.RemoveFinalizer(slice, sliceFinalizer)
			if err := r.Update(ctx, slice); err != nil {
				return ctrl.Result{}, err
			}
		}
		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	if slice.Status.DNSIP == "" {
		log.Info("Finding DNS IP")
		svc := &corev1.Service{}
		err = r.Get(ctx, types.NamespacedName{
			Namespace: controllers.ControlPlaneNamespace,
			Name:      controllers.DNSDeploymentName,
		}, svc)

		if err != nil {
			if errors.IsNotFound(err) {
				log.Error(err, "dns not found")
				log.Info("DNS service not found in the cluster, probably coredns is not deployed; continuing")
			} else {
				log.Error(err, "Unable to find DNS Service")
				return ctrl.Result{}, err
			}
		} else {
			debugLog.Info("got dns service", "svc", svc)
			slice.Status.DNSIP = svc.Spec.ClusterIP
			err = r.Status().Update(ctx, slice)
			if err != nil {
				log.Error(err, "Failed to update Slice status for dns")
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
	}

	if slice.Status.SliceConfig == nil {
		err := fmt.Errorf("slice not reconciled from hub")
		log.Error(err, "Slice is not reconciled from hub yet, skipping reconciliation")
		return ctrl.Result{}, err
	}

	res, err, reconcile := r.ReconcileSliceNamespaces(ctx, slice)
	if reconcile {
		debugLog.Info("Reconciling SliceNamespaces", "res", res, "err", err)
		return res, err
	}

	debugLog.Info("Syncing slice QoS config with NetOp pods")
	err = r.SyncSliceQosProfileWithNetOp(ctx, slice)
	if err != nil {
		log.Error(err, "Failed to sync QoS profile with netop pods")
		//post event to slice
		r.EventRecorder.Record(
			&events.Event{
				Object:    slice,
				EventType: events.EventTypeWarning,
				Reason:    "Error",
				Message:   "Failed to sync QoS profile with netop pods",
			},
		)
	}

	log.Info("ExternalGatewayConfig", "egw", slice.Status.SliceConfig)

	if slice.Status.SliceConfig.ExternalGatewayConfig != nil && slice.Status.SliceConfig.ExternalGatewayConfig.Egress.Enabled {
		debugLog.Info("Installing egress")
		err = manifest.InstallEgress(ctx, r.Client, slice)
		if err != nil {
			log.Error(err, "unable to install egress")
			//post event to slice
			r.EventRecorder.Record(
				&events.Event{
					Object:    slice,
					EventType: events.EventTypeWarning,
					Reason:    "Error",
					Message:   "Failed to install egress",
				},
			)
			return ctrl.Result{}, nil
		}
	}

	if slice.Status.SliceConfig.ExternalGatewayConfig != nil && slice.Status.SliceConfig.ExternalGatewayConfig.Ingress.Enabled {
		debugLog.Info("Installing ingress")
		err = manifest.InstallIngress(ctx, r.Client, slice)
		if err != nil {
			log.Error(err, "unable to install ingress")
			//post event to slice
			r.EventRecorder.Record(
				&events.Event{
					Object:    slice,
					EventType: events.EventTypeWarning,
					Reason:    "Error",
					Message:   "Failed to install ingress",
				},
			)
			return ctrl.Result{}, nil
		}
	}

	res, err, requeue := r.ReconcileSliceRouter(ctx, slice)
	if requeue {
		return res, err
	}

	debugLog.Info("fetching app pods")
	appPods, err := r.getAppPods(ctx, slice)
	debugLog.Info("app pods", "pods", appPods, "err", err)

	if isAppPodStatusChanged(appPods, slice.Status.AppPods) {
		log.Info("App pod status changed")

		slice.Status.AppPods = appPods
		slice.Status.AppPodsUpdatedOn = time.Now().Unix()

		err = r.Status().Update(ctx, slice)
		if err != nil {
			log.Error(err, "Failed to update Slice status for app pods")
			return ctrl.Result{}, err
		}
		log.Info("App pod status updated in slice")

		return ctrl.Result{Requeue: true}, nil
	}

	debugLog.Info("reconciling app pods")
	res, err, requeue = r.ReconcileAppPod(ctx, slice)

	if requeue {
		log.Info("app pods reconciled")

		if err != nil {
			// app pod reconciliation failed
			return res, err
		}

		// reconciliation success, update the app pod list in controller
		log.Info("updating app pod list in hub workersliceconfig status")
		sliceConfigName := slice.Name + "-" + controllers.ClusterName
		if err = r.HubClient.UpdateAppPodsList(ctx, sliceConfigName, slice.Status.AppPods); err != nil {
			log.Error(err, "Failed to update app pod list in hub")
			//post event to slice
			r.EventRecorder.Record(
				&events.Event{
					Object:    slice,
					EventType: events.EventTypeWarning,
					Reason:    "Error",
					Message:   "Failed to update app pod list in kubeslice-controller cluster",
				},
			)
			return ctrl.Result{}, err
		}

		return ctrl.Result{
			Requeue: true,
		}, nil

	}
	return ctrl.Result{
		RequeueAfter: controllers.ReconcileInterval,
	}, nil
}

func isAppPodStatusChanged(current []kubeslicev1beta1.AppPod, old []kubeslicev1beta1.AppPod) bool {
	if len(current) != len(old) {
		return true
	}

	s := make(map[string]string)

	for _, c := range old {
		s[c.PodIP] = c.PodName
	}

	for _, c := range current {
		if s[c.PodIP] != c.PodName {
			return true
		}
	}

	return false
}

// SetupWithManager sets up the controller with the Manager.
func (r *SliceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kubeslicev1beta1.Slice{}).
		Owns(&appsv1.Deployment{}).
		Owns(&kubeslicev1beta1.SliceGateway{}).
		Complete(r)
}
