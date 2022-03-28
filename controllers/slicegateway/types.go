package slicegateway

import (
	"context"
)

// NetOpPod contains details of NetOp Pod running in the cluster
type NetOpPod struct {
	PodIP   string
	PodName string
	Node    string
}

type HubClientProvider interface {
	UpdateNodePortForSliceGwServer(ctx context.Context, sliceGwNodePort int32, sliceGwName string) error
}
