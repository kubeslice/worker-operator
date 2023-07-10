package vpnkeyrotation

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"time"

	hubv1alpha1 "github.com/kubeslice/apis/pkg/controller/v1alpha1"
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"

	"github.com/kubeslice/worker-operator/controllers"

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
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func NewReconciler(mc client.Client, hc client.Client, er *events.EventRecorder, mf metrics.MetricsFactory, workerRecyclerClient WorkerRecyclerClientProvider) *Reconciler {
	return &Reconciler{
		WorkerClient:         mc,
		ControllerClient:     hc,
		EventRecorder:        er,
		WorkerRecyclerClient: workerRecyclerClient,
	}
}

func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (ctrl.Result, error) {
	log := logger.FromContext(ctx).WithName("vpnkeyrotation-reconciler")
	ctx = logger.WithLogger(ctx, log)
	vpnKeyRotation := &hubv1alpha1.VpnKeyRotation{}
	err := r.Get(ctx, req.NamespacedName, vpnKeyRotation)
	log.Info("got vpnkeyrotation from controller", "vpnKeyRotation", vpnKeyRotation)
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
	log.Info("gateways under cluster", "allGwsUnderCluster", allGwsUnderCluster)
	if len(vpnKeyRotation.Status.CurrentRotationState) == 0 && len(allGwsUnderCluster) > 0 {
		// When vpnkeyrotation is initially created, update the LastUpdatedTimestamp for all gateways
		// as same as CertificateCreationTime for vpnKeyRotation cr. Then stop the rquest and return.
		log.Info("initiation of StatusOfKeyRotation with certification creation data")
		err = r.updateInitialRotationStatusForAllGws(ctx, vpnKeyRotation, allGwsUnderCluster)
		if err != nil {
			log.Error(err, "error while updating vpnKeyRotation status in the initial state")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// handle cluster attach/detach cases
	requeue, err := r.syncCurrentRotationState(ctx, vpnKeyRotation, allGwsUnderCluster)
	if requeue {
		return ctrl.Result{Requeue: true}, nil
	}
	if err != nil {
		log.Error(err, "error while updating vpnKeyRotation status while rotation status is out of sync")
		return ctrl.Result{}, err
	}

	// Step 2: If VPN rotation is currently in progress, verify the status of workerslicegwrecyclers to determine
	// if the process has been completed - error or success
	currentDate := time.Now()
	// no hour check??
	certificationCreationDate := vpnKeyRotation.Spec.CertificateCreationTime
	if certificationCreationDate.Year() == currentDate.Year() &&
		certificationCreationDate.Month() == currentDate.Month() &&
		certificationCreationDate.Day() == currentDate.Day() {
		for _, selectedGw := range allGwsUnderCluster {
			gwName := selectedGw
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

				// Upon certificate update, if the selected gateway is a server, await the client pod to transition into the SECRET_UPDATED state
				// then trigger gateway recycle FSM at server side
				if !isServer(sliceGw) {
					gwName = sliceGw.Status.Config.SliceGatewayRemoteGatewayID
				}
				recyclers, err := r.ControllerClient.(*hub.HubClientConfig).ListWorkerSliceGwRecycler(ctx, gwName)
				if err != nil {
					return ctrl.Result{}, err
				}
				if len(recyclers) > 0 {
					for _, v := range recyclers {
						if v.Spec.State == "error" {
							log.Info("gateway recycler is in error state", "gateway", v.Name)
							utils.RecordEvent(ctx, r.EventRecorder, vpnKeyRotation, nil, ossEvents.EventGatewayRecyclingFailed, controllerName)
							if err := r.updateRotationStatusWithTimeStamp(ctx, selectedGw, hubv1alpha1.Error, vpnKeyRotation, metav1.Time{Time: time.Now()}); err != nil {
								return ctrl.Result{}, err
							}
							return ctrl.Result{}, nil
						} else {
							// This means that recycling is in progress.
							// We will queue the task again and recheck after a five-minute interval.
							log.Info("gateway recycler is in progress state", "gateway", v.Name)
							return ctrl.Result{RequeueAfter: time.Minute * 5}, nil
						}
					}
				}
				// Update the rotation status to Complete with TimeStamp
				utils.RecordEvent(ctx, r.EventRecorder, vpnKeyRotation, nil, ossEvents.EventGatewayRecyclingSuccessful, controllerName)
				if err := r.updateRotationStatusWithTimeStamp(ctx, selectedGw, hubv1alpha1.Complete, vpnKeyRotation, metav1.Time{Time: time.Now()}); err != nil {
					return ctrl.Result{}, nil
				}
				// clean up for deletion for old secrets
				if err = r.removeOldSecrets(ctx, vpnKeyRotation.Spec.RotationCount, selectedGw); err != nil {
					return ctrl.Result{}, err
				}
			}
		}
	}

	isUpdated := false
	// Step 3: We choose a gateway from the array and compare its LastUpdatedTimestamp with the CertificateCreationTime.
	// If the time difference matches, we process the request; otherwise, we move on to the next gateway in the array.
	for _, selectedGw := range allGwsUnderCluster {
		rotationStatus, ok := vpnKeyRotation.Status.CurrentRotationState[selectedGw]
		if !ok {
			// this can occur when there is new cluster onboarded to the slice
			// isn;t it is already handled by syncCurrentRotationState()?
			if err := r.updateRotationStatusWithTimeStamp(ctx, selectedGw, hubv1alpha1.Complete,
				vpnKeyRotation,
				*vpnKeyRotation.Spec.CertificateCreationTime); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{Requeue: true}, nil
		}
		rotationTimeDiff := vpnKeyRotation.Spec.CertificateCreationTime.Time.Sub(rotationStatus.LastUpdatedTimestamp.Time)
		if rotationTimeDiff.Hours() > 0 {
			log.Info("Rotation interval has elapsed", "rotationTimeDiff", rotationTimeDiff.Hours())
			utils.RecordEvent(ctx, r.EventRecorder, vpnKeyRotation, nil, ossEvents.EventGatewayCertificateRecyclingTriggered, controllerName)
			// Unsure why we need to update this status; doesn't seem to serve any purpose
			if err := r.updateRotationStatus(ctx, selectedGw, hubv1alpha1.SecretReadInProgress, vpnKeyRotation); err != nil {
				return ctrl.Result{}, err
			}

			result, requeue, err := r.updateCertificates(ctx, vpnKeyRotation.Spec.RotationCount, selectedGw, req)
			if requeue {
				return result, err
			}
			if err != nil {
				log.Error(err, "Failed to update certificates")
				return result, err
			}

			// If certificates are updated, proceed to update the status
			log.Info("Certificates are updated for gw", "gateway", selectedGw)
			utils.RecordEvent(ctx, r.EventRecorder, vpnKeyRotation, nil, ossEvents.EventGatewayCertificateUpdated, controllerName)
			if err := r.updateRotationStatus(ctx, selectedGw, hubv1alpha1.SecretUpdated, vpnKeyRotation); err != nil {
				return ctrl.Result{}, err
			}

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

			// Upon certificate update, if the selected gateway is a server, await the client pod to transition into the SECRET_UPDATED state
			// then trigger gateway recycle FSM at server side
			if isServer(sliceGw) {
				clientGWName := sliceGw.Status.Config.SliceGatewayRemoteGatewayID
				clientGWRotation, ok := vpnKeyRotation.Status.CurrentRotationState[clientGWName]
				if !ok {
					log.Info("Waiting for the inclusion of the client status in the rotation status")
					return ctrl.Result{
						RequeueAfter: 5 * time.Second,
					}, nil
				}
				if clientGWRotation.Status == hubv1alpha1.SecretReadInProgress {
					log.Info("secret has not been updated in the gateway client yet; requeuing..")
					return ctrl.Result{}, errors.New("client gateway is not is secret updated state, is in progress")
				}
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
						log.Error(err, "err")
						return ctrl.Result{}, err
					}
					if err := r.updateRotationStatus(ctx, selectedGw, hubv1alpha1.InProgress, vpnKeyRotation); err != nil {
						return ctrl.Result{}, err
					}
					if err := r.updateRotationStatus(ctx, clientGWName, hubv1alpha1.InProgress, vpnKeyRotation); err != nil {
						return ctrl.Result{}, err
					}
					for i, v := range podsUnderGw.Items {
						// trigger FSM to recylce both gateway pod pairs
						slice, err := controllers.GetSlice(ctx, r.WorkerClient, sliceName)
						if err != nil {
							log.Error(err, "Failed to get Slice", "slice", sliceName)
							return ctrl.Result{}, err
						}
						created, err := r.WorkerRecyclerClient.TriggerFSM(sliceGw, slice, &v, controllerName, selectedGw+"-"+fmt.Sprint(i), 2)
						if err != nil {
							return ctrl.Result{}, err
						}
						utils.RecordEvent(ctx, r.EventRecorder, vpnKeyRotation, nil, ossEvents.EventTriggeredFSMToRecycleGateways, controllerName)
						if created {
							// if WorkerSliceGwRecycler is created even for one pod,
							// we mark isUpdated to true so we can requeue
							isUpdated = created
						}
					}
				} else {
					return ctrl.Result{Requeue: true}, nil
				}
			} else { // requeue client to check the status of recycling after five minutes
				return ctrl.Result{
					RequeueAfter: 5 * time.Minute,
				}, nil
			}
		}
	}
	if isUpdated {
		// We are requeuing the task after 5 minutes because gateway recycling usually takes approximately 5 minutes.
		// After the recycling process, we will check the status of the recycling and update vpnkeyrotation.
		return ctrl.Result{
			RequeueAfter: 5 * time.Minute,
		}, nil
	}
	return ctrl.Result{}, nil
}

