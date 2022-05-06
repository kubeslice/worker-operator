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

	spokev1alpha1 "github.com/kubeslice/apis/pkg/spoke/v1alpha1"
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	"github.com/kubeslice/worker-operator/internal/logger"
	"github.com/kubeslice/worker-operator/pkg/events"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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

func (r *SliceGwReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := logger.FromContext(ctx)

	sliceGw := &spokev1alpha1.SpokeSliceGateway{}
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

	// Return if the slice gw resource does not belong to our cluster
	if sliceGw.Spec.LocalGatewayConfig.ClusterName != r.ClusterName {
		log.Info("sliceGw doesn't belong to this cluster", "sliceGw", sliceGw.Name, "cluster", clusterName, "slicegw cluster", sliceGw.Spec.LocalGatewayConfig.ClusterName)
		return reconcile.Result{}, nil
	}

	meshSliceGwCerts := &corev1.Secret{}
	err = r.MeshClient.Get(ctx, types.NamespacedName{
		Name:      sliceGw.Name,
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
				}, nil
			}
			meshSliceGwCerts := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      sliceGw.Name,
					Namespace: ControlPlaneNamespace,
				},
				Data: sliceGwCerts.Data,
			}
			err = r.MeshClient.Create(ctx, meshSliceGwCerts)
			if err != nil {
				log.Error(err, "unable to create secret to store slicegw certs in spoke cluster", "sliceGw", sliceGw.Name)
				r.EventRecorder.Record(
					&events.Event{
						Object:    sliceGw,
						EventType: events.EventTypeWarning,
						Reason:    "Error",
						Message:   "Error creating secret for storing gateway certs on spoke cluster , slicegateway " + sliceGw.Name + " cluster " + clusterName,
					},
				)
				return reconcile.Result{}, err
			}
			log.Info("sliceGw secret created in spoke cluster")
		} else {
			log.Error(err, "unable to fetch slicegw certs from the spoke", "sliceGw", sliceGw.Name)
			return reconcile.Result{}, err
		}
	}

	sliceGwName := sliceGw.Name
	sliceName := sliceGw.Spec.SliceName

	meshSliceGw := &kubeslicev1beta1.SliceGateway{}

	sliceGwRef := client.ObjectKey{
		Name:      sliceGwName,
		Namespace: ControlPlaneNamespace,
	}

	err = r.MeshClient.Get(ctx, sliceGwRef, meshSliceGw)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, create it in the spoke cluster
			log.Info("SliceGw resource not found in spoke cluster, creating")
			meshSliceGw = &kubeslicev1beta1.SliceGateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      sliceGwName,
					Namespace: ControlPlaneNamespace,
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
				return reconcile.Result{}, err
			}
			if err := controllerutil.SetControllerReference(sliceOnSpoke, meshSliceGw, r.MeshClient.Scheme()); err != nil {
				log.Error(err, "Failed to set slice as owner of slicegw")
				return reconcile.Result{}, err
			}
			//finally create the meshSliceGw object
			err = r.MeshClient.Create(ctx, meshSliceGw)
			if err != nil {
				log.Error(err, "unable to create sliceGw in spoke cluster", "sliceGw", sliceGwName)
				r.EventRecorder.Record(
					&events.Event{
						Object:    sliceGw,
						EventType: events.EventTypeWarning,
						Reason:    "Error",
						Message:   "Error creating slicegw on spoke cluster , slicegateway " + sliceGw.Name + " cluster " + clusterName,
					},
				)
				return reconcile.Result{}, err
			}
			log.Info("sliceGw created in spoke cluster", "sliceGw", sliceGwName)
			//post event to the workerslicegateway
			r.EventRecorder.Record(
				&events.Event{
					Object:    sliceGw,
					EventType: events.EventTypeNormal,
					Reason:    "Created",
					Message:   "Created slicegw on spoke cluster , slicegateway " + sliceGw.Name + " cluster " + clusterName,
				},
			)
			//post event to the slice created on spoke cluster
			r.EventRecorder.Record(
				&events.Event{
					Object:    sliceOnSpoke,
					EventType: events.EventTypeNormal,
					Reason:    "Created",
					Message:   "Created slicegw on spoke cluster , slicegateway " + sliceGw.Name,
				},
			)

		} else {
			log.Error(err, "unable to fetch sliceGw in spoke cluster", "sliceGw", sliceGwName)
			return reconcile.Result{}, err
		}
	}

	meshSliceGw.Status.Config = kubeslicev1beta1.SliceGatewayConfig{
		SliceName:                   sliceGw.Spec.SliceName,
		SliceGatewayID:              sliceGw.Spec.LocalGatewayConfig.GatewayName,
		SliceGatewaySubnet:          sliceGw.Spec.LocalGatewayConfig.GatewaySubnet,
		SliceGatewayRemoteSubnet:    sliceGw.Spec.RemoteGatewayConfig.GatewaySubnet,
		SliceGatewayHostType:        sliceGw.Spec.GatewayHostType,
		SliceGatewayRemoteNodeIP:    sliceGw.Spec.RemoteGatewayConfig.NodeIp,
		SliceGatewayRemoteNodePort:  sliceGw.Spec.RemoteGatewayConfig.NodePort,
		SliceGatewayRemoteClusterID: sliceGw.Spec.RemoteGatewayConfig.ClusterName,
		SliceGatewayRemoteGatewayID: sliceGw.Spec.RemoteGatewayConfig.GatewayName,
		SliceGatewayLocalVpnIP:      sliceGw.Spec.LocalGatewayConfig.VpnIp,
		SliceGatewayRemoteVpnIP:     sliceGw.Spec.RemoteGatewayConfig.VpnIp,
		SliceGatewayName:            strconv.Itoa(sliceGw.Spec.GatewayNumber),
	}
	err = r.MeshClient.Status().Update(ctx, meshSliceGw)
	if err != nil {
		log.Error(err, "unable to update sliceGw status in spoke cluster", "sliceGw", sliceGwName)
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, err
}

func (r *SliceGwReconciler) InjectClient(c client.Client) error {
	r.Client = c
	return nil
}
