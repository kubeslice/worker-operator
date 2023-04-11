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
	"os"
	"strconv"
	"strings"

	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	"github.com/kubeslice/worker-operator/controllers"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func getClusterName() string {
	return os.Getenv("CLUSTER_NAME")
}

// portListToDisplayString converts list of ports to a single string
func portListToDisplayString(servicePorts []kubeslicev1beta1.ServicePort) string {
	ports := []string{}
	for _, port := range servicePorts {
		protocol := "TCP"
		if port.Protocol != "" {
			protocol = string(port.Protocol)
		}
		ports = append(ports, strconv.Itoa(int(port.ContainerPort))+"/"+protocol)
	}
	return strings.Join(ports, ",")
}

// Get NSM Ip of an app pod
func getNsmIP(pod *corev1.Pod, appPods []kubeslicev1beta1.AppPod) string {
	for _, appPod := range appPods {
		if pod.Name == appPod.PodName && pod.Namespace == appPod.PodNamespace {
			return appPod.NsmIP
		}
	}
	return ""
}

// Determine if there is a change in existing service pods list
func isServiceAppPodChanged(current []kubeslicev1beta1.ServicePod, old []kubeslicev1beta1.ServicePod) bool {
	if len(current) != len(old) {
		return true
	}

	s := make(map[string]kubeslicev1beta1.ServicePod)

	for _, c := range old {
		s[c.Name] = c
	}

	for _, c := range current {
		if s[c.Name].NsmIP != c.NsmIP {
			return true
		}
		if s[c.Name].PodIp != c.PodIp {
			return true
		}
	}

	return false
}

// Get Apppods connected to a slice
func getAppPodsInSlice(ctx context.Context, c client.Client, sliceName string) ([]kubeslicev1beta1.AppPod, error) {
	log := ctrl.Log.WithName("util")

	slice, err := controllers.GetSlice(ctx, c, sliceName)

	if err != nil {
		log.Error(err, "Failed to get Slice")
		return nil, err
	}

	return slice.Status.AppPods, nil
}

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func getServiceProtocol(se *kubeslicev1beta1.ServiceExport) kubeslicev1beta1.ServiceProtocol {
	// currently we only support single port to be exposed
	if len(se.Spec.Ports) != 1 {
		return kubeslicev1beta1.ServiceProtocolTCP
	}

	p := se.Spec.Ports[0].Name

	if strings.HasPrefix(p, "http") {
		return kubeslicev1beta1.ServiceProtocolHTTP
	}

	return kubeslicev1beta1.ServiceProtocolTCP

}

func arrayContainsString(a []string, s string) bool {
	for _, i := range a {
		if i == s {
			return true
		}
	}
	return false
}
