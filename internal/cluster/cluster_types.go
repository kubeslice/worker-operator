/*
 *  Copyright (c) 2022 Avesha, Inc. All rights reserved.
 *
 *  SPDX-License-Identifier: Apache-2.0
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */

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
