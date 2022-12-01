package workerslicegwrecycler

import (
	"context"

	sidecar "github.com/kubeslice/router-sidecar/pkg/sidecar/sidecarpb"
	"github.com/kubeslice/worker-operator/pkg/gwsidecar"
	"github.com/kubeslice/worker-operator/pkg/router"
)

type WorkerGWSidecarClientProvider interface {
	GetStatus(ctx context.Context, serverAddr string) (*gwsidecar.GwStatus, error)
}
type WorkerRouterClientProvider interface {
	UpdateEcmpRoutes(ctx context.Context, serverAddr string, ecmpUpdateInfo *router.UpdateEcmpInfo) error
	GetRouteInKernel(ctx context.Context, serverAddr string, sliceRouterConnCtx *router.GetRouteConfig) (*sidecar.VerifyRouteAddResponse, error)
}
