package rolodex

import (
	"errors"
	"os"

	"code.cloudfoundry.org/lager"

	"cred-alert/gitclient"
	"cred-alert/metrics"
)

type syncer struct {
	repoUrl   string
	repoPath  string
	gitClient gitclient.Client
	teamRepo  TeamRepository

	logger         lager.Logger
	successCounter metrics.Counter
	failureCounter metrics.Counter
}

type Syncer interface {
	Sync()
}

func NewSyncer(logger lager.Logger, emitter metrics.Emitter, repoUrl, repoPath string, gitClient gitclient.Client, teamRepo TeamRepository) Syncer {
	syncLogger := logger.Session("syncer", lager.Data{
		"upstream": repoUrl,
		"local":    repoPath,
	})

	return &syncer{
		repoUrl:   repoUrl,
		repoPath:  repoPath,
		gitClient: gitClient,
		teamRepo:  teamRepo,

		logger:         syncLogger,
		successCounter: emitter.Counter("rolodex.syncer.fetch.success"),
		failureCounter: emitter.Counter("rolodex.syncer.fetch.failure"),
	}
}

const remoteMaster = "refs/remotes/origin/master"

func (s *syncer) Sync() {
	if _, err := os.Stat(s.repoPath); os.IsNotExist(err) {
		_, err := s.gitClient.Clone(s.repoUrl, s.repoPath)
		if err != nil {
			s.logger.Error("cloning", err)
			return
		}

		s.teamRepo.Reload()

		return
	}

	heads, err := s.gitClient.Fetch(s.repoPath)
	if err != nil {
		s.failureCounter.Inc(s.logger)
		s.logger.Error("fetching", err)
		return
	}

	if len(heads) == 0 {
		return
	}

	upstream, found := heads[remoteMaster]
	if !found {
		s.failureCounter.Inc(s.logger)
		s.logger.Error("failed-to-find-updated-master", errors.New("no remote master branch found"))
		return
	}

	err = s.gitClient.HardReset(s.repoPath, upstream[1])
	if err != nil {
		s.failureCounter.Inc(s.logger)
		s.logger.Error("reseting", err)
		return
	}

	s.successCounter.Inc(s.logger)

	s.teamRepo.Reload()
}
