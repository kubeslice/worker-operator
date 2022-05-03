module github.com/kubeslice/operator

go 1.16

require (
	github.com/go-logr/logr v1.2.0
	github.com/go-logr/zapr v1.2.0
	github.com/golang/protobuf v1.5.2
	github.com/google/go-cmp v0.5.6
	github.com/kubeslice/apis v0.0.0-20220425053255-0b3f573992ea
	github.com/kubeslice/gateway-sidecar v0.0.0-20220214062837-3c7d18a8439e
	github.com/kubeslice/netops v0.0.0-20220323113922-61ae902d49f7
	github.com/kubeslice/router-sidecar v0.0.0-20220221080454-e2276f57c978
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

replace github.com/kubeslice/gateway-sidecar => /home/lenovo/code/namechange/gateway-sidecar

replace github.com/kubeslice/netops => /home/lenovo/code/namechange/netops

replace github.com/kubeslice/router-sidecar => /home/lenovo/code/namechange/router-sidecar

replace github.com/kubeslice/operator => /home/lenovo/code/namechange/kubeslice-operator

replace github.com/kubeslice/apis => /home/lenovo/code/namechange/apis
