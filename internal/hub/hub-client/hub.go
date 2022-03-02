package hub

import (
	"context"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	log "sigs.k8s.io/controller-runtime/pkg/log"

	"bitbucket.org/realtimeai/kubeslice-operator/internal/logger"
	spokev1alpha1 "bitbucket.org/realtimeai/mesh-apis/pkg/spoke/v1alpha1"
)

var scheme = runtime.NewScheme()
var hubClient client.Client

func init() {
	log.SetLogger(logger.NewLogger())
	clientgoscheme.AddToScheme(scheme)
	utilruntime.Must(spokev1alpha1.AddToScheme(scheme))
	hubClient, _ = client.New(&rest.Config{
		Host:            os.Getenv("HUB_HOST_ENDPOINT"),
		BearerTokenFile: HubTokenFile,
		TLSClientConfig: rest.TLSClientConfig{
			CAFile: HubCAFile,
		}},
		client.Options{
			Scheme: scheme,
		},
	)
}

func UpdateNodePortForSliceGwServer(ctx context.Context, sliceGwNodePort int32, sliceGwName string) error {
	sliceGw := &spokev1alpha1.SpokeSliceGateway{}
	err := hubClient.Get(ctx, types.NamespacedName{
		Name:      sliceGwName,
		Namespace: ProjectNamespace,
	}, sliceGw)
	if err != nil {
		return err
	}

	if sliceGw.Spec.LocalGatewayConfig.NodePort == int(sliceGwNodePort) {
		// No update needed
		return nil
	}

	sliceGw.Spec.LocalGatewayConfig.NodePort = int(sliceGwNodePort)

	return hubClient.Update(ctx, sliceGw)
}
