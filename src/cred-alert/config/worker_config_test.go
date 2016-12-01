package config_test

import (
	"cred-alert/config"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("WorkerConfig", func() {
	Describe("Merge", func() {
		var (
			c, other *config.WorkerConfig
			mergeErr error
		)

		BeforeEach(func() {
			c = &config.WorkerConfig{
				LogLevel: "orig-log-level",
				WorkDir:  "orig-work-dir",
			}

			other = &config.WorkerConfig{
				LogLevel:  "",
				WorkDir:   "new-work-dir",
				Whitelist: []string{"some-whitelist"},
			}
		})

		JustBeforeEach(func() {
			mergeErr = c.Merge(other)
		})

		It("replaces values on the destination when a non-default value is present on the source", func() {
			Expect(mergeErr).NotTo(HaveOccurred())
			Expect(c).To(Equal(&config.WorkerConfig{
				LogLevel:  "orig-log-level",
				WorkDir:   "new-work-dir",
				Whitelist: []string{"some-whitelist"},
			}))
		})
	})
})
