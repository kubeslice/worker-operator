package slice

const (
	ControlPlaneNamespace                = "kubeslice-system"
	AllowedNamespaceSelectorLabelKey     = "kubeslice.io/namespace"
	ApplicationNamespaceSelectorLabelKey = "kubeslice.io/slice"
	AllowedNamespaceAnnotationKey        = "kubeslice.io/trafficAllowedToSlices"
)
