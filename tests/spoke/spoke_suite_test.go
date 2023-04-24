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
	"path/filepath"
	"testing"
	"time"

	"github.com/kubeslice/kubeslice-monitoring/pkg/metrics"
	nsmv1 "github.com/networkservicemesh/sdk-k8s/pkg/tools/k8s/apis/networkservicemesh.io/v1"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	hubv1alpha1 "github.com/kubeslice/apis/pkg/controller/v1alpha1"
	mevents "github.com/kubeslice/kubeslice-monitoring/pkg/events"
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	"github.com/kubeslice/worker-operator/controllers/serviceexport"
	"github.com/kubeslice/worker-operator/controllers/serviceimport"
	"github.com/kubeslice/worker-operator/controllers/slice"
	"github.com/kubeslice/worker-operator/controllers/slicegateway"
	ossEvents "github.com/kubeslice/worker-operator/events"
	"github.com/kubeslice/worker-operator/pkg/cluster"
	"github.com/kubeslice/worker-operator/pkg/events"
	hub "github.com/kubeslice/worker-operator/pkg/hub/hubclient"
	namespace "github.com/kubeslice/worker-operator/pkg/namespace/controllers"
	"github.com/kubeslice/worker-operator/pkg/networkpolicy"
	hce "github.com/kubeslice/worker-operator/tests/emulator/hubclient"
	workernetop "github.com/kubeslice/worker-operator/tests/emulator/workerclient/netop"
	workerrouter "github.com/kubeslice/worker-operator/tests/emulator/workerclient/router"
	workergw "github.com/kubeslice/worker-operator/tests/emulator/workerclient/sidecargw"
	istiov1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	//+kubebuilder:scaffold:imports
)

func TestWorker(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Worker Controller Suite")
}

var k8sClient client.Client
var testEnv *envtest.Environment
var ctx context.Context
var cancel context.CancelFunc
var workerClientSidecarGwEmulator *workergw.ClientEmulator
var workerClientRouterEmulator *workerrouter.ClientEmulator
var workerClientNetopEmulator *workernetop.ClientEmulator

const CONTROL_PLANE_NS = "kubeslice-system"
const PROJECT_NS = "kubeslice-cisco"

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

	err = kubeslicev1beta1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = hubv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = istiov1beta1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = nsmv1.AddToScheme(scheme.Scheme)
	Expect(err).ToNot(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	hubClientEmulator, err := hce.NewHubClientEmulator(k8sClient)
	Expect(err).ToNot(HaveOccurred())

	// Create control plane namespace
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: CONTROL_PLANE_NS,
		},
	}
	Expect(k8sClient.Create(ctx, ns)).Should(Succeed())

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	Expect(err).ToNot(HaveOccurred())

	workerClientSidecarGwEmulator, err = workergw.NewClientEmulator()
	Expect(err).ToNot(HaveOccurred())

	workerClientRouterEmulator, err = workerrouter.NewClientEmulator()
	Expect(err).ToNot(HaveOccurred())

	workerClientNetopEmulator, err = workernetop.NewClientEmulator()
	Expect(err).ToNot(HaveOccurred())

	mf, err := metrics.NewMetricsFactory(ctrlmetrics.Registry, metrics.MetricsFactoryOptions{
		Cluster:             "cluster-test",
		Project:             PROJECT_NS,
		ReportingController: "worker-operator",
		Namespace:           CONTROL_PLANE_NS,
	})
	Expect(err).ToNot(HaveOccurred())

	err = (&slice.SliceReconciler{
		Client: k8sManager.GetClient(),
		Scheme: k8sManager.GetScheme(),
		Log:    ctrl.Log.WithName("SliceTest"),
		EventRecorder: mevents.NewEventRecorder(k8sClient, k8sManager.GetScheme(), ossEvents.EventsMap, mevents.EventRecorderOptions{
			Cluster:   hub.ClusterName,
			Project:   PROJECT_NS,
			Component: "worker-operator",
			Namespace: CONTROL_PLANE_NS,
		}),
		HubClient:          hubClientEmulator,
		WorkerRouterClient: workerClientRouterEmulator,
		WorkerNetOpClient:  workerClientNetopEmulator,
	}).Setup(k8sManager, mf)
	Expect(err).ToNot(HaveOccurred())

	testSvcExEventRecorder := events.NewEventRecorder(k8sManager.GetEventRecorderFor("test-SvcEx-controller"))

	err = (&serviceexport.Reconciler{
		Client:        k8sManager.GetClient(),
		Scheme:        k8sManager.GetScheme(),
		Log:           ctrl.Log.WithName("SliceGwTest"),
		HubClient:     hubClientEmulator,
		EventRecorder: testSvcExEventRecorder,
	}).Setup(k8sManager, mf)
	Expect(err).ToNot(HaveOccurred())

	err = (&slicegateway.SliceGwReconciler{
		Client: k8sClient,
		Scheme: k8sClient.Scheme(),
		Log:    ctrl.Log.WithName("SliceTest"),
		EventRecorder: &events.EventRecorder{
			Recorder: &record.FakeRecorder{},
		},
		HubClient:             hubClientEmulator,
		WorkerGWSidecarClient: workerClientSidecarGwEmulator,
		WorkerRouterClient:    workerClientRouterEmulator,
		WorkerNetOpClient:     workerClientNetopEmulator,
		NumberOfGateways:      2,
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&serviceimport.Reconciler{
		Client: k8sManager.GetClient(),
		Scheme: k8sManager.GetScheme(),
		Log:    ctrl.Log.WithName("SvcImTest"),
		EventRecorder: &events.EventRecorder{
			Recorder: &record.FakeRecorder{},
		},
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&namespace.Reconciler{
		Client: k8sManager.GetClient(),
		Scheme: k8sManager.GetScheme(),
		Log:    ctrl.Log.WithName("NamespaceTest"),
		EventRecorder: &events.EventRecorder{
			Recorder: &record.FakeRecorder{},
		},
		Hubclient: &hub.HubClientConfig{
			Client: k8sClient,
		},
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	netpolEventRecorder := mevents.NewEventRecorder(k8sClient, k8sManager.GetScheme(), ossEvents.EventsMap, mevents.EventRecorderOptions{
		Cluster:   hub.ClusterName,
		Project:   PROJECT_NS,
		Component: "networkpolicy-reconciler",
		Namespace: CONTROL_PLANE_NS,
	})
	err = (&networkpolicy.NetpolReconciler{
		Client:        k8sManager.GetClient(),
		Log:           ctrl.Log.WithName("controllers").WithName("networkpolicy"),
		Scheme:        k8sManager.GetScheme(),
		EventRecorder: netpolEventRecorder,
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&cluster.NodeReconciler{
		Client: k8sManager.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("node reconciller"),
	}).SetupWithManager(k8sManager)
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
