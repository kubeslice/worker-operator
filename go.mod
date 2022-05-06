module github.com/kubeslice/worker-operator

go 1.16

require (
	github.com/go-logr/logr v1.2.0
	github.com/go-logr/zapr v1.2.0
	github.com/golang/protobuf v1.5.2
	github.com/google/go-cmp v0.5.6
	github.com/kubeslice/apis v0.0.0-20220503083146-b9e5ed39a74a
	github.com/kubeslice/gateway-sidecar v0.0.0-20220506071225-eac5ccc17b42
	github.com/kubeslice/netops v0.0.0-20220506082959-81fef290c265
	github.com/kubeslice/router-sidecar v1.0.0
	github.com/networkservicemesh/networkservicemesh/k8s/pkg/apis v0.0.0-20211028170547-e58ac1200f18
	github.com/onsi/ginkgo/v2 v2.1.3
	github.com/onsi/gomega v1.18.1
	github.com/stretchr/testify v1.7.0
	go.uber.org/zap v1.21.0
	golang.org/x/net v0.0.0-20220225172249-27dd8689420f // indirect
	golang.org/x/sys v0.0.0-20220227234510-4e6760a101f9 // indirect
	google.golang.org/genproto v0.0.0-20220303160752-862486edd9cc // indirect
	google.golang.org/grpc v1.45.0
	google.golang.org/protobuf v1.27.1
	gopkg.in/yaml.v2 v2.4.0
	istio.io/api v0.0.0-20210204223132-d90b2f705958
	istio.io/client-go v1.9.0
	k8s.io/api v0.23.5
	k8s.io/apimachinery v0.23.5
	k8s.io/client-go v0.23.5
	sigs.k8s.io/controller-runtime v0.11.1
)
