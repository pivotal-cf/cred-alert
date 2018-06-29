package mimetype

import (
	"bufio"
	"io"
	"strings"

	"bitbucket.org/taruti/mimemagic"

	"code.cloudfoundry.org/lager"
)

var archiveMimetypes = []string{
	"application/x-gzip",
	"application/gzip",
	"application/x-tar",
	"application/zip",
}

func Mimetype(logger lager.Logger, r *bufio.Reader) string {
	bs, err := r.Peek(512)
	if err != nil && err != io.EOF {
		logger.Error("failed-to-peek", err, lager.Data{
			"bytes": string(bs),
		})
	}

	if len(bs) == 0 {
		return ""
	}

	return mimemagic.Match("", bs)
}

func IsArchive(logger lager.Logger, r *bufio.Reader) (string, bool) {
	mime := Mimetype(logger, r)
	for i := range archiveMimetypes {
		if strings.HasPrefix(mime, archiveMimetypes[i]) {
			return archiveMimetypes[i], true
		}
	}

	return mime, false
}
