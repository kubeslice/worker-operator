package controllers

import (
	"context"

	meshv1beta1 "bitbucket.org/realtimeai/kubeslice-operator/api/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *SliceReconciler) cleanupSliceResources(ctx context.Context, slice *meshv1beta1.Slice) {
	r.Log.Info("Cleaning the slice resources!!")
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
	serviceImportList := meshv1beta1.ServiceImportList{}
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
	serviceExportList := meshv1beta1.ServiceExportList{}
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
