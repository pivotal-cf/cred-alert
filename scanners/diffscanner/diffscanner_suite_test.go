package diffscanner_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestDiffscanner(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Diffscanner Suite")
}
