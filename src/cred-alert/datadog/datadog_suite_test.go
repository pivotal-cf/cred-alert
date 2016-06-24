package datadog_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestDatadog(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Datadog Suite")
}
