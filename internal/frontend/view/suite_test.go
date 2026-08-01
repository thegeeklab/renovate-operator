package view

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestView(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "View Suite")
}
