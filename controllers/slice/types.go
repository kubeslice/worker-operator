package slice

import (
	"context"

	kubeslicev1beta1 "github.com/kubeslice/operator/api/v1beta1"
)

// NetOpPod contains details of NetOp Pod running in the cluster
type NetOpPod struct {
	PodIP   string
	PodName string
	Node    string
}

type HubClientProvider interface {
	UpdateAppPodsList(ctx context.Context, sliceConfigName string, appPods []kubeslicev1beta1.AppPod) error
}
