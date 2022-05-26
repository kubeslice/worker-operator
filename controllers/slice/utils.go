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

package slice

import (
	"context"

	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *SliceReconciler) cleanupSliceResources(ctx context.Context, slice *kubeslicev1beta1.Slice) {
	r.Log.Info("Cleaning the slice resources!!")
	//cleanup slice namespaces label and netpol
	r.cleanupSliceNamespaces(ctx, slice)
	//cleanup slice router network service
	r.cleanupSliceRouter(ctx, slice.Name)
	//cleanup Service Discovery objects - serviceimport and export objects that belong to this slice
	r.cleanupServiceDiscoveryObjects(ctx, slice.Name)
}

func (r *SliceReconciler) cleanupServiceDiscoveryObjects(ctx context.Context, sliceName string) error {
	var err error
	if err = r.cleanupServiceImport(ctx, sliceName); err != nil {
		r.Log.Error(err, "Error cleaning up service import objects.. please remove it manually")
	}

	if err = r.cleanupServiceExport(ctx, sliceName); err != nil {
		r.Log.Error(err, "Error cleaning up service export objects.. please remove it manually")
	}
	return err
}

func (r *SliceReconciler) cleanupServiceImport(ctx context.Context, sliceName string) error {
	listOpts := []client.ListOption{
		client.MatchingLabels(map[string]string{
			"kubeslice.io/slice": sliceName,
		},
		),
	}
	serviceImportList := kubeslicev1beta1.ServiceImportList{}
	if err := r.List(ctx, &serviceImportList, listOpts...); err != nil {
		if errors.IsNotFound(err) {
			//early exit since there are no object found
			return nil
		}
		return err
	}
	for _, serviceimport := range serviceImportList.Items {
		if err := r.Delete(ctx, &serviceimport); err != nil {
			return err
		}
	}
	return nil
}
func (r *SliceReconciler) cleanupServiceExport(ctx context.Context, sliceName string) error {
	//delete the service export objects that belong to this slice
	listOpts := []client.ListOption{
		client.MatchingLabels(map[string]string{
			"kubeslice.io/slice": sliceName,
		},
		),
	}
	serviceExportList := kubeslicev1beta1.ServiceExportList{}
	if err := r.List(ctx, &serviceExportList, listOpts...); err != nil {
		if errors.IsNotFound(err) {
			//early exit since there are no object found
			return nil
		}
		return err
	}

	for _, serviceexport := range serviceExportList.Items {
		if err := r.Delete(ctx, &serviceexport); err != nil {
			return err
		}
	}
	return nil
}