func (r *Reconciler) updateRotationStatus(ctx context.Context, gatewayName, rotationStatus string, vpnKeyRotation *hubv1alpha1.VpnKeyRotation) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if getErr := r.Get(ctx,
			types.NamespacedName{Name: vpnKeyRotation.Name, Namespace: vpnKeyRotation.Namespace},
			vpnKeyRotation); getErr != nil {
			return getErr
		}
		if len(vpnKeyRotation.Status.CurrentRotationState) == 0 {
			return errors.New("current state is empty")
		}
		vpnKeyRotation.Status.CurrentRotationState[gatewayName] = hubv1alpha1.StatusOfKeyRotation{
			Status:               rotationStatus,
			LastUpdatedTimestamp: vpnKeyRotation.Status.CurrentRotationState[gatewayName].LastUpdatedTimestamp,
		}
		return r.Status().Update(ctx, vpnKeyRotation)
	})
	if err != nil {
		return err
	}
	return nil
}

func (r *Reconciler) updateRotationStatusWithTimeStamp(ctx context.Context, gatewayName, rotationStatus string, vpnKeyRotation *hubv1alpha1.VpnKeyRotation,
	timestamp metav1.Time) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if getErr := r.Get(ctx,
			types.NamespacedName{Name: vpnKeyRotation.Name, Namespace: vpnKeyRotation.Namespace},
			vpnKeyRotation); getErr != nil {
			return getErr
		}
		rotation := hubv1alpha1.StatusOfKeyRotation{
			Status:               rotationStatus,
			LastUpdatedTimestamp: timestamp,
		}
		vpnKeyRotation.Status.CurrentRotationState[gatewayName] = rotation

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
		return r.Status().Update(ctx, vpnKeyRotation)
	})
	if err != nil {
		return err
	}
	return nil
}

