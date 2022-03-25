package controllers

import (
	"context"

	meshv1beta1 "bitbucket.org/realtimeai/kubeslice-operator/api/v1beta1"
	"bitbucket.org/realtimeai/kubeslice-operator/internal/logger"
	"bitbucket.org/realtimeai/kubeslice-operator/internal/netop"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NetOpPod contains details of NetOp Pod running in the cluster
type NetOpPod struct {
	PodIP   string
	PodName string
	Node    string
}

func (r *SliceGwReconciler) getNetOpPods(ctx context.Context, slicegateway *meshv1beta1.SliceGateway) error {
	log := logger.FromContext(ctx).WithValues("type", "net_op")
	debugLog := log.V(1)

	netOpPods, err := GetNetOpPods(ctx, slicegateway.Namespace, r.List)
	if err != nil {
		log.Error(err, "Failed to list net_op pods")
		return err
	}
	debugLog.Info("got netop pods", "pods", netOpPods)
	r.NetOpPods = netOpPods
	return nil
}

// GetNetOpPods returns the netop pods in the cluster
func GetNetOpPods(ctx context.Context, namespace string,
	listFn func(context.Context, client.ObjectList, ...client.ListOption) error) ([]NetOpPod, error) {
	labels := map[string]string{"app": "app_net_op"}

	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingLabels(labels),
	}
	if err := listFn(ctx, podList, listOpts...); err != nil {
		return nil, err
	}

	pods := []NetOpPod{}
	for _, pod := range podList.Items {
		pods = append(pods, NetOpPod{
			PodName: pod.Name,
			PodIP:   pod.Status.PodIP,
			Node:    pod.Spec.NodeName,
		})
	}
	return pods, nil
}

// SyncSliceQosProfileWithNetOp Syncs slice qos profile with netop pods
func (r *SliceReconciler) SyncSliceQosProfileWithNetOp(ctx context.Context, slice *meshv1beta1.Slice) error {
	log := logger.FromContext(ctx).WithValues("type", "net_op")

	// Get the current list of netop pods.
	// This populates the NetOpPods map in the slice reconciler structure.
	err := r.getNetOpPods(ctx, slice.Name, slice.Namespace)
	if err != nil {
		return err
	}

	for _, n := range r.NetOpPods {
		sidecarGrpcAddress := n.PodIP + ":5000"
		err := netop.UpdateSliceQosProfile(ctx, sidecarGrpcAddress, slice)
		if err != nil {
			log.Error(err, "Failed to send qos to netop. PodIp: %v, PodName: %v", n.PodIP, n.PodName)
			return err
		}
	}
	return nil
}

func (r *SliceReconciler) getNetOpPods(ctx context.Context, sliceName string, namespace string) error {
	log := logger.FromContext(ctx).WithValues("type", "net_op")
	debugLog := log.V(1)

	netOpPods, err := GetNetOpPods(ctx, namespace, r.List)
	if err != nil {
		log.Error(err, "Failed to list net_op pods")
		return err
	}

	debugLog.Info("got netop pods", "pods", netOpPods)
	r.NetOpPods = netOpPods
	return nil
}

// SendSliceDeletionEventToNetOp sends slice deletion event to netop pods for cleanup
func (r *SliceReconciler) SendSliceDeletionEventToNetOp(ctx context.Context, sliceName string, namespace string) error {
	log := logger.FromContext(ctx).WithValues("type", "net_op")
	// Get the current list of netop pods.
	// This populates the NetOpPods map in the slice reconciler structure.
	err := r.getNetOpPods(ctx, sliceName, namespace)
	if err != nil {
		return err
	}

	for _, n := range r.NetOpPods {
		sidecarGrpcAddress := n.PodIP + ":5000"
		err := netop.SendSliceLifeCycleEventToNetOp(ctx, sidecarGrpcAddress, sliceName, netop.EventType_EV_DELETE)
		if err != nil {
			log.Error(err, "Failed to send slice lifecycle event to netop. PodIp: %v, PodName: %v", n.PodIP, n.PodName)
		}
	}

	return nil
}
