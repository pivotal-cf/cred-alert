package revok

import (
	"os"

	"cred-alert/db"
	"cred-alert/metrics"
	"cred-alert/notifications"
	"cred-alert/sniff"

	"code.cloudfoundry.org/lager"
	"github.com/tedsuo/ifrit"
)

type Rescanner struct {
	logger         lager.Logger
	scanRepo       db.ScanRepository
	credRepo       db.CredentialRepository
	scanner        Scanner
	notifier       notifications.Notifier
	successCounter metrics.Counter
	failedCounter  metrics.Counter
}

func NewRescanner(
	logger lager.Logger,
	scanRepo db.ScanRepository,
	credRepo db.CredentialRepository,
	scanner Scanner,
	notifier notifications.Notifier,
	emitter metrics.Emitter,
) ifrit.Runner {
	return &Rescanner{
		logger:         logger,
		scanRepo:       scanRepo,
		credRepo:       credRepo,
		scanner:        scanner,
		notifier:       notifier,
		successCounter: emitter.Counter("revok.rescanner.success"),
		failedCounter:  emitter.Counter("revok.rescanner.failed"),
	}
}

func (r *Rescanner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	logger := r.logger.Session("rescanner")
	logger.Info("started")

	close(ready)

	defer logger.Info("done")

	_ = r.work(logger)

	<-signals

	return nil
}

func (r *Rescanner) work(logger lager.Logger) error {
	priorScans, err := r.scanRepo.ScansNotYetRunWithVersion(logger, sniff.RulesVersion)
	if err != nil {
		logger.Error("failed-getting-prior-scans", err)
		return err
	}

	if len(priorScans) == 0 {
		logger.Info("no-prior-scans-for-rules-version", lager.Data{
			"rules_version": sniff.RulesVersion - 1,
		})
		return nil
	}

	for _, priorScan := range priorScans {
		oldCredentials, err := r.credRepo.ForScanWithID(priorScan.ID)
		if err != nil {
			r.failedCounter.Inc(logger)
			logger.Error("failed-getting-prior-credentials", err)
			continue
		}

		credMap := map[string]db.Credential{}
		for _, cred := range oldCredentials {
			credMap[cred.Hash()] = cred
		}

		newCredentials, err := r.scanner.ScanNoNotify(logger, priorScan.Owner, priorScan.Repository, priorScan.StartSHA, priorScan.StopSHA)
		if err != nil {
			logger.Error("failed-to-scan", err)
			r.failedCounter.Inc(logger)
		} else {
			r.successCounter.Inc(logger)
		}

		var batch []notifications.Notification
		for _, cred := range newCredentials {
			if _, ok := credMap[cred.Hash()]; !ok {
				batch = append(batch, notifications.Notification{
					Owner:      cred.Owner,
					Repository: cred.Repository,
					SHA:        cred.SHA,
					Path:       cred.Path,
					LineNumber: cred.LineNumber,
					Private:    cred.Private,
				})
			}
		}

		if len(batch) > 0 {
			err = r.notifier.SendBatchNotification(logger, batch)
			if err != nil {
				logger.Error("failed-to-notify", err)
			}
		}
	}

	return nil
}
