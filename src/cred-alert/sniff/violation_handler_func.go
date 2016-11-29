package sniff

import (
	"cred-alert/scanners"
	"io"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/lager"
)

type ViolationHandlerFunc func(lager.Logger, scanners.Violation) error

// NewArchiveViolationHandlerFunc returns a ViolationHandlerFunc that copies
// the file a credential was found from the inflateDir to the violationDir
func NewArchiveViolationHandlerFunc(
	inflateDir string,
	violationDir string,
	handler ViolationHandlerFunc,
) ViolationHandlerFunc {
	return func(logger lager.Logger, violation scanners.Violation) error {
		line := violation.Line

		relPath, err := filepath.Rel(inflateDir, line.Path)
		if err != nil {
			return err
		}

		destPath := filepath.Join(violationDir, relPath)

		_, err = os.Lstat(destPath)
		if os.IsNotExist(err) {
			err = os.MkdirAll(filepath.Dir(destPath), os.ModePerm)
			if err != nil {
				return err
			}

			destFile, err := os.Create(destPath)
			if err != nil {
				return err
			}
			defer destFile.Close()

			srcFile, err := os.Open(line.Path)
			if err != nil {
				return err
			}
			defer srcFile.Close()

			_, err = io.Copy(destFile, srcFile)
			if err != nil {
				return err
			}
		}

		return handler(logger, violation)
	}
}
