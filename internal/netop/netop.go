package netop

import (
	"context"

	sidecar "bitbucket.org/realtimeai/kubeslice-netops/pkg/proto"
	meshv1beta1 "bitbucket.org/realtimeai/kubeslice-operator/api/v1beta1"
	"google.golang.org/grpc"
)

// Generic event types enum
type EventType int32

const (
	EventType_EV_CREATE EventType = 0
	EventType_EV_UPDATE EventType = 1
	EventType_EV_DELETE EventType = 2
)

func UpdateSliceQosProfile(ctx context.Context, addr string, slice *meshv1beta1.Slice) error {
	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		return err
	}
	defer conn.Close()

	client := sidecar.NewNetOpsServiceClient(conn)

	// TODO change later if we add more TC types
	tcType := sidecar.TcType_BANDWIDTH_CONTROL

	qop := &sidecar.SliceQosProfile{
		SliceName:      slice.Name,
		SliceId:        slice.Status.SliceConfig.SliceID,
		QosProfileName: slice.Name,
		TcType:         tcType,
		ClassType:      sidecar.ClassType_HTB,
		BwCeiling:      uint32(slice.Status.SliceConfig.QosProfileDetails.BandwidthCeilingKbps),
		BwGuaranteed:   uint32(slice.Status.SliceConfig.QosProfileDetails.BandwidthGuaranteedKbps),
		Priority:       uint32(slice.Status.SliceConfig.QosProfileDetails.Priority),
		DscpClass:      slice.Status.SliceConfig.QosProfileDetails.DscpClass,
	}

	_, err = client.UpdateSliceQosProfile(ctx, qop)
	return err
}

func SendSliceLifeCycleEventToNetOp(ctx context.Context, addr string, sliceName string, eventType EventType) error {
	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		return err
	}
	defer conn.Close()

	client := sidecar.NewNetOpsServiceClient(conn)

	// Transform event type enum in this pacakge to event type in netops package
	et := map[EventType]sidecar.EventType{
		EventType_EV_CREATE: 0,
		EventType_EV_UPDATE: 1,
		EventType_EV_DELETE: 2,
	}[eventType]

	sliceEvent := &sidecar.SliceLifeCycleEvent{
		SliceName: sliceName,
		Event:     et,
	}

	_, err = client.UpdateSliceLifeCycleEvent(ctx, sliceEvent)
	return err
}
