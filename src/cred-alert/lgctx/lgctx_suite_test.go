package lgctx_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestLgctx(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Lgctx Suite")
}
