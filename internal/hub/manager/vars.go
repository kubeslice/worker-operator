package manager

import (
	"os"
)

var (
	imagePullSecretName = GetEnvOrDefault("IMAGE_PULL_SECRET_NAME", "kubeslice-avesha-nexus")
	ProjectNamespace    = os.Getenv("HUB_PROJECT_NAMESPACE")
	HubEndpoint         = os.Getenv("HUB_HOST_ENDPOINT")
	HubTokenFile        = GetEnvOrDefault("HUB_TOKEN_FILE", "/var/run/secrets/kubernetes.io/hub-serviceaccount/token")
	HubCAFile           = GetEnvOrDefault("HUB_CA_FILE", "/var/run/secrets/kubernetes.io/hub-serviceaccount/ca.crt")
)

func GetEnvOrDefault(key, def string) string {
	val, ok := os.LookupEnv(key)
	if !ok {
		return def
	}
	return val
}
