package patterns_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestPatterns(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Patterns Suite")
}
