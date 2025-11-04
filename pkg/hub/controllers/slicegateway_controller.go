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
	"strconv"
	"time"

	hubv1alpha1 "github.com/kubeslice/apis/pkg/controller/v1alpha1"
	spokev1alpha1 "github.com/kubeslice/apis/pkg/worker/v1alpha1"

	"github.com/kubeslice/kubeslice-monitoring/pkg/events"
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	"github.com/kubeslice/worker-operator/controllers"
	ossEvents "github.com/kubeslice/worker-operator/events"
	hubutils "github.com/kubeslice/worker-operator/pkg/hub"
	"github.com/kubeslice/worker-operator/pkg/logger"
	"github.com/kubeslice/worker-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type SliceGwReconciler struct {
	client.Client
	MeshClient    client.Client
	EventRecorder *events.EventRecorder
	ClusterName   string
}

var sliceGWController = "workersliceGWController"
var sliceGwFinalizer = "controller.kubeslice.io/sliceGw-finalizer"

func (r *SliceGwReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := logger.FromContext(ctx)

	sliceGw := &spokev1alpha1.WorkerSliceGateway{}
	err := r.Get(ctx, req.NamespacedName, sliceGw)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Return and don't requeue
			log.Info("SliceGw resource not found in hub. Ignoring since object must be deleted")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	log.Info("got sliceGw from hub", "sliceGw", sliceGw.Name)
	*r.EventRecorder = (*r.EventRecorder).WithSlice(sliceGw.Spec.SliceName)
	// Return if the slice gw resource does not belong to our cluster
	if sliceGw.Spec.LocalGatewayConfig.ClusterName != r.ClusterName {
		log.Info("sliceGw doesn't belong to this cluster", "sliceGw", sliceGw.Name, "cluster", clusterName, "slicegw cluster", sliceGw.Spec.LocalGatewayConfig.ClusterName)
		return reconcile.Result{}, nil
	}
	requeue, result, err := r.handleSliceGWDeletion(sliceGw, ctx, req)
	if requeue {
		return result, err
	}

	result, err = r.createSliceGwCerts(ctx, sliceGw, req)
	if err != nil {
		return result, err
	}

	sliceGwName := sliceGw.Name
	meshSliceGw := &kubeslicev1beta1.SliceGateway{}

	err = r.createSliceGwOnSpoke(ctx, sliceGw, meshSliceGw)
	if err != nil {
		utils.RecordEvent(ctx, r.EventRecorder, sliceGw, nil, ossEvents.EventSliceGWCreateFailed, sliceGWController)
		return reconcile.Result{}, err
	} else {
		utils.RecordEvent(ctx, r.EventRecorder, sliceGw, nil, ossEvents.EventSliceGWCreated, sliceGWController)
	}

	// Update the slicegateway CR on the worker cluster only if something has changed.
	toUpdate := false
	sliceGwRef := client.ObjectKey{
		Name:      sliceGwName,
		Namespace: ControlPlaneNamespace,
	}
	err = r.MeshClient.Get(ctx, sliceGwRef, meshSliceGw)
	if err != nil {
		return reconcile.Result{}, err
	}
	// First check all the static fields.
	if meshSliceGw.Status.Config.SliceGatewayID != sliceGw.Spec.LocalGatewayConfig.GatewayName ||
		meshSliceGw.Status.Config.SliceGatewaySubnet != sliceGw.Spec.LocalGatewayConfig.GatewaySubnet ||
		meshSliceGw.Status.Config.SliceGatewayRemoteSubnet != sliceGw.Spec.RemoteGatewayConfig.GatewaySubnet ||
		meshSliceGw.Status.Config.SliceGatewayHostType != sliceGw.Spec.GatewayHostType ||
		meshSliceGw.Status.Config.SliceGatewayRemoteClusterID != sliceGw.Spec.RemoteGatewayConfig.ClusterName ||
		meshSliceGw.Status.Config.SliceGatewayRemoteGatewayID != sliceGw.Spec.RemoteGatewayConfig.GatewayName ||
		meshSliceGw.Status.Config.SliceGatewayName != strconv.Itoa(sliceGw.Spec.GatewayNumber) ||
		meshSliceGw.Status.Config.SliceGatewayConnectivityType != sliceGw.Spec.GatewayConnectivityType ||
		meshSliceGw.Status.Config.SliceGatewayProtocol != sliceGw.Spec.GatewayProtocol ||
		meshSliceGw.Status.Config.SliceGatewayType != sliceGw.Spec.GatewayType {
		toUpdate = true
	}
	// If no change in static fields, check the dynamic fields
	if !toUpdate {
		// For client type, check the following fields
		if meshSliceGw.Status.Config.SliceGatewayHostType == "Client" {
			if !hubutils.ListEqual(meshSliceGw.Status.Config.SliceGatewayRemoteNodeIPs, sliceGw.Spec.RemoteGatewayConfig.NodeIps) {
				log.Info("Update from hub: Node IPs changed", "SliceGw", sliceGw.Name, "NodeIPs", sliceGw.Spec.RemoteGatewayConfig.NodeIps)
				toUpdate = true
			}
			// Next check node port
			if !toUpdate {
				if !hubutils.ListEqual(meshSliceGw.Status.Config.SliceGatewayNodePorts, sliceGw.Spec.RemoteGatewayConfig.NodePorts) {
					log.Info("Update from hub: NodePort numbers changed", "SliceGw", sliceGw.Name, "NodePorts", sliceGw.Spec.RemoteGatewayConfig.NodePorts)
					toUpdate = true
				}
			}
		}
		// Nothing dynamic to check for the server type
	}

	if toUpdate {
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			sliceGwRef := client.ObjectKey{
				Name:      sliceGwName,
				Namespace: ControlPlaneNamespace,
			}
			err := r.MeshClient.Get(ctx, sliceGwRef, meshSliceGw)
			if err != nil {
				return err
			}
			meshSliceGw.Status.Config = kubeslicev1beta1.SliceGatewayConfig{
				SliceName:                           sliceGw.Spec.SliceName,
				SliceGatewayID:                      sliceGw.Spec.LocalGatewayConfig.GatewayName,
				SliceGatewaySubnet:                  sliceGw.Spec.LocalGatewayConfig.GatewaySubnet,
				SliceGatewayRemoteSubnet:            sliceGw.Spec.RemoteGatewayConfig.GatewaySubnet,
				SliceGatewayHostType:                sliceGw.Spec.GatewayHostType,
				SliceGatewayRemoteNodeIPs:           sliceGw.Spec.RemoteGatewayConfig.NodeIps,
				SliceGatewayRemoteNodePorts:         sliceGw.Spec.RemoteGatewayConfig.NodePorts,
				SliceGatewayRemoteClusterID:         sliceGw.Spec.RemoteGatewayConfig.ClusterName,
				SliceGatewayRemoteGatewayID:         sliceGw.Spec.RemoteGatewayConfig.GatewayName,
				SliceGatewayLocalVpnIP:              sliceGw.Spec.LocalGatewayConfig.VpnIp,
				SliceGatewayRemoteVpnIP:             sliceGw.Spec.RemoteGatewayConfig.VpnIp,
				SliceGatewayName:                    strconv.Itoa(sliceGw.Spec.GatewayNumber),
				SliceGatewayType:                    sliceGw.Spec.GatewayType,
				SliceGatewayIntermediateDeployments: meshSliceGw.Status.Config.SliceGatewayIntermediateDeployments,
				SliceGatewayConnectivityType:        sliceGw.Spec.GatewayConnectivityType,
				SliceGatewayProtocol:                sliceGw.Spec.GatewayProtocol,
				SliceGatewayServerLBIPs:             sliceGw.Spec.RemoteGatewayConfig.LoadBalancerIps,
			}

			err = r.MeshClient.Status().Update(ctx, meshSliceGw)
			if err != nil {
				return err
			}
			utils.RecordEvent(ctx, r.EventRecorder, sliceGw, nil, ossEvents.EventSliceGWUpdated, sliceGWController)
			return nil
		})
		if err != nil {
			utils.RecordEvent(ctx, r.EventRecorder, sliceGw, nil, ossEvents.EventSliceGWUpdateFailed, sliceGWController)
			log.Error(err, "unable to update sliceGw status in spoke cluster", "sliceGw", sliceGwName)
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}

