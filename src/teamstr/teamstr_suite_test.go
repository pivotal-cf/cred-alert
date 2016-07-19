package teamstr_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestTeamstr(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Teamstr Suite")
}
