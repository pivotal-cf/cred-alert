package revok

import (
	"cred-alert/db"
	"cred-alert/gitclient"
	"cred-alert/kolsch"
	"cred-alert/metrics"
	"cred-alert/scanners"
	"cred-alert/scanners/diffscanner"
	"cred-alert/scanners/dirscanner"
	"cred-alert/sniff"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
)

const successMetric = "revok.success_jobs"
const failedMetric = "revok.failed_jobs"

type worker struct {
	logger               lager.Logger
	clock                clock.Clock
	workdir              string
	ghClient             GitHubClient
	gitClient            gitclient.Client
	sniffer              sniff.Sniffer
	interval             time.Duration
	scanRepository       db.ScanRepository
	repositoryRepository db.RepositoryRepository
	fetchRepository      db.FetchRepository
	successCounter       metrics.Counter
	failedCounter        metrics.Counter
}

func New(
	logger lager.Logger,
	clock clock.Clock,
	workdir string,
	ghClient GitHubClient,
	gitClient gitclient.Client,
	sniffer sniff.Sniffer,
	interval time.Duration,
	scanRepository db.ScanRepository,
	repositoryRepository db.RepositoryRepository,
	fetchRepository db.FetchRepository,
	emitter metrics.Emitter,
) *worker {
	return &worker{
		logger:               logger,
		clock:                clock,
		ghClient:             ghClient,
		gitClient:            gitClient,
		sniffer:              sniffer,
		interval:             interval,
		scanRepository:       scanRepository,
		repositoryRepository: repositoryRepository,
		fetchRepository:      fetchRepository,
		workdir:              workdir,
		successCounter:       emitter.Counter(successMetric),
		failedCounter:        emitter.Counter(failedMetric),
	}
}

func (w *worker) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	logger := w.logger.Session("revok")
	logger.Info("started")

	close(ready)

	timer := w.clock.NewTicker(w.interval)

	defer func() {
		logger.Info("done")
		timer.Stop()
	}()

	w.work(logger)

	for {
		select {
		case <-timer.C():
			w.work(logger)
		case <-signals:
			return nil
		}
	}

	return nil
}

func (w *worker) work(logger lager.Logger) {
	logger = logger.Session("doing-work")
	defer logger.Info("done")

	repos, err := w.ghClient.ListRepositories(logger)
	if err != nil {
		logger.Error("failed-listing-repositories", err)
		return
	}

	quietLogger := kolsch.NewLogger()

	for _, repo := range repos {
		dest := filepath.Join(w.workdir, repo.Owner, repo.Name)

		repoLogger := logger.WithData(lager.Data{
			"owner":       repo.Owner,
			"repository":  repo.Name,
			"destination": dest,
		})

		repository := &db.Repository{
			Owner:         repo.Owner,
			Name:          repo.Name,
			SSHURL:        repo.SSHURL,
			Private:       repo.Private,
			DefaultBranch: repo.DefaultBranch,
			RawJSON:       repo.RawJSON,
			Path:          dest,
		}

		err = w.repositoryRepository.FindOrCreate(repository)
		if err != nil {
			repoLogger.Error("failed-to-find-or-create-repository", err)
			continue
		}

		_, err = os.Lstat(dest)
		if os.IsNotExist(err) {
			err = w.gitClient.Clone(repo.SSHURL, dest)
			if err != nil {
				repoLogger.Error("failed-to-clone", err)
				err = os.RemoveAll(dest)
				if err != nil {
					repoLogger.Error("failed-to-clean-up", err)
				}
				continue
			}

			scan := w.scanRepository.Start(repoLogger, "dir-scan", repository, nil)
			_ = dirscanner.New(handler(scan, repo), w.sniffer).Scan(quietLogger, dest)
			finishScan(repoLogger, scan, w.successCounter, w.failedCounter)
		} else {
			changes, err := w.gitClient.Fetch(dest)
			if err != nil {
				repoLogger.Error("failed-to-fetch", err)
				continue
			}

			bs, err := json.Marshal(changes)
			if err != nil {
				repoLogger.Error("failed-to-marshal-json", err)
			}

			fetch := &db.Fetch{
				Repository: *repository,
				Path:       dest,
				Changes:    bs,
			}

			err = w.fetchRepository.SaveFetch(repoLogger, fetch)
			if err != nil {
				repoLogger.Error("failed-to-save-fetch", err)
				continue
			}

			for _, oids := range changes {
				diff, err := w.gitClient.Diff(dest, oids[0], oids[1])
				if err != nil {
					repoLogger.Error("failed-to-get-diff", err, lager.Data{
						"from": oids[0].String(),
						"to":   oids[1].String(),
					})
					continue
				}

				scan := w.scanRepository.Start(quietLogger, "diff-scan", repository, fetch)
				scanner := diffscanner.NewDiffScanner(strings.NewReader(diff))
				w.sniffer.Sniff(repoLogger, scanner, handler(scan, repo))
				finishScan(repoLogger, scan, w.successCounter, w.failedCounter)
			}
		}
	}
}

func handler(scan db.ActiveScan, repo GitHubRepository) sniff.ViolationHandlerFunc {
	return func(logger lager.Logger, line scanners.Line) error {
		scan.RecordCredential(db.Credential{
			Owner:      repo.Owner,
			Repository: repo.Name,
			Path:       line.Path,
			LineNumber: line.LineNumber,
		})
		return nil
	}
}

func finishScan(logger lager.Logger, scan db.ActiveScan, success, failed metrics.Counter) {
	err := scan.Finish()
	if err != nil {
		failed.Inc(logger)
	} else {
		success.Inc(logger)
	}
}
