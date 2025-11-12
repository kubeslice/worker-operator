package e2e

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	monitoringEvents "github.com/kubeslice/kubeslice-monitoring/pkg/events"
	ossEvents "github.com/kubeslice/worker-operator/events"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	controllerv1alpha1 "github.com/kubeslice/apis/pkg/controller/v1alpha1"
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	"github.com/kubeslice/worker-operator/controllers"
	slicecontroller "github.com/kubeslice/worker-operator/controllers/slice"
	slicegatewaycontroller "github.com/kubeslice/worker-operator/controllers/slicegateway"
)

var (
	cfg       *rest.Config
	k8sClient client.Client
	testEnv   *envtest.Environment
	scheme    = runtime.NewScheme()
	mgrCtx    context.Context
	mgrCancel context.CancelFunc
)

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Worker Operator E2E Test Suite")
}

var _ = BeforeSuite(func() {
	log.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "config", "crd", "bases"),
		},
		ErrorIfCRDPathMissing: true,
	}

	var err error
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	Expect(clientgoscheme.AddToScheme(scheme)).To(Succeed())
	Expect(kubeslicev1beta1.AddToScheme(scheme)).To(Succeed())
	Expect(controllerv1alpha1.AddToScheme(scheme)).To(Succeed())

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{Scheme: scheme})
	Expect(err).NotTo(HaveOccurred())
	k8sClient = mgr.GetClient()

	ctx := context.Background()
	createNamespaceIfNotExists(ctx, controllers.ControlPlaneNamespace)
	createDNSSvc(ctx)
	createWorkerConfigMap(ctx)
	createSliceConfigSecret(ctx)

	// Use monitoringEvents.NewEventRecorder (matches main.go)
	recorder := monitoringEvents.NewEventRecorder(
		mgr.GetClient(),
		scheme,
		ossEvents.EventsMap,
		monitoringEvents.EventRecorderOptions{
			Version:   "v1",
			Slice:     "test-slice",
			Cluster:   "test-cluster",
			Project:   "test-project",
			Component: "slice-controller",
			Namespace: controllers.ControlPlaneNamespace,
		},
	)

	// Pass recorder to reconciler (as pointer)
	err = (&slicecontroller.SliceReconciler{
		Client:        mgr.GetClient(),
		Scheme:        mgr.GetScheme(),
		EventRecorder: &recorder,
	}).SetupWithManager(mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&slicegatewaycontroller.SliceGwReconciler{
		Client:        mgr.GetClient(),
		Scheme:        mgr.GetScheme(),
		EventRecorder: &recorder,
	}).SetupWithManager(mgr)
	Expect(err).NotTo(HaveOccurred())

	mgrCtx, mgrCancel = context.WithCancel(context.Background())
	go func() {
		defer GinkgoRecover()
		err = mgr.Start(mgrCtx)
		Expect(err).NotTo(HaveOccurred())
	}()
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	if mgrCancel != nil {
		mgrCancel()
		time.Sleep(200 * time.Millisecond)
	}
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})

//
// Helper functions
//

func createWorkerConfigMap(ctx context.Context) {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubeslice-worker-config",
			Namespace: controllers.ControlPlaneNamespace,
		},
		Data: map[string]string{
			"cluster_name": "test-cluster",
			"project_name": "test-project",
		},
	}
	_ = k8sClient.Create(ctx, cm)
}

func createSliceConfigSecret(ctx context.Context) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubeslice-worker-secret",
			Namespace: controllers.ControlPlaneNamespace,
		},
		Data: map[string][]byte{
			"ca.crt":  []byte("fake-ca"),
			"tls.crt": []byte("fake-cert"),
			"tls.key": []byte("fake-key"),
		},
	}
	_ = k8sClient.Create(ctx, secret)
}

func createNamespaceIfNotExists(ctx context.Context, name string) {
	ns := &corev1.Namespace{}
	err := k8sClient.Get(ctx, client.ObjectKey{Name: name}, ns)
	if err == nil {
		return
	}

	ns = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	Expect(k8sClient.Create(ctx, ns)).To(Succeed())
}

func createTestPod(ctx context.Context, namespace, podName string, labels map[string]string) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "test-container",
					Image: "nginx:latest",
				},
			},
		},
	}
	Expect(k8sClient.Create(ctx, pod)).To(Succeed())
}

func createSlice(ctx context.Context, sliceName string) {
	slice := &kubeslicev1beta1.Slice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sliceName,
			Namespace: controllers.ControlPlaneNamespace,
			Labels: map[string]string{
				"kubeslice.io/origin": "hub",
			},
			Annotations: map[string]string{
				"kubeslice.io/reconciled-from-hub": "true",
			},
		},
		Spec: kubeslicev1beta1.SliceSpec{},
		Status: kubeslicev1beta1.SliceStatus{
			SliceConfig: &kubeslicev1beta1.SliceConfig{
				SliceID:                           "test-id",
				SliceType:                         "test-type",
				SliceDisplayName:                  "test-display",
				SliceOverlayNetworkDeploymentMode: controllerv1alpha1.NONET,
			},
		},
	}

	Expect(k8sClient.Create(ctx, slice)).To(Succeed())
}

func createDNSSvc(ctx context.Context) {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      controllers.DNSDeploymentName,
			Namespace: controllers.ControlPlaneNamespace,
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "10.0.0.10",
			Ports: []corev1.ServicePort{
				{
					Name:     "dns",
					Protocol: corev1.ProtocolUDP,
					Port:     53,
				},
			},
		},
	}
	_ = k8sClient.Create(ctx, svc)
}
