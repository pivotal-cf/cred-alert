package filescanner_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestFilescanner(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Filescanner Suite")
}
