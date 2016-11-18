package inflator

import (
	"bufio"
	"cred-alert/mimetype"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . Inflator

type Inflator interface {
	Inflate(lager.Logger, string, string, string) error
	LogPath() string
	Close() error
}

type inflator struct {
	logfile *os.File
}

func New() Inflator {
	f, err := ioutil.TempFile("", "inflator-errors")
	if err != nil {
		panic("failed creating temp file: " + err.Error())
	}

	return &inflator{
		logfile: f,
	}
}

func (i *inflator) LogPath() string {
	return i.logfile.Name()
}

func (i *inflator) Close() error {
	return i.logfile.Close()
}

func (i *inflator) Inflate(logger lager.Logger, mime, archivePath, destination string) error {
	err := i.extractFile(logger, mime, archivePath, destination)
	if err != nil {
		return err
	}

	i.recursivelyExtractArchivesInDir(logger, destination)

	return nil
}

func (i *inflator) extractFile(logger lager.Logger, mime, path, destination string) error {
	err := os.MkdirAll(destination, 0755)
	if err != nil {
		logger.Error("failed-to-mkdir", err, lager.Data{
			"path": destination,
		})
		return err
	}

	var cmd *exec.Cmd
	switch mime {
	case "application/zip":
		cmd = exec.Command("unzip", "-P", "", "-d", destination, path)
	case "application/x-tar":
		cmd = exec.Command("tar", "xf", path, "-C", destination)
	case "application/gzip", "application/x-gzip":
		fileName := filepath.Base(path)
		fileNameWithoutExt := fileName[:len(fileName)-len(filepath.Ext(fileName))]
		output, err := os.Create(filepath.Join(destination, fileNameWithoutExt))
		if err != nil {
			panic(err.Error())
		}
		defer output.Close()

		cmd = exec.Command("gunzip", "-c", path)
		cmd.Stdout = output
	default:
		logger.Error("unknown-archive-type", err, lager.Data{
			"mimetype": mime,
		})
		return errors.New(fmt.Sprintf("don't know how to extract %s", mime))
	}

	cmd.Stderr = i.logfile
	err = cmd.Run()
	if err != nil {
		// We've already logged the output to a file. Let's just keep going.
	}

	return nil
}

func (i *inflator) recursivelyExtractArchivesInDir(logger lager.Logger, dir string) {
	children, err := ioutil.ReadDir(dir)
	if err != nil {
		logger.Error("failed-to-read-dir", err, lager.Data{
			"path": dir,
		})
		return
	}

	for c := range children {
		basename := children[c].Name()
		absPath := filepath.Join(dir, basename)

		if children[c].IsDir() {
			i.recursivelyExtractArchivesInDir(logger, absPath)
			continue
		}

		if !children[c].Mode().IsRegular() {
			continue
		}

		_, found := nonArchiveExtensions[filepath.Ext(basename)]
		if !found {
			fh, err := os.Open(absPath)
			if err != nil {
				logger.Error("failed-to-open-path", err, lager.Data{
					"path": absPath,
				})
				continue
			}

			mime, isArchive := mimetype.IsArchive(logger, bufio.NewReader(fh))

			err = fh.Close()
			if err != nil {
				logger.Error("failed-to-close-fh", err, lager.Data{
					"fh": fh.Name(),
				})
			}

			if isArchive {
				extractDir := absPath + "-contents"
				i.extractFile(logger, mime, absPath, extractDir)

				err = os.RemoveAll(absPath)
				if err != nil {
					logger.Error("failed-to-clean-up-path", err, lager.Data{
						"path": absPath,
					})
				}

				i.recursivelyExtractArchivesInDir(logger, extractDir)
			}
		}
	}
}

var nonArchiveExtensions = map[string]struct{}{
	".MF":           {},
	".S":            {},
	".a":            {},
	".am":           {},
	".article":      {},
	".au":           {},
	".autotest":     {},
	".bash":         {},
	".bat":          {},
	".builder":      {},
	".c":            {},
	".ca":           {},
	".cc":           {},
	".cert":         {},
	".cfg":          {},
	".class":        {},
	".classpath":    {},
	".cmake":        {},
	".cnf":          {},
	".column":       {},
	".conf":         {},
	".cpp":          {},
	".crt":          {},
	".css":          {},
	".csv":          {},
	".dat":          {},
	".data":         {},
	".decTest":      {},
	".def":          {},
	".devtools":     {},
	".dir":          {},
	".document":     {},
	".dtd":          {},
	".dumped":       {},
	".ec":           {},
	".ecpp":         {},
	".editorconfig": {},
	".ejava":        {},
	".ejs":          {},
	".eot":          {},
	".eperl":        {},
	".ephp":         {},
	".erb":          {},
	".erubis":       {},
	".eruby":        {},
	".escheme":      {},
	".example":      {},
	".exe":          {},
	".exp":          {},
	".fcgi":         {},
	".feature":      {},
	".gemfile":      {},
	".gemspec":      {},
	".gemtest":      {},
	".gif":          {},
	".gitignore":    {},
	".gitkeep":      {},
	".gitmodules":   {},
	".go":           {},
	".golden":       {},
	".gyp":          {},
	".h":            {},
	".haml":         {},
	".hoerc":        {},
	".hp":           {},
	".hpp":          {},
	".html":         {},
	".ico":          {},
	".iml":          {},
	".in":           {},
	".input":        {},
	".irbrc":        {},
	".iso":          {}, // to be removed when we support .iso
	".java":         {},
	".jpeg":         {},
	".jpg":          {},
	".jrubydir":     {},
	".js":           {},
	".json":         {},
	".jsp":          {},
	".keep":         {},
	".key":          {},
	".kpeg":         {},
	".liquid":       {},
	".list":         {},
	".lock":         {},
	".log":          {},
	".m4":           {},
	".mab":          {},
	".markdown":     {},
	".md":           {},
	".md5sums":      {},
	".mf":           {},
	".mk":           {},
	".mo":           {},
	".monitrc":      {},
	".msg":          {},
	".mspec":        {},
	".nokogiri":     {},
	".npmignore":    {},
	".obj":          {},
	".opts":         {},
	".out":          {},
	".ovf":          {},
	".patch":        {},
	".pdf":          {},
	".pem":          {},
	".php":          {},
	".phpt":         {},
	".pl":           {},
	".pm":           {},
	".png":          {},
	".po":           {},
	".postinst":     {},
	".postrm":       {},
	".project":      {},
	".properties":   {},
	".proto":        {},
	".psf":          {},
	".py":           {},
	".pyc":          {},
	".pyo":          {},
	".radius":       {},
	".rake":         {},
	".rake_example": {},
	".rb":           {},
	".rdoc":         {},
	".reek":         {},
	".reg":          {},
	".result":       {},
	".rhtml":        {},
	".rid":          {},
	".rl":           {},
	".rsc":          {},
	".rspec":        {},
	".rst":          {},
	".ru":           {},
	".ruby-gemset":  {},
	".ruby-version": {},
	".ry":           {},
	".s":            {},
	".sample":       {},
	".sass":         {},
	".sgml":         {},
	".sh":           {},
	".slim":         {},
	".sng":          {},
	".so":           {},
	".sql":          {},
	".src":          {},
	".str":          {},
	".supp":         {},
	".svg":          {},
	".t":            {},
	".test":         {},
	".text":         {},
	".thor":         {},
	".tmpl":         {},
	".tsv":          {},
	".tt":           {},
	".ttf":          {},
	".txt":          {},
	".utf8":         {},
	".vcproj":       {},
	".vmdk":         {},
	".x":            {},
	".xhtml":        {},
	".xml":          {},
	".xsd":          {},
	".xyz":          {},
	".y":            {},
	".yaml":         {},
	".yardopts":     {},
	".yml":          {},
}
