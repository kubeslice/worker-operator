package spoke_test

import (
	"fmt"
	"os"
	"time"

	hubv1alpha1 "github.com/kubeslice/apis/pkg/controller/v1alpha1"
	clusterpkg "github.com/kubeslice/worker-operator/pkg/cluster"
	hub "github.com/kubeslice/worker-operator/pkg/hub/hubclient"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Gateway redundancy test scenario", func() {
	// var sliceGw *kubeslicev1beta1.SliceGateway
	// var createdSliceGw *kubeslicev1beta1.SliceGateway
	var cluster *hubv1alpha1.Cluster
	// var slice *kubeslicev1beta1.Slice
	// var createdSlice *kubeslicev1beta1.Slice
	// var svc *corev1.Service
	var ns *corev1.Namespace
	var node1, node2 *corev1.Node
	// var vl3ServiceEndpoint *nsmv1alpha1.NetworkServiceEndpoint

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
				Spec: corev1.NodeSpec{},
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
				Spec: corev1.NodeSpec{},
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

			// sliceGw = &kubeslicev1beta1.SliceGateway{
			// 	ObjectMeta: metav1.ObjectMeta{
			// 		Name:      "test-slicegw-pod",
			// 		Namespace: CONTROL_PLANE_NS,
			// 	},
			// 	Spec: kubeslicev1beta1.SliceGatewaySpec{
			// 		SliceName: "test-slice-gw",
			// 	},
			// }

			// svc = &corev1.Service{
			// 	ObjectMeta: metav1.ObjectMeta{
			// 		Name:      "kubeslice-dns",
			// 		Namespace: CONTROL_PLANE_NS,
			// 	},
			// 	Spec: corev1.ServiceSpec{
			// 		ClusterIP: "10.0.0.20",
			// 		Ports: []corev1.ServicePort{{
			// 			Port: 52,
			// 		}},
			// 	},
			// }

			// slice = &kubeslicev1beta1.Slice{
			// 	ObjectMeta: metav1.ObjectMeta{
			// 		Name:      "test-slice-gw",
			// 		Namespace: CONTROL_PLANE_NS,
			// 	},
			// 	Spec: kubeslicev1beta1.SliceSpec{},
			// }

			// vl3ServiceEndpoint = &nsmv1alpha1.NetworkServiceEndpoint{
			// 	TypeMeta: metav1.TypeMeta{
			// 		APIVersion: "networkservicemesh.io/v1alpha1",
			// 		Kind:       "NetworkServiceEndpoint",
			// 	},
			// 	ObjectMeta: metav1.ObjectMeta{
			// 		GenerateName: "vl3-service-" + "test-slice-gw",
			// 		Namespace:    "kubeslice-system",
			// 		Labels: map[string]string{
			// 			"app":                "vl3-nse-" + "test-slice-gw",
			// 			"networkservicename": "vl3-service-" + "test-slice-gw",
			// 		},
			// 	},
			// 	Spec: nsmv1alpha1.NetworkServiceEndpointSpec{
			// 		NetworkServiceName: "vl3-service-" + "test-slice-gw",
			// 		Payload:            "IP",
			// 		NsmName:            "test-node",
			// 	},
			// }
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

		It("Should update cluster CR with List of Node IPs", func() {
			os.Setenv("CLUSTER_NAME", cluster.Name)
			os.Setenv("HUB_PROJECT_NAMESPACE", PROJECT_NS)
			nodeIpList, err := clusterpkg.GetNodeIP(k8sClient)
			Expect(err).To(BeNil())

			fmt.Println(nodeIpList, "------------------------gw")

			err = hub.PostClusterInfoToHub(ctx, k8sClient, k8sClient, "cluster-gw", "kubeslice-cisco", nodeIpList)
			Expect(err).To(BeNil())
			//get the cluster object
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, cluster)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())
			Expect(cluster.Spec.NodeIPs).Should(Equal(nodeIpList))
		})

		// It("Should Provision two client GW pods on different nodes")

		// It("Should Provision two server GW pods on different nodes")

		// It("Should validate the slicegateway cr status field")
	})
})
