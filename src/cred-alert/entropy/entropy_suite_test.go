package entropy_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestEntropy(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Entropy Suite")
}
