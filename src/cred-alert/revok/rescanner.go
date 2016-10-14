package revok

import (
	"cred-alert/db"
	"cred-alert/kolsch"
	"cred-alert/metrics"
	"cred-alert/scanners"
	"cred-alert/scanners/dirscanner"
	"cred-alert/sniff"
	"os"

	"code.cloudfoundry.org/lager"
	"github.com/tedsuo/ifrit"
)

type Rescanner struct {
	logger               lager.Logger
	sniffer              sniff.Sniffer
	repositoryRepository db.RepositoryRepository
	scanRepository       db.ScanRepository
	successCounter       metrics.Counter
	failedCounter        metrics.Counter
}

func NewRescanner(
	logger lager.Logger,
	sniffer sniff.Sniffer,
	repositoryRepository db.RepositoryRepository,
	scanRepository db.ScanRepository,
	emitter metrics.Emitter,
) ifrit.Runner {
	return &Rescanner{
		logger:               logger,
		sniffer:              sniffer,
		repositoryRepository: repositoryRepository,
		scanRepository:       scanRepository,
		successCounter:       emitter.Counter(successMetric),
		failedCounter:        emitter.Counter(failedMetric),
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
	repos, err := r.repositoryRepository.NotScannedWithVersion(sniff.RulesVersion)
	if err != nil {
		logger.Error("failed-getting-repositories", err)
		return err
	}

	for _, repo := range repos {
		repository := repo
		scan := r.scanRepository.Start(logger, "dir-scan", "", "", &repository, nil)
		scanner := dirscanner.New(
			func(logger lager.Logger, violation scanners.Violation) error {
				line := violation.Line
				scan.RecordCredential(db.NewCredential(
					repository.Owner,
					repository.Name,
					"",
					line.Path,
					line.LineNumber,
					violation.Start,
					violation.End,
				))
				return nil
			},
			r.sniffer,
		)
		_ = scanner.Scan(kolsch.NewLogger(), repository.Path)
		finishScan(logger, scan, r.successCounter, r.failedCounter)
	}

	return nil
}
