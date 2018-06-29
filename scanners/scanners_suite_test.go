package scanners_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestScanners(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Scanners Suite")
}
