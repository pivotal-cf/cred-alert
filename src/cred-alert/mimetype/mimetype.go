package mimetype

import (
	"bufio"
	"io"
	"log"
	"strings"

	"code.cloudfoundry.org/lager"

	"github.com/rakyll/magicmime"
)

//go:generate counterfeiter . Decoder

type Decoder interface {
	TypeByBuffer([]byte) (string, error)
}

var archiveMimetypes = []string{
	"application/x-gzip",
	"application/gzip",
	"application/x-tar",
	"application/zip",
}

func init() {
	err := magicmime.Open(magicmime.MAGIC_MIME_TYPE | magicmime.MAGIC_SYMLINK | magicmime.MAGIC_ERROR)
	if err != nil {
		log.Fatalf("failed to make new decoder: %s", err.Error())
	}
}

func IsArchive(logger lager.Logger, r *bufio.Reader) (string, bool) {
	if r == nil {
		return "", false
	}

	bs, err := r.Peek(512)
	if err != nil && err != io.EOF {
		logger.Error("failed-to-peek", err, lager.Data{
			"bytes": string(bs),
		})
	}

	if len(bs) == 0 {
		return "", false
	}

	mime, err := magicmime.TypeByBuffer(bs)
	if err != nil {
		logger.Error("failed-to-get-mimetype", err)
	}

	for i := range archiveMimetypes {
		if strings.HasPrefix(mime, archiveMimetypes[i]) {
			return archiveMimetypes[i], true
		}
	}

	return mime, false
}
