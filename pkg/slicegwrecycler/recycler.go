package slicegwrecycler

import (
	"context"
	"fmt"

	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	"github.com/kubeslice/worker-operator/controllers"

	"github.com/kubeslice/kubeslice-monitoring/pkg/events"
	ossEvents "github.com/kubeslice/worker-operator/events"
	sidecar "github.com/kubeslice/worker-operator/pkg/gwsidecar"
	hub "github.com/kubeslice/worker-operator/pkg/hub/hubclient"
	"github.com/kubeslice/worker-operator/pkg/logger"
	"github.com/kubeslice/worker-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	NumberOfGateways = 2
)

type recyclerClient struct {
	controllerClient client.Client
	workerClient     client.Client
	ctx              context.Context
	eventRecorder    *events.EventRecorder
}

func NewRecyclerClient(ctx context.Context, workerClient client.Client, controllerClient client.Client, er *events.EventRecorder) (*recyclerClient, error) {
	return &recyclerClient{
		controllerClient: controllerClient,
		workerClient:     workerClient,
		ctx:              ctx,
		eventRecorder:    er,
	}, nil
}

func (r recyclerClient) TriggerFSM(sliceGw *kubeslicev1beta1.SliceGateway, slice *kubeslicev1beta1.Slice,
	gatewayPod *corev1.Pod, controllerName, gwRecyclerName string, numberOfGwSvc int) (bool, error) {
	// start FSM for graceful termination of gateway pods
	// create workerslicegwrecycler on controller
	log := logger.FromContext(r.ctx).WithName("fsm-recycler")
	gwRemoteVpnIP := sliceGw.Status.Config.SliceGatewayRemoteVpnIP
	clientID, err := r.getRemoteGwPodName(gwRemoteVpnIP, gatewayPod.Status.PodIP)
	if err != nil {
		log.Error(err, "Error while fetching remote gw podName")
		// post event to slicegw
		utils.RecordEvent(r.ctx, r.eventRecorder, sliceGw, slice, ossEvents.EventSliceGWRemotePodSyncFailed, controllerName)
		return false, err
	}
	log.Info("creating workerslicegwrecycler", "gwRecyclerName", gwRecyclerName)
	err = r.controllerClient.(*hub.HubClientConfig).CreateWorkerSliceGwRecycler(r.ctx,
		gwRecyclerName,            // recycler name
		clientID, gatewayPod.Name, // gateway pod pairs to recycle
		sliceGw.Name, sliceGw.Status.Config.SliceGatewayRemoteGatewayID, // slice gateway server and client name
		sliceGw.Spec.SliceName) // slice name
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			return false, nil
		}
		return false, err
	}
	utils.RecordEvent(r.ctx, r.eventRecorder, sliceGw, slice, ossEvents.EventSliceGWRebalancingSuccess, controllerName)
	// spawn a new gw nodeport service
	log.Info("creating service", "NumberOfGateways+numberOfGwSvc", NumberOfGateways+numberOfGwSvc)
	err = r.handleSliceGwSvcCreation(sliceGw, NumberOfGateways+numberOfGwSvc, controllerName)
	if err != nil {
		//TODO:add an event and log
		return false, err
	}
	return true, nil
}

// The function was relocated to this location in order to consolidate it into a central location
// and facilitate its use as a utility function.
func (r recyclerClient) handleSliceGwSvcCreation(sliceGw *kubeslicev1beta1.SliceGateway, n int, controllerName string) error {
	log := logger.FromContext(r.ctx).WithName("fsm-recycler")
	sliceGwName := sliceGw.Name
	foundsvc := &corev1.Service{}
	var sliceGwNodePorts []int
	no, err := r.getNumberOfGatewayNodePortServices(sliceGw)
	if err != nil {
		return err

	}
	if no != n {
		// capping the number of services to be 2 for now, later i will be equal to number of gateway nodes,eg: i = len(node.GetExternalIpList())
		for i := 0; i < n; i++ {
			err := r.workerClient.Get(r.ctx, types.NamespacedName{Name: "svc-" + sliceGwName + "-" + fmt.Sprint(i), Namespace: controllers.ControlPlaneNamespace}, foundsvc)
			if err != nil {
				if errors.IsNotFound(err) {
					svc := serviceForGateway(sliceGw, i)
					log.Info("Creating a new Service", "Namespace", svc.Namespace, "Name", svc.Name)
					err = r.workerClient.Create(r.ctx, svc)
					if err != nil {
						log.Error(err, "Failed to create new Service", "Namespace", svc.Namespace, "Name", svc.Name)
						return err
					}
					return nil
				}
				log.Error(err, "Failed to get Service")
				return err
			}
		}
	}
	sliceGwNodePorts, _ = r.getNodePorts(sliceGw)
	err = r.controllerClient.(*hub.HubClientConfig).UpdateNodePortForSliceGwServer(r.ctx, sliceGwNodePorts, sliceGwName)
	if err != nil {
		log.Error(err, "Failed to update NodePort for sliceGw in the hub")
		utils.RecordEvent(r.ctx, r.eventRecorder, sliceGw, nil, ossEvents.EventSliceGWNodePortUpdateFailed, controllerName)
		return err
	}
	return nil
}

