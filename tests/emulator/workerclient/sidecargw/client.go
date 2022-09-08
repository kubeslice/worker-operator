package sidecargw

import (
	"context"

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
	gwStatus := &gwsidecar.GwStatus{
		NsmStatus: gwsidecar.NsmStatus{
			LocalIP: "3.3.3.3",
		}}
	return gwStatus, nil
}

func (ClientEmulator *ClientEmulator) SendConnectionContext(ctx context.Context, serverAddr string, gwConnCtx *gwsidecar.GwConnectionContext) error {
	return nil
}
