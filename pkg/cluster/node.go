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
	"sync"

	"github.com/kubeslice/worker-operator/controllers"
	"github.com/kubeslice/worker-operator/pkg/logger"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var log = logger.NewWrappedLogger().WithValues("type", "hub")

var nodeInfo *NodeInfo

func init() {
	// Global variable to store the list of kubeslice gateway nodes IP's in the cluster
	nodeInfo = &NodeInfo{}
}

const (
	NodeExternalIP corev1.NodeAddressType = "ExternalIP"
	NodeInternalIP corev1.NodeAddressType = "InternalIP"
)

// Node info structure.
// Protected by a mutex and contains information about the kubeslice gateway nodes in the cluster.
type NodeInfo struct {
	Client     client.Client
	NodeIPList []string
	sync.Mutex
}

//GetNodeExternalIpList gets the list of External Node IPs of kubeslice-gateway nodes

func (n *NodeInfo) getNodeExternalIpList() ([]string, error) {
	n.Lock()
	defer n.Unlock()
	err := n.populateNodeIpList()
	if err != nil {
		return nil, err
	}
	return n.NodeIPList, nil
}

func (n *NodeInfo) populateNodeIpList() error {
	ctx := context.Background()
	nodeList := corev1.NodeList{}
	labels := map[string]string{controllers.NodeTypeSelectorLabelKey: "gateway"}
	listOptions := []client.ListOption{
		client.MatchingLabels(labels),
	}
	err := n.Client.List(ctx, &nodeList, listOptions...)
	if err != nil {
		return fmt.Errorf("can't fetch node list: %+v ", err)
	}

	if len(nodeList.Items) == 0 {
		return fmt.Errorf("can't fetch node list: %+v ", err)
	}

	nodeIpArr := []corev1.NodeAddress{}
	for i := 0; i < len(nodeList.Items); i++ {
		nodeIpArr = append(nodeIpArr, nodeList.Items[i].Status.Addresses...)
	}
	var cuurentNodeIps []string
	for i := 0; i < len(nodeIpArr); i++ {
		if nodeIpArr[i].Type == NodeExternalIP {
			cuurentNodeIps = append(cuurentNodeIps, nodeIpArr[i].Address)
		}
	}
	// if the external IPs are not available, we fetch Internal IPs
	if len(cuurentNodeIps) == 0 {
		for i := 0; i < len(nodeIpArr); i++ {
			if nodeIpArr[i].Type == NodeInternalIP {
				cuurentNodeIps = append(cuurentNodeIps, nodeIpArr[i].Address)
			}
		}
	}
	n.NodeIPList = cuurentNodeIps
	return err
}

func GetNodeIP(client client.Client) ([]string, error) {
	nodeInfo.Client = client
	// nodeIPs will either have list of ExternalIPs if available, else Internal IPs
	nodeIps, err := nodeInfo.getNodeExternalIpList()
	if err != nil || len(nodeIps) == 0 {
		log.Error(err, "Getting NodeIP From kube-api-server")
		return []string{""}, err
	}
	nodeIPs := nodeIps
	log.Info("nodeIPs selected", "nodeIP ", nodeIPs)
	return nodeIPs, err
}

func GetNodeExternalIpList() []string {
	nodeInfo.Lock()
	defer nodeInfo.Unlock()
	return nodeInfo.NodeIPList
}
