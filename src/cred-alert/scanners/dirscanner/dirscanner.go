package dirscanner

import (
	"bufio"
	"cred-alert/inflator"
	"cred-alert/mimetype"
	"cred-alert/scanners"
	"cred-alert/scanners/filescanner"
	"cred-alert/sniff"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . Sniffer

type Sniffer interface {
	Sniff(lager.Logger, sniff.Scanner, sniff.ViolationHandlerFunc) error
}

type DirScanner struct {
	handler                    func(lager.Logger, scanners.Violation) error
	sniffer                    Sniffer
	inflateDir                 string
	inflator                   inflator.Inflator
	scannedDirContainsArchives bool
}

func New(
	sniffer Sniffer,
	handler sniff.ViolationHandlerFunc,
	inflateDir string,
) *DirScanner {
	return &DirScanner{
		sniffer:    sniffer,
		handler:    handler,
		inflateDir: inflateDir,
		inflator:   inflator.New(),
	}
}

func (s *DirScanner) Scan(logger lager.Logger, path string) error {
	err := s.scan(logger, path, s.handler)
	if err != nil {
		return err
	}

	if s.scannedDirContainsArchives {
		err = s.scan(logger, s.inflateDir, s.handler)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *DirScanner) scan(
	logger lager.Logger,
	path string,
	handler sniff.ViolationHandlerFunc,
) error {
	children, err := ioutil.ReadDir(path)
	if err != nil {
		log.Printf("failed to read dir: %s", path)
		return nil
	}

	for i := range children {
		child := children[i]

		_, skippable := skippableExtensions[filepath.Ext(child.Name())]
		if skippable {
			continue
		}

		wholePath := filepath.Join(path, child.Name())

		if child.IsDir() {
			err := s.scan(logger, wholePath, handler)
			if err != nil {
				return err
			}
			continue
		}

		if !child.Mode().IsRegular() {
			continue
		}

		f, err := os.Open(wholePath)
		if err != nil {
			log.Println("failed to open:", wholePath)
			continue
		}

		if probablyIsText(child.Name()) {
			scanner := filescanner.New(f, wholePath)
			err := s.sniffer.Sniff(logger, scanner, handler)
			if err != nil {
				return err
			}
		} else {
			br := bufio.NewReader(f)
			mime, isArchive := mimetype.IsArchive(logger, br)
			if isArchive {
				s.scannedDirContainsArchives = true
				destPath := filepath.Join(s.inflateDir, path, child.Name())
				srcPath := filepath.Join(path, child.Name())
				_ = s.inflator.Inflate(logger, mime, srcPath, destPath)
			} else if mime == "" || strings.HasPrefix(mime, "text") {
				scanner := filescanner.New(br, wholePath)
				err := s.sniffer.Sniff(logger, scanner, handler)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

var skippableExtensions = map[string]struct{}{
	".crt":  {},
	".pyc":  {},
	".so":   {},
	".mo":   {},
	".a":    {},
	".obj":  {},
	".png":  {},
	".jpeg": {},
	".jpg":  {},
	".exe":  {},
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
	"Gemfile":   {},
	"LICENSE":   {},
	"Makefile":  {},
	"Manifest":  {},
	"README":    {},
	"Rakefile":  {},
	"fstab":     {},
	"metadata":  {},
	"monit":     {},
	"packaging": {},
	"passwd":    {},
}

var probablyTextExtensions = map[string]struct{}{
	".MF":           {},
	".article":      {},
	".bash":         {},
	".bat":          {},
	".c":            {},
	".cc":           {},
	".cert":         {},
	".cfg":          {},
	".classpath":    {},
	".cmake":        {},
	".cnf":          {},
	".conf":         {},
	".cpp":          {},
	".crt":          {},
	".css":          {},
	".csv":          {},
	".document":     {},
	".dtd":          {},
	".erb":          {},
	".feature":      {},
	".gemfile":      {},
	".gemspec":      {},
	".gemtest":      {},
	".gitignore":    {},
	".gitkeep":      {},
	".gitmodules":   {},
	".go":           {},
	".h":            {},
	".haml":         {},
	".hoerc":        {},
	".hpp":          {},
	".html":         {},
	".irbrc":        {},
	".java":         {},
	".js":           {},
	".json":         {},
	".jsp":          {},
	".key":          {},
	".lock":         {},
	".log":          {},
	".m4":           {},
	".markdown":     {},
	".md":           {},
	".md5sums":      {},
	".mf":           {},
	".monitrc":      {},
	".npmignore":    {},
	".patch":        {},
	".pem":          {},
	".php":          {},
	".phpt":         {},
	".pl":           {},
	".po":           {},
	".properties":   {},
	".proto":        {},
	".py":           {},
	".rake":         {},
	".rake_example": {},
	".rb":           {},
	".rd":           {},
	".rdoc":         {},
	".reek":         {},
	".reg":          {},
	".rhtml":        {},
	".rl":           {},
	".rspec":        {},
	".rst":          {},
	".ru":           {},
	".ruby-gemset":  {},
	".ruby-version": {},
	".rvmrc":        {},
	".sass":         {},
	".sgml":         {},
	".sh":           {},
	".slim":         {},
	".sql":          {},
	".t":            {},
	".text":         {},
	".thor":         {},
	".tmpl":         {},
	".tsv":          {},
	// .txt seems like the one extension you can count on to be text.
	//
	// Unfortunately not.
	//
	// There is a file in the Go codebase that is used to test the `tar` package
	// in the standard library. It's called `16gb.txt` and it contains 16
	// gigabytes of null bytes. We assume this is text and try to read the lines
	// into memory which ends poorly.
	//
	// ".txt":          struct{}{},
	".utf8":     {},
	".xhtml":    {},
	".xml":      {},
	".xsd":      {},
	".yaml":     {},
	".yardopts": {},
	".yml":      {},
}
