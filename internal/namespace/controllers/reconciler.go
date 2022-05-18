package namespace

import (
	"context"

	"github.com/go-logr/logr"
	hub "github.com/kubeslice/worker-operator/internal/hub/hubclient"
	"github.com/kubeslice/worker-operator/internal/logger"
	"github.com/kubeslice/worker-operator/pkg/events"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
)

//+kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch;create;update;patch;delete

type Reconciler struct {
	client.Client
	EventRecorder *events.EventRecorder
	Scheme        *runtime.Scheme
	Log           logr.Logger
	Hubclient     *hub.HubClientConfig
}

var excludedNs = []string{"kube-system", "default", "kubeslice-system", "kube-node-lease",
	"kube-public", "istio-system"}

func (c *Reconciler) getSliceNameFromNs(ns string) (string, error) {
	namespace := corev1.Namespace{}
	err := c.Client.Get(context.Background(), types.NamespacedName{Name: ns}, &namespace)
	if err != nil {
		c.Log.Error(err, "error while retrieving namespace")
		return "", err
	}
	// TODO: label should not be hardcoded, it should come as a constant from slice reconciler pkg
	return namespace.Labels["kubeslice.io/slice"], nil
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	for _, v := range excludedNs {
		if v == req.Name {
			return ctrl.Result{}, nil
		}
	}
	log := r.Log.WithValues("namespaceReconciler", req.Name)
	ctx = logger.WithLogger(ctx, log)
	namespace := corev1.Namespace{}
	err := r.Get(ctx, req.NamespacedName, &namespace)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("Namespace deleted on worker cluster, updating the cluster CR on conrtoller cluster")
			err := hub.DeleteNamespaceInfoFromHub(ctx, r.Hubclient, req.Name)
			if err != nil {
				log.Error(err, "Failed to delete namespace on controller cluster")
				return ctrl.Result{}, err
			}
			// Return and don't requeue
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	//get the slice name from namespace label
	sliceName, err := r.getSliceNameFromNs(namespace.Name)
	if err != nil {
		log.Error(err, "error while retrieving labels from namespace")
		return ctrl.Result{}, err
	}
	err = hub.UpdateNamespaceInfoToHub(ctx, r.Hubclient, namespace.Name, sliceName)
	if err != nil {
		log.Error(err, "Failed to post namespace on controller cluster")
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up reconciler with manager
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Namespace{}).
		Complete(r)
}
