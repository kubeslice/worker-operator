package vpnkeyrotation

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"
	"time"

	hubv1alpha1 "github.com/kubeslice/apis/pkg/controller/v1alpha1"
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"

	"github.com/kubeslice/worker-operator/controllers"
	"github.com/kubeslice/worker-operator/controllers/slicegateway"

	"github.com/kubeslice/kubeslice-monitoring/pkg/events"
	"github.com/kubeslice/kubeslice-monitoring/pkg/metrics"

	ossEvents "github.com/kubeslice/worker-operator/events"
	hub "github.com/kubeslice/worker-operator/pkg/hub/hubclient"
	"github.com/kubeslice/worker-operator/pkg/logger"
	"github.com/kubeslice/worker-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func NewReconciler(mc client.Client, hc client.Client, er *events.EventRecorder, mf metrics.MetricsFactory, workerRecyclerClient WorkerRecyclerClientProvider) *Reconciler {
	return &Reconciler{
		Client:               hc,
		WorkerClient:         mc,
		ControllerClient:     hc,
		EventRecorder:        er,
		WorkerRecyclerClient: workerRecyclerClient,
	}
}

func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (ctrl.Result, error) {
	log := logger.FromContext(ctx).WithName("vpnkeyrotation-reconciler").WithValues("name", req.Name, "namespace", req.Namespace)
	ctx = logger.WithLogger(ctx, log)
	vpnKeyRotation := &hubv1alpha1.VpnKeyRotation{}
	err := r.Get(ctx, req.NamespacedName, vpnKeyRotation)
	log.Info("got vpnkeyrotation from controller", "vpnKeyRotation", vpnKeyRotation.Name)
	// Request object not found, could have been deleted after reconcile request.
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Return and don't requeue
			log.Info("vpnkeyrotation resource not found in hub. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	if vpnKeyRotation.Spec.CertificateCreationTime == nil {
		return ctrl.Result{
			RequeueAfter: time.Second,
		}, nil
	}
	// Step 1: Begin the process of initializing the status of key rotation using certification creation data.
	allGwsUnderCluster := vpnKeyRotation.Spec.ClusterGatewayMapping[os.Getenv("CLUSTER_NAME")]
	// contains all the gateways associated with the cluster for a particular slice
	log.V(1).Info("gateways under cluster", "allGwsUnderCluster", allGwsUnderCluster)
	requeue, err := r.syncCurrentRotationState(ctx, vpnKeyRotation, allGwsUnderCluster)
	if requeue {
		return ctrl.Result{Requeue: true}, nil
	}
	if err != nil {
		log.Error(err, "error while updating vpnKeyRotation status while rotation status is out of sync")
		return ctrl.Result{}, err
	}

	// Step 2: We choose a gateway from the array and compare its LastUpdatedTimestamp with the CertificateCreationTime.
	// If the time difference matches, we process the request; otherwise, we move on to the next gateway in the array.
	// The control should not move to step 3 before step 2 is completed for each gateway and each pod
	for _, selectedGw := range allGwsUnderCluster {
		// TODO: handle !ok
		rotationStatus := vpnKeyRotation.Status.CurrentRotationState[selectedGw]
		rotationTimeDiff := vpnKeyRotation.Spec.CertificateCreationTime.Equal(&rotationStatus.LastUpdatedTimestamp)
		if !rotationTimeDiff && rotationStatus.Status != hubv1alpha1.InProgress {
			log.Info("rotation interval has elapsed, starting recycling", "rotationTimeDiff", rotationTimeDiff)
			utils.RecordEvent(ctx, r.EventRecorder, vpnKeyRotation, nil, ossEvents.EventGatewayCertificateRecyclingTriggered, controllerName)
			// if error occurs , the reconcile loop will be re-triggered and it should not update back to
			// hubv1alpha1.SecretReadInProgress if status was already hubv1alpha1.SecretUpdated
			if (rotationStatus.Status != hubv1alpha1.SecretUpdated) &&
				(rotationStatus.Status != hubv1alpha1.SecretReadInProgress) {
				// Check if status is not in SecretReadInProgress , if not update it else ignore and move ahead
				if err := r.updateRotationStatus(ctx, selectedGw, hubv1alpha1.SecretReadInProgress, vpnKeyRotation); err != nil {
					return ctrl.Result{}, err
				}
				return ctrl.Result{Requeue: true}, nil
			}

			result, requeue, err := r.updateCertificates(ctx, vpnKeyRotation.Spec.RotationCount, selectedGw, req)
			if requeue {
				return result, nil
			}
			if err != nil {
				log.Error(err, "failed to update certificates")
				return result, err
			}

			log.V(1).Info("certificates are updated for gw", "gateway", selectedGw)
			utils.RecordEvent(ctx, r.EventRecorder, vpnKeyRotation, nil, ossEvents.EventGatewayCertificateUpdated, controllerName)
			// if error occurs , the reconcile loop will be re-triggered and it should not update back to
			// hubv1alpha1.SecretUpdated if status was already hubv1alpha1.InProgress
			if (rotationStatus.Status != hubv1alpha1.InProgress) &&
				(rotationStatus.Status != hubv1alpha1.SecretUpdated) {
				if err := r.updateRotationStatus(ctx, selectedGw, hubv1alpha1.SecretUpdated, vpnKeyRotation); err != nil {
					return ctrl.Result{}, err
				}
			}
			// fetch the slicegateway to check if it client or server
			sliceGw := &kubeslicev1beta1.SliceGateway{}
			err = r.WorkerClient.Get(ctx, types.NamespacedName{
				Name:      selectedGw,
				Namespace: ControlPlaneNamespace,
			}, sliceGw)

			if err != nil {
				log.Error(err, "Err():", err.Error())
				return ctrl.Result{}, err
			}
			// Upon certificate update, if the selected gateway is a server,
			// await the client pod to transition into the hubv1alpha1.SecretUpdated state
			// then trigger gateway recycle FSM at server side
			if isServer(sliceGw) {
				clientGWName := sliceGw.Status.Config.SliceGatewayRemoteGatewayID
				clientGWRotation, ok := vpnKeyRotation.Status.CurrentRotationState[clientGWName]
				if !ok {
					log.V(1).Info("waiting for the inclusion of the client status in the rotation status")
					return ctrl.Result{
						RequeueAfter: 5 * time.Second,
					}, nil
				}

				// if client is in hubv1alpha1.SecretUpdated state, trigger the FSM
				if clientGWRotation.Status == hubv1alpha1.SecretUpdated {
					sliceName := sliceGw.Spec.SliceName
					podsUnderGw := corev1.PodList{}
					listOptions := []client.ListOption{
						client.MatchingLabels(
							map[string]string{
								"kubeslice.io/pod-type":              "slicegateway",
								ApplicationNamespaceSelectorLabelKey: sliceName,
								"kubeslice.io/slice-gw":              sliceGw.Name},
						),
					}
					err := r.WorkerClient.List(ctx, &podsUnderGw, listOptions...)
					if err != nil {
						log.Error(err, "err listing gw pods")
						return ctrl.Result{}, err
					}

					// STEP 1: Validate all gateway pods are ready before triggering FSM
					log.Info("Validating gateway pod readiness before FSM trigger",
						"gateway", selectedGw,
						"podCount", len(podsUnderGw.Items))

					for _, pod := range podsUnderGw.Items {
						if err := slicegateway.ValidateGatewayPodReadiness(sliceGw, pod.Name); err != nil {
							errMsg := err.Error()
							// Check for transient errors that should trigger a requeue
							if strings.Contains(errMsg, "not found in gateway status") {
								log.Info("Gateway pod status not yet populated, requeueing", "pod", pod.Name)
								return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
							}
							if strings.Contains(errMsg, "tunnel not up") || strings.Contains(errMsg, "tunnel interface name not set") {
								log.Info("Waiting for tunnel to come up, requeueing", "pod", pod.Name)
								return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
							}
							if strings.Contains(errMsg, "tunnel IPs not configured") || strings.Contains(errMsg, "peer pod name not available") {
								log.Info("Waiting for tunnel configuration, requeueing", "pod", pod.Name)
								return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
							}
							log.Error(err, "Failed to validate gateway pod readiness", "pod", pod.Name)
							return ctrl.Result{}, err
						}
					}

					log.Info("All gateway pods validated and ready for FSM trigger",
						"gateway", selectedGw,
						"podCount", len(podsUnderGw.Items))

					// STEP 2: trigger FSM for all the gateway pods before updating the status and timestamp
					for _, v := range podsUnderGw.Items {
						// trigger FSM to recylce both gateway pod pairs
						slice, err := controllers.GetSlice(ctx, r.WorkerClient, sliceName)
						if err != nil {
							log.Error(err, "Failed to get Slice", "slice", sliceName)
							return ctrl.Result{}, err
						}

						peerPodName, err := slicegateway.GetPeerGwPodName(v.Name, sliceGw)
						if err != nil {
							// Wrap transient errors as retryable
							errMsg := err.Error()
							if strings.Contains(errMsg, "not found in status") {
								log.Info("Gateway pod status not yet populated, requeueing", "pod", v.Name)
								return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
							}
							if strings.Contains(errMsg, "tunnel is down") {
								log.Info("Waiting for tunnel to come up, requeueing", "pod", v.Name)
								return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
							}
							if strings.Contains(errMsg, "peer pod info unavailable") {
								log.Info("Peer pod information not yet synchronized, requeueing", "pod", v.Name)
								return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
							}
							log.Error(err, "Failed to get peer pod name for gw pod", "pod", v.Name)
							return ctrl.Result{}, err
						}

						serverID := slicegateway.GetDepNameFromPodName(sliceGw.Status.Config.SliceGatewayID, v.Name)
						clientID := slicegateway.GetDepNameFromPodName(sliceGw.Status.Config.SliceGatewayRemoteGatewayID, peerPodName)

						// STEP 2b: Trigger FSM with retry on transient errors
						err = retry.OnError(
							retry.DefaultRetry, // Uses exponential backoff
							func(err error) bool {
								// Retry on specific transient errors
								if err == nil {
									return false
								}
								errStr := err.Error()
								return apierrors.IsTimeout(err) ||
									apierrors.IsServerTimeout(err) ||
									apierrors.IsServiceUnavailable(err) ||
									strings.Contains(errStr, "connection refused") ||
									strings.Contains(errStr, "i/o timeout") ||
									strings.Contains(errStr, "deadline exceeded")
							},
							func() error {
								return r.WorkerRecyclerClient.TriggerFSM(sliceGw, slice, serverID, clientID, controllerName)
							},
						)
						if err != nil {
							log.Error(err, "Err(): triggering FSM after retries",
								"pod", v.Name,
								"serverID", serverID,
								"clientID", clientID)
							// Return with requeue instead of failing permanently
							return ctrl.Result{RequeueAfter: 30 * time.Second}, err
						}

						log.Info("Successfully triggered FSM for gateway pod",
							"pod", v.Name,
							"serverID", serverID,
							"clientID", clientID)
					}
					utils.RecordEvent(ctx, r.EventRecorder, vpnKeyRotation, nil, ossEvents.EventTriggeredFSMToRecycleGateways, controllerName)
					// update the last Updated Timestamp so even if it requeues the rotation Time Diff will not be greater than 0
					if err := r.updateRotationStatusWithTimeStamp(ctx, clientGWName, hubv1alpha1.InProgress, vpnKeyRotation, vpnKeyRotation.Spec.CertificateCreationTime); err != nil {
						log.Error(err, "Err:() updating client gateway status")
						return ctrl.Result{}, err
					}
					// since server updates the client status, we first update the client status and then server's
					// update the last Updated Timestamp so even if it requeues the rotation Time Diff will not be greater than 0
					if err := r.updateRotationStatusWithTimeStamp(ctx, selectedGw, hubv1alpha1.InProgress, vpnKeyRotation, vpnKeyRotation.Spec.CertificateCreationTime); err != nil {
						log.Error(err, "Err:() updating server gateway status")
						return ctrl.Result{}, err
					}
					return ctrl.Result{Requeue: true}, nil
				} else {
					return ctrl.Result{Requeue: true}, nil
				}
			}
		}
	}

	// Step 3: If VPN rotation is currently in progress, verify the status of workerslicegwrecyclers to determine
	// if the process has been completed - error or success
	// client gw will only move to in-progress once the FSM is started by server.
	currentDate := time.Now()
	certificationCreationDate := vpnKeyRotation.Spec.CertificateCreationTime
	if certificationCreationDate.Year() == currentDate.Year() &&
		certificationCreationDate.Month() == currentDate.Month() &&
		certificationCreationDate.Day() == currentDate.Day() {
		for _, selectedGw := range allGwsUnderCluster {
			// check for status read in progress
			if vpnKeyRotation.Status.CurrentRotationState[selectedGw].Status == hubv1alpha1.InProgress {
				// if client then check for server
				sliceGw := &kubeslicev1beta1.SliceGateway{}
				err = r.WorkerClient.Get(ctx, types.NamespacedName{
					Name:      selectedGw,
					Namespace: ControlPlaneNamespace,
				}, sliceGw)
				if err != nil {
					if apierrors.IsNotFound(err) {
						log.Error(err, "slice gateway doesn't exist")
						return ctrl.Result{}, nil
					}
					log.Error(err, "error fetching slice gateway")
					return ctrl.Result{}, err
				}

				if isServer(sliceGw) {
					recyclers, err := r.ControllerClient.(*hub.HubClientConfig).ListWorkerSliceGwRecycler(ctx, selectedGw)
					if err != nil {
						return ctrl.Result{}, err
					}
					updatedTime := vpnKeyRotation.Spec.CertificateCreationTime
					if len(recyclers) > 0 {
						for _, v := range recyclers {
							if v.Spec.State == "error" || v.Status.Client.Response == "error" {
								log.V(1).Info("gateway recycler is in error state", "gateway", v.Name)
								utils.RecordEvent(ctx, r.EventRecorder, vpnKeyRotation, nil, ossEvents.EventGatewayRecyclingFailed, controllerName)
								if err := r.updateRotationStatusWithTimeStamp(ctx, sliceGw.Status.Config.SliceGatewayRemoteGatewayID, hubv1alpha1.Error, vpnKeyRotation, updatedTime); err != nil {
									return ctrl.Result{}, nil
								}
								if err := r.updateRotationStatusWithTimeStamp(ctx, selectedGw, hubv1alpha1.Error, vpnKeyRotation, updatedTime); err != nil {
									return ctrl.Result{}, err
								}
								return ctrl.Result{}, nil
							} else {
								// This means that recycling is in progress.
								// We will queue the task again and recheck after a five-minute interval.
								log.V(1).Info("gateway recycler is in progress state", "gateway", v.Name)
								return ctrl.Result{RequeueAfter: time.Minute * 2}, nil
							}
						}
					} else {
						// Update the rotation status to Complete with TimeStamp
						utils.RecordEvent(ctx, r.EventRecorder, vpnKeyRotation, nil, ossEvents.EventGatewayRecyclingSuccessful, controllerName)
						log.Info("gateway recycling is sucessfully done", "gateway", selectedGw)
						if err := r.updateRotationStatusWithTimeStamp(ctx, selectedGw, hubv1alpha1.Complete, vpnKeyRotation, updatedTime); err != nil {
							return ctrl.Result{}, nil
						}
						if err := r.updateRotationStatusWithTimeStamp(ctx, sliceGw.Status.Config.SliceGatewayRemoteGatewayID, hubv1alpha1.Complete, vpnKeyRotation, updatedTime); err != nil {
							return ctrl.Result{}, nil
						}
						// clean up for deletion for old secrets
						if err = r.removeOldSecrets(ctx, vpnKeyRotation.Spec.RotationCount, selectedGw); err != nil {
							return ctrl.Result{}, err
						}
					}
				}
			}
		}
	}

	return ctrl.Result{}, nil
}

