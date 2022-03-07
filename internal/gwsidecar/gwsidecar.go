package gwsidecar

import (
	"context"

	sidecar "bitbucket.org/realtimeai/kubeslice-gw-sidecar/pkg/sidecar/sidecarpb"
	empty "github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"
)

type NsmStatus struct {
	IntfName string
	LocalIP  string
}

type TunnelStatus struct {
	IntfName string
	LocalIP  string
	RemoteIP string
	Latency  uint64
	TxRate   uint64
	RxRate   uint64
}

type GwStatus struct {
	NsmStatus
	TunnelStatus
}

type GwConnectionContext struct {
	RemoteSliceGwVpnIP     string
	RemoteSliceGwNsmSubnet string
}

// GetStatus retrieves sidecar status
func GetStatus(ctx context.Context, serverAddr string) (*GwStatus, error) {
	conn, err := grpc.Dial(serverAddr, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	client := sidecar.NewGwSidecarServiceClient(conn)
	res, err := client.GetStatus(ctx, &empty.Empty{})
	if err != nil {
		return nil, err
	}

	gwStatus := &GwStatus{}

	if res.NsmIntfStatus != nil {
		gwStatus.NsmStatus = NsmStatus{
			IntfName: res.NsmIntfStatus.NsmInterfaceName,
			LocalIP:  res.NsmIntfStatus.NsmIP,
		}
	}
	if res.TunnelStatus != nil {
		gwStatus.TunnelStatus = TunnelStatus{
			IntfName: res.TunnelStatus.NetInterface,
			LocalIP:  res.TunnelStatus.LocalIP,
			RemoteIP: res.TunnelStatus.PeerIP,
		}
	}

	return gwStatus, err
}

// SendConnectionContext sends connection context info to sidecar
func SendConnectionContext(ctx context.Context, serverAddr string, gwConnCtx *GwConnectionContext) error {
	conn, err := grpc.Dial(serverAddr, grpc.WithInsecure())
	if err != nil {
		return err
	}
	defer conn.Close()

	client := sidecar.NewGwSidecarServiceClient(conn)

	msg := &sidecar.SliceGwConnectionContext{
		RemoteSliceGwVpnIP:     gwConnCtx.RemoteSliceGwVpnIP,
		RemoteSliceGwNsmSubnet: gwConnCtx.RemoteSliceGwNsmSubnet,
	}

	_, err = client.UpdateConnectionContext(ctx, msg)

	return err
}
