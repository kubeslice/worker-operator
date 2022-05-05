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

	meshv1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	networkingv1beta1 "istio.io/api/networking/v1beta1"
	istiov1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	"k8s.io/apimachinery/pkg/types"
)

func virtualServiceFromAppPodName(serviceimport *meshv1beta1.ServiceImport) string {
	return serviceimport.Name
}

func virtualServiceFromEgressName(serviceimport *meshv1beta1.ServiceImport) string {
	return serviceimport.Name + "-" + serviceimport.Namespace
}

// Initially weights are equally distributed to all endpoints
func calculateInitialWeight(i int, serviceImport *meshv1beta1.ServiceImport) int32 {

	l := len(serviceImport.Status.Endpoints)
	weight := 100 / l
	offset := 100 - (weight * l)

	// For the first few entries, an additional weight is added to make the total 100
	if i < offset {
		weight = weight + 1
	}

	return int32(weight)

}

func (r *Reconciler) getVirtualServiceFromAppPod(ctx context.Context, serviceimport *meshv1beta1.ServiceImport) (*istiov1beta1.VirtualService, error) {
	vs := &istiov1beta1.VirtualService{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      virtualServiceFromAppPodName(serviceimport),
		Namespace: serviceimport.Namespace,
	}, vs)

	if err != nil {
		return nil, err
	}

	return vs, nil
}

func getVirtualServiceHTTPRoutes(serviceImport *meshv1beta1.ServiceImport) []*networkingv1beta1.HTTPRouteDestination {
	routes := []*networkingv1beta1.HTTPRouteDestination{}
	port := serviceImport.Spec.Ports[0].ContainerPort

	for i, endpoint := range serviceImport.Status.Endpoints {
		weight := calculateInitialWeight(i, serviceImport)

		routes = append(routes, &networkingv1beta1.HTTPRouteDestination{
			Destination: &networkingv1beta1.Destination{
				Host: endpoint.DNSName,
				Port: &networkingv1beta1.PortSelector{
					Number: uint32(port),
				},
			},
			Weight: int32(weight),
		})
	}

	return routes
}

func getVirtualServiceTCPRoutes(serviceImport *meshv1beta1.ServiceImport) []*networkingv1beta1.RouteDestination {
	routes := []*networkingv1beta1.RouteDestination{}
	port := serviceImport.Spec.Ports[0].ContainerPort

	for i, endpoint := range serviceImport.Status.Endpoints {
		weight := calculateInitialWeight(i, serviceImport)

		routes = append(routes, &networkingv1beta1.RouteDestination{
			Destination: &networkingv1beta1.Destination{
				Host: endpoint.DNSName,
				Port: &networkingv1beta1.PortSelector{
					Number: uint32(port),
				},
			},
			Weight: int32(weight),
		})
	}

	return routes
}

func hasVirtualServiceRoutesChanged(vs *istiov1beta1.VirtualService, serviceImport *meshv1beta1.ServiceImport) bool {
	// http service
	if getServiceProtocol(serviceImport) == meshv1beta1.ServiceProtocolHTTP {
		if len(vs.Spec.Http) != 1 {
			return true
		}

		if len(vs.Spec.Http[0].Route) != len(serviceImport.Status.Endpoints) {
			return true
		}

		for i, route := range vs.Spec.Http[0].Route {
			if route.Destination.Host != serviceImport.Status.Endpoints[i].DNSName {
				return true
			}
		}

		return false
	}

	if len(vs.Spec.Tcp) != 1 {
		return true
	}

	if len(vs.Spec.Tcp[0].Route) != len(serviceImport.Status.Endpoints) {
		return true
	}

	for i, route := range vs.Spec.Tcp[0].Route {
		if route.Destination.Host != serviceImport.Status.Endpoints[i].DNSName {
			return true
		}
	}

	return false
}
