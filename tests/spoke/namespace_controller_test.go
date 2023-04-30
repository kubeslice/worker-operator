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

	"k8s.io/apimachinery/pkg/api/errors"

	hubv1alpha1 "github.com/kubeslice/apis/pkg/controller/v1alpha1"
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
)

var _ = Describe("ClusterInfoUpdate", func() {
	Context("With Namespace Created at spoke cluster", func() {
		var ns *corev1.Namespace
		var slice *kubeslicev1beta1.Slice
		var cluster *hubv1alpha1.Cluster
		var applicationNS *corev1.Namespace

		BeforeEach(func() {
			ns = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "kubeslice-kubeslice-1",
				},
			}
			os.Setenv("HUB_PROJECT_NAMESPACE", ns.Name)
			cluster = &hubv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-1",
					Namespace: ns.Name,
				},
				Spec:   hubv1alpha1.ClusterSpec{},
				Status: hubv1alpha1.ClusterStatus{},
			}
			os.Setenv("CLUSTER_NAME", cluster.Name)
			slice = &kubeslicev1beta1.Slice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slice",
					Namespace: "kubeslice-kubeslice-1",
				},
				Spec:   kubeslicev1beta1.SliceSpec{},
				Status: kubeslicev1beta1.SliceStatus{},
			}
			applicationNS = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "application-namespace",
					Labels: map[string]string{"kubeslice.io/slice": "test-slice"},
				},
			}

			DeferCleanup(func() {
				ctx := context.Background()
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
				Expect(k8sClient.Delete(ctx, slice)).Should(Succeed())
				Expect(k8sClient.Delete(ctx, ns)).Should(Succeed())
				Expect(k8sClient.Delete(ctx, applicationNS)).Should(Succeed())
			})
		})

		It("Should update cluster CR with namespace", func() {
			Expect(k8sClient.Create(ctx, ns)).Should(Succeed())
			Expect(k8sClient.Create(ctx, cluster)).Should(Succeed())
			Expect(k8sClient.Create(ctx, slice)).Should(Succeed())
			Expect(k8sClient.Create(ctx, applicationNS)).Should(Succeed())
			//get the cluster object
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name: cluster.Name, Namespace: ns.Name,
				}, cluster)
				if err != nil {
					return false
				}
				idAdded := false
				for _, v := range cluster.Status.Namespaces {
					if v.Name == applicationNS.Name && v.SliceName == slice.Name {
						idAdded = true
					}
				}
				return idAdded
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())
		})

	})
	Context("With Namespace Created at spoke cluster", func() {
		var ns *corev1.Namespace
		var cluster *hubv1alpha1.Cluster
		var applicationNS *corev1.Namespace
		BeforeEach(func() {
			ns = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "kubeslice-kubeslice-3",
				},
			}
			os.Setenv("HUB_PROJECT_NAMESPACE", ns.Name)
			cluster = &hubv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-1",
					Namespace: ns.Name,
				},
				Spec: hubv1alpha1.ClusterSpec{},
				Status: hubv1alpha1.ClusterStatus{
					Namespaces: []hubv1alpha1.NamespacesConfig{{Name: "application-namespace-2"}},
				},
			}
			os.Setenv("CLUSTER_NAME", cluster.Name)

			applicationNS = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "application-namespace-2",
				},
			}
			DeferCleanup(func() {
				ctx := context.Background()
				k8sClient.Delete(ctx, cluster)
				Expect(k8sClient.Delete(ctx, ns)).Should(Succeed())
				Expect(k8sClient.Delete(ctx, applicationNS)).Should(Succeed())
			})
		})
		It("Should not update cluster CR with namespace", func() {
			Expect(k8sClient.Create(ctx, ns)).Should(Succeed())
			Expect(k8sClient.Create(ctx, cluster)).Should(Succeed())
			Expect(k8sClient.Create(ctx, applicationNS)).Should(Succeed())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name: cluster.Name, Namespace: ns.Name,
				}, cluster)
				if err != nil {
					return false
				}
				count := 0
				// the namespace is already present in cluster CR,
				// hence it should not be duplicated when a same namepace gets reconciled again
				// the cound should be one
				for _, v := range cluster.Status.Namespaces {
					if v.Name == applicationNS.Name {
						count++
					}
				}
				return count == 1
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

		})
	})
	Context("With Namespace Deleted at spoke cluster", func() {
		var ns *corev1.Namespace
		var cluster *hubv1alpha1.Cluster
		var applicationNSToDelete *corev1.Namespace

		BeforeEach(func() {
			ns = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "kubeslice-kubeslice-2",
				},
			}
			os.Setenv("HUB_PROJECT_NAMESPACE", ns.Name)

			cluster = &hubv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-1",
					Namespace: ns.Name,
				},
				Spec:   hubv1alpha1.ClusterSpec{},
				Status: hubv1alpha1.ClusterStatus{},
			}
			os.Setenv("CLUSTER_NAME", cluster.Name)
			applicationNSToDelete = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "application-spoke-1",
				},
			}

			DeferCleanup(func() {
				ctx := context.Background()
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
			})

		})

		It("Should update cluster CR with namespace", func() {
			Expect(k8sClient.Create(ctx, ns)).Should(Succeed())
			Expect(k8sClient.Create(ctx, cluster)).Should(Succeed())
			Expect(k8sClient.Create(ctx, applicationNSToDelete)).Should(Succeed())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: ns.Name}, cluster)
				if err != nil {
					return false
				}
				return len(cluster.Status.Namespaces) > 0
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			Expect(k8sClient.Delete(ctx, applicationNSToDelete)).Should(Succeed())
			// Normally the kube-controller-manager would handle finalization
			// and garbage collection of namespaces, but with envtest, we aren't
			// running a kube-controller-manager. Instead we're gonna approximate
			// (poorly) the kube-controller-manager by explicitly deleting some
			// resources within the namespace and then removing the `kubernetes`
			// finalizer from the namespace resource so it can finish deleting.
			// Note that any resources within the namespace that we don't
			// successfully delete could reappear if the namespace is ever
			// recreated with the same name.

			clientGo, err := kubernetes.NewForConfig(testEnv.Config)
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: applicationNSToDelete.Name},
					applicationNSToDelete)
				if err != nil {
					return err
				}
				applicationNSToDelete.Spec = corev1.NamespaceSpec{}
				applicationNSToDelete.Spec.Finalizers = nil
				// We have to use the k8s.io/client-go library here to expose
				// ability to patch the /finalize subresource on the namespace
				_, err = clientGo.CoreV1().Namespaces().Finalize(ctx, applicationNSToDelete, metav1.UpdateOptions{})
				return err
			}, time.Second*10, time.Millisecond*250).Should(BeNil())

			Expect(err).To(BeNil())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: applicationNSToDelete.Name}, applicationNSToDelete)
				return errors.IsNotFound(err)
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: ns.Name}, cluster)
				if err != nil {
					return false
				}
				isDeleted := true
				for _, v := range cluster.Status.Namespaces {
					if v.Name == applicationNSToDelete.Name {
						isDeleted = false
					}
				}
				return isDeleted
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())
		})
	})
	Context("With Existing Namespace at spoke cluster and no slice name", func() {
		var ns *corev1.Namespace
		var cluster *hubv1alpha1.Cluster
		var applicationNS *corev1.Namespace

		BeforeEach(func() {
			ns = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "kubeslice-kubeslice-4",
				},
			}
			os.Setenv("HUB_PROJECT_NAMESPACE", ns.Name)
			cluster = &hubv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-1",
					Namespace: ns.Name,
				},
				Spec:   hubv1alpha1.ClusterSpec{},
				Status: hubv1alpha1.ClusterStatus{},
			}
			os.Setenv("CLUSTER_NAME", cluster.Name)
			applicationNS = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "application-namespace-5"},
			}
			DeferCleanup(func() {
				ctx := context.Background()
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
			})
		})
		It("Should update cluster CR with updated application namespace", func() {
			sliceName := "test-slice"
			Expect(k8sClient.Create(ctx, ns)).Should(Succeed())
			Expect(k8sClient.Create(ctx, cluster)).Should(Succeed())
			Expect(k8sClient.Create(ctx, applicationNS)).Should(Succeed())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name: cluster.Name, Namespace: ns.Name,
				}, cluster)
				if err != nil {
					return false
				}
				idAdded := false
				for _, v := range cluster.Status.Namespaces {
					if v.Name == applicationNS.Name {
						idAdded = true
					}
				}
				return idAdded
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())
			Eventually(func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name: applicationNS.Name,
				}, applicationNS)
				if err != nil {
					return err
				}
				applicationNS.Labels = map[string]string{"kubeslice.io/slice": sliceName}
				return k8sClient.Status().Update(ctx, applicationNS)
			}, time.Second*10, time.Millisecond*250).Should(BeNil())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name: cluster.Name, Namespace: ns.Name,
				}, cluster)
				if err != nil {
					return false
				}
				idAdded := false
				for _, v := range cluster.Status.Namespaces {
					if v.Name == applicationNS.Name && v.SliceName == sliceName {
						idAdded = true
					}
				}
				return idAdded
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())
		})
	})
	Context("With Existing Namespace at spoke cluster and Slice changed", func() {
		var ns *corev1.Namespace
		var slice *kubeslicev1beta1.Slice
		var cluster *hubv1alpha1.Cluster
		var applicationNS *corev1.Namespace

		BeforeEach(func() {
			ns = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "kubeslice-kubeslice-5",
				},
			}
			os.Setenv("HUB_PROJECT_NAMESPACE", ns.Name)
			cluster = &hubv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-1",
					Namespace: ns.Name,
				},
				Spec:   hubv1alpha1.ClusterSpec{},
				Status: hubv1alpha1.ClusterStatus{},
			}
			os.Setenv("CLUSTER_NAME", cluster.Name)
			slice = &kubeslicev1beta1.Slice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slice",
					Namespace: "kubeslice-kubeslice-5",
				},
				Spec:   kubeslicev1beta1.SliceSpec{},
				Status: kubeslicev1beta1.SliceStatus{},
			}
			applicationNS = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "application-namespace-3",
					Labels: map[string]string{"kubeslice.io/slice": "test-slice"},
				},
			}
			DeferCleanup(func() {
				ctx := context.Background()
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
			})
		})
		It("Should update cluster CR with updated slice name", func() {
			Expect(k8sClient.Create(ctx, ns)).Should(Succeed())
			Expect(k8sClient.Create(ctx, cluster)).Should(Succeed())
			Expect(k8sClient.Create(ctx, slice)).Should(Succeed())
			//get the cluster object
			err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name: cluster.Name, Namespace: ns.Name,
				}, cluster)
				if err != nil {
					return err
				}
				cluster.Status = hubv1alpha1.ClusterStatus{
					Namespaces: []hubv1alpha1.NamespacesConfig{{
						Name:      "application-namespace-4",
						SliceName: "test-slice-old"}},
				}
				return k8sClient.Status().Update(ctx, cluster)
			})
			Expect(err).To(BeNil())
			Expect(k8sClient.Create(ctx, applicationNS)).Should(Succeed())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name: cluster.Name, Namespace: ns.Name,
				}, cluster)
				if err != nil {
					return false
				}
				idAdded := false
				for _, v := range cluster.Status.Namespaces {
					if v.Name == applicationNS.Name && v.SliceName == slice.Name {
						idAdded = true
					}
				}
				return idAdded
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())
		})

	})
})
