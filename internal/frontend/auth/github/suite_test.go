package github

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestGitHub(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "GitHub Auth Suite")
}
