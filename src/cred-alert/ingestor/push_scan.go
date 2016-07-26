package ingestor

import "github.com/google/go-github/github"

type PushScan struct {
	Owner      string
	Repository string
	From       string
	To         string
	Private    bool
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
		Private:    *event.Repo.Private,
	}, true
}
