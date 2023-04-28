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

package main

import (
	"flag"
	"os"

	"github.com/kubeslice/kubeslice-monitoring/pkg/metrics"
	"github.com/kubeslice/worker-operator/controllers"
	"github.com/kubeslice/worker-operator/pkg/events"
	"github.com/kubeslice/worker-operator/pkg/monitoring"
	namespacecontroller "github.com/kubeslice/worker-operator/pkg/namespace/controllers"
	"github.com/prometheus/client_golang/prometheus"
	"go.opencensus.io/stats/view"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.

	_ "k8s.io/client-go/plugin/pkg/client/auth"

	sidecar "github.com/kubeslice/worker-operator/pkg/gwsidecar"
	netop "github.com/kubeslice/worker-operator/pkg/netop"
	router "github.com/kubeslice/worker-operator/pkg/router"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	nsmv1 "github.com/networkservicemesh/sdk-k8s/pkg/tools/k8s/apis/networkservicemesh.io/v1"
	istiov1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"

	ocprom "contrib.go.opencensus.io/exporter/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	"github.com/kubeslice/worker-operator/controllers/serviceexport"
	"github.com/kubeslice/worker-operator/controllers/serviceimport"
	"github.com/kubeslice/worker-operator/controllers/slice"
	"github.com/kubeslice/worker-operator/controllers/slicegateway"
	hub "github.com/kubeslice/worker-operator/pkg/hub/hubclient"
	"github.com/kubeslice/worker-operator/pkg/hub/manager"
	"github.com/kubeslice/worker-operator/pkg/logger"
	"github.com/kubeslice/worker-operator/pkg/networkpolicy"
	"github.com/kubeslice/worker-operator/pkg/utils"
	podwh "github.com/kubeslice/worker-operator/pkg/webhook/pod"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = logger.NewLogger().With("name", "setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(nsmv1.AddToScheme(scheme))
	utilruntime.Must(istiov1beta1.AddToScheme(scheme))
	utilruntime.Must(kubeslicev1beta1.AddToScheme(scheme))
	utilruntime.Must(istiov1beta1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(logger.NewWrappedLogger())
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "f7425d89.kubeslice.io",
		CertDir:                utils.GetEnvOrDefault("WEBHOOK_CERTS_DIR", "/etc/webhook/certs"),
	})

	er := &monitoring.EventRecorder{
		Client:    mgr.GetClient(),
		Scheme:    mgr.GetScheme(),
		Logger:    logger.NewLogger(),
		Cluster:   os.Getenv("CLUSTER_NAME"),
		Component: "worker-operator",
	}

	// Use an environment variable to be able to disable webhooks, so that we can run the operator locally
	if utils.GetEnvOrDefault("ENABLE_WEBHOOKS", "true") == "true" {
		mgr.GetWebhookServer().Register("/mutate-webhook", &webhook.Admission{
			Handler: &podwh.WebhookServer{
				Client:          mgr.GetClient(),
				SliceInfoClient: podwh.NewWebhookClient(),
			},
		})
	}
	if err != nil {
		setupLog.With("error", err).Error("unable to start manager")
		os.Exit(1)
	}

	setupLog.Info("Creating operator metrics exporter")
	exporter, err := ocprom.NewExporter(ocprom.Options{
		Registry: ctrlmetrics.Registry.(*prometheus.Registry),
	})
	if err != nil {
		setupLog.With("error", err).Error("Error while building exporter ..")
	} else {
		view.RegisterExporter(exporter)
		// It helps you to setup customize reporting period to push gateway
		//view.SetReportingPeriod(10 * time.Millisecond)
	}
	hubClient, err := hub.NewHubClientConfig(er)
	if err != nil {
		setupLog.With("error", err).Error("could not create hub client for slice gateway reconciler")
		os.Exit(1)
	}
	workerRouterClient, err := router.NewWorkerRouterClientProvider()
	if err != nil {
		setupLog.With("error", err).Error("could not create spoke router client for slice gateway reconciler")
		os.Exit(1)
	}

	workerNetOPClient, err := netop.NewWorkerNetOpClientProvider()
	if err != nil {
		setupLog.With("error", err).Error("could not create spoke netop client for slice gateway reconciler")
		os.Exit(1)
	}

	clientForHubMgr, err := client.New(ctrl.GetConfigOrDie(), client.Options{
		Scheme: scheme,
	})

	mf, err := metrics.NewMetricsFactory(ctrlmetrics.Registry, metrics.MetricsFactoryOptions{
		Cluster:             controllers.ClusterName,
		Project:             hub.ProjectNamespace,
		ReportingController: "worker-operator",
		Namespace:           controllers.ControlPlaneNamespace,
	})
	if err != nil {
		setupLog.With("error", err).Error("unable to initializ metrics factory")
		os.Exit(1)
	}

	sliceEventRecorder := events.NewEventRecorder(mgr.GetEventRecorderFor("slice-controller"))
	if err = (&slice.SliceReconciler{
		Client:             mgr.GetClient(),
		Log:                ctrl.Log.WithName("controllers").WithName("Slice"),
		Scheme:             mgr.GetScheme(),
		HubClient:          hubClient,
		EventRecorder:      sliceEventRecorder,
		WorkerRouterClient: workerRouterClient,
		WorkerNetOpClient:  workerNetOPClient,
	}).Setup(mgr, mf); err != nil {
		setupLog.With("error", err).Error("unable to create controller", "controller", "Slice")
		os.Exit(1)
	}

	sliceGwEventRecorder := events.NewEventRecorder(mgr.GetEventRecorderFor("sliceGw-controller"))
	workerGWClient, err := sidecar.NewWorkerGWSidecarClientProvider()
	if err != nil {
		setupLog.With("error", err).Error("could not create spoke sidecar gateway client for slice gateway reconciler")
		os.Exit(1)
	}
	if err = (&slicegateway.SliceGwReconciler{
		Client:                mgr.GetClient(),
		Log:                   ctrl.Log.WithName("controllers").WithName("SliceGw"),
		Scheme:                mgr.GetScheme(),
		HubClient:             hubClient,
		WorkerGWSidecarClient: workerGWClient,
		WorkerRouterClient:    workerRouterClient,
		WorkerNetOpClient:     workerNetOPClient,
		EventRecorder:         sliceGwEventRecorder,
		NumberOfGateways:      2,
	}).SetupWithManager(mgr); err != nil {
		setupLog.With("error", err).Error("unable to create controller", "controller", "SliceGw")
		os.Exit(1)
	}

	serviceExportEventRecorder := events.NewEventRecorder(mgr.GetEventRecorderFor("serviceExport-controller"))
	if err = (&serviceexport.Reconciler{
		Client:        mgr.GetClient(),
		Log:           ctrl.Log.WithName("controllers").WithName("ServiceExport"),
		Scheme:        mgr.GetScheme(),
		HubClient:     hubClient,
		EventRecorder: serviceExportEventRecorder,
	}).Setup(mgr, mf); err != nil {
		setupLog.With("error", err, "controller", "ServiceExport").Error("unable to create controller")
		os.Exit(1)
	}

	serviceImportEventRecorder := events.NewEventRecorder(mgr.GetEventRecorderFor("serviceImport-controller"))
	if err = (&serviceimport.Reconciler{
		Client:        mgr.GetClient(),
		Log:           ctrl.Log.WithName("controllers").WithName("ServiceImport"),
		Scheme:        mgr.GetScheme(),
		EventRecorder: serviceImportEventRecorder,
	}).SetupWithManager(mgr); err != nil {
		setupLog.With("error", err, "controller", "ServiceImport").Error("unable to create controller")
		os.Exit(1)
	}

	namespaceEventRecorder := events.NewEventRecorder(mgr.GetEventRecorderFor("namespace-controller"))
	if err = (&namespacecontroller.Reconciler{
		Client:        mgr.GetClient(),
		Log:           ctrl.Log.WithName("controllers").WithName("namespace"),
		Scheme:        mgr.GetScheme(),
		EventRecorder: namespaceEventRecorder,
		Hubclient:     hubClient,
	}).SetupWithManager(mgr); err != nil {
		setupLog.With("error", err, "controller", "namespace").Error("unable to create controller")
		os.Exit(1)
	}
	netpolEventRecorder := events.NewEventRecorder(mgr.GetEventRecorderFor("networkpolicy-controller"))
	if err = (&networkpolicy.NetpolReconciler{
		Client:        mgr.GetClient(),
		Log:           ctrl.Log.WithName("controllers").WithName("networkpolicy"),
		Scheme:        mgr.GetScheme(),
		EventRecorder: netpolEventRecorder,
	}).Setup(mgr, mf); err != nil {
		setupLog.With("error", err, "controller", "networkpolicy").Error("unable to create controller")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.With("error", err).Error("unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.With("error", err).Error("unable to set up ready check")
		os.Exit(1)
	}
	ctx := ctrl.SetupSignalHandler()

	if err != nil {
		setupLog.With("error", err).Error("unable to create kube client for hub manager")
		os.Exit(1)
	}
	go func() {
		setupLog.Info("starting hub manager")
		manager.Start(clientForHubMgr, ctx)
	}()

	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.With("error", err).Error("problem running manager")
		os.Exit(1)
	}
}
