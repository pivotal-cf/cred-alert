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

func IsArchive(logger lager.Logger, r *bufio.Reader) (string, bool) {
	bs, err := r.Peek(512)
	if err != nil && err != io.EOF {
		logger.Error("failed-to-peek", err, lager.Data{
			"bytes": string(bs),
		})
	}

	if len(bs) == 0 {
		return "", false
	}

	mime := mimemagic.Match("", bs)

	for i := range archiveMimetypes {
		if strings.HasPrefix(mime, archiveMimetypes[i]) {
			return archiveMimetypes[i], true
		}
	}

	return mime, false
}
