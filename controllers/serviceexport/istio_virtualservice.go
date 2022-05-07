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

package serviceexport

import (
	"context"
	"fmt"

	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	"github.com/kubeslice/worker-operator/controllers"
	"github.com/kubeslice/worker-operator/internal/logger"
	networkingv1beta1 "istio.io/api/networking/v1beta1"
	istiov1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *Reconciler) ReconcileVirtualService(ctx context.Context, serviceexport *kubeslicev1beta1.ServiceExport) (ctrl.Result, error, bool) {
	log := logger.FromContext(ctx).WithValues("type", "Istio VS ingress")
	debugLog := log.V(1)

	debugLog.Info("reconciling istio virtualService for serviceexport", "serviceexport", serviceexport)

	vs, err := r.getVirtualService(ctx, serviceexport)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("virtualService resource not found")

			if len(serviceexport.Status.Pods) == 0 {
				log.Info("Endpoints are 0, skipping virtualService creation")
				return ctrl.Result{}, nil, false
			}

			vs = r.virtualService(serviceexport)
			err := r.Create(ctx, vs)
			if err != nil {
				log.Error(err, "Failed to create virtualService for", "serviceexport", serviceexport)
				return ctrl.Result{}, err, true
			}
			log.Info("virtualService resource created")
			return ctrl.Result{Requeue: true}, nil, true
		}
		log.Error(err, "unable to get virtualService")
		return ctrl.Result{}, err, true
	}

	if len(serviceexport.Status.Pods) == 0 {
		log.Info("Endpoints are 0, deleting existing virtualService")
		err = r.Delete(ctx, vs)
		if err != nil {
			log.Error(err, "Unable to delete virtualService")
			return ctrl.Result{}, err, true
		}
		log.Info("deleted virtualService")
		return ctrl.Result{}, nil, false
	}

	if hasVirtualServiceRoutesChanged(vs, serviceexport) {
		log.Info("virtualService routes changed, updating")
		if getServiceProtocol(serviceexport) == kubeslicev1beta1.ServiceProtocolHTTP {
			httpRoutes := getVirtualServiceHTTPRoutes(serviceexport)
			log.Info("new routes", "http", httpRoutes)
			vs.Spec.Http = []*networkingv1beta1.HTTPRoute{{
				Route: httpRoutes,
			}}
		} else {
			tcpRoutes := getVirtualServiceTCPRoutes(serviceexport)
			log.Info("new routes", "tcp", tcpRoutes)
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

func virtualServiceName(serviceexport *kubeslicev1beta1.ServiceExport) string {
	return serviceexport.Name + "-" + serviceexport.Namespace
}

func (r *Reconciler) getVirtualService(ctx context.Context, serviceexport *kubeslicev1beta1.ServiceExport) (*istiov1beta1.VirtualService, error) {
	vs := &istiov1beta1.VirtualService{}

	err := r.Get(ctx, types.NamespacedName{
		Name:      virtualServiceName(serviceexport),
		Namespace: controllers.ControlPlaneNamespace,
	}, vs)
	if err != nil {
		return nil, err
	}

	return vs, nil
}

func (r *Reconciler) virtualService(serviceexport *kubeslicev1beta1.ServiceExport) *istiov1beta1.VirtualService {

	dnsName := fmt.Sprintf("%s-ingress.%s.%s.svc.slice.local", serviceexport.Name, controllers.ClusterName, serviceexport.Namespace)

	gw := controllers.ControlPlaneNamespace + "/" + serviceexport.Spec.Slice + "-istio-ingressgateway"

	vs := &istiov1beta1.VirtualService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      virtualServiceName(serviceexport),
			Namespace: controllers.ControlPlaneNamespace,
		},
		Spec: networkingv1beta1.VirtualService{
			Hosts: []string{
				dnsName,
				serviceexport.Status.DNSName,
			},
			Gateways: []string{
				gw,
			},
			ExportTo: []string{"."},
		},
	}

	if getServiceProtocol(serviceexport) == kubeslicev1beta1.ServiceProtocolHTTP {
		vs.Spec.Http = []*networkingv1beta1.HTTPRoute{{
			Route: getVirtualServiceHTTPRoutes(serviceexport),
		}}
	} else {
		vs.Spec.Tcp = []*networkingv1beta1.TCPRoute{{
			Route: getVirtualServiceTCPRoutes(serviceexport),
		}}
	}

	ctrl.SetControllerReference(serviceexport, vs, r.Scheme)

	return vs
}

