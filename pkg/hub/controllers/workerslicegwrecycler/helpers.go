package workerslicegwrecycler

import (
	"fmt"

	spokev1alpha1 "github.com/kubeslice/apis/pkg/worker/v1alpha1"
)

func getDeployLabels(workerslicegwrecycler *spokev1alpha1.WorkerSliceGwRecycler, isClient bool) map[string]string {
	if isClient {
		return map[string]string{
			"kubeslice.io/slicegw":                      workerslicegwrecycler.Spec.SliceGwClient,
			"kubeslice.io/slicegatewayRedundancyNumber": fmt.Sprint(workerslicegwrecycler.Spec.RedundancyNumber),
		}
	}
	return map[string]string{
		"kubeslice.io/slicegw":                      workerslicegwrecycler.Spec.SliceGwServer,
		"kubeslice.io/slicegatewayRedundancyNumber": fmt.Sprint(workerslicegwrecycler.Spec.RedundancyNumber),
	}
}

func getPodLabels(workerslicegwrecycler *spokev1alpha1.WorkerSliceGwRecycler, sliceGateway string) map[string]string {
	return map[string]string{
		"kubeslice.io/pod-type":                     "slicegateway",
		"kubeslice.io/slice":                        workerslicegwrecycler.Spec.SliceName,
		"kubeslice.io/slice-gw":                     sliceGateway,
		"kubeslice.io/slicegatewayRedundancyNumber": fmt.Sprint(workerslicegwrecycler.Spec.RedundancyNumber),
	}
}
