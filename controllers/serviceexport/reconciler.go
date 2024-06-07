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
	"reflect"
	"time"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/kubeslice/kubeslice-monitoring/pkg/events"
	"github.com/kubeslice/kubeslice-monitoring/pkg/metrics"
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	"github.com/kubeslice/worker-operator/controllers"
	sliceController "github.com/kubeslice/worker-operator/controllers/slice"
	ossEvents "github.com/kubeslice/worker-operator/events"
	"github.com/kubeslice/worker-operator/pkg/logger"
	"github.com/kubeslice/worker-operator/pkg/utils"
)

// Reconciler reconciles serviceexport resource
type Reconciler struct {
	client.Client
	Log           logr.Logger
	Scheme        *runtime.Scheme
	HubClient     HubClientProvider
	EventRecorder *events.EventRecorder

	// metrics
	gaugeEndpoints *prometheus.GaugeVec
	gaugeAliases   *prometheus.GaugeVec
}

type HubClientProvider interface {
	UpdateServiceExport(ctx context.Context, serviceexport *kubeslicev1beta1.ServiceExport) error
	UpdateServiceExportEndpointForIngressGw(ctx context.Context, serviceexport *kubeslicev1beta1.ServiceExport, ep *kubeslicev1beta1.ServicePod) error
	DeleteServiceExport(ctx context.Context, serviceexport *kubeslicev1beta1.ServiceExport) error
}

var finalizerName = "networking.kubeslice.io/serviceexport-finalizer"
var controllerName = "serviceExportController"

// +kubebuilder:rbac:groups=networking.kubeslice.io,resources=serviceexports,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.kubeslice.io,resources=serviceexports/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.kubeslice.io,resources=serviceexports/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch

