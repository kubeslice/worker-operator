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
package namespace

import (
	"context"
	"strings"

	"github.com/go-logr/logr"
	"github.com/kubeslice/kubeslice-monitoring/pkg/events"
	ossEvents "github.com/kubeslice/worker-operator/events"

	"github.com/kubeslice/worker-operator/controllers"

	hub "github.com/kubeslice/worker-operator/pkg/hub/hubclient"
	"github.com/kubeslice/worker-operator/pkg/logger"
	"github.com/kubeslice/worker-operator/pkg/utils"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
)

//+kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch;

type Reconciler struct {
	client.Client
	EventRecorder *events.EventRecorder
	Scheme        *runtime.Scheme
	Log           logr.Logger
	Hubclient     *hub.HubClientConfig
}

var excludedNs []string

var controllerName string = "namespaceReconciler"

func (c *Reconciler) getSliceNameFromNs(ns string) (string, error) {
	namespace := corev1.Namespace{}
	err := c.Client.Get(context.Background(), types.NamespacedName{Name: ns}, &namespace)
	if err != nil {
		c.Log.Error(err, "error while retrieving namespace")
		return "", err
	}
	if namespace.Labels == nil {
		return "", nil
	}
	// TODO: label should not be hardcoded, it should come as a constant from slice reconciler pkg
	return namespace.Labels[controllers.ApplicationNamespaceSelectorLabelKey], nil
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	excludedNsEnv := utils.GetEnvOrDefault("EXCLUDED_NS", utils.DefaultExcludedNS)
	excludedNs = strings.Split(excludedNsEnv, ",")
	for _, v := range excludedNs {
		if v == req.Name {
			return ctrl.Result{}, nil
		}
	}
	log := r.Log.WithValues("namespaceReconciler", req.Name)
	ctx = logger.WithLogger(ctx, log)
	namespace := corev1.Namespace{}
	err := r.Get(ctx, req.NamespacedName, &namespace)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("Namespace deleted on worker cluster, updating the cluster CR on conrtoller cluster")
			err := hub.DeleteNamespaceInfoFromHub(ctx, r.Hubclient, req.Name)
			if err != nil {
				utils.RecordEvent(ctx, r.EventRecorder, &namespace, nil, ossEvents.EventDeleteNamespaceInfoToHubFailed, controllerName)
				log.Error(err, "Failed to delete namespace on controller cluster")
				return ctrl.Result{}, err
			}
			utils.RecordEvent(ctx, r.EventRecorder, &namespace, nil, ossEvents.EventDeleteNamespaceInfoToHub, controllerName)
			// Return and don't requeue
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	//get the slice name from namespace label
	sliceName, err := r.getSliceNameFromNs(namespace.Name)
	if err != nil {
		log.Error(err, "error while retrieving labels from namespace")
		return ctrl.Result{}, err
	}
	*r.EventRecorder = (*r.EventRecorder).WithSlice(sliceName)
	err = hub.UpdateNamespaceInfoToHub(ctx, r.Hubclient, namespace.Name, sliceName)
	if err != nil {
		utils.RecordEvent(ctx, r.EventRecorder, &namespace, nil, ossEvents.EventUpdateNamespaceInfoToHubFailed, controllerName)
		log.Error(err, "Failed to post namespace on controller cluster")
		return ctrl.Result{}, err
	}
	utils.RecordEvent(ctx, r.EventRecorder, &namespace, nil, ossEvents.EventUpdatedNamespaceInfoToHub, controllerName)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up reconciler with manager
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Namespace{}).
		Complete(r)
}
