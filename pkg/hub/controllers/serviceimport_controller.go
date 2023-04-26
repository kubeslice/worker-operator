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
	"time"

	spokev1alpha1 "github.com/kubeslice/apis/pkg/worker/v1alpha1"
	"github.com/kubeslice/kubeslice-monitoring/pkg/events"
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	"github.com/kubeslice/worker-operator/controllers"
	ossEvents "github.com/kubeslice/worker-operator/events"
	"github.com/kubeslice/worker-operator/pkg/logger"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type ServiceImportReconciler struct {
	client.Client
	MeshClient    client.Client
	EventRecorder events.EventRecorder
}

var svcimFinalizer = "controller.kubeslice.io/hubWorkerServiceImport-finalizer"

func getProtocol(protocol string) corev1.Protocol {
	switch protocol {
	case "TCP":
		return corev1.ProtocolTCP
	case "UDP":
		return corev1.ProtocolUDP
	case "SCTP":
		return corev1.ProtocolSCTP
	default:
		return ""
	}
}

func getMeshServiceImportPortList(svcim *spokev1alpha1.WorkerServiceImport) []kubeslicev1beta1.ServicePort {
	portList := []kubeslicev1beta1.ServicePort{}
	for _, port := range svcim.Spec.ServiceDiscoveryPorts {
		portList = append(portList, kubeslicev1beta1.ServicePort{
			Name:          port.Name,
			ContainerPort: port.Port,
			Protocol:      getProtocol(port.Protocol),
		})
	}

	return portList
}

func getMeshServiceImportEpList(svcim *spokev1alpha1.WorkerServiceImport) []kubeslicev1beta1.ServiceEndpoint {
	epList := []kubeslicev1beta1.ServiceEndpoint{}
	for _, ep := range svcim.Spec.ServiceDiscoveryEndpoints {
		epList = append(epList, kubeslicev1beta1.ServiceEndpoint{
			Name:      ep.PodName,
			IP:        ep.NsmIp,
			Port:      ep.Port,
			ClusterID: ep.Cluster,
			DNSName:   ep.DnsName,
		})
	}

	return epList
}

func getMeshServiceImportObj(svcim *spokev1alpha1.WorkerServiceImport) *kubeslicev1beta1.ServiceImport {
	return &kubeslicev1beta1.ServiceImport{
		ObjectMeta: metav1.ObjectMeta{
			Name:      svcim.Spec.ServiceName,
			Namespace: svcim.Spec.ServiceNamespace,
			Labels: map[string]string{
				controllers.ApplicationNamespaceSelectorLabelKey: svcim.Spec.SliceName,
			},
		},
		Spec: kubeslicev1beta1.ServiceImportSpec{
			Slice:   svcim.Spec.SliceName,
			DNSName: svcim.Spec.ServiceName + "." + svcim.Spec.ServiceNamespace + ".svc.slice.local",
			Ports:   getMeshServiceImportPortList(svcim),
			Aliases: svcim.Spec.Aliases,
		},
	}
}

func (r *ServiceImportReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := logger.FromContext(ctx)

	svcim, err := r.getServiceImport(ctx, req)
	if svcim == nil {
		return reconcile.Result{}, err
	}

	log.Info("got service import from hub", "serviceimport", svcim)

	// examine DeletionTimestamp to determine if object is under deletion
	// Register finalizer.
	// The object is being deleted
	// our finalizer is present, so lets handle any external dependency
	// if fail to delete the external dependency here, return with error
	// so that it can be retried
	// remove our finalizer from the list and update it.
	// Stop reconciliation as the item is being deleted
	requeue, result, err := r.handleSvcimDeletion(svcim, ctx)
	if requeue {
		return result, err
	}

	sliceName := svcim.Spec.SliceName
	meshSlice := &kubeslicev1beta1.Slice{}
	sliceRef := client.ObjectKey{
		Name:      sliceName,
		Namespace: ControlPlaneNamespace,
	}

	err = r.MeshClient.Get(ctx, sliceRef, meshSlice)
	if err != nil {
		log.Error(err, "slice object not present for service import. Waiting...", "serviceimport", svcim.Name)
		return reconcile.Result{
			RequeueAfter: 30 * time.Second,
		}, nil
	}

	meshSvcIm, err := r.getMeshServiceImport(ctx, svcim)
	if meshSvcIm == nil {
		log.Error(err, "unable to fetch mesh service import")
		return reconcile.Result{}, err
	}

	meshSvcIm.Spec.Ports = getMeshServiceImportPortList(svcim)
	meshSvcIm.Spec.Aliases = svcim.Spec.Aliases
	err = r.MeshClient.Update(ctx, meshSvcIm)
	if err != nil {
		log.Error(err, "unable to update service import in spoke cluster", "serviceimport", svcim.Name)
		//post event to service import created on spoke
		r.EventRecorder.RecordEvent(ctx,
			&events.Event{
				Object:            meshSvcIm,
				Name:              ossEvents.EventSliceServiceImportUpdateFailed,
				ReportingInstance: "workerserviceimport_controller",
			},
		)
		return reconcile.Result{}, err
	}
	meshSvcIm.Status.Endpoints = getMeshServiceImportEpList(svcim)
	err = r.MeshClient.Status().Update(ctx, meshSvcIm)
	if err != nil {
		log.Error(err, "unable to update service import in spoke cluster", "serviceimport", svcim.Name)
		r.EventRecorder.RecordEvent(ctx,
			&events.Event{
				Object:            meshSvcIm,
				Name:              ossEvents.EventSliceServiceImportUpdateFailed,
				ReportingInstance: "workerserviceimport_controller",
			},
		)
		return reconcile.Result{}, err
	}
	r.EventRecorder.RecordEvent(ctx, &events.Event{
		Object:            meshSvcIm,
		Name:              ossEvents.EventSliceServiceImportUpdated,
		ReportingInstance: "workerserviceimport_controller",
	})

	return reconcile.Result{}, nil
}

