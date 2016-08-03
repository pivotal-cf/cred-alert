package dirscanner

import (
	"bufio"
	"cred-alert/mimetype"
	"cred-alert/scanners"
	"cred-alert/scanners/filescanner"
	"cred-alert/sniff"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/cloudfoundry/gunk/workpool"

	"code.cloudfoundry.org/lager"
)

type dirScanner struct {
	handler func(lager.Logger, scanners.Line) error
	sniffer sniff.Sniffer
}

func New(handler func(lager.Logger, scanners.Line) error, sniffer sniff.Sniffer) *dirScanner {
	return &dirScanner{
		handler: handler,
		sniffer: sniffer,
	}
}

func (s *dirScanner) Scan(logger lager.Logger, path string) error {
	workPool, err := workpool.NewWorkPool(2)
	wg := &sync.WaitGroup{}
	wg.Add(1)

	err = s.scan(logger, path, wg, workPool)

	wg.Done()
	wg.Wait()

	workPool.Stop()

	return err
}

func (s *dirScanner) scan(
	logger lager.Logger,
	path string,
	wg *sync.WaitGroup,
	workPool *workpool.WorkPool,
) error {
	children, err := ioutil.ReadDir(path)
	if err != nil {
		return err
	}

	for i := range children {
		if !shouldScan(children[i]) {
			continue
		}

		wholePath := filepath.Join(path, children[i].Name())

		if children[i].IsDir() {
			err := s.scan(logger, wholePath, wg, workPool)
			if err != nil {
				return err
			}
			continue
		}

		if !children[i].Mode().IsRegular() {
			continue
		}

		f, err := os.Open(wholePath)
		if err != nil {
			return err
		}

		wg.Add(1)
		workPool.Submit(func() {
			defer f.Close()
			defer wg.Done()

			if probablyIsText(children[i].Name()) {
				scanner := filescanner.New(f, wholePath)
				s.sniffer.Sniff(logger, scanner, s.handler)
			} else {
				br := bufio.NewReader(f)
				mime, _ := mimetype.IsArchive(logger, br)
				if strings.HasPrefix(mime, "text") {
					scanner := filescanner.New(br, wholePath)
					s.sniffer.Sniff(logger, scanner, s.handler)
				}
			}
		})
	}

	return nil
}

func shouldScan(file os.FileInfo) bool {
	_, skippable := skippableExtensions[filepath.Ext(file.Name())]
	return !skippable
}

var skippableExtensions = map[string]struct{}{
	".crt":  struct{}{},
	".pyc":  struct{}{},
	".so":   struct{}{},
	".mo":   struct{}{},
	".a":    struct{}{},
	".png":  struct{}{},
	".jpeg": struct{}{},
	".jpg":  struct{}{},
	".exe":  struct{}{},
}

func probablyIsText(basename string) bool {
	_, found := probablyTextExtensions[filepath.Ext(basename)]
	if found {
		return true
	}

	_, found = probablyTextFilenames[basename]
	return found
}

var probablyTextFilenames = map[string]struct{}{
	"Gemfile":   struct{}{},
	"LICENSE":   struct{}{},
	"Makefile":  struct{}{},
	"Manifest":  struct{}{},
	"README":    struct{}{},
	"Rakefile":  struct{}{},
	"fstab":     struct{}{},
	"metadata":  struct{}{},
	"monit":     struct{}{},
	"packaging": struct{}{},
	"passwd":    struct{}{},
}

var probablyTextExtensions = map[string]struct{}{
	".MF":           struct{}{},
	".article":      struct{}{},
	".bash":         struct{}{},
	".bat":          struct{}{},
	".c":            struct{}{},
	".cc":           struct{}{},
	".cert":         struct{}{},
	".cfg":          struct{}{},
	".classpath":    struct{}{},
	".cmake":        struct{}{},
	".cnf":          struct{}{},
	".conf":         struct{}{},
	".cpp":          struct{}{},
	".crt":          struct{}{},
	".css":          struct{}{},
	".csv":          struct{}{},
	".document":     struct{}{},
	".dtd":          struct{}{},
	".erb":          struct{}{},
	".feature":      struct{}{},
	".gemfile":      struct{}{},
	".gemspec":      struct{}{},
	".gemtest":      struct{}{},
	".gitignore":    struct{}{},
	".gitkeep":      struct{}{},
	".gitmodules":   struct{}{},
	".go":           struct{}{},
	".h":            struct{}{},
	".haml":         struct{}{},
	".hoerc":        struct{}{},
	".hpp":          struct{}{},
	".html":         struct{}{},
	".irbrc":        struct{}{},
	".java":         struct{}{},
	".js":           struct{}{},
	".json":         struct{}{},
	".jsp":          struct{}{},
	".lock":         struct{}{},
	".log":          struct{}{},
	".m4":           struct{}{},
	".markdown":     struct{}{},
	".md":           struct{}{},
	".md5sums":      struct{}{},
	".mf":           struct{}{},
	".monitrc":      struct{}{},
	".npmignore":    struct{}{},
	".patch":        struct{}{},
	".pem":          struct{}{},
	".php":          struct{}{},
	".phpt":         struct{}{},
	".pl":           struct{}{},
	".po":           struct{}{},
	".properties":   struct{}{},
	".proto":        struct{}{},
	".py":           struct{}{},
	".rake":         struct{}{},
	".rake_example": struct{}{},
	".rb":           struct{}{},
	".rd":           struct{}{},
	".rdoc":         struct{}{},
	".reek":         struct{}{},
	".reg":          struct{}{},
	".rhtml":        struct{}{},
	".rl":           struct{}{},
	".rspec":        struct{}{},
	".rst":          struct{}{},
	".ru":           struct{}{},
	".ruby-gemset":  struct{}{},
	".ruby-version": struct{}{},
	".rvmrc":        struct{}{},
	".sass":         struct{}{},
	".sgml":         struct{}{},
	".sh":           struct{}{},
	".slim":         struct{}{},
	".sql":          struct{}{},
	".t":            struct{}{},
	".text":         struct{}{},
	".thor":         struct{}{},
	".tmpl":         struct{}{},
	".tsv":          struct{}{},
	".txt":          struct{}{},
	".utf8":         struct{}{},
	".xhtml":        struct{}{},
	".xml":          struct{}{},
	".xsd":          struct{}{},
	".yaml":         struct{}{},
	".yardopts":     struct{}{},
	".yml":          struct{}{},
}
