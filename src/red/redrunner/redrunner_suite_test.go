package redrunner_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestRedrunner(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Red Runner Suite")
}
