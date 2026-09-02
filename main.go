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
	"context"
	"flag"
	"os"
	"strings"
	"time"

	"github.com/kubeslice/kubeslice-monitoring/pkg/metrics"
	"github.com/kubeslice/worker-operator/controllers"
	"github.com/kubeslice/worker-operator/pkg/monitoring"
	namespacecontroller "github.com/kubeslice/worker-operator/pkg/namespace/controllers"
	"github.com/kubeslice/worker-operator/pkg/slicegwrecycler"
	"github.com/prometheus/client_golang/prometheus"
	"go.opencensus.io/stats/view"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.

	_ "k8s.io/client-go/plugin/pkg/client/auth"

	gatewayEdge "github.com/kubeslice/worker-operator/pkg/gatewayedge"
	sidecar "github.com/kubeslice/worker-operator/pkg/gwsidecar"
	netop "github.com/kubeslice/worker-operator/pkg/netop"
	router "github.com/kubeslice/worker-operator/pkg/router"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	nsmv1 "github.com/networkservicemesh/sdk-k8s/pkg/tools/k8s/apis/networkservicemesh.io/v1"
	istiov1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"

	ocprom "contrib.go.opencensus.io/exporter/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	hubv1alpha1 "github.com/kubeslice/apis/pkg/controller/v1alpha1"
	monitoringEvents "github.com/kubeslice/kubeslice-monitoring/pkg/events"
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	"github.com/kubeslice/worker-operator/controllers/serviceexport"
	"github.com/kubeslice/worker-operator/controllers/serviceimport"
	"github.com/kubeslice/worker-operator/controllers/slice"
	"github.com/kubeslice/worker-operator/controllers/slicegateway"
	ossEvents "github.com/kubeslice/worker-operator/events"
	hubCluster "github.com/kubeslice/worker-operator/pkg/hub/controllers/cluster"
	"github.com/kubeslice/worker-operator/pkg/hub/failover"
	hub "github.com/kubeslice/worker-operator/pkg/hub/hubclient"
	"github.com/kubeslice/worker-operator/pkg/hub/manager"
	"github.com/kubeslice/worker-operator/pkg/hub/resolver"
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

	mgrMetrics := metricsserver.Options{
		BindAddress: metricsAddr,
	}

	webhookServer := webhook.NewServer(webhook.Options{
		Port:    9443,
		CertDir: utils.GetEnvOrDefault("WEBHOOK_CERTS_DIR", "/etc/webhook/certs"),
	})

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                mgrMetrics,
		WebhookServer:          webhookServer,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "f7425d89.kubeslice.io",
	})

	er := &monitoring.EventRecorder{
		Client:    mgr.GetClient(),
		Scheme:    mgr.GetScheme(),
		Logger:    logger.NewLogger(),
		Cluster:   os.Getenv("CLUSTER_NAME"),
		Component: "workerOperator",
	}

	// Use an environment variable to be able to disable webhooks, so that we can run the operator locally
	if utils.GetEnvOrDefault("ENABLE_WEBHOOKS", "true") == "true" {
		mgr.GetWebhookServer().Register("/mutate-webhook", &webhook.Admission{
			Handler: &podwh.WebhookServer{
				Client:          mgr.GetClient(),
				SliceInfoClient: podwh.NewWebhookClient(),
				Decoder:         admission.NewDecoder(mgr.GetScheme()),
			},
		})
		mgr.GetWebhookServer().Register("/validate-webhook", &webhook.Admission{
			Handler: &podwh.WebhookServer{
				Client:          mgr.GetClient(),
				SliceInfoClient: podwh.NewWebhookClient(),
				Decoder:         admission.NewDecoder(mgr.GetScheme()),
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

	// Decide which hub to talk to before any client is built. Inert unless
	// HUB_SECONDARY_HOST_ENDPOINT is set, in which case hubConn is exactly the
	// primary connection this worker has always used.
	hubConn := hub.PrimaryConnection()
	failoverCfg := failover.ConfigFromEnv()
	var hubFollower *failover.Follower
	var reconnected bool
	if failoverCfg.Enabled() {
		hubFollower, err = failover.New(failoverCfg, hubConn, ctrl.Log.WithName("hub-failover"), nil)
		if err != nil {
			setupLog.With("error", err).Error("could not configure hub failover following")
			os.Exit(1)
		}
		hubConn, reconnected = hubFollower.StartupConnection(context.Background())
	}
	connInfo := hubCluster.ConnectionInfo{Enabled: failoverCfg.Enabled(), Reconnected: reconnected}

	hubClient, err := hub.NewHubClientConfig(er, hubConn)
	if err != nil {
		setupLog.With("error", err).Error("could not create hub client for slice gateway reconciler")
		os.Exit(1)
	}

	workerRouterClient, err := router.NewWorkerRouterClientProvider()
	if err != nil {
		setupLog.With("error", err).Error("could not create worker router client for slice gateway reconciler")
		os.Exit(1)
	}

	workerNetOPClient, err := netop.NewWorkerNetOpClientProvider()
	if err != nil {
		setupLog.With("error", err).Error("could not create worker netop client for slice gateway reconciler")
		os.Exit(1)
	}

	workerGatewayEdgeClient, err := gatewayEdge.NewWorkerGatewayEdgeClientProvider()
	if err != nil {
		setupLog.With("error", err).Error("could not create worker gateway edge client for slice reconciler")
		os.Exit(1)
	}

	clientForHubMgr, err := client.New(ctrl.GetConfigOrDie(), client.Options{
		Scheme: scheme,
	})

	// Cancellable so a resolved hub failover can shut this process down the same
	// way a signal would: the manager drains, the process exits 0, and the
	// kubelet restarts it against the hub that is now Active.
	ctx, stopForHubSwitch := context.WithCancel(ctrl.SetupSignalHandler())
	defer stopForHubSwitch()

	mf, err := metrics.NewMetricsFactory(ctrlmetrics.Registry, metrics.MetricsFactoryOptions{
		Cluster:             controllers.ClusterName,
		Project:             strings.TrimPrefix(hub.ProjectNamespace, "kubeslice_"),
		ReportingController: "workerOperator",
	})
	if err != nil {
		setupLog.With("error", err).Error("unable to initializ metrics factory")
		os.Exit(1)
	}

	sliceEventRecorder := monitoringEvents.NewEventRecorder(mgr.GetClient(), scheme, ossEvents.EventsMap, monitoringEvents.EventRecorderOptions{
		Version:   utils.EventsVersion,
		Slice:     utils.NotApplicable,
		Cluster:   controllers.ClusterName,
		Project:   hub.ProjectNamespace,
		Component: "sliceController",
		Namespace: controllers.ControlPlaneNamespace,
	})
	workerRecyclerClient, err := slicegwrecycler.NewRecyclerClient(ctx, clientForHubMgr, hubClient, &sliceEventRecorder, mgr.GetScheme())
	if err != nil {
		os.Exit(1)
	}

	if err = (&slice.SliceReconciler{
		Client:                  mgr.GetClient(),
		Log:                     ctrl.Log.WithName("controllers").WithName("Slice"),
		Scheme:                  mgr.GetScheme(),
		HubClient:               hubClient,
		EventRecorder:           &sliceEventRecorder,
		WorkerRouterClient:      workerRouterClient,
		WorkerNetOpClient:       workerNetOPClient,
		WorkerGatewayEdgeClient: workerGatewayEdgeClient,
	}).Setup(mgr, mf); err != nil {
		setupLog.With("error", err).Error("unable to create controller", "controller", "Slice")
		os.Exit(1)
	}

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
		WorkerRecyclerClient:  workerRecyclerClient,
		EventRecorder:         &sliceEventRecorder,
		NumberOfGateways:      2,
	}).SetupWithManager(mgr); err != nil {
		setupLog.With("error", err).Error("unable to create controller", "controller", "SliceGw")
		os.Exit(1)
	}

	if err = (&serviceexport.Reconciler{
		Client:        mgr.GetClient(),
		Log:           ctrl.Log.WithName("controllers").WithName("ServiceExport"),
		Scheme:        mgr.GetScheme(),
		HubClient:     hubClient,
		EventRecorder: &sliceEventRecorder,
	}).Setup(mgr, mf); err != nil {
		setupLog.With("error", err, "controller", "ServiceExport").Error("unable to create controller")
		os.Exit(1)
	}

	if err = (&serviceimport.Reconciler{
		Client:        mgr.GetClient(),
		Log:           ctrl.Log.WithName("controllers").WithName("ServiceImport"),
		Scheme:        mgr.GetScheme(),
		EventRecorder: &sliceEventRecorder,
	}).Setup(mgr, mf); err != nil {
		setupLog.With("error", err, "controller", "ServiceImport").Error("unable to create controller")
		os.Exit(1)
	}

	if err = (&namespacecontroller.Reconciler{
		Client:        mgr.GetClient(),
		Log:           ctrl.Log.WithName("controllers").WithName("namespace"),
		Scheme:        mgr.GetScheme(),
		EventRecorder: &sliceEventRecorder,
		Hubclient:     hubClient,
	}).SetupWithManager(mgr); err != nil {
		setupLog.With("error", err, "controller", "namespace").Error("unable to create controller")
		os.Exit(1)
	}
	if err = (&networkpolicy.NetpolReconciler{
		Client:        mgr.GetClient(),
		Log:           ctrl.Log.WithName("controllers").WithName("networkpolicy"),
		Scheme:        mgr.GetScheme(),
		EventRecorder: &sliceEventRecorder,
	}).SetupWithManager(mgr); err != nil {
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

	if err != nil {
		setupLog.With("error", err).Error("unable to create kube client for hub manager")
		os.Exit(1)
	}
	go func() {
		setupLog.Info("starting hub manager")
		manager.Start(clientForHubMgr, hubClient, ctx, hubConn, connInfo)
	}()

	if hubFollower != nil {
		go hubFollower.Watch(ctx, hubConn, func(claim resolver.Claim) {
			// Restart rather than rebuild. Both hub connections are assembled
			// once during startup from a rest.Config, and manager.Start exits
			// the process on any hub error already, so a clean restart is both
			// the smaller change and the one this process is already built for.
			// The data plane is untouched: gateways and tunnels run in their
			// own pods.
			setupLog.With("endpoint", claim.Endpoint, "identity", claim.Identity).
				Info("active hub changed; shutting down to reconnect")
			reportConnectionLost(hubClient, &sliceEventRecorder)
			stopForHubSwitch()
		})
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.With("error", err).Error("problem running manager")
		os.Exit(1)
	}
}

// reportConnectionLostTimeout bounds the whole best-effort report below. Five
// seconds is DefaultProbeTimeout, the budget this feature already gives a
// single hub read; three calls get double it.
const reportConnectionLostTimeout = 10 * time.Second

// reportConnectionLost makes one best-effort attempt to record, on the hub
// this worker is about to leave, that it is doing so (issue #469's
// ControllerConnectionLost event and Reconnecting condition). Best-effort
// because the only connection available to write with is the one this
// worker is abandoning — if that hub is itself the reason for the switch,
// there is nothing to write to, and that failure is expected, not fatal.
func reportConnectionLost(hubClient client.Client, er *monitoringEvents.EventRecorder) {
	// One bounded budget shared by all three writes below. The hub being
	// written to is the one this worker is abandoning, so it may accept the
	// socket and then never answer; an unbounded read there would hold up the
	// restart that follows the failover until the OS TCP timeout, which is the
	// one thing this best-effort path must never do.
	ctx, cancel := context.WithTimeout(context.Background(), reportConnectionLostTimeout)
	defer cancel()

	cr := &hubv1alpha1.Cluster{}
	err := hubClient.Get(ctx, client.ObjectKey{
		Name:      controllers.ClusterName,
		Namespace: hub.ProjectNamespace,
	}, cr)
	if err != nil {
		setupLog.With("error", err).Info("could not report connection loss before reconnecting; the hub may already be unreachable")
		return
	}
	meta.SetStatusCondition(&cr.Status.Conditions, metav1.Condition{
		Type:    hubCluster.ConditionControllerConnected,
		Status:  metav1.ConditionUnknown,
		Reason:  hubCluster.ReasonReconnecting,
		Message: "following a resolved hub failover; reconnecting",
	})
	if err := hubClient.Status().Update(ctx, cr); err != nil {
		setupLog.With("error", err).Info("could not persist the Reconnecting condition before restart")
		return
	}
	utils.RecordEvent(ctx, er, cr, nil, ossEvents.EventControllerConnectionLost, "hub-failover")
}
