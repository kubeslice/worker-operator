package hub_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	meshv1beta1 "bitbucket.org/realtimeai/kubeslice-operator/api/v1beta1"
	"bitbucket.org/realtimeai/kubeslice-operator/internal/hub/controllers"
	"bitbucket.org/realtimeai/kubeslice-operator/pkg/events"
	spokev1alpha1 "bitbucket.org/realtimeai/mesh-apis/pkg/spoke/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

func TestHub(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Hub Controller Suite")
}

var cfg *rest.Config
var k8sClient client.Client
var testEnv *envtest.Environment
var ctx context.Context
var cancel context.CancelFunc

const CONTROL_PLANE_NS = "kubeslice-system"
const PROJECT_NS = "project-example"
const CLUSTER_NAME = "cluster-test"

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
	ctx, cancel = context.WithCancel(context.TODO())

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("../..", "config", "crd", "bases"),
			filepath.Join("../files/crd"),
		},
		ErrorIfCRDPathMissing: true,
		CRDInstallOptions: envtest.CRDInstallOptions{
			MaxTime: 60 * time.Second,
		},
	}

	cfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = meshv1beta1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = spokev1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	// Create control plane namespace
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: CONTROL_PLANE_NS,
		},
	}
	Expect(k8sClient.Create(ctx, ns)).Should(Succeed())

	// Create project NS where hub cluster resources will reside
	ns = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: PROJECT_NS,
		},
	}
	Expect(k8sClient.Create(ctx, ns)).Should(Succeed())

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:             scheme.Scheme,
		Namespace:          PROJECT_NS,
		MetricsBindAddress: "0",
	})
	Expect(err).ToNot(HaveOccurred())

	//k8sMeshClient, err := client.New(cfg, client.Options{Scheme: scheme.Scheme})
	//Expect(err).NotTo(HaveOccurred())
	//Expect(k8sMeshClient).NotTo(BeNil())
	sr := &controllers.SliceReconciler{
		MeshClient: k8sClient,
		Log:        ctrl.Log.WithName("hub").WithName("controllers").WithName("SliceConfig"),
		EventRecorder: &events.EventRecorder{
			Recorder: &record.FakeRecorder{},
		},
	}

	sgwr := &controllers.SliceGwReconciler{
		MeshClient: k8sClient,
		EventRecorder: &events.EventRecorder{
			Recorder: &record.FakeRecorder{},
		},
	}

	err = builder.
		ControllerManagedBy(k8sManager).
		For(&spokev1alpha1.SpokeSliceConfig{}).
		WithEventFilter(predicate.NewPredicateFuncs(func(object client.Object) bool {
			return object.GetLabels()["spoke-cluster"] == CLUSTER_NAME
		})).
		Complete(sr)
	Expect(err).ToNot(HaveOccurred())

	err = builder.
		ControllerManagedBy(k8sManager).
		For(&spokev1alpha1.SpokeSliceGateway{}).
		WithEventFilter(predicate.NewPredicateFuncs(func(object client.Object) bool {
			return object.GetLabels()["spoke-cluster"] == CLUSTER_NAME
		})).
		Complete(sgwr)
	Expect(err).ToNot(HaveOccurred())

	go func() {
		defer GinkgoRecover()
		err = k8sManager.Start(ctx)
		Expect(err).ToNot(HaveOccurred(), "failed to run manager")
	}()
})

var _ = AfterSuite(func() {
	cancel()
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})
