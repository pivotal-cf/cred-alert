package ingestor_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestIngestor(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Ingestor Suite")
}