func (r *Reconciler) syncCurrentRotationState(ctx context.Context, vpnKeyRotation *hubv1alpha1.VpnKeyRotation, allGwsUnderCluster []string) (bool, error) {
	log := logger.FromContext(ctx)

	// Create a map to store the synced rotation state
	syncedRotationState := make(map[string]hubv1alpha1.StatusOfKeyRotation)

	// Retrieve the current rotation state from vpnKeyRotation
	currentRotationState := vpnKeyRotation.Status.CurrentRotationState

	// Iterate through all gateways in allGwsUnderCluster
	for _, gw := range allGwsUnderCluster {
		// Check if the gateway exists in the current rotation state
		if obj, ok := currentRotationState[gw]; ok {
			syncedRotationState[gw] = obj
		} else {
			// If the gateway doesn't exist, create a new entry with Complete status and the certificate creation time
			syncedRotationState[gw] = hubv1alpha1.StatusOfKeyRotation{
				Status:               hubv1alpha1.Complete,
				LastUpdatedTimestamp: *vpnKeyRotation.Spec.CertificateCreationTime,
			}
		}
	}

	// Check if the synced rotation state is different from the current rotation state
	if !compareRotationStateMaps(syncedRotationState,currentRotationState) {
		log.Info("Syncing current rotation state for the gateways",
			"from", vpnKeyRotation.Status.CurrentRotationState,
			"to", syncedRotationState)

		// Update vpnKeyRotation with the synced rotation state
		vpnKeyRotation.Status.CurrentRotationState = syncedRotationState

		// Update the status of vpnKeyRotation
		err := r.Status().Update(ctx, vpnKeyRotation)
		if err != nil {
			return true, err
		}
		return true, nil
	}

	return false, nil
}

