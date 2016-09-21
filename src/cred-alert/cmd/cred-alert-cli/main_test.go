package main_test

import (
	"archive/zip"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/cloudfoundry/archiver/compressor"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Main", func() {
	var (
		cmdArgs       []string
		stdin         string
		session       *gexec.Session
		offendingText = `
			words
			AKIASOMEMORETEXTHERE
			words
		`
		offendingDiff = `
diff --git a/spec/integration/git-secrets-pattern-tests.txt b/spec/integration/git-secrets-pattern-tests.txt
index 940393e..fa5a232 100644
--- a/spec/integration/git-secrets-pattern-tests.txt
+++ b/spec/integration/git-secrets-pattern-tests.txt
@@ -28,7 +28,7 @@ header line goes here
+private_key '$should_match'

 ## Suspicious Variable Names
`
	)

	BeforeEach(func() {
		stdin = ""
	})

	JustBeforeEach(func() {
		cmdArgs = append([]string{"scan"}, cmdArgs...)
		cmd := exec.Command(cliPath, cmdArgs...)
		if stdin != "" {
			cmd.Stdin = strings.NewReader(stdin)
		}

		var err error
		session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).ToNot(HaveOccurred())
	})

	ItTellsPeopleHowToRemoveTheirCredentials := func() {
		It("tells people how to add example credentials to their tests or documentation", func() {
			Eventually(session.Out).Should(gbytes.Say("fake"))
		})

		It("tells people how to skip git hooks running for other false positives", func() {
			Eventually(session.Out).Should(gbytes.Say("-n"))
		})

		It("tells people how to reach us", func() {
			Eventually(session.Out).Should(gbytes.Say("Slack channel"))
		})
	}

	Context("when given content on stdin", func() {
		BeforeEach(func() {
			stdin = offendingText
		})

		It("scans stdin", func() {
			Eventually(session.Out).Should(gbytes.Say("[CRED]"))
			Eventually(session.Out).Should(gbytes.Say("STDIN"))
		})

		It("exits with status 3", func() {
			Eventually(session).Should(gexec.Exit(3))
		})

		ItTellsPeopleHowToRemoveTheirCredentials()

		Context("when given a --diff flag", func() {
			BeforeEach(func() {
				cmdArgs = []string{"--diff"}
				stdin = offendingDiff
			})

			It("scans the diff", func() {
				Eventually(session.Out).Should(gbytes.Say("spec/integration/git-secrets-pattern-tests.txt:28"))
			})

			Context("shows actual credential if show-suspected-credentials flag is set", func() {
				BeforeEach(func() {
					cmdArgs = append(cmdArgs, "--show-suspected-credentials")
				})

				It("shows credentials", func() {
					Eventually(session.Out).Should(gbytes.Say(`private_key '\$should_match'`))
				})
			})

			ItTellsPeopleHowToRemoveTheirCredentials()
		})
	})

	Context("when given a file flag", func() {
		var tmpFile *os.File

		BeforeEach(func() {
			var err error
			tmpFile, err = ioutil.TempFile("", "cli-main-test")
			Expect(err).NotTo(HaveOccurred())
			defer tmpFile.Close()

			ioutil.WriteFile(tmpFile.Name(), []byte(offendingText), os.ModePerm)

			cmdArgs = []string{"-f", tmpFile.Name()}
		})

		AfterEach(func() {
			os.RemoveAll(tmpFile.Name())
		})

		It("scans the file", func() {
			Eventually(session.Out).Should(gbytes.Say("[CRED]"))
		})

		Context("shows actual credential if show-suspected-credentials flag is set", func() {
			BeforeEach(func() {
				cmdArgs = append(cmdArgs, "--show-suspected-credentials")
			})

			It("shows credentials", func() {
				Eventually(session.Out).Should(gbytes.Say("AKIASOMEMORETEXTHERE"))
			})
		})

		var ItShowsHowLongItTookAndHowManyCredentialsWereFound = func() {
			It("shows how long the inflating took", func() {
				Eventually(session.Out).Should(gbytes.Say(`Time taken \(inflating\):`))
			})

			It("shows how long the scan took", func() {
				Eventually(session.Out).Should(gbytes.Say(`Time taken \(scanning\):`))
			})

			It("shows show many credentials were found", func() {
				Eventually(session.Out).Should(gbytes.Say("Credentials found: 1"))
			})
		}

		It("exits with status 3", func() {
			Eventually(session).Should(gexec.Exit(3))
		})

		ItTellsPeopleHowToRemoveTheirCredentials()

		Context("when the file is a zip file", func() {
			var (
				inDir, outDir string
			)

			AfterEach(func() {
				os.RemoveAll(inDir)
				os.RemoveAll(outDir)
			})

			Context("when given a zip without prefix bytes", func() {
				BeforeEach(func() {
					var err error
					inDir, err = ioutil.TempDir("", "zipper-unzip-in")
					Expect(err).NotTo(HaveOccurred())

					err = ioutil.WriteFile(path.Join(inDir, "file1"), []byte(offendingText), 0664)
					Expect(err).NotTo(HaveOccurred())

					outDir, err = ioutil.TempDir("", "zipper-unzip-out")
					Expect(err).NotTo(HaveOccurred())

					zipFilePath := path.Join(outDir, "out.zip")
					err = zipit(inDir, zipFilePath, "")
					Expect(err).NotTo(HaveOccurred())

					cmdArgs = []string{"-f", zipFilePath}
				})

				It("scans each text file in the zip", func() {
					Eventually(session.Out).Should(gbytes.Say("[CRED]"))
				})

				ItShowsHowLongItTookAndHowManyCredentialsWereFound()
			})
		})

		Context("when the file is a tar file", func() {
			var (
				inDir, outDir string
			)

			BeforeEach(func() {
				var err error
				inDir, err = ioutil.TempDir("", "tar-in")
				Expect(err).NotTo(HaveOccurred())

				err = ioutil.WriteFile(path.Join(inDir, "file1"), []byte(offendingText), 0664)
				Expect(err).NotTo(HaveOccurred())

				outDir, err = ioutil.TempDir("", "tar-out")
				Expect(err).NotTo(HaveOccurred())

				tarFilePath := path.Join(outDir, "out.tar")
				tarFile, err := os.Create(tarFilePath)
				Expect(err).NotTo(HaveOccurred())
				defer tarFile.Close()

				err = compressor.WriteTar(inDir, tarFile)
				Expect(err).NotTo(HaveOccurred())

				cmdArgs = []string{"-f", tarFilePath}
			})

			AfterEach(func() {
				os.RemoveAll(inDir)
				os.RemoveAll(outDir)
			})

			It("scans each text file in the tar", func() {
				Eventually(session.Out).Should(gbytes.Say("[CRED]"))
			})

			ItShowsHowLongItTookAndHowManyCredentialsWereFound()
		})

		Context("when the file is a gzipped tar file", func() {
			var (
				inDir, outDir string
			)

			BeforeEach(func() {
				var err error
				inDir, err = ioutil.TempDir("", "tar-in")
				Expect(err).NotTo(HaveOccurred())

				err = ioutil.WriteFile(path.Join(inDir, "file1"), []byte(offendingText), 0664)
				Expect(err).NotTo(HaveOccurred())

				outDir, err = ioutil.TempDir("", "tar-out")
				Expect(err).NotTo(HaveOccurred())

				tarFilePath := path.Join(outDir, "out.tar")

				c := compressor.NewTgz()
				err = c.Compress(inDir, tarFilePath)
				Expect(err).NotTo(HaveOccurred())

				cmdArgs = []string{"-f", tarFilePath}
			})

			AfterEach(func() {
				os.RemoveAll(inDir)
				os.RemoveAll(outDir)
			})

			It("scans each text file in the tar", func() {
				Eventually(session.Out).Should(gbytes.Say("[CRED]"))
			})

			ItShowsHowLongItTookAndHowManyCredentialsWereFound()
		})
	})

	Context("When no credentials are found", func() {
		var politeText = `
			words
			NotACredential
			words
		`
		var tmpFile *os.File

		Context("when given content on stdin", func() {
			BeforeEach(func() {
				cmdArgs = []string{}
				stdin = politeText
			})
			It("exits with status 0", func() {
				Eventually(session).Should(gexec.Exit(0))
			})
		})

		Context("when given a file flag", func() {
			BeforeEach(func() {
				var err error
				tmpFile, err = ioutil.TempFile("", "cli-main-test")
				Expect(err).NotTo(HaveOccurred())
				defer tmpFile.Close()

				ioutil.WriteFile(tmpFile.Name(), []byte(politeText), os.ModePerm)

				cmdArgs = []string{"-f", tmpFile.Name()}
			})

			AfterEach(func() {
				os.RemoveAll(tmpFile.Name())
			})

			It("exits with status 0", func() {
				Eventually(session).Should(gexec.Exit(0))
			})
		})
	})
})

// Thanks to Svett Ralchev
// http://blog.ralch.com/tutorial/golang-working-with-zip/
func zipit(source, target, prefix string) error {
	zipfile, err := os.Create(target)
	if err != nil {
		return err
	}
	defer zipfile.Close()

	if prefix != "" {
		_, err = io.WriteString(zipfile, prefix)
		if err != nil {
			return err
		}
	}

	archive := zip.NewWriter(zipfile)
	defer archive.Close()

	err = filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		relpath, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		header.Name = strings.TrimPrefix(relpath, source)

		if info.IsDir() {
			header.Name += string(os.PathSeparator)
		} else {
			header.Method = zip.Deflate
		}

		writer, err := archive.CreateHeader(header)
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(writer, file)
		return err
	})

	return err
}
