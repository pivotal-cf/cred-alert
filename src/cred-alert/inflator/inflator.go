package inflator

import (
	"bufio"
	"bytes"
	"cred-alert/mimetype"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"code.cloudfoundry.org/lager"
)

func RecursivelyExtractArchive(logger lager.Logger, path, destination string, cleanup bool) error {
	fh, err := os.Open(path)
	if err != nil {
		return err
	}

	br := bufio.NewReader(fh)
	if mime, isArchive := mimetype.IsArchive(logger, br); isArchive {
		basename := filepath.Base(fh.Name())
		nextLevelDestination := filepath.Join(destination, basename+"-contents")
		extractFile(mime, fh.Name(), nextLevelDestination)

		err = fh.Close()
		if err != nil {
			return err
		}

		if cleanup {
			err = os.RemoveAll(fh.Name())
			if err != nil {
				return err
			}
		}

		return recursivelyExtractArchivesInDir(logger, nextLevelDestination, nextLevelDestination)
	}

	err = fh.Close()
	if err != nil {
		return err
	}

	return nil
}

func extractFile(mime, path, destination string) {
	err := os.MkdirAll(destination, 0755)
	if err != nil {
		panic(err.Error())
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
		panic(fmt.Sprintf("don't know how to extract %s", mime))
	}

	buf := &bytes.Buffer{}
	cmd.Stderr = buf
	err = cmd.Run()
	if err != nil {
		fmt.Printf("failed-to-run-cmd: %s\nStderr:\n%s\n", err.Error(), buf.String())
	}
}

func recursivelyExtractArchivesInDir(logger lager.Logger, path, destination string) error {
	children, err := ioutil.ReadDir(path)
	if err != nil {
		return err
	}

	for i := range children {
		basename := children[i].Name()
		wholeName := filepath.Join(path, basename)

		if children[i].IsDir() {
			err := recursivelyExtractArchivesInDir(logger, wholeName, wholeName)
			if err != nil {
				return err
			}
			continue
		}

		if !children[i].Mode().IsRegular() {
			continue
		}

		_, found := nonArchiveExtensions[filepath.Ext(basename)]
		if !found {
			RecursivelyExtractArchive(logger, wholeName, destination, true)
		}
	}

	return nil
}

