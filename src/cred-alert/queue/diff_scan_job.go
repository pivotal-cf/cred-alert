package queue

import (
	"cred-alert/db"
	"cred-alert/githubclient"
	"cred-alert/metrics"
	"cred-alert/notifications"
	"cred-alert/scanners"
	"cred-alert/scanners/diffscanner"
	"cred-alert/sniff"

	"code.cloudfoundry.org/lager"
)

type DiffScanJob struct {
	DiffScanPlan

	diffScanRepository   db.DiffScanRepository
	credentialRepository db.CredentialRepository
	githubClient         githubclient.Client
	sniffer              sniff.Sniffer
	credentialCounter    metrics.Counter
	notifier             notifications.Notifier
	id                   string
}

func NewDiffScanJob(
	githubClient githubclient.Client,
	sniffer sniff.Sniffer,
	emitter metrics.Emitter,
	notifier notifications.Notifier,
	diffScanRepository db.DiffScanRepository,
	credentialRepository db.CredentialRepository,
	plan DiffScanPlan,
	id string,
) *DiffScanJob {
	credentialCounter := emitter.Counter("cred_alert.violations")

	job := &DiffScanJob{
		DiffScanPlan:         plan,
		diffScanRepository:   diffScanRepository,
		credentialRepository: credentialRepository,
		githubClient:         githubClient,
		sniffer:              sniffer,

		credentialCounter: credentialCounter,
		notifier:          notifier,
		id:                id,
	}

	return job
}

func (j *DiffScanJob) Run(logger lager.Logger) error {
	logger = logger.Session("diff-scan", lager.Data{
		"owner":      j.Owner,
		"repository": j.Repository,
		"from":       j.From,
		"to":         j.To,
		"task-id":    j.id,
		"private":    j.Private,
	})
	logger.Debug("starting")

	diff, err := j.githubClient.CompareRefs(logger, j.Owner, j.Repository, j.From, j.To)
	if err != nil {
		logger.Error("failed", err)
		return err
	}

	scanner := diffscanner.NewDiffScanner(diff)
	credentialsFound := false
	handleViolation := j.createHandleViolation(j.To, j.Owner+"/"+j.Repository, &credentialsFound)

	err = j.sniffer.Sniff(logger, scanner, handleViolation)
	if err != nil {
		logger.Error("failed", err)
		return err
	}

	err = j.diffScanRepository.SaveDiffScan(logger, &db.DiffScan{
		Owner:           j.Owner,
		Repository:      j.Repository,
		FromCommit:      j.From,
		ToCommit:        j.To,
		CredentialFound: credentialsFound,
	})
	if err != nil {
		logger.Error("failed", err)
		return err
	}

	logger.Debug("done")
	return nil
}

func (j *DiffScanJob) createHandleViolation(sha string, repoName string, credentialsFound *bool) func(lager.Logger, scanners.Line) error {
	return func(logger lager.Logger, line scanners.Line) error {
		logger = logger.Session("handle-violation", lager.Data{
			"path":        line.Path,
			"line-number": line.LineNumber,
			"sha":         sha,
		})
		logger.Debug("starting")

		credential := &db.Credential{
			Owner:          j.Owner,
			Repository:     j.Repository,
			SHA:            sha,
			Path:           line.Path,
			LineNumber:     line.LineNumber,
			ScanningMethod: "diff-scan",
			RulesVersion:   sniff.RulesVersion,
		}

		err := j.credentialRepository.RegisterCredential(logger, credential)
		if err != nil {
			logger.Error("failed", err)
			return err
		}

		notification := notifications.Notification{
			Owner:      j.Owner,
			Repository: j.Repository,
			Private:    j.Private,
			SHA:        sha,
			Path:       line.Path,
			LineNumber: line.LineNumber,
		}

		err = j.notifier.SendNotification(logger, notification)
		if err != nil {
			logger.Error("failed", err)
			return err
		}

		*credentialsFound = true
		tag := "public"
		if j.Private {
			tag = "private"
		}
		j.credentialCounter.Inc(logger, tag)

		logger.Debug("done")
		return nil
	}
}
