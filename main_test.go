package main_test

import (
	"archive/zip"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/pivotal-cf/cred-alert/scanners/diffscanner"

	"code.cloudfoundry.org/archiver/compressor"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Main", func() {
	var (
		cmdArgs       []string
		fakeTempDir   string
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
+AKIAJDHEYSPVNSHFKSMS

 ## Suspicious Variable Names
`
	)

	var politeText = `
		words
		NotACredential
		words
	`

	BeforeEach(func() {
		stdin = ""
		cmdArgs = []string{}
	})

	Describe("ScanCommand", func() {
		JustBeforeEach(func() {
			finalArgs := append([]string{"scan"}, cmdArgs...)
			cmd := exec.Command(cliPath, finalArgs...)

			var err error
			fakeTempDir, err = ioutil.TempDir("", "cred-alert-main")
			Expect(err).NotTo(HaveOccurred())

			originalTemp := os.Getenv("TMPDIR")
			Expect(os.Setenv("TMPDIR", fakeTempDir)).To(Succeed())
			cmd.Env = os.Environ()
			Expect(os.Setenv("TMPDIR", originalTemp)).To(Succeed())

			if stdin != "" {
				cmd.Stdin = strings.NewReader(stdin)
			}

			session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			Expect(os.RemoveAll(fakeTempDir)).To(Succeed())
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

		ItShowsTheCredentialInTheOutput := func(expectedCredential string) {
			Context("shows actual credential if show-suspected-credentials flag is set", func() {
				BeforeEach(func() {
					cmdArgs = append(cmdArgs, "--show-suspected-credentials")
				})

				It("shows credentials", func() {
					Eventually(session.Out).Should(gbytes.Say(expectedCredential))
				})
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
			ItShowsTheCredentialInTheOutput("AKIASOMEMORETEXTHERE")

			Context("when given a --diff flag", func() {
				BeforeEach(func() {
					cmdArgs = []string{"--diff"}
					stdin = offendingDiff
				})

				It("scans the diff", func() {
					Eventually(session.Out).Should(gbytes.Say("spec/integration/git-secrets-pattern-tests.txt:28"))
				})

				ItShowsTheCredentialInTheOutput("AKIAJDHEYSPVNSHFKSMS")
				ItTellsPeopleHowToRemoveTheirCredentials()
			})

			Context("when parsing a large diff", func() {
				var buf []byte
				BeforeEach(func() {
					cmdArgs = []string{"--diff"}
					buf = make([]byte, diffscanner.MaxLineSize*3)
					for i := range buf {
						buf[i] = 'A'
					}
				})

				Context("when diff has credentials", func() {
					BeforeEach(func() {
						stdin = offendingDiff + "\n" + "+" + string(buf) + "\n"
					})
					It("fails with error when a line is too long", func() {
						Eventually(session.Out).Should(gbytes.Say("FAILED.*scanning failed"))
						Eventually(session).Should(gexec.Exit(3))
					})
				})

				Context("when diff has no credentials", func() {
					BeforeEach(func() {
						stdin = "+" + string(buf) + "\n"
					})
					It("fails with error when a line is too long", func() {
						Eventually(session.Out).Should(gbytes.Say("FAILED.*scanning failed"))
						Eventually(session).ShouldNot(gexec.Exit(0))
					})
				})
			})

			Context("when no credentials are found", func() {

				BeforeEach(func() {
					cmdArgs = []string{}
					stdin = politeText
				})

				It("exits with status 0", func() {
					Eventually(session).Should(gexec.Exit(0))
				})
			})
		})

		Context("when given regexp flag with a value", func() {
			BeforeEach(func() {
				cmdArgs = []string{
					"--diff",
					"--regexp=random",
				}
				stdin = `
diff --git a/spec/integration/git-secrets-pattern-tests.txt b/spec/integration/git-secrets-pattern-tests.txt
index 940393e..fa5a232 100644
--- a/spec/integration/git-secrets-pattern-tests.txt
+++ b/spec/integration/git-secrets-pattern-tests.txt
@@ -28,7 +28,7 @@ header line goes here
+randomunsuspectedthing

 ## Suspicious Variable Names
`
			})

			It("uses the given regexp pattern", func() {
				Eventually(session.Out).Should(gbytes.Say("[CRED]"))
			})
			Context("when regex-file flags is set", func() {
				BeforeEach(func() {
					cmdArgs = append(cmdArgs, "--regexp-file=some-non-existing-file")
				})

				It("prints warning message", func() {
					Eventually(session.Out).Should(gbytes.Say("[WARN]"))
				})

				It("uses the given regexp pattern", func() {
					Eventually(session.Out).Should(gbytes.Say("[CRED]"))
				})
			})

		})

		Context("when given regex-file flag and the file reads successfully", func() {
			var (
				tmpFile *os.File
				err     error
			)

			BeforeEach(func() {
				tmpFile, err = ioutil.TempFile("", "tmp-file")
				Expect(err).NotTo(HaveOccurred())

				regexpContent := `this-does-not-match
another-pattern
does-not-match`

				err = ioutil.WriteFile(tmpFile.Name(), []byte(regexpContent), 0644)
				Expect(err).NotTo(HaveOccurred())

				cmdArgs = []string{
					fmt.Sprintf("--regexp-file=%s", tmpFile.Name()),
					"--diff",
					"--show-suspected-credentials",
				}
			})

			AfterEach(func() {
				err := tmpFile.Close()
				Expect(err).NotTo(HaveOccurred())
				os.RemoveAll(tmpFile.Name())
			})

			Context("uses the regex", func() {
				Context("and there are no matches", func() {
					BeforeEach(func() {
						stdin = offendingDiff
					})

					It("returns not match", func() {
						Consistently(session.Out).ShouldNot(gbytes.Say("[CRED]"))
					})
				})

				Context("and multiple regex matches", func() {
					BeforeEach(func() {
						stdin = `
diff --git a/spec/integration/git-secrets-pattern-tests.txt b/spec/integration/git-secrets-pattern-tests.txt
index 940393e..fa5a232 100644
--- a/spec/integration/git-secrets-pattern-tests.txt
+++ b/spec/integration/git-secrets-pattern-tests.txt
@@ -28,7 +28,7 @@ header line goes here
+this-does-not-match
+another-pattern
+pattern-another

 ## Suspicious Variable Names
`
					})

					It("scans the diff", func() {
						Eventually(session.Out).Should(gbytes.Say("[CRED]"))
					})
				})
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

			ItTellsPeopleHowToRemoveTheirCredentials()

			It("scans the file", func() {
				Eventually(session.Out).Should(gbytes.Say("[CRED]"))
			})

			It("exits with status 3", func() {
				Eventually(session).Should(gexec.Exit(3))
			})

			Context("shows actual credential if show-suspected-credentials flag is set", func() {
				BeforeEach(func() {
					cmdArgs = append(cmdArgs, "--show-suspected-credentials")
				})

				It("shows credentials", func() {
					Eventually(session.Out).Should(gbytes.Say("AKIASOMEMORETEXTHERE"))
				})
			})

			var ItShowsHowLongInflationTook = func() {
				It("shows how long the inflating took", func() {
					Eventually(session.Out).Should(gbytes.Say(`Time taken \(inflating\):`))
				})
			}

			Context("when the file is a folder", func() {
				var (
					inDir, outDir string
				)

				AfterEach(func() {
					os.RemoveAll(inDir)
					os.RemoveAll(outDir)
				})

				Context("when given a folder", func() {
					BeforeEach(func() {
						var err error
						outDir, err = ioutil.TempDir("", "folder-in")
						Expect(err).NotTo(HaveOccurred())

						err = ioutil.WriteFile(path.Join(outDir, "file1"), []byte(offendingText), 0644)
						Expect(err).NotTo(HaveOccurred())

						cmdArgs = []string{"-f", outDir}
					})

					It("scans each text file in the folder", func() {
						Eventually(session.Out).Should(gbytes.Say("[CRED]"))
					})

					ItTellsPeopleHowToRemoveTheirCredentials()

					Context("and there is an archive in the folder", func() {
						BeforeEach(func() {
							Expect(os.RemoveAll(filepath.Join(outDir, "file1"))).To(Succeed())

							var err error
							inDir, err = ioutil.TempDir("", "tar-in")
							Expect(err).NotTo(HaveOccurred())

							err = ioutil.WriteFile(path.Join(inDir, "file1"), []byte(offendingText), 0664)
							Expect(err).NotTo(HaveOccurred())

							tarFilePath := path.Join(outDir, "out.tar")
							tarFile, err := os.Create(tarFilePath)
							Expect(err).NotTo(HaveOccurred())
							defer tarFile.Close()

							err = compressor.WriteTar(inDir, tarFile)
							Expect(err).NotTo(HaveOccurred())
						})

						It("leaves a temporary violations directory with the content of the violation", func() {
							Eventually(session).Should(gexec.Exit(3))

							files, err := ioutil.ReadDir(fakeTempDir)
							Expect(err).NotTo(HaveOccurred())

							Expect(len(files)).To(Equal(1))
						})
					})

					Context("when vendor directories are present", func() {
						BeforeEach(func() {
							Expect(os.RemoveAll(filepath.Join(outDir, "file1"))).To(Succeed())

							vendorDir := path.Join(outDir, "vendor")
							err := os.MkdirAll(vendorDir, 0755)
							Expect(err).NotTo(HaveOccurred())

							err = ioutil.WriteFile(path.Join(vendorDir, "file1"), []byte(offendingText), 0644)
							Expect(err).NotTo(HaveOccurred())
						})

						It("ignores credentials in top level vendor directories", func() {
							Eventually(session).Should(gexec.Exit(0))
						})

						Context("when there are nested vendor directories", func() {
							BeforeEach(func() {
								nestedVendor := path.Join(outDir, "foo", "vendor")
								err := os.MkdirAll(nestedVendor, 0755)
								Expect(err).NotTo(HaveOccurred())

								err = ioutil.WriteFile(path.Join(nestedVendor, "file1"), []byte(offendingText), 0644)
								Expect(err).NotTo(HaveOccurred())
							})

							It("ignores credentials in nested level vendor directories", func() {
								Eventually(session).Should(gexec.Exit(0))
							})
						})

						Context("when there are nested vendor directories", func() {
							BeforeEach(func() {
								nestedVendor := path.Join(outDir, "foo", "vendorsarecool")
								err := os.MkdirAll(nestedVendor, 0755)
								Expect(err).NotTo(HaveOccurred())

								err = ioutil.WriteFile(path.Join(nestedVendor, "file1"), []byte(offendingText), 0644)
								Expect(err).NotTo(HaveOccurred())
							})

							It("detects credentials in nested level directories with the word vendor in them", func() {
								Eventually(session).Should(gexec.Exit(3))
							})
						})
					})
				})
			})
			Context("when the file is a zip file with a vendor directory", func() {
				var (
					inDir, outDir, zipFilePath string
				)

				BeforeEach(func() {
					var err error
					inDir, err = ioutil.TempDir("", "zipper-unzip-in")
					Expect(err).NotTo(HaveOccurred())

					vendorDir := path.Join(inDir, "vendor")
					err = os.MkdirAll(vendorDir, 0755)
					Expect(err).NotTo(HaveOccurred())

					err = ioutil.WriteFile(path.Join(vendorDir, "file1"), []byte(offendingText), 0644)
					Expect(err).NotTo(HaveOccurred())

					outDir, err = ioutil.TempDir("", "zipper-unzip-out")
					Expect(err).NotTo(HaveOccurred())

					zipFilePath = path.Join(outDir, "out.zip")
					err = zipit(inDir, zipFilePath, "")
					Expect(err).NotTo(HaveOccurred())
					cmdArgs = []string{"-f", zipFilePath}
				})

				AfterEach(func() {
					os.RemoveAll(inDir)
					os.RemoveAll(outDir)
				})

				It("ignores credentials in nested level vendor directories", func() {
					Eventually(session).Should(gexec.Exit(0))
				})
			})
			Context("when the file is a zip file", func() {
				var (
					inDir, outDir, zipFilePath string
				)

				BeforeEach(func() {
					var err error
					inDir, err = ioutil.TempDir("", "zipper-unzip-in")
					Expect(err).NotTo(HaveOccurred())

					err = ioutil.WriteFile(path.Join(inDir, "file1"), []byte(offendingText), 0644)
					Expect(err).NotTo(HaveOccurred())

					outDir, err = ioutil.TempDir("", "zipper-unzip-out")
					Expect(err).NotTo(HaveOccurred())

					zipFilePath = path.Join(outDir, "out.zip")
					err = zipit(inDir, zipFilePath, "")
					Expect(err).NotTo(HaveOccurred())
				})

				AfterEach(func() {
					os.RemoveAll(inDir)
					os.RemoveAll(outDir)
				})

				Context("and it contains another zip inside it", func() {
					var DZOutDir string

					BeforeEach(func() {
						var err error
						DZOutDir, err = ioutil.TempDir("", "Double-zip-out")
						Expect(err).NotTo(HaveOccurred())

						zipFilePath = path.Join(DZOutDir, "out.zip")
						err = zipit(outDir, zipFilePath, "")
						Expect(err).NotTo(HaveOccurred())

						cmdArgs = []string{"-f", zipFilePath}
					})

					AfterEach(func() {
						Expect(os.RemoveAll(DZOutDir)).To(Succeed())
					})

					It("leaves a temporary violations directory with the content of the violation", func() {
						Eventually(session).Should(gexec.Exit(3))

						files, err := ioutil.ReadDir(fakeTempDir)
						Expect(err).NotTo(HaveOccurred())

						Expect(len(files)).To(Equal(1))
					})
				})

				Context("when given a zip without prefix bytes", func() {
					BeforeEach(func() {
						cmdArgs = []string{"-f", zipFilePath}
					})

					It("scans each text file in the zip", func() {
						Eventually(session.Out).Should(gbytes.Say("[CRED]"))
					})

					ItShowsHowLongInflationTook()
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

				ItShowsHowLongInflationTook()
				ItShowsTheCredentialInTheOutput("AKIASOMEMORETEXTHERE")
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

					tarFilePath := path.Join(outDir, "out.tgz")

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

				ItShowsHowLongInflationTook()
			})

			Context("when no credentials are found", func() {
				var tmpFile *os.File

				BeforeEach(func() {
					var err error
					tmpFile, err = ioutil.TempFile("", "cli-main-test")
					Expect(err).NotTo(HaveOccurred())
					defer tmpFile.Close()

					err = ioutil.WriteFile(tmpFile.Name(), []byte(politeText), os.ModePerm)
					Expect(err).NotTo(HaveOccurred())

					cmdArgs = []string{"-f", tmpFile.Name()}
				})

				AfterEach(func() {
					os.RemoveAll(tmpFile.Name())
				})

				It("exits with status 0", func() {
					Eventually(session).Should(gexec.Exit(0))
				})

				It("removes the violations directory", func() {
					Eventually(session).Should(gexec.Exit(0))

					files, err := ioutil.ReadDir(fakeTempDir)
					Expect(err).NotTo(HaveOccurred())
					Expect(len(files)).To(Equal(0))
				})
			})
		})
	})

	Describe("VersionCommand", func() {

		var (
			cmd *exec.Cmd
		)

		BeforeEach(func() {
			finalArgs := append([]string{"version"}, cmdArgs...)
			cmd = exec.Command(cliPath, finalArgs...)
			if stdin != "" {
				cmd.Stdin = strings.NewReader(stdin)
			}
		})

		It("prints the version", func() {
			var err error
			session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))
			Expect(session.Out).Should(gbytes.Say("dev"))
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
