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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
)

var (
	deregisterJobName       = "cluster-deregisteration-job"
	operatorClusterRoleName = "kubeslice-manager-role"
)

var _ = Describe("Hub ClusterController", func() {
	Context("With Cluster CR Created at hub cluster", func() {
		var ns *corev1.Namespace
		var cluster *hubv1alpha1.Cluster
		var node *corev1.Node
		var nsmconfig *corev1.ConfigMap
		var clusterRole *rbacv1.ClusterRole
		var cleanUpJob *batchv1.Job
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

			clusterRole = &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: operatorClusterRoleName,
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
				Spec: hubv1alpha1.ClusterSpec{
					ClusterProperty: hubv1alpha1.ClusterProperty{
						Monitoring: hubv1alpha1.Monitoring{
							KubernetesDashboard: hubv1alpha1.KubernetesDashboard{
								Enabled: true,
							},
						},
					},
				},
				Status: hubv1alpha1.ClusterStatus{},
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
				Eventually(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, cluster)
					return errors.IsNotFound(err)
				}, time.Second*100, time.Millisecond*250).Should(BeTrue())
			})

		})

		FIt("Should update cluster CR with nodeIP and geographical info", func() {
			os.Setenv("SCRIPT_PATH", "../../scripts")
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

			Expect(k8sClient.Delete(ctx, cluster)).Should(Succeed())

			Eventually(func() error {
				err = k8sClient.Get(ctx,
					types.NamespacedName{Name: operatorClusterRoleName},
					clusterRole)
				if err != nil {
					return err
				}
				return nil
			}, time.Second*120, time.Millisecond*500).ShouldNot(HaveOccurred())

			Eventually(func() error {
				cleanUpJob = &batchv1.Job{}
				err = k8sClient.Get(ctx,
					types.NamespacedName{Name: deregisterJobName, Namespace: CONTROL_PLANE_NS},
					cleanUpJob)
				if err != nil {
					return err
				}
				return nil
			}, time.Second*120, time.Millisecond*500).ShouldNot(HaveOccurred())
		})
	})
})
