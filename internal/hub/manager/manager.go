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

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	log "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	workerv1alpha1 "github.com/kubeslice/apis/pkg/worker/v1alpha1"
	kubeslicev1beta1 "github.com/kubeslice/operator/api/v1beta1"
	"github.com/kubeslice/operator/internal/hub/controllers"
	"github.com/kubeslice/operator/internal/logger"
	"github.com/kubeslice/operator/pkg/events"
	ctrl "sigs.k8s.io/controller-runtime"
)

var scheme = runtime.NewScheme()

func init() {
	log.SetLogger(logger.NewLogger())
	clientgoscheme.AddToScheme(scheme)
	utilruntime.Must(workerv1alpha1.AddToScheme(scheme))
	utilruntime.Must(kubeslicev1beta1.AddToScheme(scheme))
}

func Start(kubesliceClient client.Client, ctx context.Context) {

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

	// create slice-controller recorder
	workerSliceEventRecorder := events.NewEventRecorder(mgr.GetEventRecorderFor("workerSlice-controller"))

	sliceReconciler := &controllers.SliceReconciler{
		KubeSliceClient: kubesliceClient,
		Log:             ctrl.Log.WithName("hub").WithName("controllers").WithName("SliceConfig"),
		EventRecorder:   workerSliceEventRecorder,
	}
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

	// create slice-controller recorder
	workerSliceGatewayEventRecorder := events.NewEventRecorder(mgr.GetEventRecorderFor("workerSliceGateway-controller"))

	sliceGwReconciler := &controllers.SliceGwReconciler{
		KubeSliceClient: kubesliceClient,
		EventRecorder:   workerSliceGatewayEventRecorder,
		ClusterName:     ClusterName,
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

	workerServiceImportEventRecorder := events.NewEventRecorder(mgr.GetEventRecorderFor("workerServiceImport-controller"))

	serviceImportReconciler := &controllers.ServiceImportReconciler{
		MeshClient:    kubesliceClient,
		EventRecorder: workerServiceImportEventRecorder,
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

	if err := mgr.Start(ctx); err != nil {
		log.Error(err, "could not start manager")
		os.Exit(1)
	}
}
