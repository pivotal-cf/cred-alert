package credhandler_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestCredhandler(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Credhandler Suite")
}
