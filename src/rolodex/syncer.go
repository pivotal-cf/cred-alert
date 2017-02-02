package rolodex

import (
	"errors"
	"os"

	"code.cloudfoundry.org/lager"

	"cred-alert/gitclient"
)

type syncer struct {
	logger    lager.Logger
	repoUrl   string
	repoPath  string
	gitClient gitclient.Client
	teamRepo  TeamRepository
}

type Syncer interface {
	Sync()
}

func NewSyncer(logger lager.Logger, repoUrl, repoPath string, gitClient gitclient.Client, teamRepo TeamRepository) Syncer {
	syncLogger := logger.Session("syncer", lager.Data{
		"upstream": repoUrl,
		"local":    repoPath,
	})

	return &syncer{
		logger:    syncLogger,
		repoUrl:   repoUrl,
		repoPath:  repoPath,
		gitClient: gitClient,
		teamRepo:  teamRepo,
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
		s.logger.Error("fetching", err)
		return
	}

	if len(heads) == 0 {
		return
	}

	upstream, found := heads[remoteMaster]
	if !found {
		s.logger.Error("failed-to-find-updated-master", errors.New("no remote master branch found"))
		return
	}

	err = s.gitClient.HardReset(s.repoPath, upstream[1])
	if err != nil {
		s.logger.Error("reseting", err)
		return
	}

	s.teamRepo.Reload()
}
