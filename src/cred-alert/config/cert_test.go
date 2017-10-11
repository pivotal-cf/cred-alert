package config_test

import (
	"cred-alert/config"
	"io/ioutil"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Cert", func() {
	Describe("LoadCertificateFromFiles", func() {
		It("loads a certificate and an unencrypted private key", func() {
			certFile := "_fixtures/My_Special_Unencrypted_Certificate.crt"
			keyFile := "_fixtures/My_Special_Unencrypted_Certificate.key"

			cert, err := config.LoadCertificateFromFiles(certFile, keyFile, "")
			Expect(err).NotTo(HaveOccurred())

			Expect(cert.Certificate).To(HaveLen(1))
		})

		It("returns an error when the passphrase is incorrect", func() {
			certFile := "_fixtures/My_Special_Encrypted_Certificate.crt"
			keyFile := "_fixtures/My_Special_Encrypted_Certificate.key"

			_, err := config.LoadCertificateFromFiles(certFile, keyFile, "My Incorrect Passphrase")
			Expect(err).To(MatchError(ContainSubstring("decryption password incorrect")))
		})

		It("loads a certificate and an encrypted private key", func() {
			certFile := "_fixtures/My_Special_Encrypted_Certificate.crt"
			keyFile := "_fixtures/My_Special_Encrypted_Certificate.key"

			cert, err := config.LoadCertificateFromFiles(certFile, keyFile, "My Special Passphrase")
			Expect(err).NotTo(HaveOccurred())

			Expect(cert.Certificate).To(HaveLen(1))
		})
	})

	Describe("LoadCertificate", func() {
		It("loads a certificate and an unencrypted private key", func() {
			certBytes, err := ioutil.ReadFile("_fixtures/My_Special_Unencrypted_Certificate.crt")
			Expect(err).NotTo(HaveOccurred())

			keyBytes, err := ioutil.ReadFile("_fixtures/My_Special_Unencrypted_Certificate.key")
			Expect(err).NotTo(HaveOccurred())

			cert, err := config.LoadCertificate(certBytes, keyBytes, "")
			Expect(err).NotTo(HaveOccurred())

			Expect(cert.Certificate).To(HaveLen(1))
		})

		It("returns an error when the passphrase is incorrect", func() {
			certBytes, err := ioutil.ReadFile("_fixtures/My_Special_Encrypted_Certificate.crt")
			Expect(err).NotTo(HaveOccurred())

			keyBytes, err := ioutil.ReadFile("_fixtures/My_Special_Encrypted_Certificate.key")
			Expect(err).NotTo(HaveOccurred())

			_, err = config.LoadCertificate(certBytes, keyBytes, "My Incorrect Passphrase")
			Expect(err).To(MatchError(ContainSubstring("decryption password incorrect")))
		})

		It("loads a certificate and an encrypted private key", func() {
			certBytes, err := ioutil.ReadFile("_fixtures/My_Special_Encrypted_Certificate.crt")
			Expect(err).NotTo(HaveOccurred())

			keyBytes, err := ioutil.ReadFile("_fixtures/My_Special_Encrypted_Certificate.key")
			Expect(err).NotTo(HaveOccurred())

			cert, err := config.LoadCertificate(certBytes, keyBytes, "My Special Passphrase")
			Expect(err).NotTo(HaveOccurred())

			Expect(cert.Certificate).To(HaveLen(1))
		})
	})

	Describe("LoadCertificatePool", func() {
		It("loads a certificate pool", func() {
			certBytes1, err := ioutil.ReadFile("_fixtures/My_Special_Unencrypted_Certificate.crt")
			Expect(err).NotTo(HaveOccurred())

			certBytes2, err := ioutil.ReadFile("_fixtures/My_Special_Encrypted_Certificate.crt")
			Expect(err).NotTo(HaveOccurred())

			certPool, err := config.LoadCertificatePool(certBytes1, certBytes2)
			Expect(err).NotTo(HaveOccurred())

			Expect(certPool.Subjects()).To(HaveLen(2))
		})
	})

	Describe("LoadCertificatePoolFromFiles", func() {
		It("loads a certificate pool", func() {
			certPath1 := "_fixtures/My_Special_Unencrypted_Certificate.crt"
			certPath2 := "_fixtures/My_Special_Encrypted_Certificate.crt"

			certPool, err := config.LoadCertificatePoolFromFiles(certPath1, certPath2)
			Expect(err).NotTo(HaveOccurred())

			Expect(certPool.Subjects()).To(HaveLen(2))
		})
	})
})
