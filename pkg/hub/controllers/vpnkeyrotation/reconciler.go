package vpnkeyrotation

import (
	"context"
	"errors"
	"strconv"
	"time"

	hubv1alpha1 "github.com/kubeslice/apis/pkg/controller/v1alpha1"
	spokev1alpha1 "github.com/kubeslice/apis/pkg/worker/v1alpha1"
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"

	"github.com/kubeslice/kubeslice-monitoring/pkg/metrics"
	"github.com/kubeslice/worker-operator/pkg/events"
	"github.com/kubeslice/worker-operator/pkg/hub/controllers"
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

type Reconciler struct {
	client.Client
	MeshClient    client.Client
	HubClient     client.Client
	EventRecorder *events.EventRecorder

	// metrics
	gaugeClusterUp   *prometheus.GaugeVec
	gaugeComponentUp *prometheus.GaugeVec

	ReconcileInterval time.Duration
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

		ReconcileInterval: 120 * time.Second,
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

	for _, selectedGw := range allGwsUnderCluster {
		// We choose a gateway from the array and compare its LastUpdatedTimestamp with the CertificateCreationTime.
		// If the time difference matches, we process the request; otherwise, we move on to the next gateway in the array.
		rotationStatus, ok := vpnKeyRotation.Status.CurrentRotationState[selectedGw]
		if !ok {
			//key not present - initial state
		}
		rotationTimeDiff := vpnKeyRotation.Spec.CertificateCreationTime.Time.Sub(rotationStatus.LastUpdatedTimestamp.Time)
		rotationTimeDiffInDays := int(rotationTimeDiff.Hours() / 24)
		if rotationTimeDiffInDays >= vpnKeyRotation.Spec.RotationInterval {
			log.Info("Rotation interval has elapsed")
			// Unsure why we need to update this status; doesn't seem to serve any purpose
			if err := r.updateRotationStatus(ctx, selectedGw, hubv1alpha1.SecretReadInProgress, vpnKeyRotation); err != nil {
				return ctrl.Result{}, err
			}
			sliceGw := &spokev1alpha1.WorkerSliceGateway{}
			err = r.MeshClient.Get(ctx, types.NamespacedName{
				Name:      selectedGw,
				Namespace: ControlPlaneNamespace,
			}, sliceGw)

			meshSliceGw := &kubeslicev1beta1.SliceGateway{}
			err = r.MeshClient.Get(ctx, types.NamespacedName{
				Name:      selectedGw,
				Namespace: ControlPlaneNamespace,
			}, meshSliceGw)

			result, requeue, err := r.updateCertificates(ctx, vpnKeyRotation.Status.RotationCount, sliceGw, req)
			if err != nil {
				log.Error(err, "Failed to update certificates")
				return result, err
			}
			if requeue {
				// If certificates are updated, proceed to update the status and then requeue
				if err := r.updateRotationStatus(ctx, selectedGw, hubv1alpha1.SecretUpdated, vpnKeyRotation); err != nil {
					return ctrl.Result{}, err
				} else {
					// no requeue needed - test once
					return ctrl.Result{
						Requeue: true,
					}, nil
				}
			}
			// Upon certificate update, if the selected gateway is a server, await the client pod to transition into the SECRET_UPDATED state
			if isServer(meshSliceGw) {
				clientGWName := meshSliceGw.Status.Config.SliceGatewayRemoteGatewayID
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
				// do it for both the pods
				// this will create two workerslicegwrecyler crs
				PodList := corev1.PodList{}
				labels := map[string]string{"kubeslice.io/pod-type": "slicegateway", ApplicationNamespaceSelectorLabelKey: sliceGw.Spec.SliceName,
					"kubeslice.io/slice-gw": sliceGw.Name}
				listOptions := []client.ListOption{
					client.MatchingLabels(labels),
				}
				ctx := context.Background()
				err := r.Client.List(ctx, &PodList, listOptions...)
				if err != nil {
					return ctrl.Result{}, err
				}
				for _, v := range PodList.Items {
					slicegwrecycler.TriggerFSM(ctx, meshSliceGw, r.HubClient.(*hub.HubClientConfig), r.Client, &v)
				}
			}

			// update status & LastUpdatedTimestamp
			// requeue
		}
	}
	return ctrl.Result{RequeueAfter: r.ReconcileInterval}, nil
}

// func (r *Reconciler) triggerFSM(ctx context.Context, sliceGw *kubeslicev1beta1.SliceGateway) (ctrl.Result, error) {
// }

func (r *Reconciler) updateRotationStatus(ctx context.Context, gatewayName, rotationStatus string, vpnKeyRotation *hubv1alpha1.VpnKeyRotation) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if getErr := r.Get(ctx,
			types.NamespacedName{Name: vpnKeyRotation.Name, Namespace: controllers.ControlPlaneNamespace},
			vpnKeyRotation); getErr != nil {
			return getErr
		}
		vpnKeyRotation.Status.CurrentRotationState[gatewayName] = hubv1alpha1.StatusOfKeyRotation{
			Status:               rotationStatus,
			LastUpdatedTimestamp: metav1.Time{Time: time.Now()},
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

func (r *Reconciler) updateCertificates(ctx context.Context, rotationVersion int, sliceGw *spokev1alpha1.WorkerSliceGateway,
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