func (r *Reconciler) updateRotationStatus(ctx context.Context, gatewayName, rotationStatus string, vpnKeyRotation *hubv1alpha1.VpnKeyRotation) error {
	err := retry.RetryOnConflict(wait.Backoff{
		Steps:    100,
		Duration: 10 * time.Millisecond,
		Factor:   1.0,
		Jitter:   0.1,
	}, func() error {
		if getErr := r.Get(ctx,
			types.NamespacedName{Name: vpnKeyRotation.Name, Namespace: vpnKeyRotation.Namespace},
			vpnKeyRotation); getErr != nil {
			return getErr
		}
		if len(vpnKeyRotation.Status.CurrentRotationState) == 0 {
			return errors.New("current state is empty")
		}

		if status, ok := vpnKeyRotation.Status.CurrentRotationState[gatewayName]; ok && status.Status != rotationStatus {
			status.Status = rotationStatus
			vpnKeyRotation.Status.CurrentRotationState[gatewayName] = status
			return r.Status().Update(ctx, vpnKeyRotation)
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func (r *Reconciler) updateRotationStatusWithTimeStamp(ctx context.Context, gatewayName, rotationStatus string, vpnKeyRotation *hubv1alpha1.VpnKeyRotation,
	timestamp *metav1.Time) error {
	err := retry.RetryOnConflict(wait.Backoff{
		Steps:    100,
		Duration: 10 * time.Millisecond,
		Factor:   1.0,
		Jitter:   0.1,
	}, func() error {
		if getErr := r.Get(ctx,
			types.NamespacedName{Name: vpnKeyRotation.Name, Namespace: vpnKeyRotation.Namespace},
			vpnKeyRotation); getErr != nil {
			return getErr
		}
		rotation := hubv1alpha1.StatusOfKeyRotation{
			Status:               rotationStatus,
			LastUpdatedTimestamp: *timestamp,
		}
		vpnKeyRotation.Status.CurrentRotationState[gatewayName] = rotation
		if rotationStatus == hubv1alpha1.Complete || rotationStatus == hubv1alpha1.Error {
			if vpnKeyRotation.Status.StatusHistory == nil {
				vpnKeyRotation.Status.StatusHistory = make(map[string][]hubv1alpha1.StatusOfKeyRotation)
			}
			history := vpnKeyRotation.Status.StatusHistory[gatewayName]

			// Prepend current rotation to history, maintaining a maximum of 5 rotations
			// StatusHistory is a stack n number of gateways rotation status
			history = append([]hubv1alpha1.StatusOfKeyRotation{rotation}, history...)
			if len(history) >= 5 {
				// Remove the oldest rotation from the history
				history = history[:5]
			}
			vpnKeyRotation.Status.StatusHistory[gatewayName] = history

		}

		return r.Status().Update(ctx, vpnKeyRotation)
	})
	if err != nil {
		return err
	}
	return nil
}
func arrayDifference(arr1, arr2 []string) []string {
	// Create a map to track the elements in arr1
	elements := make(map[string]bool)

	// Store elements of arr1 in the map
	for _, elem := range arr1 {
		elements[elem] = true
	}

	// Create a slice to store the elements in arr2 that are not in arr1
	difference := []string{}

	// Check if elements of arr2 are not present in arr1
	for _, elem := range arr2 {
		if !elements[elem] {
			difference = append(difference, elem)
		}
	}
	return difference
}

func (r *Reconciler) syncCurrentRotationState(ctx context.Context,
	vpnKeyRotation *hubv1alpha1.VpnKeyRotation, allGwsUnderCluster []string) (bool, error) {
	log := logger.FromContext(ctx)
	requeue := false
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if getErr := r.Get(ctx,
			types.NamespacedName{Name: vpnKeyRotation.Name, Namespace: vpnKeyRotation.Namespace},
			vpnKeyRotation); getErr != nil {
			return getErr
		}
		currentRotationState := vpnKeyRotation.Status.CurrentRotationState
		if len(currentRotationState) == 0 {
			currentRotationState = make(map[string]hubv1alpha1.StatusOfKeyRotation)
		}
		allGwsUnderCluster = vpnKeyRotation.Spec.ClusterGatewayMapping[os.Getenv("CLUSTER_NAME")]
		keysInStatus := []string{}
		for key := range currentRotationState {
			if strings.Contains(key, os.Getenv("CLUSTER_NAME")) && !strings.HasSuffix(key, os.Getenv("CLUSTER_NAME")) {
				keysInStatus = append(keysInStatus, key)
			}
		}
		keysToDeleteFromStatus := arrayDifference(allGwsUnderCluster, keysInStatus)
		syncedRotationState := make(map[string]hubv1alpha1.StatusOfKeyRotation)
		for _, gw := range allGwsUnderCluster {
			obj, ok := currentRotationState[gw]

			if ok {
				syncedRotationState[gw] = obj
			} else if !ok {
				if !r.areGatewayPodsReady(ctx, gw) {
					requeue = true
					return errors.New("gateway pods are not ready")
				}
				syncedRotationState[gw] = hubv1alpha1.StatusOfKeyRotation{
					Status:               hubv1alpha1.Complete,
					LastUpdatedTimestamp: *vpnKeyRotation.Spec.CertificateCreationTime,
				}
			}
		}

		// Add any remaining gateways from currentRotationState to SyncedRotationState
		for gw, obj := range currentRotationState {
			if _, ok := syncedRotationState[gw]; !ok {
				syncedRotationState[gw] = obj
			}
		}
		if len(syncedRotationState) != len(vpnKeyRotation.Status.CurrentRotationState) || len(keysToDeleteFromStatus) > 0 {

			// Merge the new syncedRotationState with the existing state
			for gw, obj := range syncedRotationState {
				currentRotationState[gw] = obj
			}
			if len(keysToDeleteFromStatus) > 0 {
				// this means cluster detach scenario
				for _, key := range keysToDeleteFromStatus {
					delete(currentRotationState, key)
				}
			}
			log.Info("syncing current rotation state for the gateways",
				"from", vpnKeyRotation.Status.CurrentRotationState,
				"to", currentRotationState)
			vpnKeyRotation.Status.CurrentRotationState = currentRotationState
			requeue = true
			return r.Status().Update(ctx, vpnKeyRotation)
		}
		return nil
	})
	if err != nil {
		return requeue, err
	}
	return requeue, nil
}

func isServer(sliceGw *kubeslicev1beta1.SliceGateway) bool {
	return sliceGw.Status.Config.SliceGatewayHostType == "Server"
}

func (r *Reconciler) updateCertificates(ctx context.Context, rotationVersion int, sliceGwName string,
	req reconcile.Request) (ctrl.Result, bool, error) {
	log := logger.FromContext(ctx)
	currentSecretName := sliceGwName + "-" + strconv.Itoa(rotationVersion)
	meshSliceGwCerts := &corev1.Secret{}
	err := r.WorkerClient.Get(ctx, types.NamespacedName{
		Name:      currentSecretName,
		Namespace: ControlPlaneNamespace,
	}, meshSliceGwCerts)
	if err != nil {
		if apierrors.IsNotFound(err) {
			sliceGwCerts := &corev1.Secret{}
			err = r.Get(ctx, types.NamespacedName{
				Name:      sliceGwName,
				Namespace: req.Namespace,
			}, sliceGwCerts)
			if err != nil {
				log.Error(err, "unable to fetch slicegw certs from the hub", "sliceGw", sliceGwName)
				return ctrl.Result{
					RequeueAfter: 10 * time.Second,
				}, true, nil
			}
			meshSliceGwCerts := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      currentSecretName,
					Namespace: ControlPlaneNamespace,
					Labels:    map[string]string{"kubeslice.io/slice-gw": sliceGwName},
				},
				Data: sliceGwCerts.Data,
			}
			err = r.WorkerClient.Create(ctx, meshSliceGwCerts)
			if err != nil {
				log.Error(err, "unable to create secret to store slicegw certs in worker cluster", "sliceGw", sliceGwName)
				return ctrl.Result{}, false, err
			}
			log.V(1).Info("sliceGw secret created in worker cluster")
			// this required requeueing
			// return ctrl.Result{Requeue: true}, true, nil
		} else {
			log.Error(err, "unable to fetch slicegw certs from the worker", "sliceGw", sliceGwName)
			return ctrl.Result{}, false, err
		}
	}
	// secret with current rotation version already exists, no requeue
	return ctrl.Result{}, false, nil
}

