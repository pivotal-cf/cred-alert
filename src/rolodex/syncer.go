package rolodex

import (
	"errors"
	"os"

	"code.cloudfoundry.org/lager"

	"cred-alert/gitclient"
	"cred-alert/metrics"
)

type syncer struct {
	repoURL   string
	repoPath  string
	gitClient gitclient.Client
	teamRepo  TeamRepository

	logger         lager.Logger
	successCounter metrics.Counter
	failureCounter metrics.Counter

	fetchTimer metrics.Timer
}

type Syncer interface {
	Sync()
}

func NewSyncer(logger lager.Logger, emitter metrics.Emitter, repoURL, repoPath string, gitClient gitclient.Client, teamRepo TeamRepository) Syncer {
	syncLogger := logger.Session("syncer", lager.Data{
		"upstream": repoURL,
		"local":    repoPath,
	})

	return &syncer{
		repoURL:   repoURL,
		repoPath:  repoPath,
		gitClient: gitClient,
		teamRepo:  teamRepo,

		logger:         syncLogger,
		successCounter: emitter.Counter("rolodex.syncer.fetch.success"),
		failureCounter: emitter.Counter("rolodex.syncer.fetch.failure"),

		fetchTimer: emitter.Timer("rolodex.syncer.fetch.time"),
	}
}

const remoteMaster = "refs/remotes/origin/master"

var errMissingMaster = errors.New("no remote master branch found")

func (s *syncer) Sync() {
	if _, err := os.Stat(s.repoPath); os.IsNotExist(err) {
		err := s.gitClient.Clone(s.repoURL, s.repoPath)
		if err != nil {
			s.logger.Error("cloning", err)
			return
		}

		s.teamRepo.Reload()

		return
	}

	var fetchErr error

	s.fetchTimer.Time(s.logger, func() {
		heads, err := s.gitClient.Fetch(s.repoPath)
		if err != nil {
			s.logger.Error("fetching", err)
			fetchErr = err
			return
		}

		if len(heads) == 0 {
			return
		}

		upstream, found := heads[remoteMaster]
		if !found {
			s.logger.Error("failed-to-find-updated-master", errMissingMaster)
			fetchErr = errMissingMaster
			return
		}

		if err = s.gitClient.HardReset(s.repoPath, upstream[1]); err != nil {
			s.logger.Error("reseting", err)
			fetchErr = err
			return
		}
	})

	if fetchErr != nil {
		s.failureCounter.Inc(s.logger)
		return
	}

	s.successCounter.Inc(s.logger)
	s.teamRepo.Reload()
}
