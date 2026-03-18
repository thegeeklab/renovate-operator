package v1beta1

//nolint:revive
const (
	PlatformType_GITHUB = "github"
	PlatformType_GITEA  = "gitea"

	// LabelRenovator is the label used to associate resources with a Renovator instance.
	LabelRenovator = "renovate.thegeeklab.de/renovator"
	// LabelGitRepo is the label used to associate resources with a specific Git repository.
	LabelGitRepo = "renovate.thegeeklab.de/gitrepo"
	// LabelLogsCollected is the annotation used to mark jobs whose logs
	// have already been archived to the persistent store.
	LabelLogsCollected = "renovate.thegeeklab.de/logs-collected"

	// RenovatorOperation is the annotation used to trigger operations.
	RenovatorOperation = "renovate.thegeeklab.de/operation"
	// RenovatorOperationSeparator is the separator used to separate parallel operations in the
	// RenovatorOperation annotation.
	RenovatorOperationSeparator = ";"

	// OperationDiscover is the value used to trigger immediate discovery.
	OperationDiscover = "discover"
	// OperationRenovate is the value used to trigger immediate renovate run.
	OperationRenovate = "renovate"

	// ValueTrue represents the string boolean "true" for labels and annotations.
	ValueTrue = "true"
	// ValueFalse represents the string boolean "false" for labels and annotations.
	ValueFalse = "false"

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
