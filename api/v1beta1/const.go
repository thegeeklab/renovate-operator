package v1beta1

const (
	// Standard Kubernetes Label Keys.
	LabelAppName      = "app.kubernetes.io/name"
	LabelAppInstance  = "app.kubernetes.io/instance"
	LabelAppComponent = "app.kubernetes.io/component"
	LabelAppManagedBy = "app.kubernetes.io/managed-by"

	// Standard Kubernetes Label Values.
	OperatorManagedBy = "renovate-operator"
	OperatorName      = "renovate"

	ComponentDiscovery = "discovery"
	ComponentRunner    = "runner"
)
