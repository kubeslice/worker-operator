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

package spoke_test

import (
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

var _ = Describe("ClusterInfoUpdate", func() {
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
				},
			}

			ns = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: PROJECT_NS,
				},
			}
			cluster = &hubv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-1",
					Namespace: PROJECT_NS,
				},
				Spec:   hubv1alpha1.ClusterSpec{},
				Status: hubv1alpha1.ClusterStatus{},
			}
			nsmconfig = configMap("nsm-config", "kubeslice-system", `
prefixes:
- 192.168.0.0/16
- 10.96.0.0/12`)

			DeferCleanup(func() {
				Expect(k8sClient.Delete(ctx, node)).Should(Succeed())
			})

		})

		It("Should update cluster CR with nodeIP and geographical info", func() {
			Expect(k8sClient.Create(ctx, node))
			err := k8sClient.Get(ctx, types.NamespacedName{Name: ns.Name}, ns)
			if errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, ns)).Should(Succeed())
			}
			Expect(k8sClient.Create(ctx, cluster)).Should(Succeed())

			err = k8sClient.Get(ctx, types.NamespacedName{Name: nsmconfig.Name, Namespace: nsmconfig.Namespace}, nsmconfig)
			if errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, nsmconfig)).Should(Succeed())
			}

			nodeIP, err := clusterpkg.GetNodeIP(k8sClient)
			Expect(err).To(BeNil())
			Expect(nodeIP[0]).Should(Equal("35.235.10.1"))
			//post GeoLocation and other metadata to cluster CR on Hub cluster
			err = hub.PostClusterInfoToHub(ctx, k8sClient, k8sClient, "cluster-1", "kubeslice-cisco", nodeIP)
			Expect(err).To(BeNil())

			//get the cluster object
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, cluster)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())
			Expect(cluster.Spec.NodeIPs[0]).Should(Equal("35.235.10.1"))
			Expect(cluster.Spec.ClusterProperty.GeoLocation.CloudProvider).Should(Equal("gcp"))
			Expect(cluster.Spec.ClusterProperty.GeoLocation.CloudRegion).Should(Equal("us-east-1"))
			Expect(cluster.Status.CniSubnet).Should(Equal([]string{"192.168.0.0/16", "10.96.0.0/12"}))
		})
	})

	Context("With Cluster CR Created at controller cluster", func() {
		var ns *corev1.Namespace
		var cluster *hubv1alpha1.Cluster
		var operatorSecret *corev1.Secret
		hostname := "127.0.0.1:6443"

		BeforeEach(func() {
			ns = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: PROJECT_NS,
				},
			}
			os.Setenv("HUB_PROJECT_NAMESPACE", ns.Name)
			cluster = &hubv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-2",
					Namespace: PROJECT_NS,
				},
				Spec: hubv1alpha1.ClusterSpec{
					NodeIPs: []string{"35.235.10.1"},
				},
				Status: hubv1alpha1.ClusterStatus{},
			}
			os.Setenv("CLUSTER_NAME", cluster.Name)
			operatorSecret = getSecret("kubeslice-kubernetes-dashboard", CONTROL_PLANE_NS)

		})
		It("should create secret in controller's project namespace", func() {
			os.Setenv("CLUSTER_ENDPOINT", hostname)
			Expect(k8sClient.Create(ctx, cluster)).Should(Succeed())
			Expect(k8sClient.Create(ctx, operatorSecret)).Should(Succeed())

			//get the created operator secret
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: operatorSecret.Name,
					Namespace: operatorSecret.Namespace}, operatorSecret)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			err := hub.PostCredsToHub(ctx, k8sClient, k8sClient, operatorSecret)
			Expect(err).To(BeNil())

			//get the secret on controller
			Eventually(func() bool {
				hubSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      cluster.Name + hub.HubSecretSuffix,
						Namespace: PROJECT_NS,
					}}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: hubSecret.Name, Namespace: hubSecret.Namespace}, hubSecret)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())
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
})

func configMap(name, namespace, data string) *corev1.ConfigMap {
	configMapData := make(map[string]string)
	configMapData["excluded_prefixes.yaml"] = data
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