var nonArchiveExtensions = map[string]struct{}{
	".MF":           struct{}{},
	".S":            struct{}{},
	".a":            struct{}{},
	".am":           struct{}{},
	".article":      struct{}{},
	".au":           struct{}{},
	".autotest":     struct{}{},
	".bash":         struct{}{},
	".bat":          struct{}{},
	".builder":      struct{}{},
	".c":            struct{}{},
	".ca":           struct{}{},
	".cc":           struct{}{},
	".cert":         struct{}{},
	".cfg":          struct{}{},
	".class":        struct{}{},
	".classpath":    struct{}{},
	".cmake":        struct{}{},
	".cnf":          struct{}{},
	".column":       struct{}{},
	".conf":         struct{}{},
	".cpp":          struct{}{},
	".crt":          struct{}{},
	".css":          struct{}{},
	".csv":          struct{}{},
	".dat":          struct{}{},
	".data":         struct{}{},
	".decTest":      struct{}{},
	".def":          struct{}{},
	".devtools":     struct{}{},
	".dir":          struct{}{},
	".document":     struct{}{},
	".dtd":          struct{}{},
	".dumped":       struct{}{},
	".ec":           struct{}{},
	".ecpp":         struct{}{},
	".editorconfig": struct{}{},
	".ejava":        struct{}{},
	".ejs":          struct{}{},
	".eot":          struct{}{},
	".eperl":        struct{}{},
	".ephp":         struct{}{},
	".erb":          struct{}{},
	".erubis":       struct{}{},
	".eruby":        struct{}{},
	".escheme":      struct{}{},
	".example":      struct{}{},
	".exe":          struct{}{},
	".exp":          struct{}{},
	".fcgi":         struct{}{},
	".feature":      struct{}{},
	".gemfile":      struct{}{},
	".gemspec":      struct{}{},
	".gemtest":      struct{}{},
	".gif":          struct{}{},
	".gitignore":    struct{}{},
	".gitkeep":      struct{}{},
	".gitmodules":   struct{}{},
	".go":           struct{}{},
	".golden":       struct{}{},
	".gyp":          struct{}{},
	".h":            struct{}{},
	".haml":         struct{}{},
	".hoerc":        struct{}{},
	".hp":           struct{}{},
	".hpp":          struct{}{},
	".html":         struct{}{},
	".ico":          struct{}{},
	".iml":          struct{}{},
	".in":           struct{}{},
	".input":        struct{}{},
	".irbrc":        struct{}{},
	".iso":          struct{}{}, // to be removed when we support .iso
	".java":         struct{}{},
	".jpeg":         struct{}{},
	".jpg":          struct{}{},
	".jrubydir":     struct{}{},
	".js":           struct{}{},
	".json":         struct{}{},
	".jsp":          struct{}{},
	".keep":         struct{}{},
	".key":          struct{}{},
	".kpeg":         struct{}{},
	".liquid":       struct{}{},
	".list":         struct{}{},
	".lock":         struct{}{},
	".log":          struct{}{},
	".m4":           struct{}{},
	".mab":          struct{}{},
	".markdown":     struct{}{},
	".md":           struct{}{},
	".md5sums":      struct{}{},
	".mf":           struct{}{},
	".mk":           struct{}{},
	".mo":           struct{}{},
	".monitrc":      struct{}{},
	".msg":          struct{}{},
	".mspec":        struct{}{},
	".nokogiri":     struct{}{},
	".npmignore":    struct{}{},
	".obj":          struct{}{},
	".opts":         struct{}{},
	".out":          struct{}{},
	".ovf":          struct{}{},
	".patch":        struct{}{},
	".pdf":          struct{}{},
	".pem":          struct{}{},
	".php":          struct{}{},
	".phpt":         struct{}{},
	".pl":           struct{}{},
	".pm":           struct{}{},
	".png":          struct{}{},
	".po":           struct{}{},
	".postinst":     struct{}{},
	".postrm":       struct{}{},
	".project":      struct{}{},
	".properties":   struct{}{},
	".proto":        struct{}{},
	".psf":          struct{}{},
	".py":           struct{}{},
	".pyc":          struct{}{},
	".pyo":          struct{}{},
	".radius":       struct{}{},
	".rake":         struct{}{},
	".rake_example": struct{}{},
	".rb":           struct{}{},
	".rdoc":         struct{}{},
	".reek":         struct{}{},
	".reg":          struct{}{},
	".result":       struct{}{},
	".rhtml":        struct{}{},
	".rid":          struct{}{},
	".rl":           struct{}{},
	".rsc":          struct{}{},
	".rspec":        struct{}{},
	".rst":          struct{}{},
	".ru":           struct{}{},
	".ruby-gemset":  struct{}{},
	".ruby-version": struct{}{},
	".ry":           struct{}{},
	".s":            struct{}{},
	".sample":       struct{}{},
	".sass":         struct{}{},
	".sgml":         struct{}{},
	".sh":           struct{}{},
	".slim":         struct{}{},
	".sng":          struct{}{},
	".so":           struct{}{},
	".sql":          struct{}{},
	".src":          struct{}{},
	".str":          struct{}{},
	".supp":         struct{}{},
	".svg":          struct{}{},
	".t":            struct{}{},
	".test":         struct{}{},
	".text":         struct{}{},
	".thor":         struct{}{},
	".tmpl":         struct{}{},
	".tsv":          struct{}{},
	".tt":           struct{}{},
	".ttf":          struct{}{},
	".txt":          struct{}{},
	".utf8":         struct{}{},
	".vcproj":       struct{}{},
	".vmdk":         struct{}{},
	".x":            struct{}{},
	".xhtml":        struct{}{},
	".xml":          struct{}{},
	".xsd":          struct{}{},
	".xyz":          struct{}{},
	".y":            struct{}{},
	".yaml":         struct{}{},
	".yardopts":     struct{}{},
	".yml":          struct{}{},
}
