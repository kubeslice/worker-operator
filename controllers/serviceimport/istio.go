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
	"fmt"

	meshv1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	"github.com/kubeslice/worker-operator/controllers"
	"github.com/kubeslice/worker-operator/internal/logger"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

// reconcile istio resources like virtualService and serviceentry to facilitate service to service connectivity
func (r *Reconciler) reconcileIstio(ctx context.Context, serviceimport *meshv1beta1.ServiceImport) (ctrl.Result, error, bool) {
	log := logger.FromContext(ctx).WithValues("type", "Istio")
	debugLog := log.V(1)

	slice, err := controllers.GetSlice(ctx, r.Client, serviceimport.Spec.Slice)
	if err != nil {
		log.Error(err, "Unable to fetch slice for serviceexport")
		return ctrl.Result{}, err, true
	}

	if slice.Status.SliceConfig == nil {
		err := fmt.Errorf("sliceconfig is not reconciled from hub")
		log.Error(err, "unable to reconcile istio")
		return ctrl.Result{}, err, true
	}

	if slice.Status.SliceConfig.ExternalGatewayConfig == nil ||
		slice.Status.SliceConfig.ExternalGatewayConfig.GatewayType != "istio" {
		debugLog.Info("istio not enabled for slice, skipping reconcilation")
		return ctrl.Result{}, nil, false
	}

	debugLog.Info("reconciling istio")

	// Create k8s service for app pods to connect
	res, err, requeue := r.ReconcileService(ctx, serviceimport)
	if requeue {
		return res, err, requeue
	}

	egressGatewayConfig := slice.Status.SliceConfig.ExternalGatewayConfig.Egress

	// Add service entries and virtualServices for app pods in the slice to be able to route traffic through slice.
	// Endpoints are load balanced at equal weights.
	if egressGatewayConfig != nil && egressGatewayConfig.Enabled {
		res, err, requeue = r.ReconcileServiceEntries(ctx, serviceimport, controllers.ControlPlaneNamespace)
		if requeue {
			return res, err, requeue
		}
		res, err, requeue = r.ReconcileVirtualServiceEgress(ctx, serviceimport)
		if requeue {
			return res, err, requeue
		}
	} else {
		res, err, requeue = r.ReconcileServiceEntries(ctx, serviceimport, serviceimport.Namespace)
		if requeue {
			return res, err, requeue
		}
		res, err, requeue = r.ReconcileVirtualServiceNonEgress(ctx, serviceimport)
		if requeue {
			return res, err, requeue
		}
	}

	return ctrl.Result{}, nil, false
}

func (r *Reconciler) ReconcileService(ctx context.Context, serviceimport *meshv1beta1.ServiceImport) (ctrl.Result, error, bool) {
	log := logger.FromContext(ctx).WithValues("type", "Istio Service")

	svc := &corev1.Service{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      serviceimport.Name,
		Namespace: serviceimport.Namespace,
	}, svc)
	if err != nil {
		if errors.IsNotFound(err) {
			// Define a new service
			svc := r.serviceForServiceImport(serviceimport)
			log.Info("Creating a new Service", "Namespace", svc.Namespace, "Name", svc.Name)
			err = r.Create(ctx, svc)
			if err != nil {
				log.Error(err, "Failed to create new Service", "Namespace", svc.Namespace, "Name", svc.Name)
				return ctrl.Result{}, err, true
			}
			return ctrl.Result{Requeue: true}, nil, true
		}
		log.Error(err, "Failed to get Service")
		return ctrl.Result{}, err, true
	}

	// TODO handle change of ports

	return ctrl.Result{}, nil, false
}

func (r *Reconciler) DeleteIstioResources(ctx context.Context, serviceimport *meshv1beta1.ServiceImport, slice *meshv1beta1.Slice) error {
	// We should only clean up resources that were created in the control plane namespace. Setting the service import object
	// in the app namespace as the owner reference does not clean up resources in other namespaces.
	// Resources in application namespaces are garbage collected because the owner reference for them is set to be the
	// service import object, so we do not have to delete them explicitly here.
	if slice.Status.SliceConfig.ExternalGatewayConfig == nil ||
		slice.Status.SliceConfig.ExternalGatewayConfig.Egress == nil ||
		!slice.Status.SliceConfig.ExternalGatewayConfig.Egress.Enabled {
		return nil
	}

	err := r.DeleteIstioServiceEntries(ctx, serviceimport)
	if err != nil {
		return err
	}

	err = r.DeleteIstioVirtualServicesEgress(ctx, serviceimport)
	if err != nil {
		return err
	}

	return nil
}
