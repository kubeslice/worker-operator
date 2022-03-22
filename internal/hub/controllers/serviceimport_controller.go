package controllers

import (
	"context"
	"time"

	meshv1beta1 "bitbucket.org/realtimeai/kubeslice-operator/api/v1beta1"
	"bitbucket.org/realtimeai/kubeslice-operator/internal/logger"
	spokev1alpha1 "bitbucket.org/realtimeai/mesh-apis/pkg/spoke/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type ServiceImportReconciler struct {
	client.Client
	MeshClient client.Client
}

func getProtocol(protocol string) corev1.Protocol {
	switch protocol {
	case "TCP":
		return corev1.ProtocolTCP
	case "UDP":
		return corev1.ProtocolUDP
	case "SCTP":
		return corev1.ProtocolSCTP
	default:
		return ""
	}
}

func getMeshServiceImportPortList(svcim *spokev1alpha1.SpokeServiceImport) []meshv1beta1.ServicePort {
	portList := []meshv1beta1.ServicePort{}
	for _, port := range svcim.Spec.ServiceDiscoveryPorts {
		portList = append(portList, meshv1beta1.ServicePort{
			Name:          port.Name,
			ContainerPort: port.Port,
			Protocol:      getProtocol(port.Protocol),
		})
	}

	return portList
}

func getMeshServiceImportEpList(svcim *spokev1alpha1.SpokeServiceImport) []meshv1beta1.ServiceEndpoint {
	epList := []meshv1beta1.ServiceEndpoint{}
	for _, ep := range svcim.Spec.ServiceDiscoveryEndpoints {
		epList = append(epList, meshv1beta1.ServiceEndpoint{
			Name:      ep.PodName,
			IP:        ep.NsmIp,
			Port:      ep.Port,
			ClusterID: ep.Cluster,
			DNSName:   ep.DnsName,
		})
	}

	return epList
}

func getMeshServiceImportObj(svcim *spokev1alpha1.SpokeServiceImport) *meshv1beta1.ServiceImport {
	return &meshv1beta1.ServiceImport{
		ObjectMeta: metav1.ObjectMeta{
			Name:      svcim.Spec.ServiceName,
			Namespace: svcim.Spec.ServiceNamespace,
			Labels: map[string]string{
				"kubeslice.io/slice": svcim.Spec.SliceName,
			},
		},
		Spec: meshv1beta1.ServiceImportSpec{
			Slice:   svcim.Spec.SliceName,
			DNSName: svcim.Spec.ServiceName + "." + svcim.Spec.ServiceNamespace + ".svc.slice.local",
			Ports:   getMeshServiceImportPortList(svcim),
		},
	}
}

func (r *ServiceImportReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := logger.FromContext(ctx)

	svcim := &spokev1alpha1.SpokeServiceImport{}
	err := r.Get(ctx, req.NamespacedName, svcim)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Return and don't requeue
			log.Info("Slice resource not found in hub. Ignoring since object must be deleted")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	log.Info("got service import from hub", "serviceimport", svcim)

	sliceName := svcim.Spec.SliceName
	meshSlice := &meshv1beta1.Slice{}
	sliceRef := client.ObjectKey{
		Name:      sliceName,
		Namespace: ControlPlaneNamespace,
	}

	err = r.MeshClient.Get(ctx, sliceRef, meshSlice)
	if err != nil {
		log.Error(err, "slice object not present for service import. Waiting...", "serviceimport", svcim.Name)
		return reconcile.Result{
			RequeueAfter: 30 * time.Second,
		}, nil
	}

	meshSvcIm := &meshv1beta1.ServiceImport{}
	err = r.MeshClient.Get(ctx, client.ObjectKey{
		Name:      svcim.Spec.ServiceName,
		Namespace: svcim.Spec.ServiceNamespace,
	}, meshSvcIm)
	if err != nil {
		if errors.IsNotFound(err) {
			meshSvcIm = getMeshServiceImportObj(svcim)
			err = r.MeshClient.Create(ctx, meshSvcIm)
			if err != nil {
				log.Error(err, "unable to create service import in spoke cluster", "serviceimport", svcim.Name)
				return reconcile.Result{}, err
			}

			meshSvcIm.Status.Endpoints = getMeshServiceImportEpList(svcim)
			err = r.MeshClient.Status().Update(ctx, meshSvcIm)
			if err != nil {
				log.Error(err, "unable to update service import in spoke cluster", "serviceimport", svcim.Name)
				return reconcile.Result{}, err
			}

			return reconcile.Result{}, nil

		}

		return reconcile.Result{}, err
	}

	meshSvcIm.Status.Endpoints = getMeshServiceImportEpList(svcim)
	err = r.MeshClient.Status().Update(ctx, meshSvcIm)
	if err != nil {
		log.Error(err, "unable to update service import in spoke cluster", "serviceimport", svcim.Name)
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (a *ServiceImportReconciler) InjectClient(c client.Client) error {
	a.Client = c
	return nil
}
