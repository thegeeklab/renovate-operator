package renovate

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestReceiverRenovate(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Renovate Receiver Suite")
}
