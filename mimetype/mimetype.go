package mimetype

import (
	"strings"
)

func IsArchive(filename string) (string, bool) {
	if strings.HasSuffix(filename, ".tar") ||
		strings.HasSuffix(filename, ".tar.gz") ||
		strings.HasSuffix(filename, ".tgz") {
		return "application/x-tar", true
	} else if strings.HasSuffix(filename, ".zip") ||
		strings.HasSuffix(filename, ".jar") {
		return "application/zip", true
	} else if strings.HasSuffix(filename, ".gz") {
		return "application/gzip", true
	} else {
		return "", false
	}
}
