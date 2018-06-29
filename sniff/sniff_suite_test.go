package sniff_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestSniff(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Sniff Suite")
}
