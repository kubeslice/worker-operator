package kube

import (
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	once       sync.Once
	kubeClient *Client
)

// Client contains Kubernetes clients to call APIs
type Client struct {
	KubeCli      kubernetes.Interface
	DynamicCli   dynamic.Interface
	DiscoveryCli discovery.DiscoveryInterface
	Interval     time.Duration
	Timeout      time.Duration
}

// NewClient initializes and returns kubernetes client object
// we follow singleton pattern using sync.Once.Do , which ensures that only one client object is produced
func NewClient() (*Client, error) {
	kubeConfig, err := getConfig()
	if err != nil {
		return nil, err
	}
	if kubeClient == nil {
		once.Do(
			func() {
				kubeClient.DynamicCli, err = dynamic.NewForConfig(kubeConfig)
				kubeClient.DiscoveryCli, err = discovery.NewDiscoveryClientForConfig(kubeConfig)
				kubeClient.KubeCli, err = kubernetes.NewForConfig(kubeConfig)
				kubeClient.Interval = 500 * time.Millisecond
				kubeClient.Timeout = 60 * time.Second
			},
		)
	}
	return kubeClient, nil
}

// Returns client config for accessing kubernetes apikubernertes
// Check if kubeconfig file is present in home/.kube and use it
// Else use in-cluster config
func getConfig() (*rest.Config, error) {
	if home := homedir.HomeDir(); home != "" {
		kubeconfig := filepath.Join(home, ".kube", "config")
		if _, err := os.Stat(kubeconfig); os.IsNotExist(err) {
			return rest.InClusterConfig()
		}
		return clientcmd.BuildConfigFromFlags("", kubeconfig)
	} else {
		return rest.InClusterConfig()
	}
}
