package controllers

import (
	"os"
	"time"

	"bitbucket.org/realtimeai/kubeslice-operator/internal/utils"
)

var (
	sliceBakendUpdateInterval int64 = 30 // seconds between backend poll
	// ControlPlaneNamespace is the namespace where slice operator is running
	ControlPlaneNamespace = "kubeslice-system"
	// DNSDeploymentName is the name of coredns deployment running in the cluster
	DNSDeploymentName = "mesh-dns"

	ClusterName = os.Getenv("CLUSTER_NAME")

	nodeIP = os.Getenv("NODE_IP")

	ImagePullSecretName = utils.GetEnvOrDefault("IMAGE_PULL_SECRET_NAME", "avesha-nexus")

	gwSidecarImage           = os.Getenv("AVESHA_GW_SIDECAR_IMAGE")
	gwSidecarImagePullPolicy = os.Getenv("AVESHA_GW_SIDECAR_IMAGE_PULLPOLICY")

	openVpnServerImage      = os.Getenv("AVESHA_OPENVPN_SERVER_IMAGE")
	openVpnClientImage      = os.Getenv("AVESHA_OPENVPN_CLIENT_IMAGE")
	openVpnServerPullPolicy = os.Getenv("AVESHA_OPENVPN_SERVER_PULLPOLICY")
	openVpnClientPullPolicy = os.Getenv("AVESHA_OPENVPN_CLIENT_PULLPOLICY")

	ReconcileInterval = 10 * time.Second
)
