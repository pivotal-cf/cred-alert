package revok

import (
	"fmt"
	"os"
	"time"

	"code.cloudfoundry.org/lager"

	"context"
	"cred-alert/db"
	"cred-alert/metrics"
	"cred-alert/notifications"
	"cred-alert/sniff"
)

//go:generate counterfeiter . RescannerScanner

type RescannerScanner interface {
	Scan(lager.Logger, string, string, map[string]struct{}, string, string, string) ([]db.Credential, error)
}

type Rescanner struct {
	logger         lager.Logger
	scanRepo       db.ScanRepository
	credRepo       db.CredentialRepository
	scanner        RescannerScanner
	router         notifications.Router
	successCounter metrics.Counter
	failedCounter  metrics.Counter
	maxAge         time.Duration
}

func NewRescanner(
	logger lager.Logger,
	scanRepo db.ScanRepository,
	credRepo db.CredentialRepository,
	scanner RescannerScanner,
	router notifications.Router,
	emitter metrics.Emitter,
	maxAge time.Duration,
) *Rescanner {
	return &Rescanner{
		logger:         logger,
		scanRepo:       scanRepo,
		credRepo:       credRepo,
		scanner:        scanner,
		router:         router,
		successCounter: emitter.Counter("revok.rescanner.success"),
		failedCounter:  emitter.Counter("revok.rescanner.failed"),
		maxAge:         maxAge,
	}
}

func (r *Rescanner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	logger := r.logger.Session("rescanner")
	logger.Info("started")

	defer logger.Info("done")

	close(ready)

	priorScans, err := r.scanRepo.ScansNotYetRunWithVersion(logger, sniff.RulesVersion)
	if err != nil {
		logger.Error("failed-getting-prior-scans", err)
	}

	for _, priorScan := range priorScans {
		select {
		case <-signals:
			return nil
		default:
			err := r.work(logger, priorScan)
			if err != nil {
				r.failedCounter.Inc(logger)
				logger.Error("failed-to-rescan", err, lager.Data{
					"scan-id": priorScan.ID,
				})
			}
		}
	}

	logger.Info("all-scans-up-to-date")
	<-signals
	return nil
}

func (r *Rescanner) work(logger lager.Logger, priorScan db.PriorScan) error {
	logger.Info("rescanning", lager.Data{
		"owner":   priorScan.Owner,
		"repo":    priorScan.Repository,
		"scan-id": priorScan.ID,
	})

	oldCredentials, err := r.credRepo.ForScanWithID(priorScan.ID)
	if err != nil {
		logger.Error("failed-getting-prior-credentials", err)
		return err
	}

	var latestCred time.Time

	credMap := make(map[string]db.Credential, len(oldCredentials))
	for _, cred := range oldCredentials {
		if cred.CreatedAt.After(latestCred) {
			latestCred = cred.CreatedAt
		}
		credMap[cred.Hash()] = cred
	}

	newCredentials, err := r.scanner.Scan(
		logger,
		priorScan.Owner,
		priorScan.Repository,
		map[string]struct{}{},
		priorScan.Branch,
		priorScan.StartSHA,
		priorScan.StopSHA,
	)
	if err != nil {
		return err
	}

	r.successCounter.Inc(logger)

	// Maybe: check against creds table using: owner, repo, sha, path, line, match start/end

	// \
	//

	// CEV: De-dupe based on Hash(), I'm not sure how
	// effective/ineffective this is (I'm assuming it
	// must be missing some stuff since since we're
	// working on a story to reduce dupes).
	//
	fmt.Printf("Old Map size is %#v\n", len(credMap))
	fmt.Printf("New list size is %#v\n", len(newCredentials))
	var batch []notifications.Notification
	for _, cred := range newCredentials {
		credReported, err := r.credRepo.CredentialReported(&cred, sniff.RulesVersion)
		if err != nil {
			return err
		}
		if !credReported {
			if _, ok := credMap[cred.Hash()]; !ok {
				fmt.Printf("key is %s\n", cred.Hash())
				batch = append(batch, notifications.Notification{
					Owner:      cred.Owner,
					Repository: cred.Repository,
					SHA:        cred.SHA,
					Path:       cred.Path,
					LineNumber: cred.LineNumber,
					Private:    cred.Private,
				})
			}
		} else {
			fmt.Printf("Not notifying for: %s\n\n", cred.Hash())
		}
	}

	// TODO (CEV): Filter based on cred created_at date, we don't
	// want to remove creds that we haven't alerted for.
	//
	// This could probably be combined with the above filtering loop.
	//
	if r.maxAge > 0 && time.Since(latestCred) >= r.maxAge {
		// do filtering things
	}

	if len(batch) > 0 {
		err = r.router.Deliver(context.TODO(), logger, batch)
		if err != nil {
			logger.Error("failed-to-notify", err)
		}
	}

	return nil
}
