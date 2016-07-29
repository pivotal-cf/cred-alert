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

var decoder *magicmime.Decoder
var archiveMimetypes = []string{
	"application/x-gzip",
	"application/gzip",
	"application/x-tar",
	"application/zip",
}

func init() {
	var err error
	decoder, err = magicmime.NewDecoder(magicmime.MAGIC_MIME_TYPE | magicmime.MAGIC_SYMLINK | magicmime.MAGIC_ERROR)
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

	mime, err := decoder.TypeByBuffer(bs)
	if err != nil {
		logger.Error("failed-to-get-mimetype", err)
	}

	for _, mimetype := range archiveMimetypes {
		if strings.HasPrefix(mime, mimetype) {
			return mimetype, true
		}
	}

	return mime, false
}
