package gitscanner_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestGitscanner(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Gitscanner Suite")
}
