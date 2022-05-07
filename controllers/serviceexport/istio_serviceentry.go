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

	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	"github.com/kubeslice/worker-operator/controllers"
	"github.com/kubeslice/worker-operator/internal/logger"
	networkingv1beta1 "istio.io/api/networking/v1beta1"
	istiov1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *Reconciler) ReconcileServiceEntries(ctx context.Context, serviceexport *kubeslicev1beta1.ServiceExport) (ctrl.Result, error, bool) {
	log := logger.FromContext(ctx).WithValues("type", "Istio ServiceEntry")
	debugLog := log.V(1)

	debugLog.Info("reconciling istio serviceentries")

	entries, err := getServiceEntries(ctx, r, serviceexport)
	if err != nil {
		log.Error(err, "Failed to retrieve serviceentry list for", "serviceexport", serviceexport)
		return ctrl.Result{}, err, true
	}

	for _, endpoint := range serviceexport.Status.Pods {
		if !serviceEntryExists(entries, endpoint) {
			log.Info("serviceentry resource not found; creating", "serviceexport", serviceexport)
			se := createServiceEntryForEndpoint(serviceexport, &endpoint)
			err := r.Create(ctx, se)
			if err != nil {
				log.Error(err, "Failed to create serviceentry for", "endpoint", endpoint)
				return ctrl.Result{}, err, true
			}
			ctrl.SetControllerReference(serviceexport, se, r.Scheme)
			log.Info("serviceentry resource created for endpoint", "endpoint", endpoint)
			return ctrl.Result{Requeue: true}, nil, true
		}
	}

	// There are no additional serviceentries to be deleted
	if len(serviceexport.Status.Pods) == len(entries) {
		return ctrl.Result{}, nil, false
	}

	toDelete := servicesEntriesToDelete(entries, serviceexport)

	for _, se := range toDelete {
		log.Info("Deleting serviceentry", "se", se)
		err = r.Delete(ctx, &se)
		if err != nil {
			log.Error(err, "Unable to delete serviceentry")
			return ctrl.Result{}, err, true
		}
		debugLog.Info("deleted serviceentry")
	}

	return ctrl.Result{}, nil, false
}

// getServiceEntries returns all the serviceentries belongs to a serviceexport
func getServiceEntries(ctx context.Context, r client.Reader, serviceexport *kubeslicev1beta1.ServiceExport) ([]istiov1beta1.ServiceEntry, error) {
	seList := &istiov1beta1.ServiceEntryList{}
	listOpts := []client.ListOption{
		client.MatchingLabels(labelsForServiceEntry(serviceexport)),
		client.InNamespace(controllers.ControlPlaneNamespace),
	}
	if err := r.List(ctx, seList, listOpts...); err != nil {
		return nil, err
	}

	ses := []istiov1beta1.ServiceEntry{}

	ses = append(ses, seList.Items...)

	return ses, nil
}

// Create serviceEntry based on serviceExport endpoint spec
func createServiceEntryForEndpoint(serviceexport *kubeslicev1beta1.ServiceExport, endpoint *kubeslicev1beta1.ServicePod) *istiov1beta1.ServiceEntry {
	ports := []*networkingv1beta1.Port{}

	for _, p := range serviceexport.Spec.Ports {
		po := &networkingv1beta1.Port{
			Name:       p.Name,
			Protocol:   string(p.Protocol),
			Number:     uint32(p.ContainerPort),
			TargetPort: uint32(p.ContainerPort),
		}
		ports = append(ports, po)
	}

	ip := endpoint.NsmIP

	// use cni ip if nsmip is not available
	if ip == "" {
		ip = endpoint.PodIp
	}

	se := &istiov1beta1.ServiceEntry{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceEntryName(endpoint, serviceexport.Namespace),
			Namespace: controllers.ControlPlaneNamespace,
			Labels:    labelsForServiceEntry(serviceexport),
		},
		Spec: networkingv1beta1.ServiceEntry{
			ExportTo: []string{"."},
			Hosts: []string{
				endpoint.DNSName,
			},
			Location:   networkingv1beta1.ServiceEntry_MESH_INTERNAL,
			Ports:      ports,
			Resolution: networkingv1beta1.ServiceEntry_STATIC,
			Endpoints: []*networkingv1beta1.WorkloadEntry{{
				Address: ip,
			}},
		},
	}

	return se
}

func serviceEntryName(endpoint *kubeslicev1beta1.ServicePod, ns string) string {
	return endpoint.Name + "-" + ns + "-ingress"
}

func servicesEntriesToDelete(seList []istiov1beta1.ServiceEntry, se *kubeslicev1beta1.ServiceExport) []istiov1beta1.ServiceEntry {

	exists := struct{}{}
	dnsSet := make(map[string]struct{})
	toDelete := []istiov1beta1.ServiceEntry{}

	for _, e := range se.Status.Pods {
		dnsSet[e.DNSName] = exists
	}

	for _, s := range seList {
		if _, ok := dnsSet[s.Spec.Hosts[0]]; !ok {
			toDelete = append(toDelete, s)
		}
	}

	return toDelete
}

func labelsForServiceEntry(se *kubeslicev1beta1.ServiceExport) map[string]string {
	return map[string]string{
		"avesha-service":    se.Name,
		"avesha-service-ns": se.Namespace,
		"avesha-slice":      se.Spec.Slice,
	}
}

func serviceEntryExists(seList []istiov1beta1.ServiceEntry, e kubeslicev1beta1.ServicePod) bool {
	for _, se := range seList {
		if len(se.Spec.Hosts) > 0 && se.Spec.Hosts[0] == e.DNSName {
			return true
		}
	}

	return false
}

func (r *Reconciler) DeleteIstioServiceEntries(ctx context.Context, serviceexport *kubeslicev1beta1.ServiceExport) error {
	entries, err := getServiceEntries(ctx, r, serviceexport)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	for _, se := range entries {
		err = r.Delete(ctx, &se)
		if err != nil {
			return err
		}
	}

	return nil
}