// getRemoteGwPodName returns the remote gw PodName.
func (r recyclerClient) getRemoteGwPodName(gwRemoteVpnIP string, podIP string) (string, error) {
	log := logger.FromContext(r.ctx).WithName("fsm-recycler")
	log.Info("calling gw sidecar to get PodName", "type", "slicegw")
	sidecarGrpcAddress := podIP + ":5000"
	workerGWClient, err := sidecar.NewWorkerGWSidecarClientProvider()
	if err != nil {
		return "", err
	}
	remoteGwPodName, err := workerGWClient.GetSliceGwRemotePodName(r.ctx, gwRemoteVpnIP, sidecarGrpcAddress)
	log.Info("slicegw remote pod name", "slicegw", remoteGwPodName)
	if err != nil {
		log.Error(err, "Failed to get slicegw remote pod name. PodIp: %v", podIP)
		return "", err
	}
	return remoteGwPodName, nil
}

func (r recyclerClient) getNumberOfGatewayNodePortServices(sliceGw *kubeslicev1beta1.SliceGateway) (int, error) {
	listOpts := []client.ListOption{
		client.MatchingLabels(map[string]string{
			controllers.ApplicationNamespaceSelectorLabelKey: sliceGw.Spec.SliceName,
			"kubeslice.io/slicegw":                           sliceGw.Name,
		}),
		client.InNamespace(controllers.ControlPlaneNamespace),
	}
	services := corev1.ServiceList{}
	if err := r.workerClient.List(r.ctx, &services, listOpts...); err != nil {
		return 0, err
	}
	return len(services.Items), nil
}

func serviceForGateway(g *kubeslicev1beta1.SliceGateway, i int) *corev1.Service {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "svc-" + g.Name + "-" + fmt.Sprint(i),
			Namespace: g.Namespace,
			Labels: map[string]string{
				controllers.ApplicationNamespaceSelectorLabelKey: g.Spec.SliceName,
				"kubeslice.io/slicegw":                           g.Name,
			},
		},
		Spec: corev1.ServiceSpec{
			Type:     "NodePort",
			Selector: labelsForSliceGwService(g.Name, i),
			Ports: []corev1.ServicePort{{
				Port:       11194,
				Protocol:   corev1.ProtocolUDP,
				TargetPort: intstr.FromInt(11194),
			}},
		},
	}
	//TODO:rework here
	// if len(g.Status.Config.SliceGatewayNodePorts) != 0 {
	// 	svc.Spec.Ports[0].NodePort = int32(g.Status.Config.SliceGatewayNodePort)
	// }
	// ctrl.SetControllerReference(g, svc, r.Scheme)
	return svc
}

func labelsForSliceGwService(name string, i int) map[string]string {
	return map[string]string{
		"kubeslice.io/slice-gw":         name,
		"kubeslice.io/slicegateway-pod": fmt.Sprint(i),
	}
}
func (r recyclerClient) getNodePorts(sliceGw *kubeslicev1beta1.SliceGateway) ([]int, error) {
	var nodePorts []int
	listOpts := []client.ListOption{
		client.MatchingLabels(map[string]string{
			controllers.ApplicationNamespaceSelectorLabelKey: sliceGw.Spec.SliceName,
			"kubeslice.io/slicegw":                           sliceGw.Name,
		}),
	}
	services := corev1.ServiceList{}
	if err := r.workerClient.List(r.ctx, &services, listOpts...); err != nil {
		return nil, err
	}
	for _, service := range services.Items {
		nodePorts = append(nodePorts, int(service.Spec.Ports[0].NodePort))
	}
	return nodePorts, nil
}
