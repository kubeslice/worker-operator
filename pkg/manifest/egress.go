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

package manifest

import (
	"context"

	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	istiov1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Install istio egress gw resources on the given cluster in a slice
// Resources:
//
//	deployment (adds annotations to add the egress pod to the slice)
//	serviceaccount
//	role
//	rolebinding
//	service (type clusterip)
//	gateway
func InstallEgress(ctx context.Context, c client.Client, slice *kubeslicev1beta1.Slice) error {
	sliceName := slice.Name
	templates := map[string]string{"SLICE": sliceName}

	deploy := &appsv1.Deployment{}
	err := NewManifest("egress-deploy", templates).Parse(deploy)
	if err != nil {
		return err
	}

	svc := &corev1.Service{}
	err = NewManifest("egress-svc", templates).Parse(svc)
	if err != nil {
		return err
	}

	role := &rbacv1.Role{}
	err = NewManifest("egress-role", templates).Parse(role)
	if err != nil {
		return err
	}

	sa := &corev1.ServiceAccount{}
	err = NewManifest("egress-sa", templates).Parse(sa)
	if err != nil {
		return err
	}

	rb := &rbacv1.RoleBinding{}
	err = NewManifest("egress-rolebinding", templates).Parse(rb)
	if err != nil {
		return err
	}

	gw := &istiov1beta1.Gateway{}
	err = NewManifest("egress-gw", templates).Parse(gw)
	if err != nil {
		return err
	}

	objects := []client.Object{
		deploy,
		svc,
		role,
		sa,
		rb,
		gw,
	}

	for _, o := range objects {

		// Set slice as the owner for the object
		ctrl.SetControllerReference(slice, o, c.Scheme())

		if err := c.Create(ctx, o); err != nil {
			// Ignore if already exists
			if errors.IsAlreadyExists(err) {
				continue
			}
			return err
		}
	}

	return nil
}

// Uninstall istio egress (EW) resources fo a slice from a given cluster
// Resources:
//
//	deployment
//	serviceaccount
//	role
//	rolebinding
//	service
//	gateway
func UninstallEgress(ctx context.Context, c client.Client, sliceName string) error {

	log.Info("deleting EW egress gw for the slice", "slice", sliceName)

	objects := []client.Object{
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      sliceName + "-istio-egressgateway",
				Namespace: "kubeslice-system",
			},
		},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      sliceName + "-istio-egressgateway",
				Namespace: "kubeslice-system",
			},
		},
		&rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      sliceName + "-istio-egressgateway-sds",
				Namespace: "kubeslice-system",
			},
		},
		&corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      sliceName + "-istio-egressgateway-service-account",
				Namespace: "kubeslice-system",
			},
		},
		&rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      sliceName + "-istio-egressgateway-sds",
				Namespace: "kubeslice-system",
			},
		},
		&istiov1beta1.Gateway{
			ObjectMeta: metav1.ObjectMeta{
				Name:      sliceName + "-istio-egressgateway",
				Namespace: "kubeslice-system",
			},
		},
	}

	for _, o := range objects {
		if err := c.Delete(ctx, o); err != nil {
			// Ignore the error if the resource is already deleted
			// return error only if there is some other error
			if !errors.IsNotFound(err) {
				return err
			}
		}
	}

	return nil
}
