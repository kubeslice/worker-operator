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

	meshv1beta1 "bitbucket.org/realtimeai/kubeslice-operator/api/v1beta1"
	"bitbucket.org/realtimeai/kubeslice-operator/internal/hub/controllers"
	"bitbucket.org/realtimeai/kubeslice-operator/internal/logger"
	"bitbucket.org/realtimeai/kubeslice-operator/pkg/events"
	spokev1alpha1 "bitbucket.org/realtimeai/mesh-apis/pkg/spoke/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
)

var scheme = runtime.NewScheme()

func init() {
	log.SetLogger(logger.NewLogger())
	clientgoscheme.AddToScheme(scheme)
	utilruntime.Must(spokev1alpha1.AddToScheme(scheme))
	utilruntime.Must(meshv1beta1.AddToScheme(scheme))
}

func Start(meshClient client.Client, ctx context.Context) {

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
	spokeSliceEventRecorder := events.NewEventRecorder(mgr.GetEventRecorderFor("spokeSlice-controller"))

	sliceReconciler := &controllers.SliceReconciler{
		MeshClient:    meshClient,
		Log:           ctrl.Log.WithName("hub").WithName("controllers").WithName("SliceConfig"),
		EventRecorder: spokeSliceEventRecorder,
	}
	err = builder.
		ControllerManagedBy(mgr).
		For(&spokev1alpha1.SpokeSliceConfig{}).
		WithEventFilter(predicate.NewPredicateFuncs(func(object client.Object) bool {
			return object.GetLabels()["spoke-cluster"] == ClusterName
		})).
		Complete(sliceReconciler)
	if err != nil {
		log.Error(err, "could not create controller")
		os.Exit(1)
	}

	// create slice-controller recorder
	spokeSliceGatewayEventRecorder := events.NewEventRecorder(mgr.GetEventRecorderFor("spokeSliceGateway-controller"))

	sliceGwReconciler := &controllers.SliceGwReconciler{
		MeshClient:    meshClient,
		EventRecorder: spokeSliceGatewayEventRecorder,
		ClusterName:   ClusterName,
	}
	err = builder.
		ControllerManagedBy(mgr).
		For(&spokev1alpha1.SpokeSliceGateway{}).
		WithEventFilter(predicate.NewPredicateFuncs(func(object client.Object) bool {
			return object.GetLabels()["spoke-cluster"] == ClusterName
		})).
		Complete(sliceGwReconciler)
	if err != nil {
		log.Error(err, "could not create controller")
		os.Exit(1)
	}

	spokeServiceImportEventRecorder := events.NewEventRecorder(mgr.GetEventRecorderFor("spokeServiceImport-controller"))

	serviceImportReconciler := &controllers.ServiceImportReconciler{
		MeshClient:    meshClient,
		EventRecorder: spokeServiceImportEventRecorder,
	}
	err = builder.
		ControllerManagedBy(mgr).
		For(&spokev1alpha1.SpokeServiceImport{}).
		WithEventFilter(predicate.NewPredicateFuncs(func(object client.Object) bool {
			return object.GetLabels()["spoke-cluster"] == ClusterName
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
