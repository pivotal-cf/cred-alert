package config_test

import (
	"cred-alert/config"
	"time"

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
				RepositoryDiscoveryInterval: 1 * time.Hour,
				WorkDir:                     "orig-work-dir",
			}

			other = &config.WorkerConfig{
				RepositoryDiscoveryInterval: 0,
				WorkDir:                     "new-work-dir",
				Whitelist:                   []string{"some-whitelist"},
			}
		})

		JustBeforeEach(func() {
			mergeErr = c.Merge(other)
		})

		It("replaces values on the destination when a non-default value is present on the source", func() {
			Expect(mergeErr).NotTo(HaveOccurred())
			Expect(c).To(Equal(&config.WorkerConfig{
				RepositoryDiscoveryInterval: 1 * time.Hour,
				WorkDir:                     "new-work-dir",
				Whitelist:                   []string{"some-whitelist"},
			}))
		})
	})
})
