package config_test

import (
	"cred-alert/config"
	"io/ioutil"

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
			cert1, err := ioutil.ReadFile("_fixtures/My_Special_Unencrypted_Certificate.crt")
			Expect(err).NotTo(HaveOccurred())

			cert2, err := ioutil.ReadFile("_fixtures/My_Special_Encrypted_Certificate.crt")
			Expect(err).NotTo(HaveOccurred())

			certPool, err := config.LoadCertificatePool(string(cert1), string(cert2))
			Expect(err).NotTo(HaveOccurred())

			Expect(certPool.Subjects()).To(HaveLen(2))
		})
	})
})
