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

package slicegateway

import (
	"context"

	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	"github.com/kubeslice/worker-operator/pkg/hub/controllers"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *SliceGwReconciler) cleanupSliceGwResources(ctx context.Context, slicegw *kubeslicev1beta1.SliceGateway) error {
	//delete gateway secret
	listOpts := []client.ListOption{
		client.InNamespace(slicegw.Namespace),
		client.MatchingLabels{"kubeslice.io/slice-gw": slicegw.Name},
	}
	secretList := &corev1.SecretList{}
	if err := r.List(ctx, secretList, listOpts...); err != nil {
		r.Log.Error(err, "Failed to list gateway secrets")

	}
	for _, v := range secretList.Items {
		meshSliceGwCerts := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      v.Name,
				Namespace: controllers.ControlPlaneNamespace,
			},
		}
		if err := r.Delete(ctx, meshSliceGwCerts); err != nil {
			r.Log.Error(err, "Error Deleting Gateway Secret while cleaning up.. Please Delete it before installing slice again")
			return err
		}
	}
	return nil
}

func selectNodePort(nodePort int) bool {
	// Loop through the map
	for _, v := range GwMap {
		// We found a uniqe match
		if v == nodePort {
			return false
		}
	}
	return true
}
func checkIfNodePortIsAlreadyUsed(nodePort int) bool {
	for _, n := range GwMap {
		if n == nodePort {
			return true
		}
	}
	return false
}
