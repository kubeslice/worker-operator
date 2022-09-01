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

	"github.com/go-logr/logr"
	spokev1alpha1 "github.com/kubeslice/apis/pkg/worker/v1alpha1"
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	"github.com/kubeslice/worker-operator/pkg/events"
	"github.com/kubeslice/worker-operator/pkg/logger"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type SliceReconciler struct {
	client.Client
	Log           logr.Logger
	MeshClient    client.Client
	EventRecorder *events.EventRecorder
}

var sliceFinalizer = "controller.kubeslice.io/hubSpokeSlice-finalizer"

func (r *SliceReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := r.Log.WithValues("sliceconfig", req.NamespacedName)
	ctx = logger.WithLogger(ctx, log)
	debuglog := log.V(1)
	slice := &spokev1alpha1.WorkerSliceConfig{}

	err := r.Get(ctx, req.NamespacedName, slice)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Return and don't requeue
			log.Info("Slice resource not found in hub. Ignoring since object must be deleted")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	log.Info("got slice from hub", "slice", slice.Name)
	debuglog.Info("got slice from hub", "slice", slice)
	requeue, result, err := r.handleSliceDeletion(slice, ctx, req)
	if requeue {
		return result, err
	}

	sliceName := slice.Spec.SliceName
	meshSlice := &kubeslicev1beta1.Slice{}
	sliceRef := client.ObjectKey{
		Name:      sliceName,
		Namespace: ControlPlaneNamespace,
	}

	err = r.MeshClient.Get(ctx, sliceRef, meshSlice)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, create it in the spoke cluster
			log.Info("Slice resource not found in spoke cluster, creating")
			s := &kubeslicev1beta1.Slice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      sliceName,
					Namespace: ControlPlaneNamespace,
				},
				Spec: kubeslicev1beta1.SliceSpec{},
			}

			err = r.MeshClient.Create(ctx, s)
			if err != nil {
				log.Error(err, "unable to create slice in spoke cluster", "slice", s)
				r.EventRecorder.Record(
					&events.Event{
						Object:    slice,
						EventType: events.EventTypeWarning,
						Reason:    "Error",
						Message:   "Error creating slice on spoke cluster , slice " + sliceName + " cluster " + clusterName,
					},
				)
				return reconcile.Result{}, err
			}
			log.Info("slice created in spoke cluster")
			r.EventRecorder.Record(
				&events.Event{
					Object:    slice,
					EventType: events.EventTypeNormal,
					Reason:    "Created",
					Message:   "Created slice on spoke cluster , slice " + sliceName + " cluster " + clusterName,
				},
			)
			err = r.updateSliceConfig(ctx, s, slice)
			if err != nil {
				log.Error(err, "unable to update slice status in spoke cluster", "slice", s)
				return reconcile.Result{}, err
			}
			log.Info("slice status updated in spoke cluster")

			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	err = r.updateSliceConfig(ctx, meshSlice, slice)
	if err != nil {
		log.Error(err, "unable to update slice status in spoke cluster", "slice", meshSlice)
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *SliceReconciler) updateSliceConfig(ctx context.Context, meshSlice *kubeslicev1beta1.Slice, spokeSlice *spokev1alpha1.WorkerSliceConfig) error {
	if meshSlice.Status.SliceConfig == nil {
		meshSlice.Status.SliceConfig = &kubeslicev1beta1.SliceConfig{
			SliceDisplayName: spokeSlice.Spec.SliceName,
			SliceSubnet:      spokeSlice.Spec.SliceSubnet,
			SliceIpam: kubeslicev1beta1.SliceIpamConfig{
				SliceIpamType:    spokeSlice.Spec.SliceIpamType,
				IpamClusterOctet: spokeSlice.Spec.IpamClusterOctet,
			},
			ClusterSubnetCIDR: spokeSlice.Spec.ClusterSubnetCIDR,
			SliceType:         spokeSlice.Spec.SliceType,
		}
	}
	if meshSlice.Status.SliceConfig.SliceSubnet == "" {
		meshSlice.Status.SliceConfig.SliceSubnet = spokeSlice.Spec.SliceSubnet
	}

	if meshSlice.Status.SliceConfig.SliceIpam.IpamClusterOctet == nil {
		meshSlice.Status.SliceConfig.SliceIpam.IpamClusterOctet = spokeSlice.Spec.IpamClusterOctet
	}

	if meshSlice.Status.SliceConfig.ClusterSubnetCIDR == "" || meshSlice.Status.SliceConfig.ClusterSubnetCIDR != spokeSlice.Spec.ClusterSubnetCIDR {
		meshSlice.Status.SliceConfig.ClusterSubnetCIDR = spokeSlice.Spec.ClusterSubnetCIDR
	}

	meshSlice.Status.SliceConfig.QosProfileDetails = kubeslicev1beta1.QosProfileDetails{
		QueueType:               spokeSlice.Spec.QosProfileDetails.QueueType,
		BandwidthCeilingKbps:    spokeSlice.Spec.QosProfileDetails.BandwidthCeilingKbps,
		BandwidthGuaranteedKbps: spokeSlice.Spec.QosProfileDetails.BandwidthGuaranteedKbps,
		DscpClass:               spokeSlice.Spec.QosProfileDetails.DscpClass,
		TcType:                  spokeSlice.Spec.QosProfileDetails.TcType,
		Priority:                spokeSlice.Spec.QosProfileDetails.Priority,
	}

	meshSlice.Status.SliceConfig.NamespaceIsolationProfile = &kubeslicev1beta1.NamespaceIsolationProfile{
		IsolationEnabled:      spokeSlice.Spec.NamespaceIsolationProfile.IsolationEnabled,
		AllowedNamespaces:     spokeSlice.Spec.NamespaceIsolationProfile.AllowedNamespaces,
		ApplicationNamespaces: spokeSlice.Spec.NamespaceIsolationProfile.ApplicationNamespaces,
	}

	extGwCfg := spokeSlice.Spec.ExternalGatewayConfig
	meshSlice.Status.SliceConfig.ExternalGatewayConfig = &kubeslicev1beta1.ExternalGatewayConfig{
		GatewayType: extGwCfg.GatewayType,
		Egress: &kubeslicev1beta1.ExternalGatewayConfigOptions{
			Enabled: extGwCfg.Egress.Enabled,
		},
		Ingress: &kubeslicev1beta1.ExternalGatewayConfigOptions{
			Enabled: extGwCfg.Ingress.Enabled,
		},
		NsIngress: &kubeslicev1beta1.ExternalGatewayConfigOptions{
			Enabled: extGwCfg.NsIngress.Enabled,
		},
	}

	return r.MeshClient.Status().Update(ctx, meshSlice)
}

