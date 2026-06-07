package viewmodel

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestViewModel(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ViewModel Suite")
}
