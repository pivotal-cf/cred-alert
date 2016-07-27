package textscanner_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestTextscanner(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Textscanner Suite")
}
