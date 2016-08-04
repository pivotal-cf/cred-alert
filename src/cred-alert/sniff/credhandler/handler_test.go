package credhandler_test

import (
	"cred-alert/scanners"
	"cred-alert/sniff/credhandler"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Handler", func() {
	var (
		handler    *credhandler.Handler
		handleFunc func(lager.Logger, scanners.Line) error
		logger     *lagertest.TestLogger

		calls int
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("credhandler")

		handleFunc = func(_ lager.Logger, violation scanners.Line) error {
			calls++

			return nil
		}
	})

	JustBeforeEach(func() {
		handler = credhandler.New(handleFunc)
	})

	It("calls the registered handler func", func() {
		line := scanners.Line{
			Content:    "credential",
			Path:       "/etc/shadow",
			LineNumber: 42,
		}

		err := handler.HandleViolation(logger, line)
		Expect(err).NotTo(HaveOccurred())

		Expect(calls).To(Equal(1))
	})

	It("can check whether or not a credential was found", func() {
		line := scanners.Line{
			Content:    "credential",
			Path:       "/etc/shadow",
			LineNumber: 42,
		}

		Expect(handler.CredentialsFound()).To(BeFalse())

		err := handler.HandleViolation(logger, line)
		Expect(err).NotTo(HaveOccurred())

		Expect(handler.CredentialsFound()).To(BeTrue())
	})
})
