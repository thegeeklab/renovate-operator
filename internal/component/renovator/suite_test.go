package renovator

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestRenovator(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Renovator Component Suite")
}
