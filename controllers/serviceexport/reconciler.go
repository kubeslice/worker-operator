package serviceexport

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	meshv1beta1 "bitbucket.org/realtimeai/kubeslice-operator/api/v1beta1"
	//	"bitbucket.org/realtimeai/avesha-slice-operator/controllers"
	//	"bitbucket.org/realtimeai/avesha-slice-operator/internal/metrics"
	hub "bitbucket.org/realtimeai/kubeslice-operator/internal/hub/hub-client"
	"bitbucket.org/realtimeai/kubeslice-operator/internal/logger"
	//	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// Reconciler reconciles serviceexport resource
type Reconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=mesh.avesha.io,resources=serviceexports,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=mesh.avesha.io,resources=serviceexports/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=mesh.avesha.io,resources=serviceexports/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch

// Reconcile reconciles serviceexport
func (r Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("serviceexport", req.NamespacedName)

	serviceexport := &meshv1beta1.ServiceExport{}
	err := r.Get(ctx, req.NamespacedName, serviceexport)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Return and don't requeue
			log.Info("serviceexport resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get serviceexport")
		return ctrl.Result{}, err
	}

	log = log.WithValues("slice", serviceexport.Spec.Slice)
	debugLog := log.V(1)
	ctx = logger.WithLogger(ctx, log)

	log.Info("reconciling", "serviceexport", serviceexport.Name)

	// Reconciler running for the first time. Set the initial status here
	if serviceexport.Status.ExportStatus == meshv1beta1.ExportStatusInitial {
		serviceexport.Status.DNSName = serviceexport.Name + "." + serviceexport.Namespace + ".svc.slice.local"
		serviceexport.Status.ExportStatus = meshv1beta1.ExportStatusPending
		serviceexport.Status.ExposedPorts = portListToDisplayString(serviceexport.Spec.Ports)
		err = r.Status().Update(ctx, serviceexport)
		if err != nil {
			log.Error(err, "Failed to update serviceexport initial status")
			return ctrl.Result{}, err
		}
		log.Info("serviceexport updated with initial status")
		return ctrl.Result{}, nil
	}

	if serviceexport.Status.ExposedPorts != portListToDisplayString(serviceexport.Spec.Ports) {
		serviceexport.Status.ExposedPorts = portListToDisplayString(serviceexport.Spec.Ports)
		err = r.Status().Update(ctx, serviceexport)
		if err != nil {
			log.Error(err, "Failed to update serviceexport ports")
			return ctrl.Result{}, err
		}

		log.Info("serviceexport updated with ports")

		return ctrl.Result{
			RequeueAfter: 30 * time.Second,
		}, nil
	}

	res, err, requeue := r.ReconcileAppPod(ctx, serviceexport)
	if requeue {
		log.Info("app pods reconciled")
		debugLog.Info("requeuing after app pod reconcile", "res", res, "er", err)
		return res, err
	}

	if serviceexport.Status.AvailableEndpoints != len(serviceexport.Status.Pods) {
		serviceexport.Status.AvailableEndpoints = len(serviceexport.Status.Pods)
		err = r.Status().Update(ctx, serviceexport)
		if err != nil {
			log.Error(err, "Failed to update serviceexport availableendpoints")
			return ctrl.Result{}, err
		}
		log.Info("serviceexport updated with availableendpoints")
		return ctrl.Result{Requeue: true}, nil
	}

	if serviceexport.Status.LastSync == 0 {
		err = hub.UpdateServiceExport(ctx, serviceexport)
		if err != nil {
			log.Error(err, "Failed to post serviceexport")
			serviceexport.Status.ExportStatus = meshv1beta1.ExportStatusError
			r.Status().Update(ctx, serviceexport)
			return ctrl.Result{}, err
		}
		log.Info("serviceexport sync success")
		currentTime := time.Now().Unix()
		serviceexport.Status.LastSync = currentTime
		err = r.Status().Update(ctx, serviceexport)
		if err != nil {
			log.Error(err, "Failed to update serviceexport sync time")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Set export status to ready when reconciliation is complete
	if serviceexport.Status.ExportStatus != meshv1beta1.ExportStatusReady {
		serviceexport.Status.ExportStatus = meshv1beta1.ExportStatusReady
		err = r.Status().Update(ctx, serviceexport)
		if err != nil {
			log.Error(err, "Failed to update serviceexport export status")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{
		RequeueAfter: 30 * time.Second,
	}, nil
}

// SetupWithManager setus up reconciler with manager
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&meshv1beta1.ServiceExport{}).
		Complete(r)
}