func compareRotationStateMaps(syncedRotationState, currentRotationState map[string]hubv1alpha1.StatusOfKeyRotation) bool {
	if len(syncedRotationState) != len(currentRotationState) {
		return false
	}

	for gw, obj := range syncedRotationState {
		currentObj, ok := currentRotationState[gw]
		if !ok || !reflect.DeepEqual(obj, currentObj) {
			return false
		}
	}

	return true
}


func (r *Reconciler) updateInitialRotationStatusForAllGws(ctx context.Context, vpnKeyRotation *hubv1alpha1.VpnKeyRotation,
	allGwsUnderCluster []string) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if getErr := r.Get(ctx,
			types.NamespacedName{Name: vpnKeyRotation.Name, Namespace: vpnKeyRotation.Namespace},
			vpnKeyRotation); getErr != nil {
			return getErr
		}
		m := make(map[string]hubv1alpha1.StatusOfKeyRotation)
		for _, selectedGw := range allGwsUnderCluster {
			m[selectedGw] = hubv1alpha1.StatusOfKeyRotation{
				Status:               hubv1alpha1.Complete,
				LastUpdatedTimestamp: *vpnKeyRotation.Spec.CertificateCreationTime,
			}
		}
		vpnKeyRotation.Status.CurrentRotationState = m
		return r.Status().Update(ctx, vpnKeyRotation)
	})
	if err != nil {
		return err
	}
	return nil
}

func isServer(sliceGw *kubeslicev1beta1.SliceGateway) bool {
	return sliceGw.Status.Config.SliceGatewayHostType == "Server"
}

func (r *Reconciler) updateCertificates(ctx context.Context, rotationVersion int, sliceGw string,
	req reconcile.Request) (ctrl.Result, bool, error) {
	log := logger.FromContext(ctx)
	currentSecretName := sliceGw + "-" + strconv.Itoa(rotationVersion)
	meshSliceGwCerts := &corev1.Secret{}
	err := r.WorkerClient.Get(ctx, types.NamespacedName{
		Name:      currentSecretName,
		Namespace: ControlPlaneNamespace,
	}, meshSliceGwCerts)
	if err != nil {
		if apierrors.IsNotFound(err) {
			sliceGwCerts := &corev1.Secret{}
			err = r.Get(ctx, types.NamespacedName{
				Name:      sliceGw,
				Namespace: req.Namespace,
			}, sliceGwCerts)
			if err != nil {
				// why requeue after 10 secs??
				log.Error(err, "unable to fetch slicegw certs from the hub", "sliceGw", sliceGw)
				return ctrl.Result{}, true, err
			}
			meshSliceGwCerts := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      currentSecretName,
					Namespace: ControlPlaneNamespace,
				},
				Data: sliceGwCerts.Data,
			}
			err = r.WorkerClient.Create(ctx, meshSliceGwCerts)
			if err != nil {
				log.Error(err, "unable to create secret to store slicegw certs in worker cluster", "sliceGw", sliceGw)
				return ctrl.Result{}, true, err
			}
			log.Info("sliceGw secret created in worker cluster")
			// this required requeueing
			return ctrl.Result{Requeue: true}, true, nil
		} else {
			log.Error(err, "unable to fetch slicegw certs from the worker", "sliceGw", sliceGw)
			return ctrl.Result{}, false, err
		}
	}
	// secret with current rotation version already exists, no requeue
	return ctrl.Result{}, false, nil
}

func (r *Reconciler) removeOldSecrets(ctx context.Context, rotationVersion int, sliceGwName string) error {
	log := logger.FromContext(ctx)
	currentSecretName := sliceGwName + "-" + strconv.Itoa(rotationVersion-1) // old secret
	sliceGwCerts := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      currentSecretName,
			Namespace: ControlPlaneNamespace,
		},
	}
	err := r.WorkerClient.Delete(ctx, sliceGwCerts)
	log.Info("deleting previous certificates", "secretName", currentSecretName)
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
