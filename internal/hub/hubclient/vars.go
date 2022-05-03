package hub

import (
	"os"

	"github.com/kubeslice/operator/internal/utils"
)

var (
	ProjectNamespace = os.Getenv("HUB_PROJECT_NAMESPACE")
	HubEndpoint      = os.Getenv("HUB_HOST_ENDPOINT")
	ClusterName      = os.Getenv("CLUSTER_NAME")
	HubTokenFile     = utils.GetEnvOrDefault("HUB_TOKEN_FILE", "/var/run/secrets/kubernetes.io/hub-serviceaccount/token")
	HubCAFile        = utils.GetEnvOrDefault("HUB_CA_FILE", "/var/run/secrets/kubernetes.io/hub-serviceaccount/ca.crt")
)
