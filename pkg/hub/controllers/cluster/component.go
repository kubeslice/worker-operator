package cluster

import "github.com/kubeslice/worker-operator/controllers"

type component struct {
	name          string
	labels        map[string]string
	ns            string
	ignoreMissing bool
}

var components = []component{
	{
		name: "nsmgr",
		labels: map[string]string{
			"app": "nsmgr",
		},
		ns: controllers.ControlPlaneNamespace,
	},
	{
		name: "forwarder",
		labels: map[string]string{
			"app": "forwarder-kernel",
		},
		ns: controllers.ControlPlaneNamespace,
	},
	{
		name: "admission-webhook",
		labels: map[string]string{
			"app": "kubeslice-nsm-webhook",
		},
		ns: NSM_WEBHOOK_NS,
	},
	{
		name: "netop",
		labels: map[string]string{
			"app":                   "app_net_op",
			"kubeslice.io/pod-type": "netop",
		},
		ns: controllers.ControlPlaneNamespace,
	},
	{
		name: "spire-agent",
		labels: map[string]string{
			"app": "spire-agent",
		},
		ns: "spire",
	},
	{
		name: "spire-server",
		labels: map[string]string{
			"app": "spire-server",
		},
		ns: "spire",
	},
	{
		name: "istiod",
		labels: map[string]string{
			"app":   "istiod",
			"istio": "pilot",
		},
		ns:            "istio-system",
		ignoreMissing: true,
	},
	{
		name: "node-ips",
	},
	{
		name: "cni-subnets",
	},
}