func getVirtualServiceHTTPRoutes(serviceexport *kubeslicev1beta1.ServiceExport) []*networkingv1beta1.HTTPRouteDestination {
	routes := []*networkingv1beta1.HTTPRouteDestination{}

	port := uint32(serviceexport.Spec.Ports[0].ContainerPort)

	for i, endpoint := range serviceexport.Status.Pods {
		weight := calculateInitialWeight(i, serviceexport)
		routes = append(routes, &networkingv1beta1.HTTPRouteDestination{
			Destination: &networkingv1beta1.Destination{
				Host: endpoint.DNSName,
				Port: &networkingv1beta1.PortSelector{
					Number: port,
				},
			},
			Weight: int32(weight),
		})
	}

	return routes
}

func getVirtualServiceTCPRoutes(serviceexport *kubeslicev1beta1.ServiceExport) []*networkingv1beta1.RouteDestination {
	routes := []*networkingv1beta1.RouteDestination{}

	var port uint32

	for _, p := range serviceexport.Spec.Ports {
		port = uint32(p.ContainerPort)
	}

	for i, endpoint := range serviceexport.Status.Pods {
		weight := calculateInitialWeight(i, serviceexport)
		routes = append(routes, &networkingv1beta1.RouteDestination{
			Destination: &networkingv1beta1.Destination{
				Host: endpoint.DNSName,
				Port: &networkingv1beta1.PortSelector{
					Number: port,
				},
			},
			Weight: int32(weight),
		})
	}

	return routes
}

// Initially weights are equally distributed to all endpoints
func calculateInitialWeight(i int, serviceexport *kubeslicev1beta1.ServiceExport) int32 {

	l := len(serviceexport.Status.Pods)
	weight := 100 / l
	offset := 100 - (weight * l)

	// For the first few entries, an additional weight is added to make the total 100
	if i < offset {
		weight = weight + 1
	}

	return int32(weight)

}

func hasVirtualServiceRoutesChanged(vs *istiov1beta1.VirtualService, serviceExport *kubeslicev1beta1.ServiceExport) bool {

	// http service
	if getServiceProtocol(serviceExport) == kubeslicev1beta1.ServiceProtocolHTTP {
		if len(vs.Spec.Http) != 1 {
			return true
		}

		if len(vs.Spec.Http[0].Route) != len(serviceExport.Status.Pods) {
			return true
		}

		for i, route := range vs.Spec.Http[0].Route {
			if route.Destination.Host != serviceExport.Status.Pods[i].DNSName {
				return true
			}
		}

		return false
	}

	// tcp service
	if getServiceProtocol(serviceExport) == kubeslicev1beta1.ServiceProtocolTCP {
		if len(vs.Spec.Tcp) != 1 {
			return true
		}

		if len(vs.Spec.Tcp[0].Route) != len(serviceExport.Status.Pods) {
			return true
		}

		for i, route := range vs.Spec.Tcp[0].Route {
			if route.Destination.Host != serviceExport.Status.Pods[i].DNSName {
				return true
			}
		}
	}

	return false
}

func (r *Reconciler) DeleteIstioVirtualServices(ctx context.Context, serviceexport *kubeslicev1beta1.ServiceExport) error {
	vs, err := r.getVirtualService(ctx, serviceexport)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	err = r.Delete(ctx, vs)
	if err != nil {
		return err
	}

	return nil
}
