package utils

const (
	NotApplicable     = "NA"
	EventsVersion     = "v1alpha1"
	DefaultExcludedNS = "kube-system,default,kubeslice-system,kube-node-lease,kube-public,istio-system"

	// Key for the SLURM enabled workspace label
	SLURM_WORKSPACE_LABEL_KEY = "kubeslice.io/slurm-workspace"
	// Value for the SLURM enabled workspace label
	SLURM_WORKSPACE_LABEL_VALUE = "true"
)
