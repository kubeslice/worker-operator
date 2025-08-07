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

package webhook

import (
	"context"

	podwh "github.com/kubeslice/worker-operator/pkg/webhook/pod"
	svcexportwh "github.com/kubeslice/worker-operator/pkg/webhook/serviceexport"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type UnifiedWebhookHandler struct {
	Client              client.Client
	Decoder             *admission.Decoder
	PodWebhook          *podwh.WebhookServer
	ServiceExportWebhook *svcexportwh.ServiceExportValidator
}

func (h *UnifiedWebhookHandler) Handle(ctx context.Context, req admission.Request) admission.Response {
	if req.Kind.Group == "networking.kubeslice.io" && req.Kind.Kind == "ServiceExport" {
		return h.ServiceExportWebhook.Handle(ctx, req)
	}
	return h.PodWebhook.Handle(ctx, req)
}
