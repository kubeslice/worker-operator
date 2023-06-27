package slicegwrecycler

import (
	"context"
	"fmt"
	"os"

	monitoringEvents "github.com/kubeslice/kubeslice-monitoring/pkg/events"
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	"github.com/kubeslice/worker-operator/controllers"

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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	NumberOfGateways = 2
)

type recyclerClient struct {
}

func NewRecyclerClient() (*recyclerClient, error) {
	return &recyclerClient{}, nil
}

func (r recyclerClient) TriggerFSM(ctx context.Context, sliceGw *kubeslicev1beta1.SliceGateway, slice *kubeslicev1beta1.Slice, hubClient *hub.HubClientConfig, meshClient client.Client, gatewayPod *corev1.Pod,
	eventRecorder *monitoringEvents.EventRecorder, controllerName, gwRecyclerName string) (bool, error) {
	// start FSM for graceful termination of gateway pods
	// create workerslicegwrecycler on controller
	log := logger.FromContext(ctx)

	gwRemoteVpnIP := sliceGw.Status.Config.SliceGatewayRemoteVpnIP
	clientID, err := getRemoteGwPodName(ctx, meshClient, gwRemoteVpnIP, gatewayPod.Status.PodIP)
	if err != nil {
		log.Error(err, "Error while fetching remote gw podName")
		// post event to slicegw
		utils.RecordEvent(ctx, eventRecorder, sliceGw, slice, ossEvents.EventSliceGWRemotePodSyncFailed, controllerName)
		return false, err
	}
	err = hubClient.CreateWorkerSliceGwRecycler(ctx, gwRecyclerName, clientID, gatewayPod.Name, sliceGw.Name, sliceGw.Status.Config.SliceGatewayRemoteGatewayID, sliceGw.Spec.SliceName)
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			return false, nil
		}
		return false, err
	}
	utils.RecordEvent(ctx, eventRecorder, sliceGw, slice, ossEvents.EventSliceGWRebalancingSuccess, controllerName)
	// spawn a new gw nodeport service
	_, _, _, err = handleSliceGwSvcCreation(ctx, meshClient, hubClient, sliceGw, NumberOfGateways+1, eventRecorder, controllerName)
	if err != nil {
		//TODO:add an event and log
		return false, err
	}
	return true, nil
}

// The function was relocated to this location in order to consolidate it into a central location
// and facilitate its use as a utility function.
func handleSliceGwSvcCreation(ctx context.Context, meshClient client.Client, hubClient *hub.HubClientConfig, sliceGw *kubeslicev1beta1.SliceGateway, n int, eventRecorder *monitoringEvents.EventRecorder, controllerName string) (bool, reconcile.Result, []int, error) {
	log := logger.FromContext(ctx).WithName("slicegw")
	sliceGwName := sliceGw.Name
	foundsvc := &corev1.Service{}
	var sliceGwNodePorts []int
	no, _ := getNumberOfGatewayNodePortServices(ctx, meshClient, sliceGw)
	if no != n {
		// capping the number of services to be 2 for now, later i will be equal to number of gateway nodes,eg: i = len(node.GetExternalIpList())
		for i := 0; i < n; i++ {
			err := meshClient.Get(ctx, types.NamespacedName{Name: "svc-" + sliceGwName + "-" + fmt.Sprint(i), Namespace: controllers.ControlPlaneNamespace}, foundsvc)
			if err != nil {
				if errors.IsNotFound(err) {
					svc := serviceForGateway(sliceGw, i)
					log.Info("Creating a new Service", "Namespace", svc.Namespace, "Name", svc.Name)
					err = meshClient.Create(ctx, svc)
					if err != nil {
						log.Error(err, "Failed to create new Service", "Namespace", svc.Namespace, "Name", svc.Name)
						return true, ctrl.Result{}, nil, err
					}
					return true, ctrl.Result{Requeue: true}, nil, nil
				}
				log.Error(err, "Failed to get Service")
				return true, ctrl.Result{}, nil, err
			}
		}
	}
	sliceGwNodePorts, _ = getNodePorts(ctx, meshClient, sliceGw)
	err := hubClient.UpdateNodePortForSliceGwServer(ctx, sliceGwNodePorts, sliceGwName)
	if err != nil {
		log.Error(err, "Failed to update NodePort for sliceGw in the hub")
		utils.RecordEvent(ctx, eventRecorder, sliceGw, nil, ossEvents.EventSliceGWNodePortUpdateFailed, controllerName)
		return true, ctrl.Result{}, sliceGwNodePorts, err
	}
	return false, reconcile.Result{}, sliceGwNodePorts, nil
}

func createHeadlessServiceForGwServer(slicegateway *kubeslicev1beta1.SliceGateway) *corev1.Service {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      slicegateway.Status.Config.SliceGatewayRemoteGatewayID,
			Namespace: slicegateway.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Ports:     nil,
			Selector:  nil,
			ClusterIP: "None",
		},
	}
	// ctrl.SetControllerReference(slicegateway, svc, r.Scheme)
	return svc
}

// getRemoteGwPodName returns the remote gw PodName.
func getRemoteGwPodName(ctx context.Context, meshClient client.Client, gwRemoteVpnIP string, podIP string) (string, error) {
	log := logger.FromContext(ctx)
	log.Info("calling gw sidecar to get PodName", "type", "slicegw")
	sidecarGrpcAddress := podIP + ":5000"
	workerGWClient, err := sidecar.NewWorkerGWSidecarClientProvider()
	if err != nil {
		os.Exit(1)
	}
	remoteGwPodName, err := workerGWClient.GetSliceGwRemotePodName(ctx, gwRemoteVpnIP, sidecarGrpcAddress)
	log.Info("slicegw remote pod name", "slicegw", remoteGwPodName)
	if err != nil {
		log.Error(err, "Failed to get slicegw remote pod name. PodIp: %v", podIP)
		return "", err
	}
	return remoteGwPodName, nil
}

func getNumberOfGatewayNodePortServices(ctx context.Context, meshClient client.Client, sliceGw *kubeslicev1beta1.SliceGateway) (int, error) {
	listOpts := []client.ListOption{
		client.MatchingLabels(map[string]string{
			controllers.ApplicationNamespaceSelectorLabelKey: sliceGw.Spec.SliceName,
			"kubeslice.io/slicegw":                           sliceGw.Name,
		}),
		client.InNamespace(controllers.ControlPlaneNamespace),
	}
	services := corev1.ServiceList{}
	if err := meshClient.List(ctx, &services, listOpts...); err != nil {
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
func getNodePorts(ctx context.Context, meshClient client.Client, sliceGw *kubeslicev1beta1.SliceGateway) ([]int, error) {
	var nodePorts []int
	listOpts := []client.ListOption{
		client.MatchingLabels(map[string]string{
			controllers.ApplicationNamespaceSelectorLabelKey: sliceGw.Spec.SliceName,
			"kubeslice.io/slicegw":                           sliceGw.Name,
		}),
	}
	services := corev1.ServiceList{}
	if err := meshClient.List(ctx, &services, listOpts...); err != nil {
		return nil, err
	}
	for _, service := range services.Items {
		nodePorts = append(nodePorts, int(service.Spec.Ports[0].NodePort))
	}
	return nodePorts, nil
}
