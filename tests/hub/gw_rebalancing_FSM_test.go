package hub_test

import (
	"time"

	spokev1alpha1 "github.com/kubeslice/apis/pkg/worker/v1alpha1"
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	"github.com/kubeslice/worker-operator/controllers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// install workerslicegwrecyler crd
// setup initial test structure
// different tests

var _ = Describe("GW_Reblancing_FSM", func() {

	Context("With WorkerSliceGwRecyler created on worker cluster", func() {
		sliceGw := &kubeslicev1beta1.SliceGateway{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-slicegw-redundancy",
				Namespace: CONTROL_PLANE_NS,
			},
			Spec: kubeslicev1beta1.SliceGatewaySpec{
				SliceName: "test-slice-redundancy",
			},
		}
		Context("SliceGw-Client", func() {
			var workerslicegwrecycler spokev1alpha1.WorkerSliceGwRecycler
			BeforeEach(func() {
				workerslicegwrecycler = spokev1alpha1.WorkerSliceGwRecycler{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-workerslicegwrecycler",
						Namespace: PROJECT_NS,
					},
					Spec: spokev1alpha1.WorkerSliceGwRecyclerSpec{
						SliceGwServer: "slicegw-server",
						SliceGwClient: "test-slicegw-redundancy",
						GwPair: spokev1alpha1.GwPair{
							ClientID: sliceGw.Name,
							ServerID: "xxxx",
						},
						State:     "init",
						Request:   "spawn_new_gw_pod",
						SliceName: sliceGw.Spec.SliceName,
					},
				}
				DeferCleanup(func() {
					Expect(k8sClient.Delete(ctx, sliceGw)).Should(Succeed())
					Expect(k8sClient.Delete(ctx, &workerslicegwrecycler)).Should(Succeed())
				})
			})
			createdSlicegw := kubeslicev1beta1.SliceGateway{}

			It("should spin up new gw pod", func() {
				// create slicegw
				Expect(k8sClient.Create(ctx, sliceGw)).Should(Succeed())
				// update the status to be client
				Eventually(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Namespace: CONTROL_PLANE_NS, Name: sliceGw.Name}, &createdSlicegw)
					if err != nil {
						return false
					}
					createdSlicegw.Status.Config.SliceGatewayHostType = "Client"
					return k8sClient.Status().Update(ctx, &createdSlicegw) == nil
				}, time.Second*10, time.Millisecond*250).Should(BeTrue())
				// spin up a gw pod
				Expect(k8sClient.Create(ctx, getSliceGwPod(*sliceGw)))
				// create workerslicegwrecycler
				Expect(k8sClient.Create(ctx, &workerslicegwrecycler)).Should(Succeed())
				// it should label the gw pod to be deleted in init state
				createdgwPod := v1.Pod{}
				Eventually(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Namespace: CONTROL_PLANE_NS, Name: sliceGw.Name}, &createdgwPod)
					if err != nil {
						return false
					}
					return createdgwPod.GetLabels()["kubeslice.io/pod-type"] == "toBeDeleted"
				}, time.Second*60, time.Millisecond*250).Should(BeTrue())

			})
		})
		Context("SliceGw-Server", func() {
			var workerslicegwrecycler spokev1alpha1.WorkerSliceGwRecycler
			sliceGw := &kubeslicev1beta1.SliceGateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slicegw-redundancy",
					Namespace: CONTROL_PLANE_NS,
				},
				Spec: kubeslicev1beta1.SliceGatewaySpec{
					SliceName: "test-slice-redundancy",
				},
			}
			BeforeEach(func() {
				workerslicegwrecycler = spokev1alpha1.WorkerSliceGwRecycler{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-workerslicegwrecycler",
						Namespace: PROJECT_NS,
					},
					Spec: spokev1alpha1.WorkerSliceGwRecyclerSpec{
						SliceGwServer: "test-slicegw-redundancy",
						SliceGwClient: "test-slicegw-redundancy",
						GwPair: spokev1alpha1.GwPair{
							ClientID: sliceGw.Name,
							ServerID: sliceGw.Name,
						},
						State:     "init",
						Request:   "spawn_new_gw_pod",
						SliceName: sliceGw.Spec.SliceName,
					},
				}
				DeferCleanup(func() {
					Expect(k8sClient.Delete(ctx, sliceGw)).Should(Succeed())
					Expect(k8sClient.Delete(ctx, &workerslicegwrecycler)).Should(Succeed())
				})
			})

			createdSlicegw := kubeslicev1beta1.SliceGateway{}
			It("should spin up new gw pod", func() {
				// create slicegw
				Expect(k8sClient.Create(ctx, sliceGw)).Should(Succeed())
				// update the status to be client
				Eventually(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Namespace: CONTROL_PLANE_NS, Name: sliceGw.Name}, &createdSlicegw)
					if err != nil {
						return false
					}
					createdSlicegw.Status.Config.SliceGatewayHostType = "Server"
					return k8sClient.Status().Update(ctx, &createdSlicegw) == nil
				}, time.Second*10, time.Millisecond*250).Should(BeTrue())
				// spin up a gw pod
				Expect(k8sClient.Create(ctx, getSliceGwPod(*sliceGw)))
				// create workerslicegwrecycler
				Expect(k8sClient.Create(ctx, &workerslicegwrecycler)).Should(Succeed())
				// it should label the gw pod to be deleted in init state
				createdgwPod := v1.Pod{}
				Eventually(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Namespace: CONTROL_PLANE_NS, Name: sliceGw.Name}, &createdgwPod)
					if err != nil {
						return false
					}
					return createdgwPod.GetLabels()["kubeslice.io/pod-type"] == "toBeDeleted"
				}, time.Second*60, time.Millisecond*250).Should(BeTrue())

			})
		})
	})
	//NOTE: remaining test cases make call to other components like router sidecar , which will be covered in E2E tests.
})

func getSliceGwPod(slicegw kubeslicev1beta1.SliceGateway) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      slicegw.Name,
			Namespace: CONTROL_PLANE_NS,
			Labels: map[string]string{
				"kubeslice.io/pod-type": "gateway",
			},
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  "kubeslice-sidecar",
					Image: "nginx",
				},
			},
		},
	}
}

func labelsForSliceGwDeployment(name string, slice string) map[string]string {
	return map[string]string{
		"networkservicemesh.io/app":                      name,
		"kubeslice.io/pod-type":                          "slicegateway",
		controllers.ApplicationNamespaceSelectorLabelKey: slice,
		"kubeslice.io/slice-gw":                          name,
	}
}
