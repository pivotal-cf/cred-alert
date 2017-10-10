package config_test

import (
	"cred-alert/config"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Cert", func() {
	Describe("LoadCertificate", func() {
		It("loads a certificate and an unencrypted private key", func() {
			certPath := "_fixtures/My_Special_Unencrypted_Certificate.crt"
			keyPath := "_fixtures/My_Special_Unencrypted_Certificate.key"

			cert, err := config.LoadCertificate(certPath, keyPath, "")
			Expect(err).NotTo(HaveOccurred())

			Expect(cert.Certificate).To(HaveLen(1))
		})

		It("returns an error when the passphrase is incorrect", func() {
			certPath := "_fixtures/My_Special_Encrypted_Certificate.crt"
			keyPath := "_fixtures/My_Special_Encrypted_Certificate.key"

			_, err := config.LoadCertificate(certPath, keyPath, "My Incorrect Passphrase")
			Expect(err).To(MatchError(ContainSubstring("decryption password incorrect")))
		})

		It("loads a certificate and an encrypted private key", func() {
			certPath := "_fixtures/My_Special_Encrypted_Certificate.crt"
			keyPath := "_fixtures/My_Special_Encrypted_Certificate.key"

			cert, err := config.LoadCertificate(certPath, keyPath, "My Special Passphrase")
			Expect(err).NotTo(HaveOccurred())

			Expect(cert.Certificate).To(HaveLen(1))
		})
	})

	Describe("LoadCertificatePool", func() {
		It("loads a certificate pool", func() {
			certPath1 := "_fixtures/My_Special_Unencrypted_Certificate.crt"
			certPath2 := "_fixtures/My_Special_Encrypted_Certificate.crt"

			certPool, err := config.LoadCertificatePool(certPath1, certPath2)
			Expect(err).NotTo(HaveOccurred())

			Expect(certPool.Subjects()).To(HaveLen(2))
		})
	})
})
