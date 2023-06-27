package vpnkeyrotation

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	hubv1alpha1 "github.com/kubeslice/apis/pkg/controller/v1alpha1"
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"

	"github.com/kubeslice/worker-operator/controllers"

	"github.com/kubeslice/kubeslice-monitoring/pkg/events"
	"github.com/kubeslice/kubeslice-monitoring/pkg/metrics"

	hub "github.com/kubeslice/worker-operator/pkg/hub/hubclient"
	"github.com/kubeslice/worker-operator/pkg/logger"
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
	gaugeClusterUp := mf.NewGauge("cluster_up", "Kubeslice cluster health status", []string{})
	gaugeComponentUp := mf.NewGauge("cluster_component_up", "Kubeslice cluster component health status", []string{"slice_cluster_component"})

	return &Reconciler{
		MeshClient:           mc,
		HubClient:            hc,
		EventRecorder:        er,
		gaugeClusterUp:       gaugeClusterUp,
		gaugeComponentUp:     gaugeComponentUp,
		WorkerRecyclerClient: workerRecyclerClient,
	}
}

func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (ctrl.Result, error) {
	log := logger.FromContext(ctx).WithName("vpnkeyrotation-reconciler")
	ctx = logger.WithLogger(ctx, log)
	vpnKeyRotation := &hubv1alpha1.VpnKeyRotation{}
	err := r.Get(ctx, req.NamespacedName, vpnKeyRotation)
	// Request object not found, could have been deleted after reconcile request.
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Return and don't requeue
			log.Info("vpnkeyrotation resource not found in hub. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// contains all the gateways associated with the cluster for a particular slice
	allGwsUnderCluster := vpnKeyRotation.Spec.ClusterGatewayMapping[os.Getenv("CLUSTER_NAME")]
	// for eg allGwsUnderCluster = []string{"fire-worker-2-worker-1", "fire-worker-2-worker-3", "fire-worker-2-worker-4"}
	log.V(3).Info("gateways under cluster", "allGwsUnderCluster", allGwsUnderCluster)
	if len(vpnKeyRotation.Status.CurrentRotationState) == 0 && len(allGwsUnderCluster) > 0 {
		// When vpnkeyrotation is initially created, update the LastUpdatedTimestamp for all gateways
		// as same as CertificateCreationTime for vpnKeyRotation cr. Then stop the rquest and return.
		log.V(3).Info("initiation of StatusOfKeyRotation with certification creation data")
		err = r.updateInitialRotationStatusForAllGws(ctx, vpnKeyRotation, allGwsUnderCluster)
		if err != nil {
			log.Error(err, "error while updating vpnKeyRotation status in the initial state")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}
	requeue, err := r.syncCurrentRotationState(ctx, vpnKeyRotation, allGwsUnderCluster)
	if requeue {
		return ctrl.Result{Requeue: true}, nil
	}
	if err != nil {
		log.Error(err, "error while updating vpnKeyRotation status while rotation status is out of sync")
		return ctrl.Result{}, err
	}

	currentDate := time.Now()
	certificationCreationDate := vpnKeyRotation.Spec.CertificateCreationTime
	if certificationCreationDate.Year() == currentDate.Year() &&
		certificationCreationDate.Month() == currentDate.Month() &&
		certificationCreationDate.Day() == currentDate.Day() {
		for _, selectedGw := range allGwsUnderCluster {
			// check for status read in progress
			if vpnKeyRotation.Status.CurrentRotationState[selectedGw].Status == hubv1alpha1.InProgress {
				recyclers, err := r.HubClient.(*hub.HubClientConfig).ListWorkerSliceGwRecycler(ctx, selectedGw)
				if err != nil {
					return ctrl.Result{}, err
				}
				if len(recyclers) > 0 {
					for _, v := range recyclers {
						if v.Spec.State == "Error" {
							log.V(3).Info("gateway recycler is in error state", "gateway", v.Name)
							if err := r.updateRotationStatusWithTimeStamp(ctx, selectedGw, hubv1alpha1.Error, vpnKeyRotation, metav1.Time{Time: time.Now()}); err != nil {
								return ctrl.Result{}, err
							}
							return ctrl.Result{}, nil
						} else {
							// This means that recycling is in progress.
							// We will queue the task again and recheck after a five-minute interval.
							return ctrl.Result{RequeueAfter: time.Minute * 5}, nil
						}
					}
				}
				// Update the rotation status to Complete with TimeStamp
				if err := r.updateRotationStatusWithTimeStamp(ctx, selectedGw, hubv1alpha1.Complete, vpnKeyRotation, metav1.Time{Time: time.Now()}); err != nil {
					return ctrl.Result{}, nil
				}
			}
		}
	}

	isUpdated := false
	for _, selectedGw := range allGwsUnderCluster {
		// We choose a gateway from the array and compare its LastUpdatedTimestamp with the CertificateCreationTime.
		// If the time difference matches, we process the request; otherwise, we move on to the next gateway in the array.
		rotationStatus, ok := vpnKeyRotation.Status.CurrentRotationState[selectedGw]
		if !ok {
			// this can occur when there is new cluster onboarded to the slice
			if err := r.updateRotationStatusWithTimeStamp(ctx, selectedGw, hubv1alpha1.SecretReadInProgress, vpnKeyRotation,
				vpnKeyRotation.Spec.CertificateCreationTime); err != nil {
				return ctrl.Result{}, err
			}
			// TODO: rethink about this - should we requeue or not?
			return ctrl.Result{}, nil
		}
		rotationTimeDiff := vpnKeyRotation.Spec.CertificateCreationTime.Time.Sub(rotationStatus.LastUpdatedTimestamp.Time)
		if rotationTimeDiff.Hours() > 0 {
			log.Info("Rotation interval has elapsed")
			// Unsure why we need to update this status; doesn't seem to serve any purpose
			if err := r.updateRotationStatus(ctx, selectedGw, hubv1alpha1.SecretReadInProgress, vpnKeyRotation); err != nil {
				return ctrl.Result{}, err
			}

			result, requeue, err := r.updateCertificates(ctx, vpnKeyRotation.Spec.RotationCount, selectedGw, req)
			if requeue {
				return ctrl.Result{Requeue: true}, nil
			}
			if err != nil {
				log.Error(err, "Failed to update certificates")
				return result, err
			}

			// If certificates are updated, proceed to update the status
			log.Info("Certificates are updated for gw", "gateway", selectedGw)
			if err := r.updateRotationStatus(ctx, selectedGw, hubv1alpha1.SecretUpdated, vpnKeyRotation); err != nil {
				return ctrl.Result{}, err
			}

			sliceGw := &kubeslicev1beta1.SliceGateway{}
			err = r.MeshClient.Get(ctx, types.NamespacedName{
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
					// Waiting for the inclusion of the client status in the rotation status.
					return ctrl.Result{
						RequeueAfter: 5 * time.Second,
					}, nil
				}
				if clientGWRotation.Status == hubv1alpha1.SecretReadInProgress {
					log.Info("secret has not been updated in the gateway client yet; requeuing..")
					return ctrl.Result{}, errors.New("client gateway is not is secretupdated state")
				}
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
				err := r.Client.List(ctx, &podsUnderGw, listOptions...)
				if err != nil {
					return ctrl.Result{}, err
				}
				for i, v := range podsUnderGw.Items {
					// trigger FSM to recylce both gateway pod pairs
					if err := r.updateRotationStatus(ctx, selectedGw, hubv1alpha1.InProgress, vpnKeyRotation); err != nil {
						return ctrl.Result{}, err
					}
					slice, err := controllers.GetSlice(ctx, r.Client, sliceName)
					if err != nil {
						log.Error(err, "Failed to get Slice", "slice", sliceName)
						return ctrl.Result{}, err
					}
					created, err := r.WorkerRecyclerClient.TriggerFSM(ctx, sliceGw, slice, r.HubClient.(*hub.HubClientConfig), r.Client,
						&v, r.EventRecorder, controllerName, slice.Name+"-"+fmt.Sprint(i))
					if err != nil {
						return ctrl.Result{}, err
					}
					if created {
						// if WorkerSliceGwRecycler is created even for one pod,
						// we mark isUpdated to true so we can requeue
						isUpdated = created
					}
				}
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

func (r *Reconciler) syncCurrentRotationState(ctx context.Context,
	vpnKeyRotation *hubv1alpha1.VpnKeyRotation, allGwsUnderCluster []string) (bool, error) {
	log := logger.FromContext(ctx)
	// cluster deregister handling
	requeue := false
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if getErr := r.Get(ctx,
			types.NamespacedName{Name: vpnKeyRotation.Name, Namespace: vpnKeyRotation.Namespace},
			vpnKeyRotation); getErr != nil {
			return getErr
		}
		syncedRotationState := make(map[string]hubv1alpha1.StatusOfKeyRotation)
		for _, elem := range allGwsUnderCluster {
			if obj, ok := vpnKeyRotation.Status.CurrentRotationState[elem]; ok {
				syncedRotationState[elem] = obj
			}
		}
		if len(syncedRotationState) != len(vpnKeyRotation.Status.CurrentRotationState) {
			log.V(3).Info("syncing current rotation state for the gateways",
				"from", vpnKeyRotation.Status.CurrentRotationState,
				"to", syncedRotationState)
			vpnKeyRotation.Status.CurrentRotationState = syncedRotationState
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
				LastUpdatedTimestamp: vpnKeyRotation.Spec.CertificateCreationTime,
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
	err := r.MeshClient.Get(ctx, types.NamespacedName{
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
				log.Error(err, "unable to fetch slicegw certs from the hub", "sliceGw", sliceGw)
				return ctrl.Result{
					RequeueAfter: 10 * time.Second,
				}, false, err
			}
			meshSliceGwCerts := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      currentSecretName,
					Namespace: ControlPlaneNamespace,
				},
				Data: sliceGwCerts.Data,
			}
			err = r.MeshClient.Create(ctx, meshSliceGwCerts)
			if err != nil {
				log.Error(err, "unable to create secret to store slicegw certs in worker cluster", "sliceGw", sliceGw)
				return ctrl.Result{}, false, err
			}
			log.Info("sliceGw secret created in worker cluster")
			// this required requeueing
			return ctrl.Result{}, true, nil
		} else {
			log.Error(err, "unable to fetch slicegw certs from the worker", "sliceGw", sliceGw)
			return ctrl.Result{}, false, err
		}
	}
	// secret with current rotation version already exists, no requeue
	return ctrl.Result{}, false, nil
}

func (r *Reconciler) InjectClient(c client.Client) error {
	r.Client = c
	return nil
}
