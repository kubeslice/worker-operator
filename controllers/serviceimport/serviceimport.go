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

package serviceimport

import (
	"context"
	"strconv"

	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	"github.com/kubeslice/worker-operator/controllers"
	"github.com/kubeslice/worker-operator/internal/logger"
	corev1 "k8s.io/api/core/v1"

	k8sapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *Reconciler) serviceForServiceImport(serviceImport *kubeslicev1beta1.ServiceImport) *corev1.Service {

	ports := []corev1.ServicePort{}

	for _, p := range serviceImport.Spec.Ports {
		pName := p.Name
		if pName == "" {
			pName = string(p.Protocol) + strconv.Itoa(int(p.ContainerPort))
		}
		ports = append(ports, corev1.ServicePort{
			Port:       p.ContainerPort,
			Protocol:   p.Protocol,
			Name:       pName,
			TargetPort: intstr.FromInt(int(p.ContainerPort)),
		})
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceImport.Name,
			Namespace: serviceImport.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Ports: ports,
		},
	}

	ctrl.SetControllerReference(serviceImport, svc, r.Scheme)
	return svc
}

func (r *Reconciler) DeleteServiceImportResources(ctx context.Context, serviceimport *kubeslicev1beta1.ServiceImport) error {
	log := logger.FromContext(ctx)
	slice, err := controllers.GetSlice(ctx, r.Client, serviceimport.Spec.Slice)
	if err != nil {
		if k8sapierrors.IsNotFound(err) {
			return nil
		}
		log.Error(err, "Unable to fetch slice for serviceimport cleanup")
		return err
	}

	if slice.Status.SliceConfig == nil {
		return nil
	}

	err = r.DeleteIstioResources(ctx, serviceimport, slice)
	if err != nil {
		return err
	}

	return nil
}
