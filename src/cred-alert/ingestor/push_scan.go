package ingestor

import (
	"fmt"

	"github.com/google/go-github/github"
)

type PushScan struct {
	Owner      string
	Repository string

	Diffs []PushScanDiff
}

func (p PushScan) FullRepoName() string {
	return fmt.Sprintf("%s/%s", p.Owner, p.Repository)
}

func (p PushScan) FirstCommit() string {
	return p.Diffs[0].Start
}

func (p PushScan) LastCommit() string {
	return p.Diffs[len(p.Diffs)-1].End
}

type PushScanDiff struct {
	Start string
	End   string
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
		{Start: *event.Before, End: *event.Commits[0].ID},
	}

	for i, _ := range event.Commits {
		if i == len(event.Commits)-1 {
			break
		}

		start := *event.Commits[i].ID
		end := *event.Commits[i+1].ID

		diffs = append(diffs, PushScanDiff{
			Start: start,
			End:   end,
		})
	}

	return PushScan{
		Owner:      *event.Repo.Owner.Name,
		Repository: *event.Repo.Name,
		Diffs:      diffs,
	}, true
}
