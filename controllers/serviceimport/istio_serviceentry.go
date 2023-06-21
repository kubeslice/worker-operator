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
	"github.com/kubeslice/worker-operator/controllers"
	"github.com/kubeslice/worker-operator/pkg/logger"
	networkingv1beta1 "istio.io/api/networking/v1beta1"
	istiov1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *Reconciler) ReconcileServiceEntries(ctx context.Context, serviceimport *kubeslicev1beta1.ServiceImport, ns string) (ctrl.Result, error, bool) {
	log := logger.FromContext(ctx).WithValues("type", "Istio ServiceEntry")
	debugLog := log.V(1)

	debugLog.Info("reconciling istio serviceentries", "serviceimport", serviceimport.Name, "ns", ns)

	entries, err := getServiceEntriesForSI(ctx, r.Client, serviceimport, ns)
	if err != nil {
		log.Error(err, "Failed to retrieve serviceentry list for", "serviceimport", serviceimport)
		return ctrl.Result{}, err, true
	}

	for _, endpoint := range serviceimport.Status.Endpoints {
		se := serviceEntryExists(entries, endpoint)
		if se == nil {
			log.Info("serviceentry resource not found; creating", "serviceimport", serviceimport)
			se := r.serviceEntryForEndpoint(serviceimport, &endpoint, ns)
			err := r.Create(ctx, se)
			if err != nil {
				log.Error(err, "Failed to create serviceentry for", "endpoint", endpoint)
				return ctrl.Result{}, err, true
			}
			log.Info("serviceentry resource created for endpoint", "endpoint", endpoint)
			return ctrl.Result{Requeue: true}, nil, true
		}

		if serviceEntryUpdateNeeded(se, endpoint) {
			updatedSe := r.updateServiceEntryForEndpoint(se, &endpoint)
			err := r.Update(ctx, updatedSe)
			if err != nil {
				log.Error(err, "Failed to update serviceentry for", "endpoint", endpoint)
				return ctrl.Result{}, err, true
			}
			log.Info("serviceentry resource updated for endpoint", "endpoint", endpoint)
			return ctrl.Result{Requeue: true}, nil, true
		}
	}

	// There are no additional serviceentries to be deleted
	if len(serviceimport.Status.Endpoints) == len(entries) {
		return ctrl.Result{}, nil, false
	}

	toDelete := servicesEntriesToDelete(entries, serviceimport)

	for _, se := range toDelete {
		log.Info("Deleting serviceentry", "se", se)
		err = r.Delete(ctx, &se)
		if err != nil {
			log.Error(err, "Unable to delete serviceentry")
			return ctrl.Result{}, err, true
		}
		log.Info("deleted serviceentry")
	}

	return ctrl.Result{}, nil, false

}

func (r *Reconciler) updateServiceEntryForEndpoint(se *istiov1beta1.ServiceEntry, endpoint *kubeslicev1beta1.ServiceEndpoint) *istiov1beta1.ServiceEntry {
	se.Spec.Resolution = networkingv1beta1.ServiceEntry_STATIC
	se.Spec.Endpoints = []*networkingv1beta1.WorkloadEntry{
		{
			Address: endpoint.IP,
		},
	}
	return se
}

