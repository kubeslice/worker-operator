package controllers

import (
	"context"

	meshv1beta1 "bitbucket.org/realtimeai/kubeslice-operator/api/v1beta1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetSlice returns slice object by slice name
func GetSlice(ctx context.Context, c client.Client, slice string) (*meshv1beta1.Slice, error) {
	s := &meshv1beta1.Slice{}

	err := c.Get(ctx, types.NamespacedName{
		Name:      slice,
		Namespace: ControlPlaneNamespace,
	}, s)
	if err != nil {
		return nil, err
	}

	return s, nil
}
