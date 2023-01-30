package sidecargw

import (
	"context"

	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	"github.com/kubeslice/worker-operator/pkg/gwsidecar"
	"github.com/stretchr/testify/mock"
)

type ClientEmulator struct {
	mock.Mock
}

func NewClientEmulator() (*ClientEmulator, error) {
	return new(ClientEmulator), nil
}

func (ClientEmulator *ClientEmulator) GetStatus(
	ctx context.Context, serverAddr string) (*gwsidecar.GwStatus, error) {
	gwStatus := &gwsidecar.GwStatus{}
	return gwStatus, nil
}

func (ClientEmulator *ClientEmulator) SendConnectionContext(ctx context.Context, serverAddr string, gwConnCtx *gwsidecar.GwConnectionContext) error {
	return nil
}

func (ClientEmulator *ClientEmulator) UpdateSliceQosProfile(ctx context.Context, serverAddr string, slice *kubeslicev1beta1.Slice) error {
	return nil
}
func (ClientEmulator *ClientEmulator) GetSliceGwRemotePodName(ctx context.Context, gwRemoteVpnIP string, serverAddr string) (string, error) {
	return "slicegw-blue-cluster-1-cluster2", nil
}
