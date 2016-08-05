package queue

import (
	"archive/zip"
	"bytes"
	"cred-alert/githubclient"
	"cred-alert/metrics"
	"cred-alert/mimetype"
	"cred-alert/notifications"
	"cred-alert/scanners"
	"cred-alert/scanners/filescanner"
	"cred-alert/sniff"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"

	"code.cloudfoundry.org/lager"
)

const initialCommitParentHash = "0000000000000000000000000000000000000000"

type RefScanJob struct {
	RefScanPlan
	client            githubclient.Client
	sniffer           sniff.Sniffer
	notifier          notifications.Notifier
	emitter           metrics.Emitter
	credentialCounter metrics.Counter
	mimetype          mimetype.Decoder
	id                string
}

func NewRefScanJob(
	plan RefScanPlan,
	client githubclient.Client,
	sniffer sniff.Sniffer,
	notifier notifications.Notifier,
	emitter metrics.Emitter,
	mimetype mimetype.Decoder,
	id string,
) *RefScanJob {
	credentialCounter := emitter.Counter("cred_alert.violations")

	job := &RefScanJob{
		RefScanPlan:       plan,
		client:            client,
		sniffer:           sniffer,
		notifier:          notifier,
		emitter:           emitter,
		credentialCounter: credentialCounter,
		mimetype:          mimetype,
		id:                id,
	}

	return job
}

func (j *RefScanJob) Run(logger lager.Logger) error {
	logger = logger.Session("ref-scan", lager.Data{
		"owner":      j.Owner,
		"repository": j.Repository,
		"ref":        j.Ref,
		"task-id":    j.id,
		"private":    j.Private,
	})
	logger.Debug("starting")

	if j.Ref == initialCommitParentHash {
		logger.Info("skipped-initial-nil-ref")
		logger.Debug("done")
		return nil
	}

	downloadURL, err := j.client.ArchiveLink(j.Owner, j.Repository, j.Ref)
	if err != nil {
		logger.Session("archive-link").Error("failed", err)
		if err == githubclient.ErrNotFound {
			return nil
		}
		return err
	}

	archiveFile, err := downloadArchive(logger, downloadURL)
	if err != nil {
		logger.Error("failed", err)
		return err
	}
	defer os.Remove(archiveFile.Name())
	defer archiveFile.Close()

	archiveReader, err := zip.OpenReader(archiveFile.Name())
	if err != nil {
		logger.Session("zip-open-reader").Error("failed", err)
		return err
	}
	defer archiveReader.Close()

	logger.Info("unzipped-archive", lager.Data{
		"file-count": len(archiveReader.File),
	})

	logger.Info("scanning-unzipped-files")

	for _, f := range archiveReader.File {
		l := logger.Session("archive-reader-file", lager.Data{"filename": f.Name})

		if j.shouldSkip(l, f) {
			logger.Info("skipped")
			continue
		}

		unzippedReader, err := f.Open()
		if err != nil {
			logger.Error("failed", err)
			continue
		}
		defer unzippedReader.Close()

		bufioScanner := filescanner.New(unzippedReader, f.Name)
		handleViolation := j.createHandleViolation(j.Ref, j.Owner+"/"+j.Repository)

		err = j.sniffer.Sniff(l, bufioScanner, handleViolation)
		if err != nil {
			l.Error("failed", err)
			return err
		}
	}

	logger.Debug("done")

	return nil
}

func downloadArchive(logger lager.Logger, link *url.URL) (*os.File, error) {
	logger = logger.Session("downloading-archive")

	if link == nil {
		err := errors.New("Archive link was nil")
		logger.Error("failed", err)
		return nil, err
	}

	logger.Debug("starting")

	tempFile, err := ioutil.TempFile("", "downloaded-git-archive")
	if err != nil {
		logger.Error("failed", err)
		return nil, err
	}

	resp, err := http.Get(link.String())
	defer resp.Body.Close()
	if err != nil {
		logger.Error("failed", err)
		return nil, err
	}

	_, err = io.Copy(tempFile, resp.Body)
	if err != nil {
		logger.Error("failed", err)
		return nil, err
	}

	logger.Debug("done")
	return tempFile, nil
}

func (j *RefScanJob) createHandleViolation(ref string, repoName string) func(lager.Logger, scanners.Line) error {
	return func(logger lager.Logger, line scanners.Line) error {
		logger = logger.Session("handle-violation", lager.Data{
			"path":        line.Path,
			"line-number": line.LineNumber,
			"ref":         ref,
		})
		logger.Debug("starting")

		err := j.notifier.SendNotification(logger, repoName, ref, line, j.Private)
		if err != nil {
			logger.Error("failed", err)
			return err
		}

		tag := "public"
		if j.Private {
			tag = "private"
		}

		j.credentialCounter.Inc(logger, tag)

		logger.Debug("done")
		return nil
	}
}

func (j *RefScanJob) shouldSkip(logger lager.Logger, f *zip.File) bool {
	logger = logger.Session("consider-skipping")

	unzippedReader, err := f.Open()
	if err != nil {
		logger.Error("failed", err)
		return false
	}
	defer unzippedReader.Close()

	buf := new(bytes.Buffer)
	numBytes, err := buf.ReadFrom(unzippedReader)
	if err != nil {
		logger.Error("failed", err)
		return false
	}
	if numBytes <= 0 {
		logger.Debug("done")
		return true
	}
	bytes := buf.Bytes()

	mime, err := j.mimetype.TypeByBuffer(bytes)
	if err != nil {
		logger.Error("failed", err)
		return false
	}

	if strings.HasPrefix(mime, "text") {
		logger.Debug("done")
		return false
	}

	logger.Debug("done")
	return true
}
