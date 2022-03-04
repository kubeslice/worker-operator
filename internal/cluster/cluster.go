package cluster

import (
	"context"
	"fmt"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"strings"
	"time"
)

type Cluster struct {
	Client kubernetes.Interface
	Name   string `json:"clusterName,omitempty"`
}

//NewCluster returns ClusterInterface
func NewCluster(client kubernetes.Interface, clusterName string) ClusterInterface {
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
		return nil, err
	}
	cl.ClusterProperty = ClusterProperty{
		GeoLocation: location,
	}
	return cl, nil
}

func (c *Cluster) getClusterLocation(ctx context.Context) (GeoLocation, error) {
	var g GeoLocation

	nodeList, err := c.Client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
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

func (c *Cluster) GetNsmExcludedPrefix(ctx context.Context, configmap, namespace string) ([]string, error) {
	var nsmconfig *corev1.ConfigMap
	var err error

	// wait for 5 minuites and poll for every 1 second
	wait.Poll(10*time.Second, 5*time.Minute, func() (bool, error) {
		nsmconfig, err = c.Client.CoreV1().ConfigMaps(namespace).Get(ctx, configmap, metav1.GetOptions{})
		if err != nil {
			return false, fmt.Errorf("can't get configmap %s from namespace %s: %+v ", configmap, namespace, err)
		}
		return true, nil
	})
	if err != nil {
		return nil, fmt.Errorf("unkown error occured: %+v ", err)
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
