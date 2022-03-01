package cluster

import (
	"context"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
)

type Cluster struct {
	Client client.Client
	Name   string `json:"clusterName,omitempty"`
}

func (c *Cluster) GetClusterInfo(ctx context.Context) (*ClusterInfo, error) {
	cl := &ClusterInfo{
		Name: c.Name,
	}
	location, err := c.getClusterLocation(ctx)
	if err != nil {
		return nil, err
	}
	cl.ClusterProperty = ClusterProperty{
		GeoLocation: location,
	}
	return cl, nil
}

func (c *Cluster) getClusterLocation(ctx context.Context) (GeoLocation, error) {
	var g GeoLocation

	nodeList := corev1.NodeList{}
	err := c.Client.List(ctx, &nodeList)
	if err != nil {
		return g, fmt.Errorf("can't fetch node list: %+v ", err)
	}

	if len(nodeList.Items) == 0 {
		return g, fmt.Errorf("can't fetch node list , length of node items is zero")
	}

	g.CloudRegion = nodeList.Items[0].ObjectMeta.Labels["topology.kubernetes.io/region"]

	if nodeList.Items[0].Spec.ProviderID != "" {
		g.CloudProvider = strings.Split(nodeList.Items[0].Spec.ProviderID, ":")[0]
	}

	return g, nil
}