func (r *Reconciler) areGatewayPodsReady(ctx context.Context, sliceGWName string) bool {
	podsList := corev1.PodList{}
	labels := map[string]string{
		"kubeslice.io/pod-type": "slicegateway",
		"kubeslice.io/slice-gw": sliceGWName,
	}
	listOptions := []client.ListOption{
		client.MatchingLabels(labels),
	}
	err := r.WorkerClient.List(ctx, &podsList, listOptions...)
	if err != nil {
		return false
	}
	if len(podsList.Items) == 2 {
		return true
	}
	return false
}

func (r *Reconciler) removeOldSecrets(ctx context.Context, rotationVersion int, sliceGwName string) error {
	log := logger.FromContext(ctx)
	currentSecretName := sliceGwName + "-" + strconv.Itoa(rotationVersion-1) // old secret
	log.Info("deleting previous certificates", "secretName", currentSecretName)
	sliceGwCerts := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      currentSecretName,
			Namespace: ControlPlaneNamespace,
		},
	}
	err := r.WorkerClient.Delete(ctx, sliceGwCerts)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Secret doesn't exist, no need to throw an error
			log.V(1).Info("Secret not found, skipping deletion")
			return nil
		}
		log.Error(err, "Error deleting old Gateway Secret while cleaning up")
		return err
	}

	return nil
}

func (r *Reconciler) InjectClient(c client.Client) error {
	r.Client = c
	return nil
}
