package rolodex

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

//go:generate counterfeiter . TeamRepository

type TeamRepository interface {
	GetOwners(Repository) ([]Team, error)
}

type teamRepository struct {
	teams []TeamRecord
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

func NewTeamRepository(teamsPath string) *teamRepository {
	teams, _ := loadTeamRecords(teamsPath)

	return &teamRepository{
		teams: teams,
	}
}

func (t *teamRepository) GetOwners(repo Repository) ([]Team, error) {
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

func loadTeamRecords(path string) ([]TeamRecord, error) {
	teamRecords := []TeamRecord{}

	files, err := ioutil.ReadDir(path)
	if err != nil {
		return teamRecords, err
	}

	for _, file := range files {
		filePath := filepath.Join(path, file.Name())
		fileBytes, err := ioutil.ReadFile(filePath)
		if err != nil {
			return teamRecords, err
		}

		teamRecord := TeamRecord{}
		yaml.Unmarshal(fileBytes, &teamRecord)

		teamRecords = append(teamRecords, teamRecord)
	}

	return teamRecords, nil
}
