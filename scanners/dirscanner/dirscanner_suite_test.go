package dirscanner_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestDirscanner(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Dirscanner Suite")
}
