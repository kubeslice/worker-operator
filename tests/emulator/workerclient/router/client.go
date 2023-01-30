package sidecargw

import (
	"context"

	sidecar "github.com/kubeslice/router-sidecar/pkg/sidecar/sidecarpb"
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"

	"github.com/kubeslice/worker-operator/pkg/router"
	"github.com/stretchr/testify/mock"
)

type ClientEmulator struct {
	mock.Mock
}

func NewClientEmulator() (*ClientEmulator, error) {
	return new(ClientEmulator), nil
}

func (ClientEmulator *ClientEmulator) GetClientConnectionInfo(ctx context.Context, addr string) ([]kubeslicev1beta1.AppPod, error) {
	appPods := []kubeslicev1beta1.AppPod{}
	return appPods, nil
}

func (ClientEmulator *ClientEmulator) SendConnectionContext(ctx context.Context, serverAddr string, sliceRouterConnCtx *router.SliceRouterConnCtx) error {
	return nil
}

func (ClientEmulator *ClientEmulator) UpdateEcmpRoutes(ctx context.Context, serverAddr string, ecmpInfo *router.UpdateEcmpInfo) error {
	return nil
}

func (ClientEmulator *ClientEmulator) GetRouteInKernel(ctx context.Context, serverAddr string, sliceRouterConnCtx *router.GetRouteConfig) (*sidecar.VerifyRouteAddResponse, error) {
	return &sidecar.VerifyRouteAddResponse{
		IsRoutePresent: true,
	}, nil
}
