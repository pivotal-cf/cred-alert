package queue

import (
	"archive/zip"
	"cred-alert/github"
	"cred-alert/metrics"
	"cred-alert/notifications"
	"cred-alert/scanners"
	"cred-alert/scanners/file"
	"cred-alert/sniff"
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/pivotal-golang/lager"
)

type RefScanJob struct {
	RefScanPlan
	client            github.Client
	sniff             sniff.SniffFunc
	notifier          notifications.Notifier
	emitter           metrics.Emitter
	credentialCounter metrics.Counter
}

func NewRefScanJob(plan RefScanPlan, client github.Client, sniff sniff.SniffFunc, notifier notifications.Notifier, emitter metrics.Emitter) *RefScanJob {
	credentialCounter := emitter.Counter("cred_alert.violations")

	job := &RefScanJob{
		RefScanPlan:       plan,
		client:            client,
		sniff:             sniff,
		notifier:          notifier,
		emitter:           emitter,
		credentialCounter: credentialCounter,
	}

	return job
}

func (j *RefScanJob) Run(logger lager.Logger) error {
	logger.Session("running-ref-scan-job")

	downloadURL, err := j.client.ArchiveLink(logger, j.Owner, j.Repository)
	if err != nil {
		logger.Error("Error getting download url", err)
	}

	filepath := "ref-archive.zip"
	archiveFile, err := downloadArchive(logger, downloadURL, filepath)
	if err != nil {
		logger.Error("Error downloading archive", err)
		return err
	}

	archiveReader, err := zip.OpenReader(archiveFile.Name())
	defer archiveReader.Close()
	if err != nil {
		logger.Error("Error unzipping archive", err)
		return err
	}

	for _, f := range archiveReader.File {
		unzippedReader, err := f.Open()
		if err != nil {
			logger.Error("Error reading archive", err)
		}
		bufioScanner := file.NewReaderScanner(unzippedReader, f.Name)
		handleViolation := j.createHandleViolation(logger, j.Ref, j.Owner+"/"+j.Repository)

		j.sniff(logger, bufioScanner, handleViolation)
	}

	os.Remove(filepath)

	return nil
}

func downloadArchive(logger lager.Logger, link *url.URL, filepath string) (*os.File, error) {
	out, err := os.Create(filepath)
	if err != nil {
		return nil, err
	}

	defer out.Close()
	resp, err := http.Get(link.String())
	defer resp.Body.Close()
	if err != nil {
		return nil, err
	}

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return nil, err
	}

	return out, nil
}

func unzip(archive *os.File) (io.Reader, error) {
	r, err := zip.OpenReader(archive.Name())
	if err != nil {
		return nil, err
	}
	defer r.Close()
	readCloser, err := r.File[0].Open()

	return readCloser, err
}

func (j *RefScanJob) createHandleViolation(logger lager.Logger, sha string, repoName string) func(scanners.Line) {
	return func(line scanners.Line) {
		logger.Info("found-credential", lager.Data{
			"path":        line.Path,
			"line-number": line.LineNumber,
			"sha":         sha,
		})

		j.notifier.SendNotification(logger, repoName, sha, line)

		j.credentialCounter.Inc(logger)
	}
}
