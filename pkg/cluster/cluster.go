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
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Cluster struct {
	Client client.Client
	Name   string `json:"clusterName,omitempty"`
}

// NewCluster returns ClusterInterface
func NewCluster(client client.Client, clusterName string) ClusterInterface {
	return &Cluster{
		Client: client,
		Name:   clusterName,
	}
}

func (c *Cluster) GetClusterInfo(ctx context.Context) (*ClusterInfo, error) {
	cl := &ClusterInfo{
		Name: c.Name,
	}
	location, err := c.getClusterLocation(ctx)
	if err != nil {
		log.Error(err, "Error Getting Cluster Location")
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
		log.Error(err, "Can't fetch node List")
		return g, fmt.Errorf("can't fetch node list: %+v ", err)
	}

	if len(nodeList.Items) == 0 {
		return g, fmt.Errorf("can't fetch node list , length of node items is zero")
	}

	g.CloudRegion = nodeList.Items[0].ObjectMeta.Labels["topology.kubernetes.io/region"]

	if nodeList.Items[0].Spec.ProviderID != "" {
		g.CloudProvider = strings.Split(nodeList.Items[0].Spec.ProviderID, ":")[0]
		//change gce to gcp
		if g.CloudProvider == "gce" {
			g.CloudProvider = "gcp"
		}
	}
	return g, nil
}

func (c *Cluster) GetNsmExcludedPrefix(ctx context.Context, configmap, namespace string) ([]string, error) {
	var nsmconfig corev1.ConfigMap
	var err error
	// wait for 5 minuites and poll for every 10 second
	wait.Poll(10*time.Second, 5*time.Minute, func() (bool, error) {
		err = c.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: configmap}, &nsmconfig)
		if err != nil {
			return false, fmt.Errorf("can't get configmap %s from namespace %s: %+v ", configmap, namespace, err)
		}
		return true, nil
	})
	if len(nsmconfig.Data) == 0 {
		return nil, fmt.Errorf("CNI Subnet not present")
	}

	var cmData map[string]interface{}
	err = yaml.Unmarshal([]byte(nsmconfig.Data["excluded_prefixes.yaml"]), &cmData)
	if err != nil {
		return nil, fmt.Errorf("yaml unmarshalling error: %+v ", err)
	}

	if value, ok := cmData["prefixes"]; ok {
		prefixes := make([]string, len(value.([]interface{})))
		for i, v := range value.([]interface{}) {
			prefixes[i] = fmt.Sprint(v)
		}
		return prefixes, nil
	}
	return nil, fmt.Errorf("error occured while getting excluded prefixes")
}
