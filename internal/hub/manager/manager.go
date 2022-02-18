package manager

import (
	"context"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	log "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"bitbucket.org/realtimeai/kubeslice-operator/internal/hub/controllers"
	"bitbucket.org/realtimeai/kubeslice-operator/internal/logger"
	spokev1alpha1 "bitbucket.org/realtimeai/mesh-apis/pkg/mesh/v1alpha1"
)

var scheme = runtime.NewScheme()

func init() {
	log.SetLogger(logger.NewLogger())
	clientgoscheme.AddToScheme(scheme)
	utilruntime.Must(spokev1alpha1.AddToScheme(scheme))
}

func Start(meshClient client.Client, ctx context.Context) {

	config := &rest.Config{
		Host:            os.Getenv("HUB_HOST_ENDPOINT"),
		BearerTokenFile: HubTokenFile,
		TLSClientConfig: rest.TLSClientConfig{
			CAFile: HubCAFile,
		},
	}

	var log = log.Log.WithName("hub")

	mgr, err := manager.New(config, manager.Options{
		Host:               HubEndpoint,
		Namespace:          ProjectNamespace,
		Scheme:             scheme,
		MetricsBindAddress: "0", // disable metrics for now
	})
	if err != nil {
		log.Error(err, "Could not create manager")
		os.Exit(1)
	}

	sliceReconciler := &controllers.SliceReconciler{
		MeshClient: meshClient,
	}
	err = builder.
		ControllerManagedBy(mgr).
		For(&spokev1alpha1.Slice{}).
		Complete(sliceReconciler)
	if err != nil {
		log.Error(err, "could not create controller")
		os.Exit(1)
	}

	sliceGwReconciler := &controllers.SliceGwReconciler{
		MeshClient: meshClient,
	}
	err = builder.
		ControllerManagedBy(mgr).
		For(&spokev1alpha1.SliceGateway{}).
		Complete(sliceGwReconciler)
	if err != nil {
		log.Error(err, "could not create controller")
		os.Exit(1)
	}

	if err := mgr.Start(ctx); err != nil {
		log.Error(err, "could not start manager")
		os.Exit(1)
	}
}
