package router

import (
	"context"

	meshv1beta1 "bitbucket.org/realtimeai/kubeslice-operator/api/v1beta1"
	sidecar "bitbucket.org/realtimeai/kubeslice-router-sidecar/pkg/proto"
	"google.golang.org/grpc"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

func GetClientConnectionInfo(ctx context.Context, addr string) ([]meshv1beta1.AppPod, error) {
	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	client := sidecar.NewSliceRouterSidecarServiceClient(conn)

	info, err := client.GetSliceRouterClientConnectionInfo(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, err
	}

	var appPods []meshv1beta1.AppPod

	for _, c := range info.Connection {
		appPods = append(appPods, meshv1beta1.AppPod{
			PodName:      c.PodName,
			NsmInterface: c.NsmInterface,
			NsmIP:        c.NsmIP,
			NsmPeerIP:    c.NsmPeerIP,
		})
	}

	return appPods, nil
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
