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
}

func NewRefScanJob(
	plan RefScanPlan,
	client github.Client,
	sniffer sniff.Sniffer,
	notifier notifications.Notifier,
	emitter metrics.Emitter,
	mimetype mimetype.Mimetype,
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
	}

	return job
}

func (j *RefScanJob) Run(logger lager.Logger) error {
	logger = logger.Session("ref-scan", lager.Data{
		"owner":      j.Owner,
		"repository": j.Repository,
		"ref":        j.Ref,
	})

	if j.Ref == initialCommitParentHash {
		logger.Info("skipped-initial-nil-ref")
		return nil
	}

	downloadURL, err := j.client.ArchiveLink(j.Owner, j.Repository)
	if err != nil {
		logger.Error("Error getting download url", err)
		return err
	}

	archiveFile, err := downloadArchive(logger, downloadURL)
	if err != nil {
		logger.Error("Error downloading archive", err)
		return err
	}
	defer os.Remove(archiveFile.Name())
	defer archiveFile.Close()

	archiveReader, err := zip.OpenReader(archiveFile.Name())
	if err != nil {
		logger.Error("Error unzipping archive", err)
		return err
	}
	defer archiveReader.Close()

	for _, f := range archiveReader.File {
		isText, err := j.isText(f)
		if err != nil {
			logger.Error("mimetype-error", err)
			continue
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

	return nil
}

func downloadArchive(logger lager.Logger, link *url.URL) (*os.File, error) {
	tempFile, err := ioutil.TempFile("", "downloaded-git-archive")
	if err != nil {
		logger.Error("Error creating archive temp file", err)
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

		j.credentialCounter.Inc(logger)

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
	buf.ReadFrom(unzippedReader)
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
