package serviceimport

import (
	"strconv"
	"strings"

	meshv1beta1 "bitbucket.org/realtimeai/kubeslice-operator/api/v1beta1"
)

// portListToDisplayString converts list of ports to a single string
func portListToDisplayString(servicePorts []meshv1beta1.ServicePort) string {
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

func getServiceProtocol(si *meshv1beta1.ServiceImport) meshv1beta1.ServiceProtocol {
	// currently we only support single port to be exposed
	if len(si.Spec.Ports) != 1 {
		return meshv1beta1.ServiceProtocolTCP
	}

	p := si.Spec.Ports[0].Name

	if strings.HasPrefix(p, "http") {
		return meshv1beta1.ServiceProtocolHTTP
	}

	return meshv1beta1.ServiceProtocolTCP

}

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}
