package config_test

import (
	"cred-alert/config"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("IngestorConfig", func() {
	Describe("Merge", func() {
		var (
			c, other *config.IngestorConfig
			mergeErr error
		)

		BeforeEach(func() {
			c = &config.IngestorConfig{
				Port: 42,
				PubSub: config.IngestorPubSub{
					ProjectName:    "orig-pubsub-project-name",
					Topic:          "orig-pubsub-topic",
					PrivateKeyPath: "orig-example-pubsub-private-key",
				},
			}

			other = &config.IngestorConfig{
				Port: 0,
				PubSub: config.IngestorPubSub{
					ProjectName: "new-pubsub-project-name",
				},
				GitHub: config.IngestorGitHub{
					WebhookSecretTokens: []string{"some", "tokens"},
				},
			}
		})

		JustBeforeEach(func() {
			mergeErr = c.Merge(other)
		})

		It("replaces values on the destination when a non-default value is present on the source", func() {
			Expect(mergeErr).NotTo(HaveOccurred())
			Expect(c).To(Equal(&config.IngestorConfig{
				Port: 42,
				PubSub: config.IngestorPubSub{
					ProjectName:    "new-pubsub-project-name",
					Topic:          "orig-pubsub-topic",
					PrivateKeyPath: "orig-example-pubsub-private-key",
				},
				GitHub: config.IngestorGitHub{
					WebhookSecretTokens: []string{"some", "tokens"},
				},
			}))
		})
	})
})
