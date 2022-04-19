package serviceimport

import (
	"context"

	meshv1beta1 "bitbucket.org/realtimeai/kubeslice-operator/api/v1beta1"
	"bitbucket.org/realtimeai/kubeslice-operator/controllers"
	"bitbucket.org/realtimeai/kubeslice-operator/internal/logger"
	networkingv1beta1 "istio.io/api/networking/v1beta1"
	istiov1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *Reconciler) ReconcileServiceEntries(ctx context.Context, serviceimport *meshv1beta1.ServiceImport, ns string) (ctrl.Result, error, bool) {
	log := logger.FromContext(ctx).WithValues("type", "Istio ServiceEntry")
	debugLog := log.V(1)

	debugLog.Info("reconciling istio serviceentries", "serviceimport", serviceimport.Name, "ns", ns)

	entries, err := getServiceEntriesForSI(ctx, r.Client, serviceimport, ns)
	if err != nil {
		log.Error(err, "Failed to retrieve serviceentry list for", "serviceimport", serviceimport)
		return ctrl.Result{}, err, true
	}

	for _, endpoint := range serviceimport.Status.Endpoints {
		if !serviceEntryExists(entries, endpoint) {
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

// Create serviceEntryFor based on serviceImport endpoint spec in the specified namespace
func (r *Reconciler) serviceEntryForEndpoint(serviceImport *meshv1beta1.ServiceImport, endpoint *meshv1beta1.ServiceEndpoint, ns string) *istiov1beta1.ServiceEntry {
	p := serviceImport.Spec.Ports[0]

	// TODO: This is a hack. Need a better way to set targetPort for ingress gw.
	targetPort := p.ContainerPort
	if endpoint.Name == serviceImport.Name+"-"+serviceImport.ObjectMeta.Namespace+"-ingress" {
		targetPort = 8080
	}

	ports := []*networkingv1beta1.Port{{
		Name:       p.Name,
		Protocol:   string(p.Protocol),
		Number:     uint32(p.ContainerPort),
		TargetPort: uint32(targetPort),
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
			Resolution: networkingv1beta1.ServiceEntry_DNS,
		},
	}

	if getServiceProtocol(serviceImport) == meshv1beta1.ServiceProtocolTCP {
		se.Spec.Addresses = []string{
			endpoint.IP,
		}
	}

	ctrl.SetControllerReference(serviceImport, se, r.Scheme)

	return se
}

func labelsForServiceEntry(si *meshv1beta1.ServiceImport) map[string]string {
	return map[string]string{
		"avesha-service":    si.Name,
		"avesha-service-ns": si.Namespace,
		"avesha-slice":      si.Spec.Slice,
	}
}

func serviceEntryName(endpoint *meshv1beta1.ServiceEndpoint) string {
	return endpoint.Name + "-" + endpoint.ClusterID
}

// getServiceEntriesForSI returns all the serviceentries belongs to an import
func getServiceEntriesForSI(ctx context.Context, c client.Client, serviceimport *meshv1beta1.ServiceImport, ns string) ([]istiov1beta1.ServiceEntry, error) {
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

func serviceEntryExists(seList []istiov1beta1.ServiceEntry, e meshv1beta1.ServiceEndpoint) bool {
	for _, se := range seList {
		if len(se.Spec.Hosts) > 0 && se.Spec.Hosts[0] == e.DNSName {
			return true
		}
	}

	return false
}

func servicesEntriesToDelete(seList []istiov1beta1.ServiceEntry, si *meshv1beta1.ServiceImport) []istiov1beta1.ServiceEntry {

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

func (r *Reconciler) DeleteIstioServiceEntries(ctx context.Context, serviceimport *meshv1beta1.ServiceImport) error {
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
