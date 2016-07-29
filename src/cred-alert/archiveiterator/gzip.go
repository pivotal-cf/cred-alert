package archiveiterator

import (
	"bufio"
	"compress/gzip"
	"io"

	"code.cloudfoundry.org/lager"
)

type gzipIterator struct {
	logger lager.Logger
	br     *bufio.Reader
	name   string
}

func NewGzipIterator(logger lager.Logger, br *bufio.Reader, name string) ArchiveIterator {
	return &gzipIterator{
		logger: logger,
		br:     br,
		name:   name,
	}
}

func (i *gzipIterator) Next() (io.ReadCloser, string) {
	reader, err := gzip.NewReader(i.br)
	if err != nil {
		i.logger.Error("failed-to-create-gzip-reader", err, lager.Data{
			"filename": i.name,
		})
		return nil, i.name
	}
	return reader, i.name
}

func (i *gzipIterator) Close() {}
