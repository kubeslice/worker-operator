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

	"github.com/kubeslice/worker-operator/internal/cluster"
	"github.com/kubeslice/worker-operator/pkg/events"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	nsmv1alpha1 "github.com/networkservicemesh/networkservicemesh/k8s/pkg/apis/networkservice/v1alpha1"
	istiov1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"

	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	"github.com/kubeslice/worker-operator/controllers/serviceexport"
	"github.com/kubeslice/worker-operator/controllers/serviceimport"
	"github.com/kubeslice/worker-operator/controllers/slice"
	"github.com/kubeslice/worker-operator/controllers/slicegateway"
	hub "github.com/kubeslice/worker-operator/internal/hub/hubclient"
	"github.com/kubeslice/worker-operator/internal/hub/manager"
	"github.com/kubeslice/worker-operator/internal/logger"
	"github.com/kubeslice/worker-operator/internal/networkpolicy"
	"github.com/kubeslice/worker-operator/internal/utils"
	deploywh "github.com/kubeslice/worker-operator/internal/webhook/deploy"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(nsmv1alpha1.AddToScheme(scheme))
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

	ctrl.SetLogger(logger.NewLogger())

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "f7425d89.avesha.io",
		CertDir:                utils.GetEnvOrDefault("WEBHOOK_CERTS_DIR", "/etc/webhook/certs"),
	})

	// Use an environment variable to be able to disable webhooks, so that we can run the operator locally
	if utils.GetEnvOrDefault("ENABLE_WEBHOOKS", "true") == "true" {
		mgr.GetWebhookServer().Register("/mutate-appsv1-deploy", &webhook.Admission{
			Handler: &deploywh.WebhookServer{
				Client:          mgr.GetClient(),
				SliceInfoClient: deploywh.NewWebhookClient(),
			},
		})
	}
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}
	hubClient, err := hub.NewHubClientConfig()
	if err != nil {
		setupLog.Error(err, "could not create hub client for slice gateway reconciler")
		os.Exit(1)
	}

	sliceEventRecorder := events.NewEventRecorder(mgr.GetEventRecorderFor("slice-controller"))
	if err = (&slice.SliceReconciler{
		Client:        mgr.GetClient(),
		Log:           ctrl.Log.WithName("controllers").WithName("Slice"),
		Scheme:        mgr.GetScheme(),
		HubClient:     hubClient,
		EventRecorder: sliceEventRecorder,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Slice")
		os.Exit(1)
	}

	sliceGwEventRecorder := events.NewEventRecorder(mgr.GetEventRecorderFor("sliceGw-controller"))
	if err = (&slicegateway.SliceGwReconciler{
		Client:        mgr.GetClient(),
		Log:           ctrl.Log.WithName("controllers").WithName("SliceGw"),
		Scheme:        mgr.GetScheme(),
		HubClient:     hubClient,
		EventRecorder: sliceGwEventRecorder,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "SliceGw")
		os.Exit(1)
	}

	//+kubebuilder:scaffold:builder
	hubClient, err = hub.NewHubClientConfig()
	if err != nil {
		setupLog.Error(err, "could not create hub client for serviceexport reconciler")
		os.Exit(1)
	}

	serviceExportEventRecorder := events.NewEventRecorder(mgr.GetEventRecorderFor("serviceExport-controller"))
	if err = (&serviceexport.Reconciler{
		Client:        mgr.GetClient(),
		Log:           ctrl.Log.WithName("controllers").WithName("ServiceExport"),
		Scheme:        mgr.GetScheme(),
		HubClient:     hubClient,
		EventRecorder: serviceExportEventRecorder,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ServiceExport")
		os.Exit(1)
	}

	serviceImportEventRecorder := events.NewEventRecorder(mgr.GetEventRecorderFor("serviceImport-controller"))
	if err = (&serviceimport.Reconciler{
		Client:        mgr.GetClient(),
		Log:           ctrl.Log.WithName("controllers").WithName("ServiceImport"),
		Scheme:        mgr.GetScheme(),
		EventRecorder: serviceImportEventRecorder,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ServiceImport")
		os.Exit(1)
	}
	netpolEventRecorder := events.NewEventRecorder(mgr.GetEventRecorderFor("networkpolicy-controller"))
	if err = (&networkpolicy.NetpolReconciler{
		Client:        mgr.GetClient(),
		Log:           ctrl.Log.WithName("controllers").WithName("networkpolicy"),
		Scheme:        mgr.GetScheme(),
		EventRecorder: netpolEventRecorder,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "networkpolicy")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}
	ctx := ctrl.SetupSignalHandler()

	clientForHubMgr, err := client.New(ctrl.GetConfigOrDie(), client.Options{
		Scheme: scheme,
	})
	if err != nil {
		setupLog.Error(err, "unable to create kube client for hub manager")
		os.Exit(1)
	}
	go func() {
		setupLog.Info("starting hub manager")
		manager.Start(clientForHubMgr, ctx)
	}()

	//check if user has provided NODE_IP as env variable, if not fetch the ExternalIP from gateway nodes
	nodeIP, err := cluster.GetNodeIP(clientForHubMgr)
	if err != nil {
		setupLog.Error(err, "Error Getting nodeIP")
	}

	//post GeoLocation and other metadata to cluster CR on Hub cluster
	err = hub.PostClusterInfoToHub(ctx, clientForHubMgr, hubClient, os.Getenv("CLUSTER_NAME"), nodeIP, os.Getenv("HUB_PROJECT_NAMESPACE"))
	if err != nil {
		setupLog.Error(err, "could not post Cluster Info to Hub")
	}
	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
