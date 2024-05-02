package cluster

import (
	"fmt"
	"os"
	"time"

	hubv1alpha1 "github.com/kubeslice/apis/pkg/controller/v1alpha1"
	hub "github.com/kubeslice/worker-operator/pkg/hub/hubclient"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

/*
FIXME(reduce code repetition): instead of repeating setup/cleanup steps(i.e installing node, nsm configmap, project ns, nsmgr daemonset etc)
for each Context blocks, create a common section for setup & clean up that executes before the actual validations/tests.
*/
var _ = Describe("Hub ClusterController", func() {
	var nsmgrMock *appsv1.DaemonSet
	Context("With Cluster CR Created at hub cluster", func() {
		var ns *corev1.Namespace
		var cluster *hubv1alpha1.Cluster
		var node *corev1.Node
		var nsmconfig *corev1.ConfigMap
		BeforeEach(func() {
			node = &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node",
					Labels: map[string]string{
						"topology.kubernetes.io/region": "us-east-1",
						"kubeslice.io/node-type":        "gateway",
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
					Conditions: []corev1.NodeCondition{
						{
							Type:   corev1.NodeReady,
							Status: corev1.ConditionTrue,
						},
					},
				},
			}

			ns = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: PROJECT_NS,
				},
			}
			cluster = &hubv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      CLUSTER_NAME,
					Namespace: PROJECT_NS,
				},
				Spec:   hubv1alpha1.ClusterSpec{},
				Status: hubv1alpha1.ClusterStatus{},
			}
			// nsmgr daemonset (isNetworkPresent)
			nsmgrMock = &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nsmgr",
					Namespace: CONTROL_PLANE_NS,
					Labels:    map[string]string{"app": "nsmgr"},
				},
				Spec: appsv1.DaemonSetSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "nsmgr"},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{"app": "nsmgr"},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "nsmgr",
									Image: "containerImage:tag",
								},
							},
						},
					},
				},
			}
			nsmconfig = configMap("nsm-config", "kubeslice-system", `
 Prefixes:
 - 192.168.0.0/16
 - 10.96.0.0/12`)

			DeferCleanup(func() {
				Expect(k8sClient.Delete(ctx, node)).Should(Succeed())
				// Delete cluster object
				Expect(k8sClient.Delete(ctx, cluster)).Should(Succeed())
				err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name: cluster.Name, Namespace: cluster.Namespace,
					}, cluster)
					if err != nil {
						return err
					}
					// remove finalizer from cluster CR
					cluster.ObjectMeta.SetFinalizers([]string{})
					return k8sClient.Update(ctx, cluster)
				})
				Expect(err).To(BeNil())
				// delete nsmgr
				Expect(k8sClient.Delete(ctx, nsmgrMock)).Should(Succeed())
				Eventually(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, cluster)
					return errors.IsNotFound(err)
				}, time.Second*100, time.Millisecond*250).Should(BeTrue())
			})

		})

		It("Should update cluster CR with nodeIP and geographical info", func() {
			Expect(k8sClient.Create(ctx, node))
			err := k8sClient.Get(ctx, types.NamespacedName{Name: ns.Name}, ns)
			if errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, ns)).Should(Succeed())
			}
			Expect(k8sClient.Create(ctx, nsmgrMock)).Should(Succeed())
			Expect(k8sClient.Create(ctx, cluster)).Should(Succeed())

			err = k8sClient.Get(ctx, types.NamespacedName{Name: nsmconfig.Name, Namespace: nsmconfig.Namespace}, nsmconfig)
			if errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, nsmconfig)).Should(Succeed())
			}
			//get the cluster object
			Eventually(func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, cluster)
				if err != nil {
					return err
				}
				if len(cluster.Status.NodeIPs) == 0 {
					return fmt.Errorf("nodeip not populated")
				}
				return nil
			}, time.Second*120, time.Millisecond*500).ShouldNot(HaveOccurred())
			Expect(cluster.Status.NodeIPs[0]).Should(Equal("35.235.10.1"))
			Expect(cluster.Spec.ClusterProperty.GeoLocation.CloudProvider).Should(Equal("gcp"))
			Expect(cluster.Spec.ClusterProperty.GeoLocation.CloudRegion).Should(Equal("us-east-1"))
			Expect(cluster.Status.CniSubnet).Should(Equal([]string{"192.168.0.0/16", "10.96.0.0/12"}))

			events := &corev1.EventList{}
			Eventually(func() bool {
				err := k8sClient.List(ctx, events, client.InNamespace("kubeslice-system"))
				return err == nil && len(events.Items) > 0 && eventFound(events, "ClusterNodeIpAutoDetected")
			}, 30*time.Second, 1*time.Second).Should(BeTrue())
		})
	})

	Context("With Cluster CR Created at controller cluster", func() {
		var ns *corev1.Namespace
		var cluster *hubv1alpha1.Cluster
		var operatorSecret *corev1.Secret
		var sa *corev1.ServiceAccount
		var node *corev1.Node
		hostname := "127.0.0.1:6443"

		BeforeEach(func() {
			ns = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: PROJECT_NS,
				},
			}
			node = &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node",
					Labels: map[string]string{
						"topology.kubernetes.io/region": "us-east-1",
						"kubeslice.io/node-type":        "gateway",
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
					Conditions: []corev1.NodeCondition{
						{
							Type:   corev1.NodeReady,
							Status: corev1.ConditionTrue,
						},
					},
				},
			}
			os.Setenv("HUB_PROJECT_NAMESPACE", ns.Name)
			cluster = &hubv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      CLUSTER_NAME,
					Namespace: PROJECT_NS,
				},
				Spec:   hubv1alpha1.ClusterSpec{},
				Status: hubv1alpha1.ClusterStatus{},
			}
			os.Setenv("CLUSTER_NAME", cluster.Name)
			operatorSecret = getSecret("kubeslice-kubernetes-dashboard", CONTROL_PLANE_NS)
			sa = getSA("kubeslice-kubernetes-dashboard", CONTROL_PLANE_NS, operatorSecret.Name)
			// nsmgr daemonset (isNetworkPresent)
			nsmgrMock = &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nsmgr",
					Namespace: CONTROL_PLANE_NS,
					Labels:    map[string]string{"app": "nsmgr"},
				},
				Spec: appsv1.DaemonSetSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "nsmgr"},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{"app": "nsmgr"},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "nsmgr",
									Image: "containerImage:tag",
								},
							},
						},
					},
				},
			}
		})
		It("should create secret in controller's project namespace", func() {
			os.Setenv("CLUSTER_ENDPOINT", hostname)
			Expect(k8sClient.Create(ctx, node))
			Expect(k8sClient.Create(ctx, nsmgrMock)).Should(Succeed())
			Expect(k8sClient.Create(ctx, cluster)).Should(Succeed())
			Expect(k8sClient.Create(ctx, operatorSecret)).Should(Succeed())
			Expect(k8sClient.Create(ctx, sa)).Should(Succeed())
			//get the created operator secret
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: operatorSecret.Name,
					Namespace: operatorSecret.Namespace}, operatorSecret)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			//get the secret on controller
			Eventually(func() bool {
				hubSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      cluster.Name + hub.HubSecretSuffix,
						Namespace: PROJECT_NS,
					}}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: hubSecret.Name, Namespace: hubSecret.Namespace}, hubSecret)
				return err == nil
			}, time.Second*30, time.Millisecond*250).Should(BeTrue())
		})
		It("Should update cluster CR with secret info", func() {
			//get the cluster object
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, cluster)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			Expect(cluster.Spec.ClusterProperty.Monitoring.KubernetesDashboard.AccessToken).
				Should(Equal(cluster.Name + hub.HubSecretSuffix))
		})
		It("Should update cluster CR with cluster URL", func() {
			//get the cluster object
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, cluster)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())
			Expect(cluster.Spec.ClusterProperty.Monitoring.KubernetesDashboard.Endpoint).
				Should(Equal(hostname))
		})
	})

	Context("Cluster health check", func() {
		var ns, nsSpire, nsIstio *corev1.Namespace
		var cluster *hubv1alpha1.Cluster
		var operatorSecret *corev1.Secret
		var sa *corev1.ServiceAccount
		var pods []*corev1.Pod
		var node *corev1.Node
		var nsmconfig *corev1.ConfigMap
		// var podBasedComponents []string

		BeforeEach(func() {
			ns = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: PROJECT_NS,
				},
			}
			node = &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node",
					Labels: map[string]string{
						"topology.kubernetes.io/region": "us-east-1",
						"kubeslice.io/node-type":        "gateway",
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
					Conditions: []corev1.NodeCondition{
						{
							Type:   corev1.NodeReady,
							Status: corev1.ConditionTrue,
						},
					},
				},
			}
			nsSpire = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "spire",
				},
			}
			nsIstio = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "istio-system",
				},
			}
			cluster = &hubv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      CLUSTER_NAME,
					Namespace: PROJECT_NS,
				},
				Spec:   hubv1alpha1.ClusterSpec{},
				Status: hubv1alpha1.ClusterStatus{},
			}
			// nsmgr daemonset (isNetworkPresent)
			nsmgrMock = &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nsmgr",
					Namespace: CONTROL_PLANE_NS,
					Labels:    map[string]string{"app": "nsmgr"},
				},
				Spec: appsv1.DaemonSetSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "nsmgr"},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{"app": "nsmgr"},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "nsmgr",
									Image: "containerImage:tag",
								},
							},
						},
					},
				},
			}
			nsmconfig = configMap("nsm-config", "kubeslice-system", `
 Prefixes:
 - 192.168.0.0/16
 - 10.96.0.0/12`)
			// podBasedComponents = []string{
			// 	"nsmgr",
			// 	"forwarder",
			// 	"admission-webhook",
			// 	"netop",
			// 	"spire-agent",
			// 	"spire-server",
			// 	"istiod",
			// }
			// otherComponents = []string{
			// 	"node-ips",
			// 	"cni-subnets",
			// }
			os.Setenv("HUB_PROJECT_NAMESPACE", PROJECT_NS)
			os.Setenv("CLUSTER_NAME", CLUSTER_NAME)
			operatorSecret = getSecret("kubeslice-kubernetes-dashboard", CONTROL_PLANE_NS)
			sa = getSA("kubeslice-kubernetes-dashboard", CONTROL_PLANE_NS, operatorSecret.Name)
			Expect(k8sClient.Create(ctx, node))
			Expect(k8sClient.Create(ctx, ns))
			Expect(k8sClient.Create(ctx, nsSpire))
			Expect(k8sClient.Create(ctx, nsIstio))
			Expect(k8sClient.Create(ctx, operatorSecret))
			Expect(k8sClient.Create(ctx, sa))
			err := k8sClient.Get(ctx, types.NamespacedName{Name: nsmconfig.Name, Namespace: nsmconfig.Namespace}, nsmconfig)
			if errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, nsmconfig)).Should(Succeed())
			}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: "nsmgr", Namespace: CONTROL_PLANE_NS}, nsmgrMock)
			if errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, nsmgrMock)).Should(Succeed())
			}
			Expect(k8sClient.Create(ctx, cluster))

			DeferCleanup(func() {
				Expect(k8sClient.Delete(ctx, node)).Should(Succeed())
				// Delete cluster object
				k8sClient.Delete(ctx, cluster)
				err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name: cluster.Name, Namespace: cluster.Namespace,
					}, cluster)
					if err != nil {
						return err
					}
					// remove finalizer from cluster CR
					cluster.ObjectMeta.SetFinalizers([]string{})
					return k8sClient.Update(ctx, cluster)
				})
				Expect(err).To(BeNil())
				// Wait for cluster CR to be cleaned up
				Eventually(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, cluster)
					fmt.Println("----------------- clusterobj finalizer", cluster.ObjectMeta.Finalizers)
					return err != nil && errors.IsNotFound(err)
				}, time.Second*100, time.Second*1).Should(BeTrue())
				// delete nsmgr
				Expect(k8sClient.Delete(ctx, nsmgrMock)).Should(Succeed())
				k8sClient.Delete(ctx, operatorSecret)
				k8sClient.Delete(ctx, sa)

				// delete all pods which were created for checking health status
				for _, pod := range pods {
					k8sClient.Delete(ctx, pod)
					pods = []*corev1.Pod{}
				}
			})
		})

		It("Should update cluster CR with health status updated time", func() {
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, cluster)
				return err == nil && cluster.Status.ClusterHealth != nil
			}, time.Second*10, time.Second*1).Should(BeTrue())
			last := cluster.Status.ClusterHealth.LastUpdated

			// LastUpdated should be updated every few seconds
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, cluster)
				return err == nil && cluster.Status.ClusterHealth != nil && !cluster.Status.ClusterHealth.LastUpdated.Equal(&last)
			}, time.Second*200, time.Second*1).Should(BeTrue())
		})

		It("Should update cluster CR with compoent status as error for all", func() {
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, cluster)
				GinkgoWriter.Println(cluster)
				return err == nil && cluster.Status.ClusterHealth != nil
			}, time.Second*10, time.Second*1).Should(BeTrue())
			cs := cluster.Status.ClusterHealth.ComponentStatuses

			Expect(cs).Should(HaveLen(8))

			Expect(string(cluster.Status.ClusterHealth.ClusterHealthStatus)).Should(Equal("Warning"))

			podBasedComponentLists := []string{
				"nsmgr",
				"forwarder",
				"admission-webhook",
				"netop",
				"spire-agent",
				"spire-server",
			}
			for _, c := range podBasedComponentLists {
				Expect(componentExists(cs, c)).Should(BeTrue())
				Expect(matchHealthStatus(cs, c, hubv1alpha1.ComponentHealthStatusError)).Should(BeTrue())
			}

			events := &corev1.EventList{}
			Eventually(func() bool {
				err := k8sClient.List(ctx, events, client.InNamespace("kubeslice-system"))
				return err == nil && len(events.Items) > 0 && eventFound(events, "ClusterUnhealthy")
			}, 30*time.Second, 1*time.Second).Should(BeTrue())
		})

		It("Should update cluster CR with component status as normal when pods running", func() {
			pods := []*corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "nsmgr",
						Labels: map[string]string{
							"app": "nsmgr",
						},
						Namespace: "kubeslice-system",
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "nsmgr",
							Image: "nsmgr",
						}},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "forwarder",
						Labels: map[string]string{
							"app": "forwarder-kernel",
						},
						Namespace: "kubeslice-system",
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "test",
							Image: "test",
						}},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "admission-webhook",
						Labels: map[string]string{
							"app": "admission-webhook-k8s",
						},
						Namespace: "kubeslice-system",
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "test",
							Image: "test",
						}},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "netop",
						Labels: map[string]string{
							"app":                   "app_net_op",
							"kubeslice.io/pod-type": "netop",
						},
						Namespace: "kubeslice-system",
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "test",
							Image: "test",
						}},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "spire-agent",
						Labels: map[string]string{
							"app": "spire-agent",
						},
						Namespace: "spire",
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "test",
							Image: "test",
						}},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "spire-server",
						Labels: map[string]string{
							"app": "spire-server",
						},
						Namespace: "spire",
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "test",
							Image: "test",
						}},
					},
				},
			}
			for _, pod := range pods {
				Expect(k8sClient.Create(ctx, pod)).ToNot(HaveOccurred())
				pod.Status.Phase = corev1.PodRunning
				Expect(k8sClient.Status().Update(ctx, pod)).ToNot(HaveOccurred())
			}

			// wait for next reconcile loop
			time.Sleep(10 * time.Second)

			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, cluster)
				return err == nil && cluster.Status.ClusterHealth != nil
			}, time.Second*10, time.Second*1).Should(BeTrue())

			cs := cluster.Status.ClusterHealth.ComponentStatuses
			Expect(cs).Should(HaveLen(8))

			Expect(string(cluster.Status.ClusterHealth.ClusterHealthStatus)).Should(Equal("Normal"))

			for i, pod := range pods {
				Expect((pod.ObjectMeta.Name)).Should(Equal(cs[i].Component))
				Expect(string(cs[i].ComponentHealthStatus)).Should(Equal("Normal"))
			}

			additionalComponentLists := []string{"node-ips", "cni-subnets"}
			for _, c := range additionalComponentLists {
				Expect(componentExists(cs, c)).Should(BeTrue())
				Expect(matchHealthStatus(cs, c, hubv1alpha1.ComponentHealthStatusNormal)).Should(BeTrue())
			}

			events := &corev1.EventList{}
			Eventually(func() bool {
				err := k8sClient.List(ctx, events, client.InNamespace("kubeslice-system"))
				return err == nil && len(events.Items) > 0 && eventFound(events, "ClusterHealthy")
			}, 30*time.Second, 1*time.Second).Should(BeTrue())
		})

		It("Should update cluster CR with istio status", func() {
			pods := []*corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "istiod",
						Labels: map[string]string{
							"app":   "istiod",
							"istio": "pilot",
						},
						Namespace: "istio-system",
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "test",
							Image: "test",
						}},
					},
				},
			}
			for _, pod := range pods {
				Expect(k8sClient.Create(ctx, pod)).ToNot(HaveOccurred())
				pod.Status.Phase = corev1.PodRunning
				Expect(k8sClient.Status().Update(ctx, pod)).ToNot(HaveOccurred())
			}

			// wait for next reconcile loop
			time.Sleep(10 * time.Second)

			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, cluster)
				return err == nil && cluster.Status.ClusterHealth != nil
			}, time.Second*10, time.Second*1).Should(BeTrue())

			cs := cluster.Status.ClusterHealth.ComponentStatuses
			Expect(cs).Should(HaveLen(9))

			Expect(cs[6].Component).Should(Equal(("istiod")))
			Expect(string(cs[6].ComponentHealthStatus)).Should(Equal("Normal"))

		})

	})
})

