package queue

import (
	"cred-alert/githubclient"
	"cred-alert/inflator"
	"cred-alert/metrics"
	"cred-alert/notifications"
	"cred-alert/scanners"
	"cred-alert/scanners/dirscanner"
	"cred-alert/sniff"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
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
	expander          inflator.Inflator
	scratchSpace      inflator.ScratchSpace
}

func NewRefScanJob(
	plan RefScanPlan,
	client githubclient.Client,
	sniffer sniff.Sniffer,
	notifier notifications.Notifier,
	emitter metrics.Emitter,
	expander inflator.Inflator,
	scratchSpace inflator.ScratchSpace,
) *RefScanJob {
	credentialCounter := emitter.Counter("cred_alert.violations")

	job := &RefScanJob{
		RefScanPlan:       plan,
		client:            client,
		sniffer:           sniffer,
		notifier:          notifier,
		emitter:           emitter,
		credentialCounter: credentialCounter,
		expander:          expander,
		scratchSpace:      scratchSpace,
	}

	return job
}

func (j *RefScanJob) Run(logger lager.Logger) error {
	logger = logger.Session("ref-scan", lager.Data{
		"owner":      j.Owner,
		"repository": j.Repository,
		"ref":        j.Ref,
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

	tempDir, err := ioutil.TempDir("", "download-archive")
	if err != nil {
		logger.Error("failed", err)
		return err
	}
	defer os.RemoveAll(tempDir)

	archiveFile, err := downloadArchive(logger, downloadURL, tempDir)
	if err != nil {
		logger.Error("failed", err)
		return err
	}
	defer archiveFile.Close()

	destination, err := j.scratchSpace.Make()
	if err != nil {
		logger.Error("failed", err)
		return err
	}
	defer os.RemoveAll(destination)

	err = j.expander.Inflate(logger, "application/zip", archiveFile.Name(), destination)
	if err != nil {
		logger.Error("failed", err)
		return err
	}

	handleViolation := j.createHandleViolation(destination)
	scanner := dirscanner.New(handleViolation, j.sniffer)

	err = scanner.Scan(logger, destination)
	if err != nil {
		logger.Error("failed", err)
		return err
	}

	logger.Debug("done")
	return nil
}

func downloadArchive(logger lager.Logger, link *url.URL, dest string) (*os.File, error) {
	logger = logger.Session("downloading-archive")

	if link == nil {
		err := errors.New("Archive link was nil")
		logger.Error("failed", err)
		return nil, err
	}

	logger.Debug("starting")

	f, err := os.Create(filepath.Join(dest, "archive.zip"))
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

	_, err = io.Copy(f, resp.Body)
	if err != nil {
		logger.Error("failed", err)
		return nil, err
	}

	logger.Debug("done")
	return f, nil
}

func (j *RefScanJob) createHandleViolation(stripPath string) func(lager.Logger, scanners.Line) error {
	return func(logger lager.Logger, line scanners.Line) error {
		logger = logger.Session("handle-violation", lager.Data{
			"path":        line.Path,
			"line-number": line.LineNumber,
			"ref":         j.Ref,
		})
		logger.Debug("starting")

		relPath, err := filepath.Rel(stripPath, line.Path)
		if err != nil {
			logger.Error("making-relative-path-failed", err)
			return err
		}

		parts := strings.Split(relPath, string(os.PathSeparator))
		path, err := filepath.Rel(parts[0], relPath)
		if err != nil {
			logger.Error("making-relative-path-failed", err)
			return err
		}

		notification := notifications.Notification{
			Owner:      j.Owner,
			Repository: j.Repository,
			Private:    j.Private,
			SHA:        j.Ref,
			Path:       path,
			LineNumber: line.LineNumber,
		}

		err = j.notifier.SendNotification(logger, notification)
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
