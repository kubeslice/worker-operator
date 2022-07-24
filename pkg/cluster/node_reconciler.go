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

package cluster

import (
	"context"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type NodeReconciler struct {
	client.Client
	Log logr.Logger
}

// Reconcile func
func (r *NodeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("node reconciler", req.NamespacedName)
	// get the list of nodes tagged with kubeslice gateway label
	nodeList := corev1.NodeList{}
	labels := map[string]string{"kubeslice.io/node-type": "gateway"}
	listOpts := []client.ListOption{
		client.MatchingLabels(labels),
	}
	if err := r.List(ctx, &nodeList, listOpts...); err != nil {
		log.Error(err, "Error getting kubeslice nodeList")
		return ctrl.Result{}, err
	}
	if len(nodeList.Items) == 0 {
		// no gateway nodes found
		return ctrl.Result{}, nil
	}
	nodeIpArr := []corev1.NodeAddress{}
	for i := 0; i < len(nodeList.Items); i++ {
		nodeIpArr = append(nodeIpArr, nodeList.Items[i].Status.Addresses...)
	}
	externalIPs := []string{}

	for i := 0; i < len(nodeIpArr); i++ {
		if nodeIpArr[i].Type == NodeExternalIP {
			externalIPs = append(externalIPs, nodeIpArr[i].Address)
		}
	}
	if len(externalIPs) == 0 {
		// no nodes with ExternalIPs , return and don't requeue
		// this condition will hold true for on-premise and kind clusters
		return ctrl.Result{}, nil
	}
	nodeInfo.Lock()
	defer nodeInfo.Unlock()

	if !sameStringSlice(nodeInfo.ExternalIPList, externalIPs) {
		log.Info("IPs changed,available gateway IPs", "externalIPs", externalIPs)
		nodeInfo.ExternalIPList = externalIPs
	}
	log.Info("nodeInfo.ExternalIP", "nodeInfo.ExternalIP", nodeInfo.ExternalIPList)
	return ctrl.Result{}, nil
}

func sameStringSlice(x, y []string) bool {
	if len(x) != len(y) {
		return false
	}
	// create a map of string -> int
	diff := make(map[string]int, len(x))
	for _, _x := range x {
		// 0 value for int is 0, so just increment a counter for the string
		diff[_x]++
	}
	for _, _y := range y {
		// If the string _y is not in diff bail out early
		if _, ok := diff[_y]; !ok {
			return false
		}
		diff[_y] -= 1
		if diff[_y] == 0 {
			delete(diff, _y)
		}
	}
	return len(diff) == 0
}

// SetupWithManager sets up the controller with the Manager.
func (r *NodeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Node{}).
		Complete(r)
}
