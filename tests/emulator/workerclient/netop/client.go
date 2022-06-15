package sidecargw

import (
	"context"

	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"

	"github.com/kubeslice/worker-operator/pkg/netop"
	"github.com/stretchr/testify/mock"
)

type ClientEmulator struct {
	mock.Mock
}

func NewClientEmulator() (*ClientEmulator, error) {
	return new(ClientEmulator), nil
}

func (ClientEmulator *ClientEmulator) UpdateSliceQosProfile(ctx context.Context, addr string, slice *kubeslicev1beta1.Slice) error {
	return nil
}

func (ClientEmulator *ClientEmulator) SendSliceLifeCycleEventToNetOp(ctx context.Context, addr string, sliceName string, eventType netop.EventType) error {
	return nil
}

func (ClientEmulator *ClientEmulator) SendConnectionContext(ctx context.Context, serverAddr string, gw *kubeslicev1beta1.SliceGateway, sliceGwNodePort int32) error {
	return nil
}
