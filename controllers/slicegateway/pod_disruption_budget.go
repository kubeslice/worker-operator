package slicegateway

import (
	"context"
	"fmt"

	"github.com/kubeslice/worker-operator/controllers"
	webhook "github.com/kubeslice/worker-operator/pkg/webhook/pod"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Default minAvailable value in PodDisruptionBudget
var DefaultMinAvailablePodsInPDB = intstr.FromInt(1)

// constructPodDisruptionBudget creates the PodDisruptionBudget's manifest with labels matching the slice gateway pods.
func constructPodDisruptionBudget(sliceName, sliceGwName string, minAvailable intstr.IntOrString) *policyv1.PodDisruptionBudget {
	return &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-pdb", sliceGwName),
			Namespace: controllers.ControlPlaneNamespace,
			Labels: map[string]string{
				controllers.ApplicationNamespaceSelectorLabelKey: sliceName,
				controllers.SliceGatewaySelectorLabelKey:         sliceGwName,
			},
		},
		Spec: policyv1.PodDisruptionBudgetSpec{
			MinAvailable: &minAvailable,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					controllers.ApplicationNamespaceSelectorLabelKey: sliceName,
					webhook.PodInjectLabelKey:                        "slicegateway",
					controllers.SliceGatewaySelectorLabelKey:         sliceGwName,
				},
			},
		},
	}
}

// listPodDisruptionBudgetForSliceGateway lists the PodDisruptionBudget objects that match the slice gateway pods.
func listPodDisruptionBudgetForSliceGateway(ctx context.Context, kubeClient client.Client,
	sliceName, sliceGwName string) ([]policyv1.PodDisruptionBudget, error) {
	// Options for listing the PDBs that match the slice and slice gateway
	listOpts := []client.ListOption{
		client.MatchingLabels(map[string]string{
			controllers.ApplicationNamespaceSelectorLabelKey: sliceName,
			controllers.SliceGatewaySelectorLabelKey:         sliceGwName,
		}),
		client.InNamespace(controllers.ControlPlaneNamespace),
	}

	// List PDBs from cluster that match the slice and slice gateway
	pdbList := policyv1.PodDisruptionBudgetList{}
	if err := kubeClient.List(ctx, &pdbList, listOpts...); err != nil {
		return nil, err
	}

	return pdbList.Items, nil
}
