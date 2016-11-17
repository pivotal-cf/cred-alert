package crypto_test

import (
	"cred-alert/crypto"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"io/ioutil"
	"os"
)

var _ = Describe("Parser", func() {
	Describe("ReadRSAPublicKey", func() {
		It("parses a PEM-encoded RSA public key", func() {
			contents := `-----BEGIN PUBLIC KEY-----
MFwwDQYJKoZIhvcNAQEBBQADSwAwSAJBAJi06OV+y6bUpKntORicJAp6z4OwNZt8
MLQEvxLsLUiYDyYgwJAt9k8f5dbqTMFCOlxefzCvWel9HeTjaOmrZakCAwEAAQ==
-----END PUBLIC KEY-----`

			tmp := createTempFileWithContents(contents)
			defer os.Remove(tmp)

			key, err := crypto.ReadRSAPublicKey(tmp)
			Expect(err).NotTo(HaveOccurred())

			Expect(key.E).NotTo(BeZero())
		})

		It("returns an error if the file doesn't exist", func() {
			_, err := crypto.ReadRSAPublicKey("/some/nonsense")
			Expect(err).To(HaveOccurred())
		})

		It("returns an error if the file doesn't contain PEM", func() {
			tmp := createTempFileWithContents("SOME NONSENSE")
			defer os.Remove(tmp)

			_, err := crypto.ReadRSAPublicKey(tmp)
			Expect(err).To(HaveOccurred())
		})

		It("returns an error if the file doesn't contain valid DER", func() {
			contents := "-----BEGIN PUBLIC KEY-----\n-----END PUBLIC KEY-----"

			tmp := createTempFileWithContents(contents)
			defer os.Remove(tmp)

			_, err := crypto.ReadRSAPublicKey(tmp)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("ReadRSAPrivateKey", func() {
		It("parses a PEM-encoded RSA private key", func() {
			contents := `-----BEGIN RSA PRIVATE KEY-----
MIIBOQIBAAJBAJi06OV+y6bUpKntORicJAp6z4OwNZt8MLQEvxLsLUiYDyYgwJAt
9k8f5dbqTMFCOlxefzCvWel9HeTjaOmrZakCAwEAAQJAKMDPDsAh9Wn2b+sBO9If
xDQ2QTy7cb1Y+hHyNEiXZTHWAKEEBR7dXE14RMi/45I+odYF8MWFp5TIiJTg4lYU
yQIhAMYEWCpgP3bhOquw0YFwulREEdnPaHsklkmuUTvd2wAjAiEAxWwJVLqWyUUp
zrXCEzxb9KwQdlNDNGiJRJrXtY/LucMCIHm3WepSVzBfqYy3l1AVVrNNVBuqXfKz
vp1zxQMjj+Y5AiBnxGx3I4gEDJ138CMtVymCRjp05zjIwDV+YOEGpqlPXwIgCS5u
2avDRBdaquzIl/rixRssOQlIYLR9dRlmFNXspgU=
-----END RSA PRIVATE KEY-----`

			tmp := createTempFileWithContents(contents)
			defer os.Remove(tmp)

			key, err := crypto.ReadRSAPrivateKey(tmp)
			Expect(err).NotTo(HaveOccurred())

			Expect(key.Primes).NotTo(BeEmpty())
		})

		It("returns an error if the file doesn't exist", func() {
			_, err := crypto.ReadRSAPrivateKey("/some/nonsense")
			Expect(err).To(HaveOccurred())
		})

		It("returns an error if the file doesn't contain PEM", func() {
			tmp := createTempFileWithContents("SOME NONSENSE")
			defer os.Remove(tmp)

			_, err := crypto.ReadRSAPrivateKey(tmp)
			Expect(err).To(HaveOccurred())
		})

		It("returns an error if the file doesn't contain valid DER", func() {
			contents := "-----BEGIN RSA PRIVATE KEY-----\n-----END RSA PRIVATE KEY-----"

			tmp := createTempFileWithContents(contents)
			defer os.Remove(tmp)

			_, err := crypto.ReadRSAPrivateKey(tmp)
			Expect(err).To(HaveOccurred())
		})
	})
})

func createTempFileWithContents(contents string) string {
	tmp, _ := ioutil.TempFile("", "some.key")
	tmp.WriteString(contents)
	tmp.Close()

	return tmp.Name()
}
