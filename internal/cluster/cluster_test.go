package cluster

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

func TestGetExcludedNsmIps(t *testing.T) {
	var tests = []struct {
		description string
		configMap   string
		namespace   string
		expected    []string
		objs        []runtime.Object
	}{
		{"filter namespace", "nsm-config", "demo", []string{"192.168.0.0/16", "10.96.0.0/12"}, []runtime.Object{configMap("nsm-config", "demo", `
prefixes:
- 192.168.0.0/16
- 10.96.0.0/12		
`)}},
		{"wrong namespace", "nsm-config", "wrong-ns", nil, []runtime.Object{configMap("nsm-config", "correct-ns", `
prefixes:
- 192.168.0.0/16
- 10.96.0.0/12		
`)}},
		{"single prefix data", "nsm-config", "demo", []string{"192.168.0.0/16"}, []runtime.Object{configMap("nsm-config", "demo", `
prefixes:
- 192.168.0.0/16		
`)}},

		{"no data", "nsm-config", "demo", nil, []runtime.Object{configMap("nsm-config", "demo", "")}},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			client := fake.NewSimpleClientset(test.objs...)
			cluster := NewCluster(client, "fakeCluster")
			actual, err := cluster.GetNsmExcludedPrefix(context.Background(), test.configMap, test.namespace)
			t.Log(actual)
			if err != nil {
				t.Logf("Unexpected error: %s", err)
			}
			if diff := cmp.Diff(actual, test.expected); diff != "" {
				t.Errorf("%T differ (-got, +want): %s", test.expected, diff)
				return
			}
		})
	}
}
func TestCluster_GetClusterInfo(t *testing.T) {
	var tests = []struct {
		description string
		objs        []runtime.Object
		expected    *ClusterInfo
	}{
		{
			description: "GKE Cluster Node",
			objs:        []runtime.Object{getNode("gke-rahul-3-rahul-3-main-pool-ace60e8a-ds90")},
			expected: &ClusterInfo{
				Name: "fakeCluster",
				ClusterProperty: ClusterProperty{
					GeoLocation: GeoLocation{
						CloudProvider: "gce",
						CloudRegion:   "us-west1",
					},
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			client := fake.NewSimpleClientset(test.objs...)
			cluster := NewCluster(client, "fakeCluster")
			actual, err := cluster.GetClusterInfo(context.Background())
			t.Log(actual)
			if err != nil {
				t.Logf("Unexpected error: %s", err)
			}
			if diff := cmp.Diff(actual, test.expected); diff != "" {
				t.Errorf("%T differ (-got, +want): %s", test.expected, diff)
				return
			}
		})
	}
}

func configMap(name, namespace, data string) *v1.ConfigMap {
	configMapData := make(map[string]string)
	configMapData["excluded_prefixes.yaml"] = data
	configMap := v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: configMapData,
	}
	return &configMap
}
func getNode(name string) *v1.Node {
	node := v1.Node{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Node",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"topology.kubernetes.io/region": "us-west1",
			},
		},
		Spec: v1.NodeSpec{
			ProviderID: "gce://avesha-dev/us-west1-b/gke-rahul-3-rahul-3-main-pool-ace60e8a-ds90",
		},
	}
	return &node
}
