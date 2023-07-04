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

package manager

import (
	"context"
	"os"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	log "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	hubv1alpha1 "github.com/kubeslice/apis/pkg/controller/v1alpha1"
	workerv1alpha1 "github.com/kubeslice/apis/pkg/worker/v1alpha1"
	mevents "github.com/kubeslice/kubeslice-monitoring/pkg/events"
	"github.com/kubeslice/kubeslice-monitoring/pkg/metrics"
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	ossEvents "github.com/kubeslice/worker-operator/events"
	sidecar "github.com/kubeslice/worker-operator/pkg/gwsidecar"
	"github.com/kubeslice/worker-operator/pkg/hub/controllers"
	hubCluster "github.com/kubeslice/worker-operator/pkg/hub/controllers/cluster"
	"github.com/kubeslice/worker-operator/pkg/hub/controllers/vpnkeyrotation"

	"github.com/kubeslice/worker-operator/pkg/hub/controllers/workerslicegwrecycler"
	"github.com/kubeslice/worker-operator/pkg/logger"
	"github.com/kubeslice/worker-operator/pkg/router"
	"github.com/kubeslice/worker-operator/pkg/slicegwrecycler"
	"github.com/kubeslice/worker-operator/pkg/utils"
	ctrl "sigs.k8s.io/controller-runtime"
)

var scheme = runtime.NewScheme()

func init() {
	log.SetLogger(logger.NewWrappedLogger())
	clientgoscheme.AddToScheme(scheme)
	utilruntime.Must(workerv1alpha1.AddToScheme(scheme))
	utilruntime.Must(kubeslicev1beta1.AddToScheme(scheme))
	utilruntime.Must(hubv1alpha1.AddToScheme(scheme))
}

func Start(meshClient client.Client, hubClient client.Client, ctx context.Context) {
	config := &rest.Config{
		Host:            os.Getenv("HUB_HOST_ENDPOINT"),
		BearerTokenFile: HubTokenFile,
		TLSClientConfig: rest.TLSClientConfig{
			CAFile: HubCAFile,
		},
	}

	var log = log.Log.WithName("hub")

	log.Info("Connecting to hub cluster", "endpoint", HubEndpoint, "ns", ProjectNamespace)

	mgr, err := manager.New(config, manager.Options{
		Host:               HubEndpoint,
		Namespace:          ProjectNamespace,
		Scheme:             scheme,
		MetricsBindAddress: "0", // disable metrics for now
	})
	if err != nil {
		log.Error(err, "Could not create manager")
		os.Exit(1)
	}

	mf, _ := metrics.NewMetricsFactory(
		ctrlmetrics.Registry,
		metrics.MetricsFactoryOptions{
			Project:             strings.TrimPrefix(ProjectNamespace, "kubeslice_"),
			Cluster:             ClusterName,
			ReportingController: "workerOperator",
		},
	)

	// create slice-controller recorder
	workerSliceEventRecorder := mevents.NewEventRecorder(meshClient, mgr.GetScheme(), ossEvents.EventsMap, mevents.EventRecorderOptions{
		Cluster:   ClusterName,
		Project:   ProjectNamespace,
		Component: "workerSliceController",
		Namespace: controllers.ControlPlaneNamespace,
		Version:   utils.EventsVersion,
		Slice:     utils.NotApplicable,
	})

	sliceReconciler := controllers.NewSliceReconciler(
		meshClient,
		&workerSliceEventRecorder,
		mf,
	)
	err = builder.
		ControllerManagedBy(mgr).
		For(&workerv1alpha1.WorkerSliceConfig{}).
		WithEventFilter(predicate.NewPredicateFuncs(func(object client.Object) bool {
			return object.GetLabels()["worker-cluster"] == ClusterName
		})).
		Complete(sliceReconciler)
	if err != nil {
		log.Error(err, "could not create controller")
		os.Exit(1)
	}

	sliceGwReconciler := &controllers.SliceGwReconciler{
		MeshClient:    meshClient,
		EventRecorder: &workerSliceEventRecorder,
		ClusterName:   ClusterName,
	}
	err = builder.
		ControllerManagedBy(mgr).
		For(&workerv1alpha1.WorkerSliceGateway{}).
		WithEventFilter(predicate.NewPredicateFuncs(func(object client.Object) bool {
			return object.GetLabels()["worker-cluster"] == ClusterName
		})).
		Complete(sliceGwReconciler)
	if err != nil {
		log.Error(err, "could not create controller")
		os.Exit(1)
	}

	serviceImportReconciler := &controllers.ServiceImportReconciler{
		MeshClient:    meshClient,
		EventRecorder: &workerSliceEventRecorder,
	}
	err = builder.
		ControllerManagedBy(mgr).
		For(&workerv1alpha1.WorkerServiceImport{}).
		WithEventFilter(predicate.NewPredicateFuncs(func(object client.Object) bool {
			return object.GetLabels()["worker-cluster"] == ClusterName
		})).
		Complete(serviceImportReconciler)
	if err != nil {
		log.Error(err, "could not create controller")
		os.Exit(1)
	}

	workerGWClient, err := sidecar.NewWorkerGWSidecarClientProvider()
	if err != nil {
		log.Error(err, "could not create worker sidecar gateway client for slice gateway reconciler")
		os.Exit(1)
	}
	workerRouterClient, err := router.NewWorkerRouterClientProvider()
	if err != nil {
		log.Error(err, "could not create worker router client for slice gateway reconciler")
		os.Exit(1)
	}

	if err := (&workerslicegwrecycler.Reconciler{
		MeshClient:            meshClient,
		Log:                   ctrl.Log.WithName("controllers").WithName("workerslicegwrecycler"),
		Scheme:                mgr.GetScheme(),
		Client:                mgr.GetClient(),
		WorkerGWSidecarClient: workerGWClient,
		WorkerRouterClient:    workerRouterClient,
		EventRecorder:         &workerSliceEventRecorder,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "could not create controller")
		os.Exit(1)
	}

	clusterReconciler := hubCluster.NewReconciler(
		meshClient,
		&workerSliceEventRecorder,
		mf,
	)
	err = builder.
		ControllerManagedBy(mgr).
		For(&hubv1alpha1.Cluster{}).
		WithEventFilter(predicate.NewPredicateFuncs(func(object client.Object) bool {
			return object.GetName() == ClusterName
		})).
		Complete(clusterReconciler)
	if err != nil {
		log.Error(err, "could not create cluster controller")
		os.Exit(1)
	}

	workerRecyclerClient, err := slicegwrecycler.NewRecyclerClient(
		ctx, meshClient, hubClient, &workerSliceEventRecorder,
	)
	if err != nil {
		os.Exit(1)
	}
	vpnKeyRotationReconciler := vpnkeyrotation.NewReconciler(
		meshClient,
		hubClient,
		&workerSliceEventRecorder,
		mf,
		workerRecyclerClient,
	)
	err = builder.
		ControllerManagedBy(mgr).
		For(&hubv1alpha1.VpnKeyRotation{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(vpnKeyRotationReconciler)
	if err != nil {
		log.Error(err, "could not create vpn key rotation controller")
		os.Exit(1)
	}

	if err := mgr.Start(ctx); err != nil {
		log.Error(err, "could not start manager")
		os.Exit(1)
	}
}