// Create serviceEntryFor based on serviceImport endpoint spec in the specified namespace
func (r *Reconciler) serviceEntryForEndpoint(serviceImport *kubeslicev1beta1.ServiceImport, endpoint *kubeslicev1beta1.ServiceEndpoint, ns string) *istiov1beta1.ServiceEntry {
	p := serviceImport.Spec.Ports[0]

	ports := []*networkingv1beta1.Port{{
		Name:       p.Name,
		Protocol:   string(p.Protocol),
		Number:     uint32(p.ContainerPort),
		TargetPort: uint32(endpoint.Port),
	}}

	se := &istiov1beta1.ServiceEntry{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceEntryName(endpoint),
			Namespace: ns,
			Labels:    labelsForServiceEntry(serviceImport),
		},
		Spec: networkingv1beta1.ServiceEntry{
			ExportTo: []string{"."},
			Hosts: []string{
				endpoint.DNSName,
			},
			Location:   networkingv1beta1.ServiceEntry_MESH_EXTERNAL,
			Ports:      ports,
			Resolution: networkingv1beta1.ServiceEntry_STATIC,
			Endpoints: []*networkingv1beta1.WorkloadEntry{{
				Address: endpoint.IP,
			}},
		},
	}

	if getServiceProtocol(serviceImport) == kubeslicev1beta1.ServiceProtocolTCP {
		se.Spec.Addresses = []string{
			endpoint.IP,
		}
	}

	ctrl.SetControllerReference(serviceImport, se, r.Scheme)

	return se
}

func labelsForServiceEntry(si *kubeslicev1beta1.ServiceImport) map[string]string {
	return map[string]string{
		"kubeslice-service":    si.Name,
		"kubeslice-service-ns": si.Namespace,
		"kubeslice-slice":      si.Spec.Slice,
	}
}

func serviceEntryName(endpoint *kubeslicev1beta1.ServiceEndpoint) string {
	return endpoint.Name + "-" + endpoint.ClusterID
}

// getServiceEntriesForSI returns all the serviceentries belongs to an import
func getServiceEntriesForSI(ctx context.Context, c client.Client, serviceimport *kubeslicev1beta1.ServiceImport, ns string) ([]istiov1beta1.ServiceEntry, error) {
	seList := &istiov1beta1.ServiceEntryList{}
	listOpts := []client.ListOption{
		client.MatchingLabels(labelsForServiceEntry(serviceimport)),
		client.InNamespace(ns),
	}
	if err := c.List(ctx, seList, listOpts...); err != nil {
		return nil, err
	}

	ses := []istiov1beta1.ServiceEntry{}

	ses = append(ses, seList.Items...)

	return ses, nil
}

func serviceEntryExists(seList []istiov1beta1.ServiceEntry, e kubeslicev1beta1.ServiceEndpoint) *istiov1beta1.ServiceEntry {
	for _, se := range seList {
		if len(se.Spec.Hosts) > 0 && se.Spec.Hosts[0] == e.DNSName {
			return &se
		}
	}

	return nil
}

func serviceEntryUpdateNeeded(se *istiov1beta1.ServiceEntry, endpoint kubeslicev1beta1.ServiceEndpoint) bool {
	// Mainly for backward compatibility when updating the operator from older to newer verions
	if se.Spec.Resolution != networkingv1beta1.ServiceEntry_STATIC {
		return true
	}

	if len(se.Spec.Endpoints) == 0 {
		return true
	}

	if len(se.Spec.Endpoints) > 0 && se.Spec.Endpoints[0].Address != endpoint.IP {
		return true
	}

	return false
}

func servicesEntriesToDelete(seList []istiov1beta1.ServiceEntry, si *kubeslicev1beta1.ServiceImport) []istiov1beta1.ServiceEntry {

	exists := struct{}{}
	dnsSet := make(map[string]struct{})
	toDelete := []istiov1beta1.ServiceEntry{}

	for _, e := range si.Status.Endpoints {
		dnsSet[e.DNSName] = exists
	}

	for _, si := range seList {
		if _, ok := dnsSet[si.Spec.Hosts[0]]; !ok {
			toDelete = append(toDelete, si)
		}
	}

	return toDelete
}

func (r *Reconciler) DeleteIstioServiceEntries(ctx context.Context, serviceimport *kubeslicev1beta1.ServiceImport) error {
	entries, err := getServiceEntriesForSI(ctx, r.Client, serviceimport, controllers.ControlPlaneNamespace)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}

		return err
	}

	for _, se := range entries {
		err = r.Delete(ctx, &se)
		if err != nil {
			return nil
		}
	}

	return nil
}
