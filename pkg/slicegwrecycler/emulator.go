package slicegwrecycler

import (
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	_ "github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type VPNClientEmulator struct {
	client.Client
}

func NewVPNClientEmulator(client client.Client) (*VPNClientEmulator, error) {
	return &VPNClientEmulator{
		Client: client,
	}, nil
}

func (c VPNClientEmulator) TriggerFSM(sliceGw *kubeslicev1beta1.SliceGateway,
	slice *kubeslicev1beta1.Slice, gatewayPod *corev1.Pod, controllerName, gwRecyclerName string, numberOfGwSvc int) error {
	return nil
}