// Reconcile reconciles serviceexport
func (r Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("serviceexport", req.NamespacedName)

	serviceexport, err := r.GetServiceExport(ctx, req, &log)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Return and don't requeue
			log.Info("serviceexport resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	*r.EventRecorder = (*r.EventRecorder).WithSlice(serviceexport.Spec.Slice)
	log = log.WithValues("slice", serviceexport.Spec.Slice)
	debugLog := log.V(1)
	ctx = logger.WithLogger(ctx, log)

	log.Info("reconciling", "serviceexport", serviceexport.Name)

	//Adding finalizer or deleting serviceExport if under deletion
	requeue, result, err := r.handleServiceExportDeletion(ctx, serviceexport, &log)
	if requeue {
		return result, err
	}
	// update service export with slice Label if not present
	requeue, result, err = r.labelServiceExportWithSlice(ctx, serviceexport, &debugLog)
	if requeue {
		return result, err
	}
	// Reconciler running for the first time. Set the initial status here
	if serviceexport.Status.ExportStatus == kubeslicev1beta1.ExportStatusInitial {
		serviceexport.Status.DNSName = serviceexport.Name + "." + serviceexport.Namespace + ".svc.slice.local"
		serviceexport.Status.ExportStatus = kubeslicev1beta1.ExportStatusPending
		serviceexport.Status.ExposedPorts = portListToDisplayString(serviceexport.Spec.Ports)
		err := r.Status().Update(ctx, serviceexport)
		if err != nil {
			log.Error(err, "Failed to update serviceexport initial status")
			utils.RecordEvent(ctx, r.EventRecorder, serviceexport, nil, ossEvents.EventSliceServiceExportInitialStatusUpdateFailed, controllerName)
			return ctrl.Result{}, err
		}

		log.Info("serviceexport updated with initial status")
		utils.RecordEvent(ctx, r.EventRecorder, serviceexport, nil, ossEvents.EventServiceExportInitialStatusUpdated, controllerName)
		return ctrl.Result{Requeue: true}, nil
	}

	slice, err := controllers.GetSlice(ctx, r.Client, serviceexport.Spec.Slice)
	if err != nil {
		log.Error(err, "Unable to fetch slice for serviceexport")
		utils.RecordEvent(ctx, r.EventRecorder, serviceexport, nil, ossEvents.EventServiceExportSliceFetchFailed, controllerName)
		return ctrl.Result{RequeueAfter: controllers.ReconcileInterval}, nil
	}

	if !isValidNameSpace(serviceexport.Namespace, slice) {
		log.Error(fmt.Errorf("serviceexport ns is not part of the slice"), "couldn't onboard serviceexport")
		if serviceexport.Status.ExportStatus != kubeslicev1beta1.ExportStatusPending {
			serviceexport.Status.ExportStatus = kubeslicev1beta1.ExportStatusPending
			if err := r.Status().Update(ctx, serviceexport); err != nil {
				log.Error(err, "unable to update serviceexport status")
			}
		}
		utils.RecordEvent(ctx, r.EventRecorder, serviceexport, nil, ossEvents.EventServiceExportStatusPending, controllerName)
		return ctrl.Result{RequeueAfter: controllers.ReconcileInterval}, nil
	}

	if serviceexport.Status.ExposedPorts != portListToDisplayString(serviceexport.Spec.Ports) {
		serviceexport.Status.ExposedPorts = portListToDisplayString(serviceexport.Spec.Ports)
		serviceexport.Status.LastSync = 0
		err := r.Status().Update(ctx, serviceexport)
		if err != nil {
			log.Error(err, "Failed to update serviceexport ports")
			//post event to service export
			utils.RecordEvent(ctx, r.EventRecorder, serviceexport, nil, ossEvents.EventServiceExportUpdatePortsFailed, controllerName)
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
	r.gaugeEndpoints.WithLabelValues(serviceexport.Spec.Slice, serviceexport.Namespace, serviceexport.Name).Set(float64(serviceexport.Status.AvailableEndpoints))
	res, err, requeue = r.ReconcileIngressGwPod(ctx, serviceexport)
	if err != nil {
		utils.RecordEvent(ctx, r.EventRecorder, serviceexport, nil, ossEvents.EventIngressGWPodReconcileFailed, controllerName)
		return ctrl.Result{}, err
	}
	if requeue {
		log.Info("ingress gw pod reconciled")
		utils.RecordEvent(ctx, r.EventRecorder, serviceexport, nil, ossEvents.EventIngressGWPodReconciledSuccessfully, controllerName)
		debugLog.Info("requeuing after ingress gw pod reconcile", "res", res, "er", err)
		return res, nil
	}

	res, err, requeue = r.ReconcileAliases(ctx, serviceexport)
	if err != nil {
		return ctrl.Result{}, err
	}
	if requeue {
		log.Info("aliases reconciled")
		r.gaugeAliases.WithLabelValues(serviceexport.Spec.Slice, serviceexport.Namespace, serviceexport.Name).Set(float64(len(serviceexport.Status.Aliases)))
		debugLog.Info("requeuing after aliases reconcile", "res", res, "er", err)
		return res, nil
	}

	res, err, requeue = r.SyncSvcExportStatus(ctx, serviceexport)
	if err != nil {
		return ctrl.Result{}, err
	}
	if requeue {
		log.Info("synched serviceexport status")
		debugLog.Info("requeuing after serviceexport sync", "res", res, "er", err)
		return res, nil
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

func isValidNameSpace(ns string, slice *kubeslicev1beta1.Slice) bool {
	if ns == fmt.Sprintf(sliceController.VPC_NS_FMT, slice.Name) {
		return true
	}
	return arrayContainsString(slice.Status.ApplicationNamespaces, ns)
}

// Setup ServiceExport Reconciler
// Initializes metrics and sets up with manager
func (r *Reconciler) Setup(mgr ctrl.Manager, mf metrics.MetricsFactory) error {
	gaugeEndpoints := mf.NewGauge("serviceexport_endpoints", "Active endpoints in serviceexport", []string{"slice", "slice_namespace", "slice_service"})
	gaugeAliases := mf.NewGauge("serviceexport_aliases", "Active aliases in serviceexport", []string{"slice", "slice_namespace", "slice_service"})

	r.gaugeEndpoints = gaugeEndpoints
	r.gaugeAliases = gaugeAliases

	return r.SetupWithManager(mgr)
}

func (r *Reconciler) mapPodsToServiceExport(ctx context.Context, obj client.Object) (recs []reconcile.Request) {
	log := logger.FromContext(ctx)
	debugLog := log.V(1)
	debugLog.Info("triggered watcher for svc export", "obj", obj.GetName())
	_, ok := obj.(*corev1.Pod)
	if !ok {
		debugLog.Info("Unexpected object type in ServiceExport reconciler watch predicate expected *corev1.Pod found ", reflect.TypeOf(obj))
		return
	}
	// get all svc export in the app ns
	svcexpList := &kubeslicev1beta1.ServiceExportList{}
	listOpts := []client.ListOption{
		client.InNamespace(obj.GetNamespace()),
	}
	err := r.List(ctx, svcexpList, listOpts...)
	if err != nil {
		log.Error(err, "Failed to list service export", "application namespace", obj.GetNamespace())
		return
	}
	debugLog.Info("Service export found in app ns", "count", len(svcexpList.Items))
	for _, svcexp := range svcexpList.Items {
		selector, err := metav1.LabelSelectorAsSelector(svcexp.Spec.Selector)
		if err != nil {
			log.Error(err, "Failed to parse selector", "service export", svcexp.Name, "selector", svcexp.Spec.Selector)
			continue
		}
		if selector.Matches(labels.Set(obj.GetLabels())) {
			debugLog.Info("requeueing svc export", "obj", types.NamespacedName{
				Name:      svcexp.Name,
				Namespace: svcexp.Namespace,
			})
			recs = append(recs, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      svcexp.Name,
					Namespace: svcexp.Namespace,
				},
			})
		}
	}
	return recs
}

// SetupWithManager setus up reconciler with manager
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	var labelSelector metav1.LabelSelector
	// "kubeslice.io/pod-type": "app"
	labelSelector.MatchLabels = map[string]string{controllers.PodTypeSelectorLabelKey: controllers.PodTypeSelectorValueApp}

	return ctrl.NewControllerManagedBy(mgr).
		For(&kubeslicev1beta1.ServiceExport{}).
		Watches(
			&corev1.Pod{},
			handler.EnqueueRequestsFromMapFunc(r.mapPodsToServiceExport),
			builder.WithPredicates(predicate.Funcs{
				CreateFunc: func(e event.CreateEvent) bool {
					return false
				},
				DeleteFunc: func(e event.DeleteEvent) bool {
					selector, err := metav1.LabelSelectorAsSelector(&labelSelector)
					if err != nil {
						return false
					}
					if selector.Matches(labels.Set(e.Object.GetLabels())) {
						return true
					}
					return false
				},
				UpdateFunc: func(e event.UpdateEvent) bool {
					return false
				},
				GenericFunc: func(e event.GenericEvent) bool {
					return false
				},
			}),
		).
		Complete(r)
}

func (r *Reconciler) GetServiceExport(ctx context.Context, req ctrl.Request, log *logr.Logger) (*kubeslicev1beta1.ServiceExport, error) {
	serviceexport := &kubeslicev1beta1.ServiceExport{}
	err := r.Get(ctx, req.NamespacedName, serviceexport)
	if err != nil {
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get serviceexport")
		return nil, err
	}
	return serviceexport, nil
}

// handleServiceExportDeletion adds a finalizer to the serviceexport object
// or if the finalizer is present on the serviceexport, it deletes serviceexport
// and all the releated resources
// returns requeue flag (to either requeue or stop requeing), the reconcilation result and error
func (r *Reconciler) handleServiceExportDeletion(ctx context.Context, serviceexport *kubeslicev1beta1.ServiceExport, log *logr.Logger) (bool, ctrl.Result, error) {
	// examine DeletionTimestamp to determine if object is under deletion
	if serviceexport.ObjectMeta.DeletionTimestamp.IsZero() {
		// register our finalizer
		if !containsString(serviceexport.GetFinalizers(), finalizerName) {
			controllerutil.AddFinalizer(serviceexport, finalizerName)
			if err := r.Update(ctx, serviceexport); err != nil {
				return true, ctrl.Result{}, err
			}
		}
		return false, ctrl.Result{}, nil
	}
	// The object is being deleted
	if containsString(serviceexport.GetFinalizers(), finalizerName) {
		log.Info("deleting serviceexport")
		if err := r.HubClient.DeleteServiceExport(ctx, serviceexport); err != nil {
			log.Error(err, "unable to delete service export on the hub from the spoke")
			return true, ctrl.Result{}, err
		}

		if err := r.DeleteServiceExportResources(ctx, serviceexport); err != nil {
			utils.RecordEvent(ctx, r.EventRecorder, serviceexport, nil, ossEvents.EventServiceExportDeleteFailed, controllerName)
			log.Error(err, "unable to delete service export resources")
			return true, ctrl.Result{}, err
		}

		log.Info("removing finalizer")
		controllerutil.RemoveFinalizer(serviceexport, finalizerName)
		if err := r.Update(ctx, serviceexport); err != nil {
			log.Error(err, "unable to remove finalizer from serviceexport")
			return true, ctrl.Result{}, err
		}
		utils.RecordEvent(ctx, r.EventRecorder, serviceexport, nil, ossEvents.EventServiceExportDeleted, controllerName)
	}
	return true, ctrl.Result{}, nil
}

// labelServiceExportWithSlice adds a label to the serviceexport object
// returns requeue flag (to either requeue or stop requeing), the reconcilation result and error
func (r *Reconciler) labelServiceExportWithSlice(ctx context.Context, serviceexport *kubeslicev1beta1.ServiceExport, debugLog *logr.Logger) (bool, ctrl.Result, error) {
	labels := serviceexport.GetLabels()
	if value, exists := labels[controllers.ApplicationNamespaceSelectorLabelKey]; !exists || value != serviceexport.Spec.Slice {
		// the label does not exists or the sliceName is incorrect
		if labels == nil {
			labels = make(map[string]string)
		}
		labels[controllers.ApplicationNamespaceSelectorLabelKey] = serviceexport.Spec.Slice
		serviceexport.SetLabels(labels)

		if err := r.Update(ctx, serviceexport); err != nil {
			return true, ctrl.Result{}, err
		}
		debugLog.Info("Added Label for serviceexport", "serviceexport", serviceexport.Name)
		return true, ctrl.Result{Requeue: true}, nil
	}
	return false, ctrl.Result{}, nil
}