func (a *SliceReconciler) InjectClient(c client.Client) error {
	a.Client = c
	return nil
}

func (r *SliceReconciler) deleteSliceResourceOnSpoke(ctx context.Context, slice *spokev1alpha1.WorkerSliceConfig) error {
	log := logger.FromContext(ctx)
	sliceOnSpoke := &kubeslicev1beta1.Slice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      slice.Spec.SliceName,
			Namespace: ControlPlaneNamespace,
		},
	}
	if err := r.MeshClient.Delete(ctx, sliceOnSpoke); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}
	log.Info("Deleted Slice CR on spoke cluster", "slice", sliceOnSpoke.Name)
	return nil
}

func (r *SliceReconciler) handleSliceDeletion(slice *spokev1alpha1.WorkerSliceConfig, ctx context.Context, req reconcile.Request) (bool, reconcile.Result, error) {
	if slice.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !controllerutil.ContainsFinalizer(slice, sliceFinalizer) {
			controllerutil.AddFinalizer(slice, sliceFinalizer)
			if err := r.Update(ctx, slice); err != nil {
				return true, reconcile.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(slice, sliceFinalizer) {
			// our finalizer is present, so lets handle any external dependency
			if err := r.deleteSliceResourceOnSpoke(ctx, slice); err != nil {
				// if fail to delete the external dependency here, return with error
				// so that it can be retried
				return true, reconcile.Result{}, err
			}
			// remove our finalizer from the spokeslice and update it.
			// retry on conflict
			err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				//fetch the latest spokeslice from hub
				if err := r.Get(ctx, req.NamespacedName, slice); err != nil {
					return err
				}
				//remove the finalizer
				controllerutil.RemoveFinalizer(slice, sliceFinalizer)
				if err := r.Update(ctx, slice); err != nil {
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
