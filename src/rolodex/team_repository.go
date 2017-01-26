package rolodex

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"sync"

	"code.cloudfoundry.org/lager"
	"gopkg.in/yaml.v2"
)

//go:generate counterfeiter . TeamRepository

type TeamRepository interface {
	GetOwners(Repository) ([]Team, error)

	Reload()
}

type teamRepository struct {
	logger lager.Logger

	repoPath  string
	teams     []TeamRecord
	teamsLock *sync.RWMutex
}

type TeamRecord struct {
	Name         string   `yaml:"name"`
	Repositories []string `yaml:"repositories"`

	Contact struct {
		Slack struct {
			Team    string `yaml:"team"`
			Channel string `yaml:"channel"`
		} `yaml:"slack"`
	} `yaml:"contact"`
}

func (t TeamRecord) OwnsRepository(repo Repository) bool {
	repoKey := fmt.Sprintf("%s/%s", repo.Owner, repo.Name)

	for _, repository := range t.Repositories {
		if repository == repoKey {
			return true
		}
	}

	return false
}

func NewTeamRepository(logger lager.Logger, teamsPath string) *teamRepository {
	repo := &teamRepository{
		logger:    logger.Session("team-repository"),
		repoPath:  teamsPath,
		teamsLock: &sync.RWMutex{},
	}

	repo.Reload()

	return repo
}

func (t *teamRepository) GetOwners(repo Repository) ([]Team, error) {
	t.teamsLock.RLock()
	defer t.teamsLock.RUnlock()

	matchingTeams := []Team{}

	for _, team := range t.teams {
		if team.OwnsRepository(repo) {
			matchingTeams = append(matchingTeams, Team{
				Name: team.Name,
				SlackChannel: SlackChannel{
					Team: team.Contact.Slack.Team,
					Name: team.Contact.Slack.Channel,
				},
			})
		}
	}

	return matchingTeams, nil
}

func (t *teamRepository) Reload() {
	t.teamsLock.Lock()
	defer t.teamsLock.Unlock()

	t.loadTeamRecords()
}

func (t *teamRepository) loadTeamRecords() {
	teamRecords := []TeamRecord{}

	files, err := filepath.Glob(filepath.Join(t.repoPath, "*.yml"))
	if err != nil {
		t.logger.Error("failed-to-glob", err)
		return
	}

	for _, filePath := range files {
		fileName := filepath.Base(filePath)
		fileBytes, err := ioutil.ReadFile(filePath)
		if err != nil {
			t.logger.Error("failed-to-read-file", err, lager.Data{
				"name": fileName,
			})
			continue
		}

		teamRecord := TeamRecord{}
		err = yaml.Unmarshal(fileBytes, &teamRecord)
		if err != nil {
			t.logger.Error("failed-to-parse-team", err, lager.Data{
				"name": fileName,
			})
			continue
		}

		teamRecords = append(teamRecords, teamRecord)
	}

	t.teams = teamRecords
}
