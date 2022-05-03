package cluster

import (
	"context"
	"fmt"
	"github.com/kubeslice/operator/internal/logger"
	corev1 "k8s.io/api/core/v1"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sync"
)

var log = logger.NewLogger().WithValues("type", "hub")

const (
	NodeExternalIP corev1.NodeAddressType = "ExternalIP"
)

// Node info structure.
// Protected by a mutex and contains information about the kubeslice gateway nodes in the cluster.
type NodeInfo struct {
	Client     client.Client
	ExternalIP []string
	sync.Mutex
}

//GetNodeExternalIpList gets the list of External Node IPs of avesha-gateway nodes

func (n *NodeInfo) getNodeExternalIpList(client client.Client) ([]string, error) {
	// If node IP is set as an env variable, we use that as the only
	// node IP available to us. Early exit from here, and there is no need
	// spawn the node watcher thread.
	staticNodeIp := os.Getenv("NODE_IP")
	if staticNodeIp != "" {
		n.ExternalIP = append(n.ExternalIP, staticNodeIp)
		return n.ExternalIP, nil
	}
	// Dynamic node IP deduction if there is no static node IP provided
	n.Lock()
	defer n.Unlock()

	if len(n.ExternalIP) == 0 {
		err := n.populateNodeIpList()
		if err != nil {
			return nil, err
		}
	}
	return n.ExternalIP, nil
}

func (n *NodeInfo) populateNodeIpList() error {
	ctx := context.Background()
	nodeList := corev1.NodeList{}
	labels := map[string]string{"avesha/node-type": "gateway"}
	listOptions := []client.ListOption{
		client.MatchingLabels(labels),
	}
	err := n.Client.List(ctx, &nodeList, listOptions...)
	if err != nil {
		return fmt.Errorf("can't fetch node list: %+v ", err)
	}

	nodeIpArr := []corev1.NodeAddress{}
	for i := 0; i < len(nodeList.Items); i++ {
		nodeIpArr = append(nodeIpArr, nodeList.Items[i].Status.Addresses...)
	}
	for i := 0; i < len(nodeIpArr); i++ {
		if nodeIpArr[i].Type == NodeExternalIP {
			n.ExternalIP = append(n.ExternalIP, nodeIpArr[i].Address)
		}
	}
	return err
}

func GetNodeIP(client client.Client) (string, error) {
	nodeInfo := &NodeInfo{
		Client: client,
	}
	nodeIPs, err := nodeInfo.getNodeExternalIpList(client)
	if err != nil {
		log.Error(err, "Getting NodeIP From kube-api-server")
		os.Exit(1)
	}
	nodeIP := nodeIPs[0]
	log.Info("nodeIP selected", "nodeIP ", nodeIP)
	return nodeIP, err
}
