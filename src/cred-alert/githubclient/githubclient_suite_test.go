package githubclient_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestGithubclient(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Githubclient Suite")
}
