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

package hub_test

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

var _ = Describe("Hub ClusterController", func() {
	Context("With Cluster CR Created at hub cluster", Ordered, func() {
		var ns, nsSpire, nsIstio *corev1.Namespace
		var cluster *hubv1alpha1.Cluster
		var node *corev1.Node
		var nsmconfig *corev1.ConfigMap
		var operatorSecret *corev1.Secret
		var sa *corev1.ServiceAccount
		hostname := "127.0.0.1:6443"

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
			Expect(k8sClient.Create(ctx, node))

			ns = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: PROJECT_NS,
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

			os.Setenv("HUB_PROJECT_NAMESPACE", PROJECT_NS)
			os.Setenv("CLUSTER_NAME", CLUSTER_NAME)

			nsmconfig = configMap("nsm-config", "kubeslice-system", `
 Prefixes:
 - 192.168.0.0/16
 - 10.96.0.0/12`)
			err := k8sClient.Get(ctx, types.NamespacedName{Name: nsmconfig.Name, Namespace: nsmconfig.Namespace}, nsmconfig)
			if errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, nsmconfig)).Should(Succeed())
			}
			Expect(k8sClient.Create(ctx, nsSpire))
			Expect(k8sClient.Create(ctx, nsIstio))
			operatorSecret = getSecret("kubeslice-kubernetes-dashboard", CONTROL_PLANE_NS)
			sa = getSA("kubeslice-kubernetes-dashboard", CONTROL_PLANE_NS, operatorSecret.Name)

			DeferCleanup(func() {
				Expect(k8sClient.Delete(ctx, node)).Should(Succeed())
			})
		})

		It("Should update cluster CR with nodeIP and geographical info", func() {
			Expect(k8sClient.Create(ctx, cluster)).Should(Succeed())
			err := k8sClient.Get(ctx, types.NamespacedName{Name: ns.Name}, ns)
			if errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, ns)).Should(Succeed())
			}
			nodeIP, err := clusterpkg.GetNodeIP(k8sClient)
			Expect(err).To(BeNil())
			Expect(nodeIP[0]).Should(Equal("35.235.10.1"))
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
			}, time.Second*30, time.Millisecond*500).ShouldNot(HaveOccurred())
			Expect(cluster.Status.NodeIPs[0]).Should(Equal("35.235.10.1"))
			Expect(cluster.Spec.ClusterProperty.GeoLocation.CloudProvider).Should(Equal("gcp"))
			Expect(cluster.Spec.ClusterProperty.GeoLocation.CloudRegion).Should(Equal("us-east-1"))
			Expect(cluster.Status.CniSubnet).Should(Equal([]string{"192.168.0.0/16", "10.96.0.0/12"}))
		})
		It("should create secret in controller's project namespace", func() {
			os.Setenv("CLUSTER_ENDPOINT", hostname)
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
			os.Setenv("CLUSTER_ENDPOINT", hostname)
			//get the cluster object
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, cluster)
				return err == nil
			}, time.Second*60, time.Millisecond*250).Should(BeTrue())

			Expect(cluster.Spec.ClusterProperty.Monitoring.KubernetesDashboard.AccessToken).
				Should(Equal(cluster.Name + hub.HubSecretSuffix))
		})
		It("Should update cluster CR with cluster URL", func() {
			os.Setenv("CLUSTER_ENDPOINT", hostname)
			//get the cluster object
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, cluster)
				return err == nil
			}, time.Second*60, time.Millisecond*250).Should(BeTrue())
			Expect(cluster.Spec.ClusterProperty.Monitoring.KubernetesDashboard.Endpoint).
				Should(Equal(hostname))
		})
		It("Should update cluster CR with health status updated time", func() {
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, cluster)
				return err == nil && cluster.Status.ClusterHealth != nil
			}, time.Second*60, time.Second*1).Should(BeTrue())
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
			}, time.Second*60, time.Second*1).Should(BeTrue())
			cs := cluster.Status.ClusterHealth.ComponentStatuses

			Expect(cs).Should(HaveLen(6))

			Expect(string(cluster.Status.ClusterHealth.ClusterHealthStatus)).Should(Equal("Warning"))

			for _, c := range cs {
				Expect(string(c.ComponentHealthStatus)).Should(Equal("Error"))
			}
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
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, cluster)
				if err != nil {
					return false
				}
				return len(cluster.Status.ClusterHealth.ComponentStatuses) == 6
			}, time.Second*60, time.Second*1).Should(BeTrue())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, cluster)
				if err != nil {
					return false
				}
				return string(cluster.Status.ClusterHealth.ClusterHealthStatus) == "Normal"
			}, time.Second*60, time.Second*1).Should(BeTrue())
			cs := cluster.Status.ClusterHealth.ComponentStatuses
			for i, c := range cs {
				Expect(c.Component).Should(Equal(pods[i].ObjectMeta.Name))
				Expect(string(c.ComponentHealthStatus)).Should(Equal("Normal"))
			}
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

			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, cluster)
				if err != nil {
					return false
				}
				return len(cluster.Status.ClusterHealth.ComponentStatuses) == 7
			}, time.Second*60, time.Second*1).Should(BeTrue())

			cs := cluster.Status.ClusterHealth.ComponentStatuses
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
