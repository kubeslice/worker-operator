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
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	ossEvents "github.com/kubeslice/worker-operator/events"
	"github.com/kubeslice/worker-operator/pkg/events"
	"github.com/kubeslice/worker-operator/pkg/hub/controllers"
	"github.com/kubeslice/worker-operator/pkg/hub/controllers/cluster"

	"github.com/kubeslice/worker-operator/pkg/hub/controllers/workerslicegwrecycler"
	workerrouter "github.com/kubeslice/worker-operator/tests/emulator/workerclient/router"
	workergw "github.com/kubeslice/worker-operator/tests/emulator/workerclient/sidecargw"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
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

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:             scheme.Scheme,
		Namespace:          PROJECT_NS,
		MetricsBindAddress: "0",
	})
	Expect(err).ToNot(HaveOccurred())

	workerClientSidecarGwEmulator, err := workergw.NewClientEmulator()
	Expect(err).ToNot(HaveOccurred())

	workerClientRouterEmulator, err := workerrouter.NewClientEmulator()
	Expect(err).ToNot(HaveOccurred())

	testSliceEventRecorder := events.NewEventRecorder(k8sManager.GetEventRecorderFor("test-slice-controller"))
	sr := &controllers.SliceReconciler{
		MeshClient:    k8sClient,
		Log:           ctrl.Log.WithName("hub").WithName("controllers").WithName("SliceConfig"),
		EventRecorder: testSliceEventRecorder,
	}

	testSliceGwEventRecorder := events.NewEventRecorder(k8sManager.GetEventRecorderFor("test-slicegw-controller"))
	sgwr := &controllers.SliceGwReconciler{
		MeshClient:    k8sClient,
		EventRecorder: testSliceGwEventRecorder,
		ClusterName:   CLUSTER_NAME,
	}

	err = builder.
		ControllerManagedBy(k8sManager).
		For(&spokev1alpha1.WorkerSliceConfig{}).
		WithEventFilter(predicate.NewPredicateFuncs(func(object client.Object) bool {
			return object.GetLabels()["worker-cluster"] == CLUSTER_NAME
		})).
		Complete(sr)
	Expect(err).ToNot(HaveOccurred())

	err = builder.
		ControllerManagedBy(k8sManager).
		For(&spokev1alpha1.WorkerSliceGateway{}).
		WithEventFilter(predicate.NewPredicateFuncs(func(object client.Object) bool {
			return object.GetLabels()["worker-cluster"] == CLUSTER_NAME
		})).
		Complete(sgwr)
	Expect(err).ToNot(HaveOccurred())

	testSvcimEventRecorder := events.NewEventRecorder(k8sManager.GetEventRecorderFor("test-svcim-controller"))
	serviceImportReconciler := &controllers.ServiceImportReconciler{
		MeshClient:    k8sClient,
		EventRecorder: testSvcimEventRecorder,
		Client:        k8sClient,
	}

	err = builder.
		ControllerManagedBy(k8sManager).
		For(&spokev1alpha1.WorkerServiceImport{}).
		WithEventFilter(predicate.NewPredicateFuncs(func(object client.Object) bool {
			return object.GetLabels()["worker-cluster"] == ns.ClusterName
		})).
		Complete(serviceImportReconciler)
	Expect(err).ToNot(HaveOccurred())

	workerSliceGwRecyclerEventRecorder := events.NewEventRecorder(k8sManager.GetEventRecorderFor("workerslicegwrecycler-controller"))
	if err := (&workerslicegwrecycler.Reconciler{
		MeshClient:            k8sClient,
		Log:                   ctrl.Log.WithName("controllers").WithName("workerslicegwrecycler"),
		Scheme:                k8sManager.GetScheme(),
		Client:                k8sClient,
		WorkerGWSidecarClient: workerClientSidecarGwEmulator,
		WorkerRouterClient:    workerClientRouterEmulator,
		EventRecorder:         workerSliceGwRecyclerEventRecorder,
	}).SetupWithManager(k8sManager); err != nil {
		os.Exit(1)
	}

	spokeClusterEventRecorder := mevents.NewEventRecorder(k8sClient, k8sManager.GetScheme(), ossEvents.EventsMap, mevents.EventRecorderOptions{
		Cluster:   CLUSTER_NAME,
		Project:   PROJECT_NS,
		Component: "worker-operator",
		Namespace: CONTROL_PLANE_NS,
	})
	clusterReconciler := &cluster.Reconciler{
		MeshClient:    k8sClient,
		EventRecorder: spokeClusterEventRecorder,
	}
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
