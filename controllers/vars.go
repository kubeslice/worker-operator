package controllers

import (
	"os"
	"time"

	"github.com/kubeslice/operator/internal/utils"
)

var (
	// ControlPlaneNamespace is the namespace where slice operator is running
	ControlPlaneNamespace = "kubeslice-system"
	// DNSDeploymentName is the name of coredns deployment running in the cluster
	DNSDeploymentName            = "kubeslice-dns"
	NSMIPLabelSelectorKey string = "kubeslice.io/nsmIP"

	ClusterName = os.Getenv("CLUSTER_NAME")

	NodeIP = os.Getenv("NODE_IP")

	ImagePullSecretName = utils.GetEnvOrDefault("IMAGE_PULL_SECRET_NAME", "avesha-nexus")

	ReconcileInterval = 10 * time.Second
)