func (r *ServiceImportReconciler) getMeshServiceImport(ctx context.Context, svcim *spokev1alpha1.WorkerServiceImport) (*kubeslicev1beta1.ServiceImport, error) {
	log := logger.FromContext(ctx)
	meshSvcIm := &kubeslicev1beta1.ServiceImport{}
	err := r.MeshClient.Get(ctx, client.ObjectKey{
		Name:      svcim.Spec.ServiceName,
		Namespace: svcim.Spec.ServiceNamespace,
	}, meshSvcIm)
	if err != nil {
		if errors.IsNotFound(err) {
			meshSvcIm = getMeshServiceImportObj(svcim)
			err = r.MeshClient.Create(ctx, meshSvcIm)
			if err != nil {
				log.Error(err, "unable to create service import in spoke cluster", "serviceimport", svcim.Name)
				//post event to spokeserviceimport
				r.EventRecorder.RecordEvent(ctx,
					&events.Event{
						Object:            svcim,
						Name:              ossEvents.EventWorkerServiceImportCreateFailed,
						ReportingInstance: "workerserviceimport_controller",
					},
				)
				return nil, err
			}
			//post event to spokeserviceimport
			r.EventRecorder.RecordEvent(ctx,
				&events.Event{
					Object:            svcim,
					Name:              ossEvents.EventWorkerServiceImportCreated,
					ReportingInstance: "workerserviceimport_controller",
				},
			)

			meshSvcIm.Status.Endpoints = getMeshServiceImportEpList(svcim)
			err = r.MeshClient.Status().Update(ctx, meshSvcIm)
			if err != nil {
				log.Error(err, "unable to update service import in spoke cluster", "serviceimport", svcim.Name)
				return nil, err
			}
			return nil, nil
		}
		return nil, err
	}
	return meshSvcIm, nil

}

func (r *ServiceImportReconciler) getServiceImport(ctx context.Context, req reconcile.Request) (*spokev1alpha1.WorkerServiceImport, error) {
	log := logger.FromContext(ctx)
	svcim := &spokev1alpha1.WorkerServiceImport{}
	err := r.Get(ctx, req.NamespacedName, svcim)
	// Request object not found, could have been deleted after reconcile request.
	if err != nil {
		if errors.IsNotFound(err) {
			// Return and don't requeue
			log.Info("spoke service import resource not found in hub. Ignoring since object must be deleted")
			return nil, nil
		}
		return nil, err
	}
	return svcim, nil
}

func (r *ServiceImportReconciler) handleSvcimDeletion(svcim *spokev1alpha1.WorkerServiceImport, ctx context.Context) (bool, reconcile.Result, error) {
	log := logger.FromContext(ctx)
	// examine DeletionTimestamp to determine if object is under deletion
	if svcim.ObjectMeta.DeletionTimestamp.IsZero() {
		// Register finalizer.
		// The object is being deleted
		if !controllerutil.ContainsFinalizer(svcim, svcimFinalizer) {
			log.Info("adding finalizer")
			controllerutil.AddFinalizer(svcim, svcimFinalizer)
			if err := r.Update(ctx, svcim); err != nil {
				return true, reconcile.Result{}, err
			}
		}
	} else {

		if controllerutil.ContainsFinalizer(svcim, svcimFinalizer) {
			log.Info("deleting serviceimport")
			// our finalizer is present, so lets handle any external dependency
			if err := r.DeleteServiceImportOnSpoke(ctx, svcim); err != nil {
				// if fail to delete the external dependency here, return with error
				// so that it can be retried
				log.Error(err, "unable to delete service import on spoke")
				return true, reconcile.Result{}, err
			}
			log.Info("removing serviceimport finalizer")
			// remove our finalizer from the list and update it.
			// Stop reconciliation as the item is being deleted
			controllerutil.RemoveFinalizer(svcim, svcimFinalizer)
			if err := r.Update(ctx, svcim); err != nil {
				return true, reconcile.Result{}, err
			}
		}

		return true, reconcile.Result{}, nil
	}
	return false, reconcile.Result{}, nil
}

func (r *ServiceImportReconciler) DeleteServiceImportOnSpoke(ctx context.Context, svcim *spokev1alpha1.WorkerServiceImport) error {
	log := logger.FromContext(ctx)

	svcimOnSpoke := &kubeslicev1beta1.ServiceImport{
		ObjectMeta: metav1.ObjectMeta{
			Name:      svcim.Spec.ServiceName,
			Namespace: svcim.Spec.ServiceNamespace,
		},
	}

	err := r.MeshClient.Delete(ctx, svcimOnSpoke)
	if err != nil {
		return err
	}

	log.Info("Deleted serviceimport on spoke cluster", "slice", svcimOnSpoke.Name)
	return nil
}

func (a *ServiceImportReconciler) InjectClient(c client.Client) error {
	a.Client = c
	return nil
}
