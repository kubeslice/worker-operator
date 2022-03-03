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
	"time"
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
func NewClient() (*Client, error) {
	kubeConfig, err := getConfig()
	if err != nil {
		return nil, err
	}
	dynamicClient, err := dynamic.NewForConfig(kubeConfig)
	disClient, err := discovery.NewDiscoveryClientForConfig(kubeConfig)
	if err != nil {
		return nil, err
	}
	kubeClient, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return nil, err
	}
	return &Client{
		KubeCli:      kubeClient,
		DiscoveryCli: disClient,
		DynamicCli:   dynamicClient,
		Interval:     500 * time.Millisecond,
		Timeout:      60 * time.Second,
	}, nil
}

// Returns client config for accessing kubernetes apikubernertes
// Check if kubeconfig file is present in home/.kube and use it
// Else use in-cluster config
// TODO add logs
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
