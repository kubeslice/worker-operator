package workerclient

import (
	"context"

	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	"github.com/kubeslice/worker-operator/controllers/slicegateway"
	"github.com/stretchr/testify/mock"
)

type SpokeClientEmulator struct {
	mock.Mock
}

func NewSpokeClientEmulator() (*SpokeClientEmulator, error) {
	return new(SpokeClientEmulator), nil
}

func (SpokeClientEmulator *SpokeClientEmulator) GetGwPodNameAndIP(ctx context.Context, sliceGw *kubeslicev1beta1.SliceGateway, r *slicegateway.SliceGwReconciler) (string, string) {
	return "test-slicegw", "10.0.0.1"

}
