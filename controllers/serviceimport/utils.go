package serviceimport

import (
	"strconv"
	"strings"

	kubeslicev1beta1 "github.com/kubeslice/operator/api/v1beta1"
)

// portListToDisplayString converts list of ports to a single string
func portListToDisplayString(servicePorts []kubeslicev1beta1.ServicePort) string {
	ports := []string{}
	for _, port := range servicePorts {
		protocol := "TCP"
		if port.Protocol != "" {
			protocol = string(port.Protocol)
		}
		ports = append(ports, strconv.Itoa(int(port.ContainerPort))+"/"+protocol)
	}
	return strings.Join(ports, ",")
}

func getServiceProtocol(si *kubeslicev1beta1.ServiceImport) kubeslicev1beta1.ServiceProtocol {
	// currently we only support single port to be exposed
	if len(si.Spec.Ports) != 1 {
		return kubeslicev1beta1.ServiceProtocolTCP
	}

	p := si.Spec.Ports[0].Name

	if strings.HasPrefix(p, "http") {
		return kubeslicev1beta1.ServiceProtocolHTTP
	}

	return kubeslicev1beta1.ServiceProtocolTCP

}

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}
