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

var _ = Describe("RSA Signer", func() {
	var (
		signer rcrypto.Signer
		priv   *rsa.PrivateKey
		pub    *rsa.PublicKey
	)

	BeforeEach(func() {
		var err error

		priv, err = rsa.GenerateKey(rand.Reader, 1024)
		Expect(err).NotTo(HaveOccurred())

		pub = priv.Public().(*rsa.PublicKey)

		signer = rcrypto.NewRSASigner(priv)
	})

	It("signs a message", func() {
		msg := []byte("My Special Message")

		sig, err := signer.Sign(msg)
		Expect(err).NotTo(HaveOccurred())

		hash := sha256.Sum256(msg)

		err = rsa.VerifyPKCS1v15(pub, crypto.SHA256, hash[:], sig)
		Expect(err).NotTo(HaveOccurred())
	})
})
