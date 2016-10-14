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

type DirscanUpdater struct {
	logger               lager.Logger
	sniffer              sniff.Sniffer
	repositoryRepository db.RepositoryRepository
	scanRepository       db.ScanRepository
	successCounter       metrics.Counter
	failedCounter        metrics.Counter
}

func NewDirscanUpdater(
	logger lager.Logger,
	sniffer sniff.Sniffer,
	repositoryRepository db.RepositoryRepository,
	scanRepository db.ScanRepository,
	emitter metrics.Emitter,
) ifrit.Runner {
	return &DirscanUpdater{
		logger:               logger,
		sniffer:              sniffer,
		repositoryRepository: repositoryRepository,
		scanRepository:       scanRepository,
		successCounter:       emitter.Counter(successMetric),
		failedCounter:        emitter.Counter(failedMetric),
	}
}

func (d *DirscanUpdater) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	logger := d.logger.Session("dirscan-updater")
	logger.Info("started")

	close(ready)

	defer logger.Info("done")

	_ = d.work(logger)

	<-signals

	return nil
}

func (d *DirscanUpdater) work(logger lager.Logger) error {
	repos, err := d.repositoryRepository.NotScannedWithVersion(sniff.RulesVersion)
	if err != nil {
		logger.Error("failed-getting-repositories", err)
		return err
	}

	for _, r := range repos {
		repo := r
		scan := d.scanRepository.Start(logger, "dir-scan", "", "", &repo, nil)
		scanner := dirscanner.New(
			func(logger lager.Logger, violation scanners.Violation) error {
				line := violation.Line
				scan.RecordCredential(db.NewCredential(
					repo.Owner,
					repo.Name,
					"",
					line.Path,
					line.LineNumber,
					violation.Start,
					violation.End,
				))
				return nil
			},
			d.sniffer,
		)
		_ = scanner.Scan(kolsch.NewLogger(), repo.Path)
		finishScan(logger, scan, d.successCounter, d.failedCounter)
	}

	return nil
}
