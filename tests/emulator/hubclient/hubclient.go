package hubclient

import (
	meshv1beta1 "bitbucket.org/realtimeai/kubeslice-operator/api/v1beta1"
	"context"
	"github.com/stretchr/testify/mock"
)

type HubClientEmulator struct {
	mock.Mock
}

func NewHubClientEmulator() (*HubClientEmulator, error) {
	return new(HubClientEmulator), nil
}

func (hubClientEmulator *HubClientEmulator) UpdateNodePortForSliceGwServer(
	ctx context.Context, sliceGwNodePort int32, sliceGwName string) error {
	return nil
}

func (hubClientEmulator *HubClientEmulator) UpdateServiceExport(
	ctx context.Context, serviceexport *meshv1beta1.ServiceExport) error {
	return nil
}

func (hubClientEmulator *HubClientEmulator) DeleteServiceExport(
	ctx context.Context, serviceexport *meshv1beta1.ServiceExport) error {
	return nil
}
