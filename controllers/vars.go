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

package controllers

import (
	"os"
	"time"

	"github.com/kubeslice/worker-operator/pkg/utils"
)

var (
	// ControlPlaneNamespace is the namespace where slice operator is running
	ControlPlaneNamespace = "kubeslice-system"
	// DNSDeploymentName is the name of coredns deployment running in the cluster
	DNSDeploymentName            = "kubeslice-dns"
	NSMIPLabelSelectorKey string = "kubeslice.io/nsmIP"

	ClusterName = os.Getenv("CLUSTER_NAME")

	NodeIP = os.Getenv("NODE_IP")

	ImagePullSecretName = utils.GetEnvOrDefault("IMAGE_PULL_SECRET_NAME", "kubeslice-nexus")

	ReconcileInterval = 10 * time.Second
)
