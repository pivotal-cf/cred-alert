package ingestor

import (
	"fmt"
	"time"

	"github.com/google/go-github/github"
)

type PushScan struct {
	Owner      string
	Repository string
	Ref        string
	Diffs      []PushScanDiff
}

func (p PushScan) FullRepoName() string {
	return fmt.Sprintf("%s/%s", p.Owner, p.Repository)
}

func (p PushScan) FirstCommit() string {
	return p.Diffs[0].From
}

func (p PushScan) LastCommit() string {
	return p.Diffs[len(p.Diffs)-1].To
}

type PushScanDiff struct {
	From          string
	FromTimestamp time.Time
	To            string
	ToTimestamp   time.Time
}

const initalCommitParentHash = "0000000000000000000000000000000000000000"

func Extract(event github.PushEvent) (PushScan, bool) {
	if event.Before == nil || *event.Before == initalCommitParentHash {
		return PushScan{}, false
	}

	if len(event.Commits) == 0 {
		return PushScan{}, false
	}

	diffs := []PushScanDiff{
		{
			From:          *event.Before,
			To:            *event.Commits[0].ID,
			ToTimestamp:   (*event.Commits[0].Timestamp).Time,
			FromTimestamp: time.Unix(0, 0),
		},
	}

	for i, _ := range event.Commits {
		if i == len(event.Commits)-1 {
			break
		}

		from := *event.Commits[i].ID
		fromTimestamp := (*event.Commits[i].Timestamp).Time
		to := *event.Commits[i+1].ID
		toTimestamp := (*event.Commits[i+1].Timestamp).Time

		diffs = append(diffs, PushScanDiff{
			From:          from,
			FromTimestamp: fromTimestamp,
			To:            to,
			ToTimestamp:   toTimestamp,
		})
	}

	return PushScan{
		Owner:      *event.Repo.Owner.Name,
		Repository: *event.Repo.Name,
		Ref:        *event.Ref,
		Diffs:      diffs,
	}, true
}
