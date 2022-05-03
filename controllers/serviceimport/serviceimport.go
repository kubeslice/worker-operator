package serviceimport

import (
	"context"
	"errors"
	"strconv"

	kubeslicev1beta1 "github.com/kubeslice/operator/api/v1beta1"
	"github.com/kubeslice/operator/controllers"
	"github.com/kubeslice/operator/internal/dns"
	"github.com/kubeslice/operator/internal/logger"
	corev1 "k8s.io/api/core/v1"

	k8sapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *Reconciler) reconcileDNSEntries(ctx context.Context, serviceimport *kubeslicev1beta1.ServiceImport) (ctrl.Result, error, bool) {
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

func (r *Reconciler) serviceForServiceImport(serviceImport *kubeslicev1beta1.ServiceImport) *corev1.Service {

	ports := []corev1.ServicePort{}

	for _, p := range serviceImport.Spec.Ports {
		pName := p.Name
		if pName == "" {
			pName = string(p.Protocol) + strconv.Itoa(int(p.ContainerPort))
		}
		ports = append(ports, corev1.ServicePort{
			Port:       p.ContainerPort,
			Protocol:   p.Protocol,
			Name:       pName,
			TargetPort: intstr.FromInt(int(p.ContainerPort)),
		})
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceImport.Name,
			Namespace: serviceImport.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Ports: ports,
		},
	}

	ctrl.SetControllerReference(serviceImport, svc, r.Scheme)
	return svc
}

func (r *Reconciler) DeleteDnsRecordsForServiceImport(ctx context.Context, serviceimport *kubeslicev1beta1.ServiceImport) error {
	log := logger.FromContext(ctx)
	cm := &corev1.ConfigMap{}
	err := r.Get(ctx, types.NamespacedName{
		Namespace: controllers.ControlPlaneNamespace,
		Name:      controllers.DNSDeploymentName,
	}, cm)
	if err != nil {
		log.Error(err, "Unable to fetch DNS ConfigMap, cannnot delete ServiceImport DNS entries")
		return err
	}

	updatedDnsData, err := dns.DeleteRecordsAndReconcileDNSFile(ctx, cm.Data["slice.db"], serviceimport)
	if err != nil {
		log.Error(err, "unable to delete dns records")
		return err
	}

	cm.Data["slice.db"] = updatedDnsData
	err = r.Update(ctx, cm)
	if err != nil {
		log.Error(err, "Unable to update DNS configmap, unable to delete ServiceImport DNS entries")
		return err
	}

	return nil
}

func (r *Reconciler) DeleteServiceImportResources(ctx context.Context, serviceimport *kubeslicev1beta1.ServiceImport) error {
	log := logger.FromContext(ctx)
	slice, err := controllers.GetSlice(ctx, r.Client, serviceimport.Spec.Slice)
	if err != nil {
		if k8sapierrors.IsNotFound(err) {
			return nil
		}
		log.Error(err, "Unable to fetch slice for serviceimport cleanup")
		return err
	}

	if slice.Status.SliceConfig == nil {
		return nil
	}

	// Invalidate endpoints and reconcile DNS records to remove entries specific to the service import
	err = r.DeleteDnsRecordsForServiceImport(ctx, serviceimport)
	if err != nil {
		return err
	}

	err = r.DeleteIstioResources(ctx, serviceimport, slice)
	if err != nil {
		return err
	}

	return nil
}
