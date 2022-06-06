package sidecargw

import (
	"context"

	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"

	"github.com/kubeslice/worker-operator/internal/router"
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
