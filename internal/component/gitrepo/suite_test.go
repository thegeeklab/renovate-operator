package gitrepo

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestGitRepo(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "GitRepo Suite")
}
