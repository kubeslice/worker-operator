module bitbucket.org/realtimeai/kubeslice-operator

go 1.16

require (
	bitbucket.org/realtimeai/kubeslice-netops v0.0.0-20220225091428-05183d983334
	bitbucket.org/realtimeai/kubeslice-router-sidecar v0.0.0-20220221080454-e2276f57c978
	bitbucket.org/realtimeai/mesh-apis v0.0.0-20220221023856-3c009efbc77d
	github.com/go-logr/logr v1.2.0
	github.com/go-logr/zapr v1.2.0
	github.com/networkservicemesh/networkservicemesh/k8s/pkg/apis v0.0.0-20211028170547-e58ac1200f18
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.17.0
	go.uber.org/zap v1.19.1
	google.golang.org/grpc v1.40.0
	google.golang.org/protobuf v1.27.1
	k8s.io/api v0.23.0
	k8s.io/apimachinery v0.23.3
	k8s.io/client-go v0.23.0
	sigs.k8s.io/controller-runtime v0.11.0
)
