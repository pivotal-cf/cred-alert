package sniff

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const knownSHA = "01cc8a314500f863f6520a895fd6efe544406b1933fff5e579388861a91e2c43"

func shouldScan(fi os.FileInfo) bool {
	if fi.IsDir() {
		return false
	}

	if strings.HasSuffix(fi.Name(), "_test.go") {
		return false
	}

	return true
}

var _ = Describe("Updating the RulesVersion", func() {
	It("checks if the RulesVersion needs to be updated", func() {
		pathsToScan := []string{}

		sniffFiles, err := ioutil.ReadDir(".")
		Expect(err).NotTo(HaveOccurred())

		for _, file := range sniffFiles {
			if !shouldScan(file) {
				continue
			}

			if file.Name() == "version.go" {
				continue
			}

			pathsToScan = append(pathsToScan, "./"+file.Name())
		}

		matcherFiles, err := ioutil.ReadDir("./matchers")
		Expect(err).NotTo(HaveOccurred())

		for _, file := range matcherFiles {
			if !shouldScan(file) {
				continue
			}

			pathsToScan = append(pathsToScan, "./matchers/"+file.Name())
		}

		hasher := sha256.New()

		for _, path := range pathsToScan {
			fh, err := os.Open(path)
			Expect(err).NotTo(HaveOccurred())

			_, err = io.Copy(hasher, fh)
			Expect(err).NotTo(HaveOccurred())

			fh.Close()
		}

		currentSHA := hex.EncodeToString(hasher.Sum(nil))

		helpfulMessage := fmt.Sprintf(`Uh oh! It looks the the scanning logic may have changed!

It's important that the sniff.RulesVersion is changed if a change has been made
to the scanning logic. However, if none of the scanning logic/heuristics has
been modified and you pinky swear it, then you just need to update the expected
SHA to: %s

If the scanning logic has changed then you need to update the SHA as above but
also increment the RulesVersion in sniff/version.go.
		`, currentSHA)

		Expect(knownSHA).To(Equal(currentSHA), helpfulMessage)
	})
})
