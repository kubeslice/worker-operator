package router

import (
	"context"

	meshv1beta1 "bitbucket.org/realtimeai/kubeslice-operator/api/v1beta1"
	sidecar "bitbucket.org/realtimeai/kubeslice-router-sidecar/pkg/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

type SliceRouterConnCtx struct {
	RemoteSliceGwNsmSubnet string
	LocalNsmGwPeerIP       string
}

func GetClientConnectionInfo(ctx context.Context, addr string) ([]meshv1beta1.AppPod, error) {
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
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

func SendConnectionContext(ctx context.Context, serverAddr string, sliceRouterConnCtx *SliceRouterConnCtx) error {
	conn, err := grpc.Dial(serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer conn.Close()

	msg := &sidecar.SliceGwConContext{
		RemoteSliceGwNsmSubnet: sliceRouterConnCtx.RemoteSliceGwNsmSubnet,
		LocalNsmGwPeerIP:       sliceRouterConnCtx.LocalNsmGwPeerIP,
	}

	client := sidecar.NewSliceRouterSidecarServiceClient(conn)

	_, err = client.UpdateSliceGwConnectionContext(ctx, msg)

	return err
}
