package workerslicegwrecycler

import (
	"context"

	"github.com/kubeslice/worker-operator/pkg/gwsidecar"
	"github.com/kubeslice/worker-operator/pkg/router"
)

type WorkerGWSidecarClientProvider interface {
	GetStatus(ctx context.Context, serverAddr string) (*gwsidecar.GwStatus, error)
}
type WorkerRouterClientProvider interface {
	UpdateEcmpRoutes(ctx context.Context, serverAddr string, ecmpUpdateInfo *router.UpdateEcmpInfo) error
}
