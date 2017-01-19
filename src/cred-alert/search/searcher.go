package search

import (
	"bufio"
	"bytes"
	"context"

	"code.cloudfoundry.org/lager"

	"cred-alert/db"
	"cred-alert/gitclient"
	"cred-alert/sniff/matchers"
)

type Result struct {
	Owner      string
	Repository string

	Revision   string
	Path       string
	LineNumber int
	Location   int
	Length     int

	Content []byte
}

//go:generate counterfeiter . Searcher

type Searcher interface {
	SearchCurrent(ctx context.Context, logger lager.Logger, matcher matchers.Matcher) Results
}

type searcher struct {
	repoRepository db.RepositoryRepository
	looper         gitclient.Looper
}

func NewSearcher(repoRepository db.RepositoryRepository, looper gitclient.Looper) Searcher {
	return &searcher{
		repoRepository: repoRepository,
		looper:         looper,
	}
}

func (s *searcher) SearchCurrent(ctx context.Context, logger lager.Logger, matcher matchers.Matcher) Results {
	logger = logger.Session("search-current")

	searchResults := &searchResults{
		resultChan: make(chan Result, 1024),
		err:        nil,
	}

	activeRepos, err := s.repoRepository.Active()
	if err != nil {
		searchResults.fail(err)
		return searchResults
	}

	go func() {
		for _, repo := range activeRepos {
			logger = logger.WithData(lager.Data{
				"owner": repo.Owner,
				"repo":  repo.Name,
			})

			select {
			case <-ctx.Done():
				searchResults.fail(ctx.Err())
				return
			default:
				s.looper.ScanCurrentState(repo.Path, func(sha string, path string, content []byte) {
					scanner := bufio.NewScanner(bytes.NewReader(content))

					lineNumber := 1

					for scanner.Scan() {
						line := make([]byte, len(scanner.Bytes()))
						copy(line, scanner.Bytes())

						if match, start, end := matcher.Match(line); match {
							searchResults.resultChan <- Result{
								Owner:      repo.Owner,
								Repository: repo.Name,
								Revision:   sha,
								Path:       path,
								LineNumber: lineNumber,
								Location:   start,
								Length:     end - start,
								Content:    line,
							}
						}

						lineNumber++
					}

					if err := scanner.Err(); err != nil {
						if err == bufio.ErrTooLong {
							return
						}

						logger.Error("failed-to-search-file", err, lager.Data{
							"revision":    sha,
							"path":        path,
							"line-number": lineNumber,
						})
					}
				})
			}
		}

		searchResults.succeed()
	}()

	return searchResults
}

//go:generate counterfeiter . Results

type Results interface {
	C() <-chan Result
	Err() error
}

type searchResults struct {
	err        error
	resultChan chan Result
}

func (r *searchResults) succeed() {
	close(r.resultChan)
}

func (r *searchResults) fail(err error) {
	r.err = err
	close(r.resultChan)
}

func (r *searchResults) C() <-chan Result {
	return r.resultChan
}

func (r *searchResults) Err() error {
	return r.err
}
