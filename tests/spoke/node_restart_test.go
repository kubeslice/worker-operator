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
	"context"
	"os"
	"time"

	nsmv1 "github.com/networkservicemesh/sdk-k8s/pkg/tools/k8s/apis/networkservicemesh.io/v1"

	hubv1alpha1 "github.com/kubeslice/apis/pkg/controller/v1alpha1"
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
)

var _ = Describe("NodeRestart Test Suite", func() {
	var sliceGwServer *kubeslicev1beta1.SliceGateway
	var createdSliceGw *kubeslicev1beta1.SliceGateway
	var slice *kubeslicev1beta1.Slice
	var vl3ServiceEndpoint *nsmv1.NetworkServiceEndpoint
	var node1, node2 *corev1.Node
	var ns *corev1.Namespace
	var cluster *hubv1alpha1.Cluster
	var nsmconfig *corev1.ConfigMap

	Context("With kubeslice node restarting", func() {
		BeforeEach(func() {
			//create 2 kubeslice gateway nodes
			node1 = &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "kubeslice-gw-node-1",
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
			Expect(k8sClient.Create(ctx, node1)).Should(Succeed())
			node2 = &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "kubeslice-gw-node-2",
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
							Address: "35.235.10.2",
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
			// create project namespace (simulate controller cluster behaviour)
			ns = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: PROJECT_NS,
				},
			}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: ns.Name}, ns)
			if errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, ns)).Should(Succeed())
			}
			// create cluster CR under project namespace
			cluster = &hubv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-node",
					Namespace: PROJECT_NS,
				},
				Spec:   hubv1alpha1.ClusterSpec{},
				Status: hubv1alpha1.ClusterStatus{},
			}
			Expect(k8sClient.Create(ctx, cluster)).Should(Succeed())

			sliceGwServer = &kubeslicev1beta1.SliceGateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slicegw-server",
					Namespace: CONTROL_PLANE_NS,
				},
				Spec: kubeslicev1beta1.SliceGatewaySpec{
					SliceName: "test-slice-node",
				},
			}

			slice = &kubeslicev1beta1.Slice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slice-node",
					Namespace: CONTROL_PLANE_NS,
				},
				Spec: kubeslicev1beta1.SliceSpec{},
			}
			vl3ServiceEndpoint = &nsmv1.NetworkServiceEndpoint{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "vl3-nse-" + "test-slice-node",
					Namespace: "kubeslice-system",
					Labels: map[string]string{
						"app":                "vl3-nse-" + "test-slice-node",
						"networkservicename": "vl3-service-" + "test-slice-node",
					},
				},
				Spec: nsmv1.NetworkServiceEndpointSpec{
					Name:                "vl3-service-" + "test-slice-node",
					NetworkServiceNames: []string{"\"vl3-service-\" + \"test-slice-node\""},
				},
			}
			nsmconfig = configMap("nsm-config", "kubeslice-system", `
Prefixes:
 - 192.168.0.0/16
 - 10.96.0.0/12`)
			err = k8sClient.Get(ctx, types.NamespacedName{Name: nsmconfig.Name, Namespace: nsmconfig.Namespace}, nsmconfig)
			if errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, nsmconfig)).Should(Succeed())
			}
			createdSliceGw = &kubeslicev1beta1.SliceGateway{}

			DeferCleanup(func() {
				Eventually(func() bool {
					err := k8sClient.Delete(ctx, node1)
					return errors.IsNotFound(err)
				}, time.Second*10, time.Millisecond*250).Should(BeTrue())
				Eventually(func() bool {
					err := k8sClient.Delete(ctx, node2)
					return errors.IsNotFound(err)
				}, time.Second*10, time.Millisecond*250).Should(BeTrue())
			})
		})
		It("should update cluster CR with new node IP", func() {
			os.Setenv("CLUSTER_NAME", cluster.Name)
			os.Setenv("HUB_PROJECT_NAMESPACE", PROJECT_NS)
			ctx := context.Background()

			//get the cluster object
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, cluster)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			// set nodeip as ip of node1
			cluster.Spec.NodeIPs = []string{node1.Status.Addresses[0].Address}
			Eventually(func() bool {
				err := k8sClient.Update(ctx, cluster)
				return err == nil
			}, 10*time.Second, time.Millisecond*250).Should(BeTrue())
			// start the reconcilers
			Expect(k8sClient.Create(ctx, slice)).Should(Succeed())
			Expect(k8sClient.Create(ctx, vl3ServiceEndpoint)).Should(Succeed())
			Expect(k8sClient.Create(ctx, sliceGwServer)).Should(Succeed())
			slicegwkey := types.NamespacedName{Name: "test-slicegw-server", Namespace: CONTROL_PLANE_NS}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, slicegwkey, createdSliceGw)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			createdSliceGw.Status.Config.SliceGatewayHostType = "Server"
			err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				err := k8sClient.Get(ctx, slicegwkey, createdSliceGw)
				if err != nil {
					return err
				}
				createdSliceGw.Status.Config.SliceGatewayHostType = "Server"
				err = k8sClient.Status().Update(ctx, createdSliceGw)
				if err != nil {
					return err
				}
				return nil
			})
			Expect(err).To(BeNil())
			//create another kubeslice node
			Expect(k8sClient.Create(ctx, node2)).Should(Succeed())
			// delete the node whose IP was selected to replicate node failure
			Expect(k8sClient.Delete(ctx, node1)).Should(Succeed())
			// verify if new node IP is updated on cluster CR
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, cluster)
				if err != nil {
					return false
				}
				return cluster.Spec.NodeIPs[0] == "35.235.10.2"
			}, time.Second*60, time.Millisecond*250).Should(BeTrue())
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
