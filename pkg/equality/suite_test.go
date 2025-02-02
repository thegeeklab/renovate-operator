package equality

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestEquality(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Equality Suite")
}
