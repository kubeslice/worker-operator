package slice

import (
	"context"

	meshv1beta1 "bitbucket.org/realtimeai/kubeslice-operator/api/v1beta1"
)

// NetOpPod contains details of NetOp Pod running in the cluster
type NetOpPod struct {
	PodIP   string
	PodName string
	Node    string
}

type HubClientProvider interface {
	UpdateAppPodsList(ctx context.Context, sliceConfigName string, appPods []meshv1beta1.AppPod) error
}
