package cluster

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	hubv1alpha1 "github.com/kubeslice/apis/pkg/controller/v1alpha1"
	spokev1alpha1 "github.com/kubeslice/apis/pkg/worker/v1alpha1"
	mevents "github.com/kubeslice/kubeslice-monitoring/pkg/events"
	"github.com/kubeslice/kubeslice-monitoring/pkg/metrics"
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	ossEvents "github.com/kubeslice/worker-operator/events"
	"github.com/kubeslice/worker-operator/pkg/hub/controllers"
	"github.com/prometheus/client_golang/prometheus"
)

func TestHub(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Hub Controller Suite")
}

var k8sClient client.Client
var testEnv *envtest.Environment
var ctx context.Context
var cancel context.CancelFunc

const CONTROL_PLANE_NS = "kubeslice-system"
const PROJECT_NS = "project-example"
const CLUSTER_NAME = "cluster-test"

var MetricRegistry = prometheus.NewRegistry()

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
	ctx, cancel = context.WithCancel(context.TODO())

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("../../../..", "config", "crd", "bases"),
			filepath.Join("./crds"),
		},
		ErrorIfCRDPathMissing: true,
		CRDInstallOptions: envtest.CRDInstallOptions{
			MaxTime: 60 * time.Second,
		},
	}

	cfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = kubeslicev1beta1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = spokev1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = hubv1alpha1.AddToScheme(scheme.Scheme)
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

	cacheOptions := cache.Options{
		DefaultNamespaces: map[string]cache.Config{
			PROJECT_NS: {},
		},
	}
	metricsServer := metricsserver.Options{
		BindAddress: "0",
	}

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:  scheme.Scheme,
		Cache:   cacheOptions,
		Metrics: metricsServer,
	})
	Expect(err).ToNot(HaveOccurred())

	mf, _ := metrics.NewMetricsFactory(
		MetricRegistry,
		metrics.MetricsFactoryOptions{
			Project:             PROJECT_NS,
			Cluster:             CLUSTER_NAME,
			ReportingController: "worker-operator",
			Namespace:           controllers.ControlPlaneNamespace,
		},
	)
	spokeClusterEventRecorder := mevents.NewEventRecorder(k8sClient, k8sManager.GetScheme(), ossEvents.EventsMap, mevents.EventRecorderOptions{
		Cluster:   CLUSTER_NAME,
		Project:   PROJECT_NS,
		Component: "worker-operator",
		Namespace: CONTROL_PLANE_NS,
	})
	clusterReconciler := NewReconciler(
		k8sClient,
		&spokeClusterEventRecorder,
		mf,
	)
	clusterReconciler.ReconcileInterval = 5 * time.Second
	err = builder.
		ControllerManagedBy(k8sManager).
		For(&hubv1alpha1.Cluster{}).
		Complete(clusterReconciler)
	if err != nil {
		os.Exit(1)
	}
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
