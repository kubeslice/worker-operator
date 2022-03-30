package serviceimport

import (
	"context"
	"errors"
	"strconv"

	meshv1beta1 "bitbucket.org/realtimeai/kubeslice-operator/api/v1beta1"
	"bitbucket.org/realtimeai/kubeslice-operator/controllers"
	"bitbucket.org/realtimeai/kubeslice-operator/internal/dns"
	"bitbucket.org/realtimeai/kubeslice-operator/internal/logger"
	corev1 "k8s.io/api/core/v1"

	k8sapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
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

func (r *Reconciler) serviceForServiceImport(serviceImport *meshv1beta1.ServiceImport) *corev1.Service {

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

func (r *Reconciler) DeleteServiceImportResources(ctx context.Context, serviceimport *meshv1beta1.ServiceImport) error {
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

	return r.DeleteIstioResources(ctx, serviceimport, slice)
}
