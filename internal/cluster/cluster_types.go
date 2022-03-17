package cluster

import (
	"context"
)

type ClusterInterface interface {
	GetClusterInfo(ctx context.Context) (*ClusterInfo, error)
	GetNsmExcludedPrefix(ctx context.Context, configmap, namespace string) ([]string, error)
}

type ClusterInfo struct {
	Name            string          `json:"clusterName,omitempty"`
	ClusterProperty ClusterProperty `json:"clusterProperty,omitempty"`
}

type ClusterProperty struct {
	//GeoLocation contains information regarding Geographical Location of the Cluster
	GeoLocation GeoLocation `json:"geoLocation,omitempty"`
}

// GeoLocation defines the field of ClusterSpec
type GeoLocation struct {
	//CloudProvider is the cloud service provider
	CloudProvider string `json:"cloudProvider,omitempty"`
	//CloudRegion is the region of the cloud
	CloudRegion string `json:"cloudRegion,omitempty"`
}
