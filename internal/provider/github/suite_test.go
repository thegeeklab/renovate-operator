package github

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestGitHubProvider(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "GitHub Provider Suite")
}
