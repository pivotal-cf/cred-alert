package sniff

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const expectedSHA = "5a02277c42c7fcfc4f763ef62b2d24f8d56bdc32e7a5731e3760feb3f867367d"

func shouldNotScan(fi os.FileInfo) bool {
	return fi.IsDir() || strings.HasSuffix(fi.Name(), "_test.go")
}

var _ = Describe("RulesVersion", func() {
	It("checks if the RulesVersion needs to be updated", func() {
		pathsToScan := []string{}

		sniffFiles, err := ioutil.ReadDir(".")
		Expect(err).NotTo(HaveOccurred())

		for _, file := range sniffFiles {
			if shouldNotScan(file) {
				continue
			}

			if file.Name() == "version.go" {
				continue
			}

			pathsToScan = append(pathsToScan, file.Name())
		}

		matcherFiles, err := ioutil.ReadDir("./matchers")
		Expect(err).NotTo(HaveOccurred())

		for _, file := range matcherFiles {
			if shouldNotScan(file) {
				continue
			}

			pathsToScan = append(pathsToScan, filepath.Join("matchers", file.Name()))
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

		Expect(expectedSHA).To(Equal(currentSHA), helpfulMessage)
	})
})
