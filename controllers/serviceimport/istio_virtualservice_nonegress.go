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

	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	"github.com/kubeslice/worker-operator/internal/logger"
	networkingv1beta1 "istio.io/api/networking/v1beta1"
	istiov1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *Reconciler) ReconcileVirtualServiceNonEgress(ctx context.Context, serviceimport *kubeslicev1beta1.ServiceImport) (ctrl.Result, error, bool) {
	log := logger.FromContext(ctx).WithValues("type", "Istio VS non-egress")
	debugLog := log.V(1)

	debugLog.Info("reconciling istio virtualService for serviceimport", "serviceimport", serviceimport)

	vs, err := r.getVirtualServiceFromAppPod(ctx, serviceimport)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("virtualService resource not found")

			if len(serviceimport.Status.Endpoints) == 0 {
				log.Info("Endpoints are 0, skipping virtualService creation")
				return ctrl.Result{}, nil, false
			}

			vs = r.virtualServiceNonEgress(serviceimport)
			err := r.Create(ctx, vs)
			if err != nil {
				log.Error(err, "Failed to create virtualService for", "serviceimport", serviceimport)
				return ctrl.Result{}, err, true
			}
			log.Info("virtualService resource created")
			return ctrl.Result{Requeue: true}, nil, true
		}
		log.Error(err, "unable to get virtualService")
		return ctrl.Result{}, err, true
	}

	if len(serviceimport.Status.Endpoints) == 0 {
		log.Info("Endpoints are 0, deleting existing virtualService")
		err = r.Delete(ctx, vs)
		if err != nil {
			log.Error(err, "Unable to delete virtualService")
			return ctrl.Result{}, err, true
		}
		log.Info("deleted virtualService")
		return ctrl.Result{}, nil, false
	}

	if hasVirtualServiceRoutesChanged(vs, serviceimport) {
		log.Info("virtualService routes changed, updating")
		if getServiceProtocol(serviceimport) == kubeslicev1beta1.ServiceProtocolHTTP {
			httpRoutes := getVirtualServiceHTTPRoutes(serviceimport)
			debugLog.Info("new routes", "http", httpRoutes)
			vs.Spec.Http = []*networkingv1beta1.HTTPRoute{{
				Route: httpRoutes,
			}}
		} else {
			tcpRoutes := getVirtualServiceTCPRoutes(serviceimport)
			debugLog.Info("new routes", "tcp", tcpRoutes)
			vs.Spec.Tcp = []*networkingv1beta1.TCPRoute{{
				Route: tcpRoutes,
			}}
		}
		err = r.Update(ctx, vs)
		if err != nil {
			log.Error(err, "Unable to update virtualService routes")
			return ctrl.Result{}, err, true
		}
		log.Info("virtualService routes updated")
		return ctrl.Result{Requeue: true}, nil, true
	}

	return ctrl.Result{}, nil, false
}

func (r *Reconciler) virtualServiceNonEgress(serviceImport *kubeslicev1beta1.ServiceImport) *istiov1beta1.VirtualService {

	vs := &istiov1beta1.VirtualService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      virtualServiceFromAppPodName(serviceImport),
			Namespace: serviceImport.Namespace,
		},
		Spec: networkingv1beta1.VirtualService{
			Hosts: []string{
				serviceImport.Spec.DNSName,
				serviceImport.Name,
			},
			ExportTo: []string{"."},
		},
	}

	if getServiceProtocol(serviceImport) == kubeslicev1beta1.ServiceProtocolHTTP {
		vs.Spec.Http = []*networkingv1beta1.HTTPRoute{{
			Route: getVirtualServiceHTTPRoutes(serviceImport),
		}}
	} else {
		vs.Spec.Tcp = []*networkingv1beta1.TCPRoute{{
			Route: getVirtualServiceTCPRoutes(serviceImport),
		}}
	}

	ctrl.SetControllerReference(serviceImport, vs, r.Scheme)

	return vs
}
