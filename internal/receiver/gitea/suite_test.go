package gitea

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestGiteaProvider(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Gitea Receiver Suite")
}
