package gitlab

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestGitLabProvider(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "GitLab Provider Suite")
}
