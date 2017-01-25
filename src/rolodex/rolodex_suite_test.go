package rolodex_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestRolodex(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Rolodex Suite")
}
