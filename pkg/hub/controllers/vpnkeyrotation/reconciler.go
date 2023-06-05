package vpnkeyrotation

import (
	"context"
	"errors"
	"strconv"
	"time"

	hubv1alpha1 "github.com/kubeslice/apis/pkg/controller/v1alpha1"
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"

	"github.com/kubeslice/worker-operator/controllers"

	"github.com/kubeslice/kubeslice-monitoring/pkg/events"
	"github.com/kubeslice/kubeslice-monitoring/pkg/metrics"

	hub "github.com/kubeslice/worker-operator/pkg/hub/hubclient"
	"github.com/kubeslice/worker-operator/pkg/logger"
	"github.com/kubeslice/worker-operator/pkg/slicegwrecycler"
	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var controllerName = "vpnKeyRotationController"

type Reconciler struct {
	client.Client
	MeshClient    client.Client
	HubClient     client.Client
	EventRecorder *events.EventRecorder

	// metrics
	gaugeClusterUp   *prometheus.GaugeVec
	gaugeComponentUp *prometheus.GaugeVec
}

func NewReconciler(mc client.Client, hc client.Client, er *events.EventRecorder, mf metrics.MetricsFactory) *Reconciler {
	gaugeClusterUp := mf.NewGauge("cluster_up", "Kubeslice cluster health status", []string{})
	gaugeComponentUp := mf.NewGauge("cluster_component_up", "Kubeslice cluster component health status", []string{"slice_cluster_component"})

	return &Reconciler{
		MeshClient:    mc,
		HubClient:     hc,
		EventRecorder: er,

		gaugeClusterUp:   gaugeClusterUp,
		gaugeComponentUp: gaugeComponentUp,
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
			log.Info("vpnkeyrotation  resource not found in hub. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	// contains all the gateways associated with the cluster for a particular slice
	allGwsUnderCluster := vpnKeyRotation.Spec.ClusterGatewayMapping[ClusterName]
	// for eg allGwsUnderCluster = []string{"fire-worker-2-worker-1", "fire-worker-2-worker-3", "fire-worker-2-worker-4"}

	if len(vpnKeyRotation.Status.CurrentRotationState) == 0 {
		// When vpnkeyrotation is initially created, update the LastUpdatedTimestamp for all gateways
		// as same as CertificateCreationTime for vpnKeyRotation cr. Then stop the rquest and return.
		m := make(map[string]hubv1alpha1.StatusOfKeyRotation)
		for _, selectedGw := range allGwsUnderCluster {
			m[selectedGw] = hubv1alpha1.StatusOfKeyRotation{
				Status:               hubv1alpha1.Complete,
				LastUpdatedTimestamp: vpnKeyRotation.Spec.CertificateCreationTime,
			}
		}
		vpnKeyRotation.Status.CurrentRotationState = m
		err = r.Status().Update(ctx, vpnKeyRotation)
		if err != nil {
			log.Error(err, "error while updating vpnKeyRotation status in the initial state")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

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
		rotationTimeDiffInDays := int(rotationTimeDiff.Hours() / 24)
		if rotationTimeDiffInDays >= vpnKeyRotation.Spec.RotationInterval {
			log.Info("Rotation interval has elapsed")
			// Unsure why we need to update this status; doesn't seem to serve any purpose
			if err := r.updateRotationStatus(ctx, selectedGw, hubv1alpha1.SecretReadInProgress, vpnKeyRotation); err != nil {
				return ctrl.Result{}, err
			}

			sliceGw := &kubeslicev1beta1.SliceGateway{}
			err = r.MeshClient.Get(ctx, types.NamespacedName{
				Name:      selectedGw,
				Namespace: ControlPlaneNamespace,
			}, sliceGw)

			result, updated, err := r.updateCertificates(ctx, vpnKeyRotation.Status.RotationCount, sliceGw, req)
			if err != nil {
				log.Error(err, "Failed to update certificates")
				return result, err
			}
			if updated {
				// If certificates are updated, proceed to update the status
				if err := r.updateRotationStatus(ctx, selectedGw, hubv1alpha1.SecretUpdated, vpnKeyRotation); err != nil {
					return ctrl.Result{}, err
				}
			}
			// Upon certificate update, if the selected gateway is a server, await the client pod to transition into the SECRET_UPDATED state
			if isServer(sliceGw) {
				clientGWName := sliceGw.Status.Config.SliceGatewayRemoteGatewayID
				clientGWRotation, ok := vpnKeyRotation.Status.CurrentRotationState[clientGWName]
				if !ok {
					// Waiting for the inclusion of the client status in the rotation status.
					return ctrl.Result{
						RequeueAfter: 5 * time.Second,
					}, nil
				}
				if clientGWRotation.Status != hubv1alpha1.SecretUpdated {
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
				slice, err := controllers.GetSlice(ctx, r.Client, sliceName)
				if err != nil {
					log.Error(err, "Failed to get Slice", "slice", sliceName)
					return ctrl.Result{}, err
				}
				for _, v := range podsUnderGw.Items {
					// trigger FSM to recylce both gateway pod pairs
					err = slicegwrecycler.TriggerFSM(ctx, sliceGw, slice, r.HubClient.(*hub.HubClientConfig), r.Client,
						&v, r.EventRecorder, controllerName)
					if err != nil {
						// TODO: should we requeue if any error in FSM or just update the rotation status with error?
						log.Error(err, "Error while recycling gateway pods")
						if err := r.updateRotationStatusWithTimeStamp(ctx, selectedGw, hubv1alpha1.Error, vpnKeyRotation, metav1.Time{Time: time.Now()}); err != nil {
							return ctrl.Result{}, nil
						}
						return ctrl.Result{}, nil
					}
				}
				if err := r.updateRotationStatusWithTimeStamp(ctx, selectedGw, hubv1alpha1.Complete, vpnKeyRotation, metav1.Time{Time: time.Now()}); err != nil {
					return ctrl.Result{}, err
				}
			}
		}
	}
	return ctrl.Result{}, nil
}

func (r *Reconciler) updateRotationStatus(ctx context.Context, gatewayName, rotationStatus string, vpnKeyRotation *hubv1alpha1.VpnKeyRotation) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if getErr := r.Get(ctx,
			types.NamespacedName{Name: vpnKeyRotation.Name, Namespace: controllers.ControlPlaneNamespace},
			vpnKeyRotation); getErr != nil {
			return getErr
		}
		vpnKeyRotation.Status.CurrentRotationState[gatewayName] = hubv1alpha1.StatusOfKeyRotation{
			Status: rotationStatus,
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
			types.NamespacedName{Name: vpnKeyRotation.Name, Namespace: controllers.ControlPlaneNamespace},
			vpnKeyRotation); getErr != nil {
			return getErr
		}
		vpnKeyRotation.Status.CurrentRotationState[gatewayName] = hubv1alpha1.StatusOfKeyRotation{
			Status:               rotationStatus,
			LastUpdatedTimestamp: timestamp,
		}
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

func (r *Reconciler) updateCertificates(ctx context.Context, rotationVersion int, sliceGw *kubeslicev1beta1.SliceGateway,
	req reconcile.Request) (ctrl.Result, bool, error) {
	log := logger.FromContext(ctx)
	currentSecretName := sliceGw.Name + strconv.Itoa(rotationVersion)
	meshSliceGwCerts := &corev1.Secret{}
	err := r.MeshClient.Get(ctx, types.NamespacedName{
		Name:      currentSecretName,
		Namespace: ControlPlaneNamespace,
	}, meshSliceGwCerts)
	if err != nil {
		if apierrors.IsNotFound(err) {
			sliceGwCerts := &corev1.Secret{}
			err := r.Get(ctx, req.NamespacedName, sliceGwCerts)
			if err != nil {
				log.Error(err, "unable to fetch slicegw certs from the hub", "sliceGw", sliceGw.Name)
				return ctrl.Result{
					RequeueAfter: 10 * time.Second,
				}, true, err
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
				log.Error(err, "unable to create secret to store slicegw certs in worker cluster", "sliceGw", sliceGw.Name)
				return ctrl.Result{}, true, err
			}
			log.Info("sliceGw secret created in worker cluster")
			// this required requeueing
			return ctrl.Result{}, true, nil
		} else {
			log.Error(err, "unable to fetch slicegw certs from the worker", "sliceGw", sliceGw.Name)
			return ctrl.Result{}, true, err
		}
	}
	// secret with current rotation version already exists, no requeue
	return ctrl.Result{}, false, nil
}

func (r *Reconciler) InjectClient(c client.Client) error {
	r.Client = c
	return nil
}
