module github.com/kubeslice/worker-operator

go 1.16

require (
	contrib.go.opencensus.io/exporter/prometheus v0.4.1
	github.com/go-logr/logr v1.2.3
	github.com/go-logr/zapr v1.2.0
	github.com/go-ping/ping v1.1.0 // indirect
	github.com/golang/protobuf v1.5.2
	github.com/google/go-cmp v0.5.8
	github.com/google/uuid v1.3.0 // indirect
	github.com/kubeslice/apis v0.0.0-20220906094117-7c4a10df3387
	github.com/kubeslice/gateway-sidecar v0.1.4
	github.com/kubeslice/netops v0.0.0-20220506082959-81fef290c265
	github.com/kubeslice/router-sidecar v1.1.1
	github.com/networkservicemesh/networkservicemesh/k8s/pkg/apis v0.0.0-20211028170547-e58ac1200f18
	github.com/networkservicemesh/sdk-k8s v1.6.1
	github.com/onsi/ginkgo/v2 v2.1.3
	github.com/onsi/gomega v1.18.1
	github.com/prometheus/client_golang v1.13.0
	github.com/prometheus/common v0.37.0 // indirect
	github.com/stretchr/testify v1.8.0
	github.com/vishvananda/netns v0.0.0-20211101163701-50045581ed74 // indirect
	go.opencensus.io v0.23.0
	go.uber.org/atomic v1.9.0 // indirect
	go.uber.org/multierr v1.8.0 // indirect
	go.uber.org/zap v1.21.0
	golang.org/x/crypto v0.0.0-20220829220503-c86fa9a7ed90 // indirect
	golang.org/x/net v0.0.0-20220907135653-1e95f45603a7 // indirect
	golang.org/x/sync v0.0.0-20220907140024-f12130a52804 // indirect
	golang.org/x/sys v0.0.0-20220908164124-27713097b956 // indirect
	golang.org/x/xerrors v0.0.0-20220609144429-65e65417b02f // indirect
	google.golang.org/genproto v0.0.0-20220908141613-51c1cc9bc6d0 // indirect
	google.golang.org/grpc v1.49.0
	google.golang.org/protobuf v1.28.1
	gopkg.in/yaml.v2 v2.4.0
	istio.io/api v0.0.0-20210226184957-53be27d8195b
	istio.io/client-go v1.9.0
	k8s.io/api v0.23.5
	k8s.io/apimachinery v0.23.5
	k8s.io/client-go v0.23.5
	sigs.k8s.io/controller-runtime v0.11.1
)
