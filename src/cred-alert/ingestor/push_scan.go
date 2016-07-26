package ingestor

import (
	"fmt"

	"github.com/google/go-github/github"
)

type PushScan struct {
	Owner      string
	Repository string
	From       string
	To         string
}

func (p PushScan) FullRepoName() string {
	return fmt.Sprintf("%s/%s", p.Owner, p.Repository)
}

func Extract(event github.PushEvent) (PushScan, bool) {
	if event.Repo == nil || event.After == nil || event.Before == nil {
		return PushScan{}, false
	}

	return PushScan{
		Owner:      *event.Repo.Owner.Name,
		Repository: *event.Repo.Name,
		From:       *event.Before,
		To:         *event.After,
	}, true
}
