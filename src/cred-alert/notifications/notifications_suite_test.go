package notifications_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestNotifications(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Notifications Suite")
}
