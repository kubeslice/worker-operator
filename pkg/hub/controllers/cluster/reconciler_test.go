package cluster

import (
	"context"
	"runtime/debug"
	"testing"

	hubv1alpha1 "github.com/kubeslice/apis/pkg/controller/v1alpha1"
	mevents "github.com/kubeslice/kubeslice-monitoring/pkg/events"
	"github.com/kubeslice/kubeslice-monitoring/pkg/metrics"
	"github.com/kubeslice/worker-operator/controllers"
	ossEvents "github.com/kubeslice/worker-operator/events"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	cl "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func init() {
	_ = hubv1alpha1.AddToScheme(scheme.Scheme)
}

var clusterObj = &hubv1alpha1.Cluster{
	ObjectMeta: metav1.ObjectMeta{
		Name:              "test-cluster",
		Namespace:         "kubeslice-avesha",
		DeletionTimestamp: &metav1.Time{Time: metav1.Now().Time},
		Finalizers: []string{
			"controller.kubeslice.io/cluster-finalizer",
		},
	},
	Spec: hubv1alpha1.ClusterSpec{
		NodeIPs: []string{
			"192.168.0.1",
			"192.168.0.2",
		},
		ClusterProperty: hubv1alpha1.ClusterProperty{},
	},
}

func TestCheckFinalizerInClusterCr(t *testing.T) {
	tests := []struct {
		name     string
		obj      cl.Object
		req      reconcile.Request
		expected bool
		errMsg   string
	}{
		{
			"reconiler should able to detect finalizer in cluster CR",
			clusterObj,
			reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      clusterObj.Name,
					Namespace: clusterObj.Namespace,
				},
			},
			true,
			"can't fetch node list , length of node items is zero",
		},
		{
			"reconiler should not able to detect finalizer in cluster CR",
			&hubv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-cluster-without-finalizer",
					Namespace:  clusterObj.Namespace,
					Finalizers: []string{},
				},
				Spec: hubv1alpha1.ClusterSpec{},
			},
			reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-cluster-without-finalizer",
					Namespace: clusterObj.Namespace,
				},
			},
			false,
			"can't fetch node list , length of node items is zero",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().
				WithObjects(tt.obj).
				Build()

			testClusterEventRecorder := mevents.NewEventRecorder(fakeClient, fakeClient.Scheme(), ossEvents.EventsMap, mevents.EventRecorderOptions{
				Cluster:   tt.obj.GetName(),
				Project:   "avesha",
				Component: "worker-operator",
				Namespace: controllers.ControlPlaneNamespace,
			})
			mf, _ := metrics.NewMetricsFactory(
				ctrlmetrics.Registry,
				metrics.MetricsFactoryOptions{
					Project:             "avesha",
					Cluster:             tt.obj.GetName(),
					ReportingController: "worker-operator",
					Namespace:           controllers.ControlPlaneNamespace,
				},
			)
			clusterReconciler := NewReconciler(
				fakeClient,
				fakeClient,
				testClusterEventRecorder,
				mf,
			)
			ctx := context.Background()
			_, err := clusterReconciler.Reconcile(ctx, tt.req)
			if err != nil {
				AssertEqual(t, tt.errMsg, err.Error())
			}
			newCluster := &hubv1alpha1.Cluster{}
			err = clusterReconciler.Client.Get(ctx, tt.req.NamespacedName, newCluster)
			AssertNoError(t, err)
			// calling checkFinalizer func to verify if finalizer is present
			isPresent := clusterReconciler.checkFinalizer(ctx, newCluster)
			// if clusterObj contains Finalizers and the deletion timestamp is not zero return true
			if len(newCluster.Finalizers) != 0 && !newCluster.DeletionTimestamp.IsZero() {
				AssertEqual(t, isPresent, tt.expected)
			} else {
				AssertEqual(t, isPresent, tt.expected)
			}
		})
	}
}

func AssertEqual(t *testing.T, actual interface{}, expected interface{}) {
	t.Helper()
	if actual != expected {
		t.Log("expected --", expected, "actual --", actual)
		t.Fail()
	}
}

func AssertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Errorf("Expected No Error but got %s, Stack:\n%s", err, string(debug.Stack()))
	}
}
