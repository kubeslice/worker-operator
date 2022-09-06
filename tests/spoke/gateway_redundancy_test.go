package spoke_test

import (
	"context"
	"fmt"
	"os"
	"time"

	hubv1alpha1 "github.com/kubeslice/apis/pkg/controller/v1alpha1"
	workerv1alpha1 "github.com/kubeslice/apis/pkg/worker/v1alpha1"
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	clusterpkg "github.com/kubeslice/worker-operator/pkg/cluster"
	hub "github.com/kubeslice/worker-operator/pkg/hub/hubclient"
	nsmv1alpha1 "github.com/networkservicemesh/networkservicemesh/k8s/pkg/apis/networkservice/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = FDescribe("Gateway redundancy test scenario", func() {
	var sliceGw *kubeslicev1beta1.SliceGateway
	var createdSliceGw *kubeslicev1beta1.SliceGateway
	var cluster *hubv1alpha1.Cluster
	var slice *kubeslicev1beta1.Slice
	var createdSlice *kubeslicev1beta1.Slice
	var hubSliceGw *workerv1alpha1.WorkerSliceGateway
	var hubSlice *workerv1alpha1.WorkerSliceConfig
	var svc *corev1.Service
	var nsmconfig *corev1.ConfigMap
	var ns *corev1.Namespace
	var node1, node2 *corev1.Node //node2
	var vl3ServiceEndpoint *nsmv1alpha1.NetworkServiceEndpoint
	// var createdDeploy *apps.Deployment
	Context("With Redundant GW pods", func() {
		BeforeEach(func() {
			//create 2 kubeslice GW nodes
			node1 = &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "kubeslice-gw-node-3",
					Labels: map[string]string{
						"kubeslice.io/node-type":        "gateway",
						"topology.kubernetes.io/region": "us-east-1",
						"kubeslice.io/pod-type":         "slicegateway",
						"topology.kubeslice.io/gateway": "0",
					},
				},
				Spec: corev1.NodeSpec{
					ProviderID: "gce://demo",
				},
				Status: corev1.NodeStatus{
					Addresses: []corev1.NodeAddress{
						{
							Type:    corev1.NodeExternalIP,
							Address: "35.235.10.1",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, node1)).Should(Succeed())
			node2 = &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "kubeslice-gw-node-4",
					Labels: map[string]string{
						"kubeslice.io/node-type":        "gateway",
						"topology.kubernetes.io/region": "us-east-1",
						"kubeslice.io/pod-type":         "slicegateway",
						"topology.kubeslice.io/gateway": "1",
					},
				},
				Spec: corev1.NodeSpec{
					ProviderID: "gce://demo",
				},
				Status: corev1.NodeStatus{
					Addresses: []corev1.NodeAddress{
						{
							Type:    corev1.NodeExternalIP,
							Address: "35.235.10.2",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, node2)).Should(Succeed())
			// node3 = &corev1.Node{
			// 	ObjectMeta: metav1.ObjectMeta{
			// 		Name: "kubeslice-gw-node-5",
			// 		Labels: map[string]string{
			// 			"kubeslice.io/node-type":        "gateway",
			// 			"topology.kubernetes.io/region": "us-east-1",
			// 			"kubeslice.io/pod-type":         "slicegateway",
			// 			"topology.kubeslice.io/gateway": "1",
			// 		},
			// 	},
			// 	Spec: corev1.NodeSpec{},
			// 	Status: corev1.NodeStatus{
			// 		Addresses: []corev1.NodeAddress{
			// 			{
			// 				Type:    corev1.NodeExternalIP,
			// 				Address: "35.235.10.3",
			// 			},
			// 		},
			// 	},
			// }
			//

			// create project namespace (simulate controller cluster behaviour)
			ns = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: PROJECT_NS,
				},
				Spec: corev1.NamespaceSpec{},
			}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: ns.Name}, ns)
			if errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, ns)).Should(Succeed())
			}

			// create cluster CR under project namespace
			cluster = &hubv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-gw",
					Namespace: PROJECT_NS,
				},
				Spec:   hubv1alpha1.ClusterSpec{},
				Status: hubv1alpha1.ClusterStatus{},
			}
			Expect(k8sClient.Create(ctx, cluster)).Should(Succeed())

			sliceGw = &kubeslicev1beta1.SliceGateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slicegw-pod",
					Namespace: CONTROL_PLANE_NS,
				},
				Spec: kubeslicev1beta1.SliceGatewaySpec{
					SliceName: "test-slice-gw",
				},
			}

			svc = &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kubeslice-dns",
					Namespace: CONTROL_PLANE_NS,
				},
				Spec: corev1.ServiceSpec{
					ClusterIP: "10.0.0.20",
					Ports: []corev1.ServicePort{{
						Port: 52,
					}},
				},
			}
			hubSlice = &workerv1alpha1.WorkerSliceConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slice-gw",
					Namespace: PROJECT_NS,
					Labels: map[string]string{
						"worker-cluster": CLUSTER_NAME,
					},
				},
				Spec: workerv1alpha1.WorkerSliceConfigSpec{
					SliceName:     "test-slice-gw",
					SliceType:     "Application",
					SliceSubnet:   "10.0.0.1/16",
					SliceIpamType: "Local",
					// IpamClusterOctet: 100,
				},
			}
			hubSliceGw = &workerv1alpha1.WorkerSliceGateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slice-gw-cisco",
					Namespace: PROJECT_NS,
					Labels: map[string]string{
						"worker-cluster": CLUSTER_NAME,
					},
				},
				Spec: workerv1alpha1.WorkerSliceGatewaySpec{

					SliceName: "test-slice",
					LocalGatewayConfig: workerv1alpha1.SliceGatewayConfig{
						NodeIps: []string{
							"35.235.10.2",
							"35.235.10.1",
						},
						ClusterName: CLUSTER_NAME,
					},
				},
			}
			slice = &kubeslicev1beta1.Slice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slice-gw",
					Namespace: CONTROL_PLANE_NS,
				},
				Spec: kubeslicev1beta1.SliceSpec{},
			}

			vl3ServiceEndpoint = &nsmv1alpha1.NetworkServiceEndpoint{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "networkservicemesh.io/v1alpha1",
					Kind:       "NetworkServiceEndpoint",
				},
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "vl3-service-" + "test-slice-gw",
					Namespace:    "kubeslice-system",
					Labels: map[string]string{
						"app":                "vl3-nse-" + "test-slice-gw",
						"networkservicename": "vl3-service-" + "test-slice-gw",
					},
				},
				Spec: nsmv1alpha1.NetworkServiceEndpointSpec{
					NetworkServiceName: "vl3-service-" + "test-slice-gw",
					Payload:            "IP",
					NsmName:            "test-node",
				},
			}
			nsmconfig = configMap("nsm-config", "kubeslice-system", `
prefixes:
- 192.168.0.0/16
- 10.96.0.0/12`)
			err = k8sClient.Get(ctx, types.NamespacedName{Name: nsmconfig.Name, Namespace: nsmconfig.Namespace}, nsmconfig)
			if errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, nsmconfig)).Should(Succeed())
			}
			// createdSlice = &kubeslicev1beta1.Slice{}
			// createdSliceGw = &kubeslicev1beta1.SliceGateway{}

			DeferCleanup(func() {
				Eventually(func() bool {
					err := k8sClient.Delete(ctx, node1)
					return errors.IsNotFound(err)
				}, time.Second*10, time.Millisecond*250).Should(BeTrue())
				Eventually(func() bool {
					err := k8sClient.Delete(ctx, node2)
					return errors.IsNotFound(err)
				}, time.Second*10, time.Millisecond*250).Should(BeTrue())
				Eventually(func() bool {
					err := k8sClient.Delete(ctx, cluster)
					return errors.IsNotFound(err)
				}, time.Second*10, time.Millisecond*250).Should(BeTrue())
			})
		})

		// It("Should update cluster CR with List of Node IPs", func() {
		// 	os.Setenv("CLUSTER_NAME", cluster.Name)
		// 	os.Setenv("HUB_PROJECT_NAMESPACE", PROJECT_NS)
		// 	ctx := context.Background()
		// 	nodeIP, err := clusterpkg.GetNodeIP(k8sClient)
		// 	Expect(err).To(BeNil())
		// 	//post GeoLocation and other metadata to cluster CR on Hub cluster
		// 	err = hub.PostClusterInfoToHub(ctx, k8sClient, k8sClient, "cluster-gw", PROJECT_NS, nodeIP)
		// 	Expect(err).To(BeNil())
		// 	//get the cluster object
		// 	Eventually(func() bool {
		// 		err := k8sClient.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, cluster)
		// 		return err == nil
		// 	}, time.Second*10, time.Millisecond*250).Should(BeTrue())
		// 	Expect(cluster.Spec.NodeIPs).Should(Equal(nodeIP))

		// })

		// It("Should update NodeIp List in the cluster CR with New Node Ip", func() {
		// 	os.Setenv("CLUSTER_NAME", cluster.Name)
		// 	os.Setenv("HUB_PROJECT_NAMESPACE", PROJECT_NS)
		// 	ctx := context.Background()
		// 	nodeIP, err := clusterpkg.GetNodeIP(k8sClient)
		// 	Expect(err).To(BeNil())
		// 	//post GeoLocation and other metadata to cluster CR on Hub cluster
		// 	err = hub.PostClusterInfoToHub(ctx, k8sClient, k8sClient, "cluster-gw", PROJECT_NS, nodeIP)
		// 	Expect(err).To(BeNil())
		// 	//get the cluster object
		// 	Eventually(func() bool {
		// 		err := k8sClient.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, cluster)
		// 		return err == nil
		// 	}, time.Second*10, time.Millisecond*250).Should(BeTrue())
		// 	Expect(cluster.Spec.NodeIPs).Should(Equal(nodeIP))

		// 	Expect(k8sClient.Create(ctx, svc)).Should(Succeed())
		// 	Expect(k8sClient.Create(ctx, slice)).Should(Succeed())
		// 	Expect(k8sClient.Create(ctx, vl3ServiceEndpoint)).Should(Succeed())
		// 	Expect(k8sClient.Create(ctx, sliceGw)).Should(Succeed())
		// 	slicegwkey := types.NamespacedName{Name: sliceGw.Name, Namespace: CONTROL_PLANE_NS}
		// 	Eventually(func() bool {
		// 		err := k8sClient.Get(ctx, slicegwkey, createdSliceGw)
		// 		return err == nil
		// 	}, time.Second*10, time.Millisecond*250).Should(BeTrue())

		// 	createdSliceGw.Status.Config.SliceGatewayHostType = "Server"
		// 	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// 		err := k8sClient.Get(ctx, slicegwkey, createdSliceGw)
		// 		if err != nil {
		// 			return err
		// 		}
		// 		createdSliceGw.Status.Config.SliceGatewayHostType = "Server"
		// 		err = k8sClient.Status().Update(ctx, createdSliceGw)
		// 		if err != nil {
		// 			return err
		// 		}
		// 		return nil
		// 	})
		// 	Expect(err).To(BeNil())
		// 	//create another kubeslice node
		// 	Expect(k8sClient.Create(ctx, node3)).Should(Succeed())
		// 	Expect(k8sClient.Delete(ctx, node2)).Should(Succeed())
		// 	// verify if new node IP is updated on cluster CR
		// 	Eventually(func() bool {
		// 		err := k8sClient.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, cluster)
		// 		if err != nil {
		// 			return false
		// 		}
		// 		return cluster.Spec.NodeIPs[1] == "35.235.10.3"
		// 	}, time.Second*60, time.Millisecond*250).Should(BeTrue())

		// })

		It("Should Provision two client GW pods on different nodes", func() {
			ctx := context.Background()
			// Expect(k8sClient.Create(ctx, node3)).Should(Succeed())
			os.Setenv("CLUSTER_NAME", cluster.Name)
			os.Setenv("HUB_PROJECT_NAMESPACE", PROJECT_NS)
			nodeIP, err := clusterpkg.GetNodeIP(k8sClient)
			Expect(err).To(BeNil())
			err = hub.PostClusterInfoToHub(ctx, k8sClient, k8sClient, "cluster-gw", PROJECT_NS, nodeIP)
			Expect(err).To(BeNil())
			Expect(k8sClient.Create(ctx, hubSlice)).Should(Succeed())
			Expect(k8sClient.Create(ctx, hubSliceGw)).Should(Succeed())
			Expect(k8sClient.Create(ctx, svc)).Should(Succeed())
			Expect(k8sClient.Create(ctx, slice)).Should(Succeed())
			Expect(k8sClient.Create(ctx, vl3ServiceEndpoint)).Should(Succeed())
			Expect(k8sClient.Create(ctx, sliceGw)).Should(Succeed())
			err = hub.PostClusterInfoToHub(ctx, k8sClient, k8sClient, "cluster-gw", PROJECT_NS, nodeIP)
			Expect(err).To(BeNil())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "cluster-gw", Namespace: PROJECT_NS}, cluster)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())
			fmt.Println("Node ips ------------>", cluster.Spec.NodeIPs)
			sliceKey := types.NamespacedName{Name: "test-slice-gw", Namespace: CONTROL_PLANE_NS}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, sliceKey, createdSlice)
				return err == nil
			}, time.Second*20, time.Millisecond*250).Should(BeTrue())

			slicegwkey := types.NamespacedName{Name: "test-slicegw-pod", Namespace: CONTROL_PLANE_NS}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, slicegwkey, createdSliceGw)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())
			pods := &corev1.PodList{}
			podListOptions := []client.ListOption{
				client.HasLabels{"kubeslice.io/pod-type:gateway"},
			}
			k8sClient.List(ctx, pods, podListOptions...)
			fmt.Println("Fetched gateway pods --------->", len(pods.Items))
		})

		// It("Should Provision two server GW pods on different nodes", func() {
		// 	ctx := context.Background()
		// 	Expect(k8sClient.Create(ctx, svc)).Should(Succeed())
		// 	// Expect(k8sClient.Create(ctx, slice)).Should(Succeed())
		// 	Expect(k8sClient.Create(ctx, vl3ServiceEndpoint)).Should(Succeed())
		// 	Expect(k8sClient.Create(ctx, sliceGw)).Should(Succeed())
		// 	// gwPodsKey := types.NamespacedName{Name: "test-slice-gw", Namespace: CONTROL_PLANE_NS}
		// 	pods := &corev1.PodList{}
		// 	podListOptions := []client.ListOption{
		// 		client.HasLabels{"kubeslice.io/pod-type:gateway"},
		// 	}
		// 	k8sClient.List(ctx, pods, podListOptions...)
		// 	fmt.Println("Fetched gateway pods --------->", pods)
		// })

		// It("Should validate the slicegateway cr status field")
	})
})
