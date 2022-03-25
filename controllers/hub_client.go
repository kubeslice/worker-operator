package controllers

import (
	"context"

	meshv1beta1 "bitbucket.org/realtimeai/kubeslice-operator/api/v1beta1"
)

type HubClientProvider interface {
	UpdateAppPodsList(ctx context.Context, sliceConfigName string, appPods []meshv1beta1.AppPod) error
	UpdateNodePortForSliceGwServer(ctx context.Context, sliceGwNodePort int32, sliceGwName string) error
}
