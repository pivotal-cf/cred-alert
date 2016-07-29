package archiveiterator

import (
	"archive/tar"
	"bufio"
	"io"
	"io/ioutil"

	"code.cloudfoundry.org/lager"
)

type tarIterator struct {
	r      *tar.Reader
	logger lager.Logger
}

func NewTarIterator(logger lager.Logger, br *bufio.Reader) ArchiveIterator {
	return &tarIterator{
		r:      tar.NewReader(br),
		logger: logger,
	}
}

func (i *tarIterator) Next() (io.ReadCloser, string) {
	if i.r == nil {
		return nil, ""
	}

RECUR:
	header, err := i.r.Next()
	if err == io.EOF {
		i.r = nil
		return nil, ""
	}

	if err != nil {
		i.logger.Error("failed-to-advance-tar", err)
		return nil, ""
	}

	if header.FileInfo().IsDir() {
		goto RECUR
	}

	return ioutil.NopCloser(i.r), header.Name
}

func (i *tarIterator) Close()       {}
func (i *tarIterator) Name() string { return "tar" }
