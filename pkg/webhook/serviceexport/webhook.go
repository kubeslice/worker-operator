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

package serviceexport

import (
	"context"
	"fmt"
	"net/http"

	"github.com/kubeslice/worker-operator/api/v1beta1"
	"github.com/kubeslice/worker-operator/pkg/logger"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var log = logger.NewWrappedLogger().WithName("ServiceExportWebhook")

type ServiceExportValidator struct {
	Client  client.Client
	Decoder *admission.Decoder
}

func (v *ServiceExportValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	svcExport := &v1beta1.ServiceExport{}
	err := v.Decoder.Decode(req, svcExport)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if req.Operation == "CREATE" || req.Operation == "UPDATE" {
		if err := v.validateServiceExport(ctx, svcExport, req.Namespace); err != nil {
			return admission.Denied(err.Error())
		}
	}

	return admission.Allowed("")
}

func (v *ServiceExportValidator) validateServiceExport(ctx context.Context, svcExport *v1beta1.ServiceExport, namespace string) error {
	sliceName := svcExport.Spec.Slice
	if sliceName == "" {
		return fmt.Errorf("slice name is required")
	}

	slice := &v1beta1.Slice{}
	sliceKey := client.ObjectKey{Name: sliceName, Namespace: namespace}
	if err := v.Client.Get(ctx, sliceKey, slice); err != nil {
		return fmt.Errorf("slice '%s' not found on cluster: %v", sliceName, err)
	}

	if slice.Status.SliceConfig == nil || slice.Status.SliceConfig.NamespaceIsolationProfile == nil {
		return fmt.Errorf("slice '%s' is not properly configured", sliceName)
	}

	appNamespaces := slice.Status.SliceConfig.NamespaceIsolationProfile.ApplicationNamespaces
	namespaceFound := false
	for _, appNs := range appNamespaces {
		if appNs == namespace {
			namespaceFound = true
			break
		}
	}

	if !namespaceFound {
		return fmt.Errorf("namespace '%s' is not an onboarded application namespace for slice '%s'", namespace, sliceName)
	}

	return nil
}