func (r *SliceGwReconciler) InjectClient(c client.Client) error {
	r.Client = c
	return nil
}

func (r *SliceGwReconciler) createSliceGwCerts(ctx context.Context, sliceGw *spokev1alpha1.WorkerSliceGateway, req reconcile.Request) (reconcile.Result, error) {
	log := logger.FromContext(ctx)
	meshSliceGwCerts := &corev1.Secret{}
	vpnKeyRotation := &hubv1alpha1.VpnKeyRotation{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      sliceGw.Spec.SliceName,
		Namespace: req.NamespacedName.Namespace,
	}, vpnKeyRotation)
	if err != nil {
		return reconcile.Result{}, err
	}
	meshSecretName := sliceGw.Name + "-" + strconv.Itoa(vpnKeyRotation.Spec.RotationCount) //get vpn key rotation count
	err = r.MeshClient.Get(ctx, types.NamespacedName{
		Name:      meshSecretName,
		Namespace: ControlPlaneNamespace,
	}, meshSliceGwCerts)
	if err != nil {
		if errors.IsNotFound(err) {
			sliceGwCerts := &corev1.Secret{}
			err := r.Get(ctx, req.NamespacedName, sliceGwCerts)
			if err != nil {
				log.Error(err, "unable to fetch slicegw certs from the hub", "sliceGw", sliceGw.Name)
				return reconcile.Result{
					RequeueAfter: 10 * time.Second,
				}, err
			}

			meshSliceGwCerts := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      meshSecretName,
					Namespace: ControlPlaneNamespace,
					Labels:    map[string]string{"kubeslice.io/slice-gw": sliceGw.Name},
				},
				Data: sliceGwCerts.Data,
			}
			err = r.MeshClient.Create(ctx, meshSliceGwCerts)
			if err != nil {
				log.Error(err, "unable to create secret to store slicegw certs in spoke cluster", "sliceGw", sliceGw.Name)
				return reconcile.Result{}, err
			}
			log.Info("sliceGw secret created in spoke cluster")
		} else {
			log.Error(err, "unable to fetch slicegw certs from the spoke", "sliceGw", sliceGw.Name)
			return reconcile.Result{}, err
		}
	}
	return reconcile.Result{}, nil
}

