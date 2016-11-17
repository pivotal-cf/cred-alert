package crypto_test

import (
	rcrypto "cred-alert/crypto"

	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RSA Verifier", func() {
	var (
		verifier rcrypto.Verifier
		priv     *rsa.PrivateKey
		pub      *rsa.PublicKey
	)

	BeforeEach(func() {
		var err error

		priv, err = rsa.GenerateKey(rand.Reader, 1024)
		Expect(err).NotTo(HaveOccurred())

		pub = priv.Public().(*rsa.PublicKey)

		verifier = rcrypto.NewRSAVerifier(pub)
	})

	It("verifies a valid signature", func() {
		msg := []byte("My Special Message")

		hash := sha256.Sum256(msg)

		sig, err := rsa.SignPKCS1v15(rand.Reader, priv, crypto.SHA256, hash[:])
		Expect(err).NotTo(HaveOccurred())

		err = verifier.Verify(msg, sig)
		Expect(err).NotTo(HaveOccurred())
	})

	It("returns an error for an invalid signature", func() {
		msg := []byte("My Special Message")

		err := verifier.Verify(msg, []byte("My Fake Signature"))
		Expect(err).To(HaveOccurred())
	})
})
