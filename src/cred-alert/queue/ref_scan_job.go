package queue

import (
	"archive/zip"
	"bytes"
	"cred-alert/github"
	"cred-alert/metrics"
	"cred-alert/mimetype"
	"cred-alert/notifications"
	"cred-alert/scanners"
	"cred-alert/scanners/file"
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
	mimetype          mimetype.Mimetype
	id                string
}

func NewRefScanJob(
	plan RefScanPlan,
	client github.Client,
	sniffer sniff.Sniffer,
	notifier notifications.Notifier,
	emitter metrics.Emitter,
	mimetype mimetype.Mimetype,
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

	if j.Ref == initialCommitParentHash {
		logger.Info("skipped-initial-nil-ref")
		return nil
	}

	downloadURL, err := j.client.ArchiveLink(j.Owner, j.Repository, j.Ref)
	if err != nil {
		logger.Error("error-creating-archive-link", err)
		return err
	}

	archiveFile, err := downloadArchive(logger, downloadURL)
	if err != nil {
		logger.Error("error-downloading-archive", err)
		return err
	}
	defer os.Remove(archiveFile.Name())
	defer archiveFile.Close()

	archiveReader, err := zip.OpenReader(archiveFile.Name())
	if err != nil {
		logger.Error("error-unzipping-archive", err)
		return err
	}
	defer archiveReader.Close()

	logger.Info("unzipped-archive", lager.Data{
		"file-count": len(archiveReader.File),
	})

	logger.Info("scanning-unzipped-files")

	for _, f := range archiveReader.File {
		isText, err := j.isText(f)
		if err != nil {
			logger.Error("mimetype-error", err)
		}

		if !isText {
			logger.Info("skipped-non-text-file", lager.Data{"filename": f.Name})
			continue
		}

		unzippedReader, err := f.Open()
		if err != nil {
			logger.Error("error-reading-archive", err)
			continue
		}
		defer unzippedReader.Close()

		bufioScanner := file.NewReaderScanner(unzippedReader, f.Name)
		handleViolation := j.createHandleViolation(logger, j.Ref, j.Owner+"/"+j.Repository)

		err = j.sniffer.Sniff(logger, bufioScanner, handleViolation)
		if err != nil {
			return err
		}
	}

	logger.Info("done")

	return nil
}

func downloadArchive(logger lager.Logger, link *url.URL) (*os.File, error) {
	logger.Info("downloading-archive", lager.Data{
		"url": link.String(),
	})

	tempFile, err := ioutil.TempFile("", "downloaded-git-archive")
	if err != nil {
		logger.Error("error-creating-archive-temp-file", err)
		return nil, err
	}

	resp, err := http.Get(link.String())
	defer resp.Body.Close()
	if err != nil {
		return nil, err
	}

	_, err = io.Copy(tempFile, resp.Body)
	if err != nil {
		return nil, err
	}

	return tempFile, nil
}

func (j *RefScanJob) createHandleViolation(logger lager.Logger, ref string, repoName string) func(scanners.Line) error {
	return func(line scanners.Line) error {
		logger.Info("found-credential", lager.Data{
			"path":        line.Path,
			"line-number": line.LineNumber,
			"ref":         ref,
		})

		err := j.notifier.SendNotification(logger, repoName, ref, line)
		if err != nil {
			return err
		}

		tag := "public"
		if j.Private {
			tag = "private"
		}

		j.credentialCounter.Inc(logger, tag)

		return nil
	}
}

func (j *RefScanJob) isText(f *zip.File) (bool, error) {
	unzippedReader, err := f.Open()
	if err != nil {
		return false, err
	}
	defer unzippedReader.Close()

	buf := new(bytes.Buffer)
	numBytes, err := buf.ReadFrom(unzippedReader)
	if err != nil {
		return false, err
	}
	if numBytes <= 0 {
		return false, nil
	}
	bytes := buf.Bytes()

	mime, err := j.mimetype.TypeByBuffer(bytes)
	if err != nil {
		return false, err
	}

	if strings.HasPrefix(mime, "text") {
		return true, nil
	}

	return false, nil
}
