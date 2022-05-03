package hubclient

import (
	"context"

	kubeslicev1beta1 "github.com/kubeslice/operator/api/v1beta1"
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
	ctx context.Context, serviceexport *kubeslicev1beta1.ServiceExport) error {
	return nil
}

func (hubClientEmulator *HubClientEmulator) UpdateServiceExportEndpointForIngressGw(ctx context.Context,
	serviceexport *kubeslicev1beta1.ServiceExport, ep *kubeslicev1beta1.ServicePod) error {
	return nil
}

func (hubClientEmulator *HubClientEmulator) DeleteServiceExport(
	ctx context.Context, serviceexport *kubeslicev1beta1.ServiceExport) error {
	return nil
}
