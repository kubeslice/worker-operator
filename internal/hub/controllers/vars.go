package controllers

import (
	"os"
)

var (
	ControlPlaneNamespace = "kubeslice-system"
	clusterName           = os.Getenv("CLUSTER_NAME")
)
