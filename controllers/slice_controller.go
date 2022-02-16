/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	meshv1beta1 "bitbucket.org/realtimeai/kubeslice-operator/api/v1beta1"
	"bitbucket.org/realtimeai/kubeslice-operator/internal/logger"
)

// SliceReconciler reconciles a Slice object
type SliceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
}

//+kubebuilder:rbac:groups=mesh.avesha.io,resources=slice,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=mesh.avesha.io,resources=slice/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=mesh.avesha.io,resources=slice/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:webhook:path=/mutate-appsv1-deploy,mutating=true,failurePolicy=fail,groups="apps",resources=deployments,verbs=create;update,versions=v1,name=mdeploy.avesha.io,admissionReviewVersions=v1,sideEffects=NoneOnDryRun

func (r *SliceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("slice", req.NamespacedName)
	debugLog := log.V(1)
	ctx = logger.WithLogger(ctx, log)

	slice := &meshv1beta1.Slice{}

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

	if slice.Status.DNSIP == "" {
		log.Info("Finding DNS IP")
		svc := &corev1.Service{}
		err = r.Get(ctx, types.NamespacedName{
			Namespace: ControlPlaneNamespace,
			Name:      DNSDeploymentName,
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

	res, err, requeue := r.ReconcileSliceRouter(ctx, slice)
	if requeue {
		return res, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SliceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&meshv1beta1.Slice{}).
		Complete(r)
}
