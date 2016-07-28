package archiveiterator

import (
	"bufio"
	"fmt"
	"io"

	"code.cloudfoundry.org/lager"
)

type ArchiveIterator interface {
	Next() (io.ReadCloser, string)
	Close()
}

type NopIterator struct{}

func (i *NopIterator) Next() (io.ReadCloser, string) {
	return nil, ""
}

func (i *NopIterator) Close() {}

func NewIterator(
	logger lager.Logger,
	br *bufio.Reader,
	mime string,
	name string,
) ArchiveIterator {
	switch mime {
	case "application/zip":
		return NewZipIterator(logger, br, name)
	case "application/x-tar":
		return NewTarIterator(logger, br)
	case "application/gzip", "application/x-gzip":
		return NewGzipIterator(logger, br, name)
	default:
		panic(fmt.Sprintf("there is no %s iterator", mime))
	}

	return &NopIterator{}
}