func configMap(name, namespace, data string) *corev1.ConfigMap {
	configMapData := make(map[string]string)
	configMapData["excluded_prefixes_output.yaml"] = data
	configMap := corev1.ConfigMap{
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

func getSA(name, namespace, secret string) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ServiceAccount",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Secrets: []corev1.ObjectReference{{
			Namespace: namespace,
			Name:      secret,
		}},
	}
}

func getSecret(name, namespace string) *corev1.Secret {
	secretData := make(map[string][]byte)
	secretData["token"] = []byte("my-token")
	secretData["ca.crt"] = []byte("my-ca-cert")
	secret := corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: secretData,
	}
	return &secret
}

func eventFound(events *corev1.EventList, eventTitle string) bool {
	for _, event := range events.Items {
		if event.Labels["eventTitle"] == eventTitle {
			return true
		}
	}
	return false
}

func componentExists(cs []hubv1alpha1.ComponentStatus, componentName string) bool {
	for _, c := range cs {
		if c.Component == componentName {
			return true
		}
	}
	return false
}

func matchHealthStatus(cs []hubv1alpha1.ComponentStatus, componentName string, expectedStatus hubv1alpha1.ComponentHealthStatus) bool {
	for _, c := range cs {
		if c.Component == componentName && c.ComponentHealthStatus == expectedStatus {
			return true
		}
	}
	return false
}
