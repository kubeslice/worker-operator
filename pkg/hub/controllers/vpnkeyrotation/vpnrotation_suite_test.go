package vpnkeyrotation

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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	hubv1alpha1 "github.com/kubeslice/apis/pkg/controller/v1alpha1"
	spokev1alpha1 "github.com/kubeslice/apis/pkg/worker/v1alpha1"
	mevents "github.com/kubeslice/kubeslice-monitoring/pkg/events"
	"github.com/kubeslice/kubeslice-monitoring/pkg/metrics"
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	"github.com/kubeslice/worker-operator/controllers/slice"
	"github.com/kubeslice/worker-operator/controllers/slicegateway"
	ossEvents "github.com/kubeslice/worker-operator/events"
	"github.com/kubeslice/worker-operator/pkg/hub/controllers"
	hce "github.com/kubeslice/worker-operator/tests/emulator/hubclient"
	workernetop "github.com/kubeslice/worker-operator/tests/emulator/workerclient/netop"
	workerrouter "github.com/kubeslice/worker-operator/tests/emulator/workerclient/router"
	workergw "github.com/kubeslice/worker-operator/tests/emulator/workerclient/sidecargw"
	nsmv1 "github.com/networkservicemesh/sdk-k8s/pkg/tools/k8s/apis/networkservicemesh.io/v1"
	"github.com/prometheus/client_golang/prometheus"
)

var workerClientSidecarGwEmulator *workergw.ClientEmulator
var workerClientRouterEmulator *workerrouter.ClientEmulator
var workerClientNetopEmulator *workernetop.ClientEmulator

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
			filepath.Join("./crds"),
		},
		ErrorIfCRDPathMissing: true,
		CRDInstallOptions: envtest.CRDInstallOptions{
			MaxTime: 60 * time.Second,
		},
	}

	os.Setenv("CLUSTER_NAME", CLUSTER_NAME)
	cfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = kubeslicev1beta1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = spokev1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = hubv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = nsmv1.AddToScheme((scheme.Scheme))
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
	os.Setenv("HUB_PROJECT_NAMESPACE", ns.Name)

	Expect(k8sClient.Create(ctx, ns)).Should(Succeed())

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:             scheme.Scheme,
		Namespace:          PROJECT_NS,
		MetricsBindAddress: "0",
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
	rotationReconciler := NewReconciler(
		k8sClient,
		k8sClient,
		&spokeClusterEventRecorder,
		mf,
	)
	err = builder.
		ControllerManagedBy(k8sManager).
		For(&hubv1alpha1.VpnKeyRotation{}).
		Complete(rotationReconciler)
	if err != nil {
		os.Exit(1)
	}

	hubClientEmulator, err := hce.NewHubClientEmulator(k8sClient)
	Expect(err).ToNot(HaveOccurred())

	workerClientSidecarGwEmulator, err = workergw.NewClientEmulator()
	Expect(err).ToNot(HaveOccurred())

	workerClientRouterEmulator, err = workerrouter.NewClientEmulator()
	Expect(err).ToNot(HaveOccurred())

	workerClientNetopEmulator, err = workernetop.NewClientEmulator()
	Expect(err).ToNot(HaveOccurred())

	err = (&slicegateway.SliceGwReconciler{
		Client:                k8sClient,
		Scheme:                k8sClient.Scheme(),
		Log:                   ctrl.Log.WithName("SliceGwTest"),
		EventRecorder:         &spokeClusterEventRecorder,
		HubClient:             hubClientEmulator,
		WorkerGWSidecarClient: workerClientSidecarGwEmulator,
		WorkerRouterClient:    workerClientRouterEmulator,
		WorkerNetOpClient:     workerClientNetopEmulator,
		NumberOfGateways:      2,
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&slice.SliceReconciler{
		Client:             k8sManager.GetClient(),
		Scheme:             k8sManager.GetScheme(),
		Log:                ctrl.Log.WithName("SliceTest"),
		EventRecorder:      &spokeClusterEventRecorder,
		HubClient:          hubClientEmulator,
		WorkerRouterClient: workerClientRouterEmulator,
		WorkerNetOpClient:  workerClientNetopEmulator,
	}).SetupWithManager(k8sManager)

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
