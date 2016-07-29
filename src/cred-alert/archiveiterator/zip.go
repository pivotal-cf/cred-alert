package archiveiterator

import (
	"archive/zip"
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"

	"code.cloudfoundry.org/lager"
)

type zipIterator struct {
	logger lager.Logger
	br     *bufio.Reader
	cursor int
	reader *zip.ReadCloser
}

func NewZipIterator(logger lager.Logger, br *bufio.Reader, name string) ArchiveIterator {
	var reader *zip.ReadCloser

	if fileExists(name) {
		var err error
		reader, err = zip.OpenReader(name)
		if err != nil {
			logger.Error("failed-to-open-zip-reader", err)
			return &zipIterator{}
		}
	} else {
		buf, err := ioutil.ReadAll(br)
		if err != nil {
			logger.Error("failed-to-read-buffer", err)
			return &zipIterator{}
		}

		byt := bytes.NewReader(buf)
		zr, err := zip.NewReader(byt, int64(len(buf)))
		if err != nil {
			logger.Error(fmt.Sprintf("zip.NewReader(%s) failed", name), err)
			return &zipIterator{}
		}

		reader = &zip.ReadCloser{Reader: *zr}
	}

	return &zipIterator{
		logger: logger,
		reader: reader,
		br:     br,
	}
}

func fileExists(path string) bool {
	_, err := os.Lstat(path)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Fatal(err.Error())
		}

		return false
	}

	return true
}

func (i *zipIterator) Next() (io.ReadCloser, string) {
	if i.reader == nil {
		return nil, ""
	}

RECUR:
	if i.cursor <= len(i.reader.File)-1 {
		rc, err := i.reader.File[i.cursor].Open()
		if err != nil {
			i.logger.Error("failed-to-open-file", err)
			return nil, ""
		}

		if i.reader.File[i.cursor].FileInfo().IsDir() {
			i.logger.Debug("skipping-dir")
			rc.Close()
			i.cursor++
			goto RECUR
		}

		if i.reader.File[i.cursor].UncompressedSize == 0 {
			i.logger.Debug("skipping-zero-length-file")
			rc.Close()
			i.cursor++
			goto RECUR
		}

		name := i.reader.File[i.cursor].Name
		i.cursor++

		return rc, name
	}

	return nil, ""
}

func (i *zipIterator) Close() {
	i.reader.Close()
	i.reader = nil
}

func (i *zipIterator) Name() string { return "zip" }
