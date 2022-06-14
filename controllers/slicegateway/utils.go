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
)

func (r *SliceGwReconciler) cleanupSliceGwResources(ctx context.Context, slicegw *kubeslicev1beta1.SliceGateway) error {
	//delete gateway secret
	meshSliceGwCerts := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      slicegw.Name,
			Namespace: controllers.ControlPlaneNamespace,
		},
	}
	if err := r.Delete(ctx, meshSliceGwCerts); err != nil {
		r.Log.Error(err, "Error Deleting Gateway Secret while cleaning up.. Please Delete it before installing slice again")
		return err
	}
	return nil
}
