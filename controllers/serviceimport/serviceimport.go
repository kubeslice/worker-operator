package serviceimport

import (
	"context"
	"errors"

	meshv1beta1 "bitbucket.org/realtimeai/kubeslice-operator/api/v1beta1"
	"bitbucket.org/realtimeai/kubeslice-operator/controllers"
	"bitbucket.org/realtimeai/kubeslice-operator/internal/dns"
	"bitbucket.org/realtimeai/kubeslice-operator/internal/logger"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *Reconciler) reconcileDNSEntries(ctx context.Context, serviceimport *meshv1beta1.ServiceImport) (ctrl.Result, error, bool) {
	log := logger.FromContext(ctx).WithValues("type", "DNS")
	debugLog := log.V(1)

	debugLog.Info("Finding DNS Config")

	cm := &corev1.ConfigMap{}
	err := r.Get(ctx, types.NamespacedName{
		Namespace: controllers.ControlPlaneNamespace,
		Name:      controllers.DNSDeploymentName,
	}, cm)
	if err != nil {
		log.Error(err, "Unable to fetch DNS ConfigMap, cannnot reconcile ServiceImport DNS entries")
		return ctrl.Result{}, err, true
	}

	dnsData := cm.Data["slice.db"]

	reconciledDNSData, err := dns.ReconcileDNSFile(ctx, dnsData, serviceimport)
	if len(reconciledDNSData) > 100000 {
		return ctrl.Result{}, errors.New("CoreDNS configMap is too big, possibly some race condition"), true
	}
	if err != nil {
		log.Error(err, "Unable to reconcile dns entries")
		return ctrl.Result{}, err, true
	}

	// Update ConfigMap if dns records needs to be changed
	if reconciledDNSData != dnsData {
		cm.Data["slice.db"] = reconciledDNSData
		err = r.Update(ctx, cm)
		if err != nil {
			log.Error(err, "Unable to update DNS configmap")
			return ctrl.Result{}, err, true
		}
		return ctrl.Result{Requeue: true}, nil, true
	}

	return ctrl.Result{}, err, false
}
