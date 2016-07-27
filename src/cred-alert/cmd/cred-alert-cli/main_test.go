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
		stdin         io.Reader
		session       *gexec.Session
		offendingText = `
			words
			AKIASOMEMORETEXTHERE
			words
		`
	)

	JustBeforeEach(func() {
		cmd := exec.Command(cliPath, cmdArgs...)
		if stdin != nil {
			cmd.Stdin = stdin
		}

		var err error
		session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).ToNot(HaveOccurred())
	})

	Context("when given content on stdin", func() {
		BeforeEach(func() {
			stdin = strings.NewReader(offendingText)
		})

		It("scans stdin", func() {
			Eventually(session.Out).Should(gbytes.Say("Line matches pattern!"))
		})
	})

	Context("when given a directory flag", func() {
		var tmpDir string

		BeforeEach(func() {
			var err error
			tmpDir, err = ioutil.TempDir("", "cli-main-test")
			Expect(err).NotTo(HaveOccurred())

			tmpFile, err := ioutil.TempFile(tmpDir, "cli-main-test")
			Expect(err).NotTo(HaveOccurred())
			defer tmpFile.Close()

			ioutil.WriteFile(tmpFile.Name(), []byte(offendingText), os.ModePerm)

			cmdArgs = []string{"-d", filepath.Dir(tmpFile.Name())}
		})

		AfterEach(func() {
			os.RemoveAll(tmpDir)
		})

		It("scans each file in the directory", func() {
			Eventually(session.Out).Should(gbytes.Say("Line matches pattern!"))
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
			Eventually(session.Out).Should(gbytes.Say("Line matches pattern!"))
		})

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
					err = zipit(filepath.Join(inDir, "/"), zipFilePath, "")
					Expect(err).NotTo(HaveOccurred())

					cmdArgs = []string{"-f", zipFilePath}
				})

				It("scans each text file in the zip", func() {
					Eventually(session.Out).Should(gbytes.Say("Line matches pattern!"))
				})
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
				Eventually(session.Out).Should(gbytes.Say("Line matches pattern!"))
			})
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
				Eventually(session.Out).Should(gbytes.Say("Line matches pattern!"))
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

		header.Name = strings.TrimPrefix(path, source)

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
