package router

import (
	"context"

	sidecar "bitbucket.org/realtimeai/kubeslice-router-sidecar/pkg/proto"
	"google.golang.org/grpc"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

func GetClientConnectionInfo(ctx context.Context, addr string) (*sidecar.ClientConnectionInfo, error) {
	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	client := sidecar.NewSliceRouterSidecarServiceClient(conn)

	return client.GetSliceRouterClientConnectionInfo(ctx, &emptypb.Empty{})
}

func UpdateConnectionContext(ctx context.Context, addr string) (*sidecar.SidecarResponse, error) {
	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	// TODO fill
	conContext := &sidecar.SliceGwConContext{}

	client := sidecar.NewSliceRouterSidecarServiceClient(conn)

	return client.UpdateSliceGwConnectionContext(ctx, conContext)
}
