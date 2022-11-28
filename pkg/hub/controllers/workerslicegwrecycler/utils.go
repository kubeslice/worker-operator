package workerslicegwrecycler

import (
	"context"
	"time"

	spokev1alpha1 "github.com/kubeslice/apis/pkg/worker/v1alpha1"
	"github.com/looplab/fsm"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *Reconciler) spawn_new_gw_pod(e *fsm.Event) error {
	// The client cluster finds the client pod using the client-id field and removes the kubeslice gw label to drive it out of the purview of the gw deployment spec. Once the label is removed, Kubernetes spawns a new pod automatically to honor the number of replicas defined in the gw deployment spec. Once the new pod comes up, the client cluster retrieves the pod info to verify that the new pod has obtained an nsm IP. It then posts an update to the status field
	workerslicegwrecycler := e.Args[0].(spokev1alpha1.WorkerSliceGwRecycler)
	isClient := e.Args[1].(bool)

	gwPod := corev1.Pod{}
	if isClient {
		if err := r.MeshClient.Get(context.Background(), types.NamespacedName{Namespace: "kubeslice-system", Name: workerslicegwrecycler.Spec.GwPair.ClientID}, &gwPod); err != nil {
			return err
		}
	} else {
		if err := r.MeshClient.Get(context.Background(), types.NamespacedName{Namespace: "kubeslice-system", Name: workerslicegwrecycler.Spec.GwPair.ServerID}, &gwPod); err != nil {
			return err
		}
	}

	r.Log.Info("removing label from pods", "pod", gwPod)
	delete(gwPod.Labels, "kubeslice.io/pod-type")
	gwPod.Labels["kubeslice.io/pod-type"] = "toBeDeleted"
	err := r.MeshClient.Update(context.Background(), &gwPod)
	if err != nil {
		return err
	}
	// wait till the replicas are back and get the latest spawned pod
	gwDeploy := appsv1.Deployment{}
	wait.Poll(1*time.Second, 60*time.Second, func() (done bool, err error) {
		err = r.MeshClient.Get(context.Background(), types.NamespacedName{Namespace: "kubeslice-system", Name: workerslicegwrecycler.Spec.SliceGwClient}, &gwDeploy)
		if err != nil {
			return false, err
		}
		return gwDeploy.Status.Replicas == gwDeploy.Status.AvailableReplicas, nil
	})
	podList := corev1.PodList{}
	labels := map[string]string{"kubeslice.io/pod-type": "slicegateway", "kubeslice.io/slice": workerslicegwrecycler.Spec.SliceName}
	listOptions := []client.ListOption{
		client.MatchingLabels(labels),
	}
	ctx := context.Background()
	err = r.MeshClient.List(ctx, &podList, listOptions...)
	if err != nil {
		return err
	}
	newestPod := podList.Items[0]
	newestPodDuration := time.Since(podList.Items[0].CreationTimestamp.Time).Seconds()
	for _, pod := range podList.Items {
		duration := time.Since(pod.CreationTimestamp.Time).Seconds()
		if duration < newestPodDuration {
			newestPodDuration = duration
			newestPod = pod
		}
	}
	if isClient {
		workerslicegwrecycler.Status.Client.Response = "new_gw_spawned"
		workerslicegwrecycler.Status.Client.RecycledClient = newestPod.Name
	} else {
		// progress the FSM to the next state by updating the CR object with the next state: new_gw_spawned
		workerslicegwrecycler.Spec.GwPair.ClientID = workerslicegwrecycler.Status.Client.RecycledClient
		workerslicegwrecycler.Spec.GwPair.ServerID = newestPod.Name
		workerslicegwrecycler.Spec.State = new_gw_spawned
		workerslicegwrecycler.Spec.Request = update_routing_table
	}
	return r.Update(ctx, &workerslicegwrecycler)
}

func (r *Reconciler) update_routing_table(e *fsm.Event) {}

func (r *Reconciler) delete_old_gw_pods(e *fsm.Event) {}
