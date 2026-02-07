package v1beta1

// ResourceName is the name identifying various resources in a ResourceList.
type ResourceName string

const (
	// ResourceDiscoveries is the name of the Discovery resource.
	ResourceDiscoveries ResourceName = "discoveries"

	// ResourceGitRepos is the name of the GitRepo resource.
	ResourceGitRepos ResourceName = "gitrepos"

	// ResourceRenovateConfigs is the name of the RenovateConfig resource.
	ResourceRenovateConfigs ResourceName = "renovateconfigs"

	// ResourceRenovators is the name of the Renovator resource.
	ResourceRenovators ResourceName = "renovators"

	// ResourceRunners is the name of the Runner resource.
	ResourceRunners ResourceName = "runners"
)

// Returns string version of ResourceName.
func (rn ResourceName) String() string {
	return string(rn)
}
