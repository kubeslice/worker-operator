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
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
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

func getPrefixes(nsmconfig corev1.ConfigMap) ([]string, error) {
	var cmData map[string]interface{}
	err := yaml.Unmarshal([]byte(nsmconfig.Data["excluded_prefixes_output.yaml"]), &cmData)
	if err != nil {
		return nil, fmt.Errorf("yaml unmarshalling error: %+v ", err)
	}

	if value, ok := cmData["Prefixes"]; ok {
		prefixes := make([]string, len(value.([]interface{})))
		for i, v := range value.([]interface{}) {
			prefixes[i] = fmt.Sprint(v)
		}
		return prefixes, nil
	}

	return nil, fmt.Errorf("failed to read prefixes from configmap")
}

func (c *Cluster) GetNsmExcludedPrefix(ctx context.Context, configmap, namespace string) ([]string, error) {
	var nsmconfig corev1.ConfigMap
	var err error
	var prefixes []string
	err = c.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: configmap}, &nsmconfig)
	if err != nil {
		log.Error(err, "error getting nsm configmap")
		return nil, err
	}
	if len(nsmconfig.Data) == 0 {
		return nil, errors.New("prefix data not present in nsm configmap")
	}
	_, ok := nsmconfig.Data["excluded_prefixes_output.yaml"]
	if !ok {
		return nil, errors.New("cni subnet info not present in nsm configmap")
	}
	prefixes, err = getPrefixes(nsmconfig)
	if err != nil {
		return nil, errors.New("failed to get prefixes from nsm configmap")
	}
	if len(prefixes) == 0 {
		return nil, fmt.Errorf("error occured while getting excluded prefixes. err: %v, prefix len: %v", err, len(prefixes))
	}
	return prefixes, nil
}
