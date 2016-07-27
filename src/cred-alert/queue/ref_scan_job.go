package queue

import (
	"archive/zip"
	"bytes"
	"cred-alert/github"
	"cred-alert/metrics"
	"cred-alert/mimetype"
	"cred-alert/notifications"
	"cred-alert/scanners"
	"cred-alert/scanners/filescanner"
	"cred-alert/sniff"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/pivotal-golang/lager"
)

const initialCommitParentHash = "0000000000000000000000000000000000000000"

type RefScanJob struct {
	RefScanPlan
	client            github.Client
	sniffer           sniff.Sniffer
	notifier          notifications.Notifier
	emitter           metrics.Emitter
	credentialCounter metrics.Counter
	mimetype          mimetype.Decoder
	id                string
}

func NewRefScanJob(
	plan RefScanPlan,
	client github.Client,
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
	logger.Info("starting")

	if j.Ref == initialCommitParentHash {
		logger.Info("skipped-initial-nil-ref")
		logger.Info("done")
		return nil
	}

	downloadURL, err := j.client.ArchiveLink(j.Owner, j.Repository, j.Ref)
	if err != nil {
		logger.Session("archive-link").Error("failed", err)
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
		logger = logger.Session("archive-reader-file", lager.Data{"filename": f.Name})

		if j.shouldSkip(logger, f) {
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
		handleViolation := j.createHandleViolation(logger, j.Ref, j.Owner+"/"+j.Repository)

		err = j.sniffer.Sniff(logger, bufioScanner, handleViolation)
		if err != nil {
			logger.Error("failed", err)
			return err
		}
	}

	logger.Info("done")
	return nil
}

func downloadArchive(logger lager.Logger, link *url.URL) (*os.File, error) {
	logger.Info("download-archive", lager.Data{
		"url": link.String(),
	})
	logger.Info("starting")

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

	logger.Info("done")
	return tempFile, nil
}

func (j *RefScanJob) createHandleViolation(logger lager.Logger, ref string, repoName string) func(scanners.Line) error {
	return func(line scanners.Line) error {
		logger = logger.Session("handle-violation", lager.Data{
			"path":        line.Path,
			"line-number": line.LineNumber,
			"ref":         ref,
		})
		logger.Info("starting")

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

		logger.Info("done")
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
		logger.Info("done")
		return true
	}
	bytes := buf.Bytes()

	mime, err := j.mimetype.TypeByBuffer(bytes)
	if err != nil {
		logger.Error("failed", err)
		return false
	}

	if strings.HasPrefix(mime, "text") {
		logger.Info("done")
		return false
	}

	logger.Info("done")
	return true
}
