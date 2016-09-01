package revok_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestRevok(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Revok Suite")
}
