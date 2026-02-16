package cronjob

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestCronJob(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CronJob Suite")
}