func (r *SliceGwReconciler) deleteSliceGwResourceOnSpoke(ctx context.Context, sliceGw *spokev1alpha1.WorkerSliceGateway) error {
	log := logger.FromContext(ctx)
	sliceGwOnSpoke := &kubeslicev1beta1.SliceGateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sliceGw.Name,
			Namespace: ControlPlaneNamespace,
		},
	}
	if err := r.MeshClient.Delete(ctx, sliceGwOnSpoke); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}
	log.Info("Deleted SliceGateway CR on spoke cluster", "slicegw", sliceGwOnSpoke.Name)
	return nil
}

func (r *SliceGwReconciler) handleSliceGWDeletion(sliceGw *spokev1alpha1.WorkerSliceGateway, ctx context.Context, req reconcile.Request) (bool, reconcile.Result, error) {
	if sliceGw.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !controllerutil.ContainsFinalizer(sliceGw, sliceGwFinalizer) {
			controllerutil.AddFinalizer(sliceGw, sliceGwFinalizer)
			if err := r.Update(ctx, sliceGw); err != nil {
				return true, reconcile.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(sliceGw, sliceGwFinalizer) {
			// our finalizer is present, so lets handle any external dependency
			if err := r.deleteSliceGwResourceOnSpoke(ctx, sliceGw); err != nil {
				// if fail to delete the external dependency here, return with error
				// so that it can be retried
				return true, reconcile.Result{}, err
			}
			// remove our finalizer from the spokeslice and update it.
			// retry on conflict
			err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				//fetch the latest spokeslice from hub
				if err := r.Get(ctx, req.NamespacedName, sliceGw); err != nil {
					return err
				}
				//remove the finalizer
				controllerutil.RemoveFinalizer(sliceGw, sliceGwFinalizer)
				if err := r.Update(ctx, sliceGw); err != nil {
					return err
				}
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

func (r *SliceGwReconciler) createSliceGwOnSpoke(ctx context.Context, sliceGw *spokev1alpha1.WorkerSliceGateway, meshSliceGw *kubeslicev1beta1.SliceGateway) error {
	log := logger.FromContext(ctx)
	sliceGwName := sliceGw.Name
	sliceName := sliceGw.Spec.SliceName
	sliceGwRef := client.ObjectKey{
		Name:      sliceGwName,
		Namespace: ControlPlaneNamespace,
	}
	err := r.MeshClient.Get(ctx, sliceGwRef, meshSliceGw)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, create it in the spoke cluster
			log.Info("SliceGw resource not found in spoke cluster, creating")
			meshSliceGw = &kubeslicev1beta1.SliceGateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      sliceGwName,
					Namespace: ControlPlaneNamespace,
					Labels:    map[string]string{controllers.ApplicationNamespaceSelectorLabelKey: sliceName},
				},
				Spec: kubeslicev1beta1.SliceGatewaySpec{
					SliceName: sliceName,
				},
			}
			//get the slice object and set it as ownerReference
			sliceKey := types.NamespacedName{Namespace: ControlPlaneNamespace, Name: sliceGw.Spec.SliceName}
			sliceOnSpoke := &kubeslicev1beta1.Slice{}
			if err := r.MeshClient.Get(ctx, sliceKey, sliceOnSpoke); err != nil {
				log.Error(err, "Failed to get Slice CR")
				return err
			}
			if err := controllerutil.SetControllerReference(sliceOnSpoke, meshSliceGw, r.MeshClient.Scheme()); err != nil {
				log.Error(err, "Failed to set slice as owner of slicegw")
				return err
			}
			//finally create the meshSliceGw object
			err = r.MeshClient.Create(ctx, meshSliceGw)
			if err != nil {
				log.Error(err, "unable to create sliceGw in spoke cluster", "sliceGw", sliceGwName)
				return err
			}
			log.Info("sliceGw created in spoke cluster", "sliceGw", sliceGwName)
		} else {
			log.Error(err, "unable to fetch sliceGw in spoke cluster", "sliceGw", sliceGwName)
			return err
		}
	}
	return nil
}
