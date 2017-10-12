package commands

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/kardianos/osext"

	"code.cloudfoundry.org/lager"

	"cred-alert/inflator"
	credlog "cred-alert/log"
	"cred-alert/mimetype"
	"cred-alert/scanners"
	"cred-alert/scanners/diffscanner"
	"cred-alert/scanners/dirscanner"
	"cred-alert/scanners/filescanner"
	"cred-alert/sniff"
	"cred-alert/sniff/matchers"
)

type ScanCommand struct {
	File            string `short:"f" long:"file" description:"the file or directory to scan" value-name:"FILE"`
	Diff            bool   `long:"diff" description:"content to be scanned is a git diff"`
	ShowCredentials bool   `long:"show-suspected-credentials" description:"allow credentials to be shown in output"`
	Regexp          string `long:"regexp" description:"override default regexp matcher" value-name:"REGEXP"`
	RegexpFile      string `long:"regexp-file" description:"path to regexp file" value-name:"PATH"`
}

func (command *ScanCommand) Execute(args []string) error {
	warnIfOldExecutable()

	logger := lager.NewLogger("scan")
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.INFO))

	if command.Regexp != "" && command.RegexpFile != "" {
		fmt.Fprintln(os.Stderr, yellow("[WARN]"), "Two options specified for Regexp, only using: --regexp", command.Regexp)
	}

	sniffer, err := command.buildSniffer()
	if err != nil {
		return err
	}

	clean := newCleanup()
	handler := newCredentialCounter(command.ShowCredentials)

	switch {
	case command.File != "":
		if err := command.scanFile(logger, sniffer, handler.HandleViolation, clean); err != nil {
			return err
		}
	case command.Diff:
		err = sniffer.Sniff(logger, diffscanner.NewDiffScanner(os.Stdin), handler.HandleViolation)
	default:
		err = sniffer.Sniff(logger, filescanner.New(os.Stdin, "STDIN"), handler.HandleViolation)
	}

	if handler.count > 0 {
		showCredentialWarning()
		clean.exit(3)
	}

	if err != nil {
		return err
	}

	return nil
}

func (c *ScanCommand) scanFile(logger lager.Logger, sniffer sniff.Sniffer, handleFunc sniff.ViolationHandlerFunc, cleaner *cleanup) error {
	fi, err := os.Stat(c.File)
	if err != nil {
		return err
	}

	inflateDir, err := ioutil.TempDir("", "cred-alert-cli")
	if err != nil {
		return err
	}
	defer func() {
		f, err := ioutil.ReadDir(inflateDir)
		if err == nil && len(f) <= 0 {
			os.RemoveAll(inflateDir)
		}
	}()

	quietLogger := credlog.NewNullLogger()
	scanner := dirscanner.New(sniffer, handleFunc, inflateDir)
	if fi.IsDir() {
		return scanner.Scan(quietLogger, c.File)
	}

	file, err := os.Open(c.File)
	if err != nil {
		return err
	}

	br := bufio.NewReader(file)
	if mime, isArchive := mimetype.IsArchive(logger, br); isArchive {
		inflate := inflator.New()
		defer inflate.Close()

		inflateArchive(quietLogger, inflate, inflateDir, mime, c.File)

		err = scanner.Scan(quietLogger, inflateDir)
		if err != nil {
			return err
		}
	} else {
		err := sniffer.Sniff(logger, filescanner.New(br, c.File), handleFunc)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *ScanCommand) buildSniffer() (sniff.Sniffer, error) {
	var sniffer sniff.Sniffer

	switch {
	case c.Regexp != "":
		sniffer = sniff.NewSniffer(matchers.Format(c.Regexp), nil)
	case c.RegexpFile != "":
		file, err := os.Open(c.RegexpFile)
		if err != nil {
			return nil, err
		}

		matcher := matchers.UpcasedMultiMatcherFromReader(file)

		if err := file.Close(); err != nil {
			return nil, err
		}

		sniffer = sniff.NewSniffer(matcher, nil)
	default:
		sniffer = sniff.NewDefaultSniffer()
	}

	return sniffer, nil
}

type cleanup struct {
	work []func()
}

func newCleanup() *cleanup {
	clean := &cleanup{}

	signalsCh := make(chan os.Signal)
	signal.Notify(signalsCh, os.Interrupt)

	go func() {
		<-signalsCh
		log.SetFlags(0)
		log.Println("\ncleaning up...")
		clean.exit(1)
	}()

	return clean
}

func (c *cleanup) register(fn func()) {
	c.work = append(c.work, fn)
}

func (c cleanup) exit(status int) {
	for _, w := range c.work {
		w()
	}

	os.Exit(status)
}

func showCredentialWarning() {
	fmt.Println()
	fmt.Println("Yikes! Looks like we found some credentials.")
	fmt.Println()
	fmt.Println("There are a few cases for what this may be:")
	fmt.Println()
	fmt.Println("1. An actual credential in a repository which shouldn't be")
	fmt.Println("   committed! Remove it and try committing again.")
	fmt.Println()
	fmt.Println("2. An example credential in tests or documentation. You can")
	fmt.Println("   use the words 'fake' and/or 'example' in your credential so it is")
	fmt.Println("   ignored.")
	fmt.Println()
	fmt.Println("3. An actual credential in a credential repository. If you are calling this")
	fmt.Println("   via Git hook and if you want the false positive to go away, you can pass `-n`")
	fmt.Println("   to skip the hook for now.")
	fmt.Println()
	fmt.Println("4. A false positive which isn't a credential at all! Please let us know about ")
	fmt.Println("   the this case in our Slack channel (#pcf-sec-enablement).")
}

func inflateArchive(
	logger lager.Logger,
	inflate inflator.Inflator,
	inflateDir string,
	mime string,
	file string,
) error {
	inflateStart := time.Now()
	fmt.Print("Inflating archive... ")
	err := inflate.Inflate(logger, mime, file, inflateDir)
	if err != nil {
		fmt.Printf("%s\n", red("FAILED"))
		return err
	}
	fmt.Printf("%s\n", green("DONE"))

	fmt.Println()
	fmt.Println("Time taken (inflating):", time.Since(inflateStart))
	fmt.Println("Any archive inflation errors can be found in: ", inflate.LogPath())
	fmt.Println()

	return nil
}

func warnIfOldExecutable() {
	const twoWeeks = 14 * 24 * time.Hour

	exePath, err := osext.Executable()
	if err != nil {
		return
	}

	info, err := os.Stat(exePath)
	if err != nil {
		return
	}

	mtime := info.ModTime()

	if time.Since(mtime) > twoWeeks {
		fmt.Fprintln(os.Stderr, yellow("[WARN]"), "Executable is old! Please consider running `cred-alert-cli update`.")
	}
}

func newCredentialCounter(showCreds bool) *credentialCounter {
	return &credentialCounter{
		showCreds: showCreds,
	}
}

type credentialCounter struct {
	count     int
	showCreds bool
}

func (c *credentialCounter) HandleViolation(logger lager.Logger, violation scanners.Violation) error {
	line := violation.Line
	c.count++
	output := fmt.Sprintf("%s %s:%d", red("[CRED]"), line.Path, line.LineNumber)
	if c.showCreds {
		output = output + fmt.Sprintf(" [%s]", violation.Credential())
	}
	fmt.Println(output)

	return nil
}
