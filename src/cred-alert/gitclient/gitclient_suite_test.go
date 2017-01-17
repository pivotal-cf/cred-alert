package gitclient_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestGitclient(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Gitclient Suite")
}
