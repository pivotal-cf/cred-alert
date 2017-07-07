package search

import (
	"code.cloudfoundry.org/lager"

	"cred-alert/db"
	"cred-alert/gitclient"

	"gopkg.in/yaml.v2"
)

type BlobResult struct {
	Path string
	SHA  string
}

//go:generate counterfeiter . BlobSearcher

type BlobSearcher interface {
	ListBlobs(logger lager.Logger, owner string, name string) ([]BlobResult, error)
}

type blobSearcher struct {
	repoRepository db.RepositoryRepository
	fileLookup     gitclient.FileLookup
}

func NewBlobSearcher(repoRepository db.RepositoryRepository, fileLookup gitclient.FileLookup) BlobSearcher {
	return &blobSearcher{
		repoRepository: repoRepository,
		fileLookup:     fileLookup,
	}
}

func (s *blobSearcher) ListBlobs(logger lager.Logger, owner string, name string) ([]BlobResult, error) {
	repo, err := s.repoRepository.MustFind(owner, name)
	if err != nil {
		return nil, err
	}

	blobsBytes, err := s.fileLookup.FileContents(repo.Path, repo.DefaultBranch, "config/blobs.yml")
	if err != nil {
		return nil, err
	}

	var blobs boshBlobsSchema

	err = yaml.Unmarshal([]byte(blobsBytes), &blobs)
	if err != nil {
		return nil, err
	}

	blobResults := []BlobResult{}

	for blobPath, blob := range blobs {
		blobResults = append(blobResults, BlobResult{
			Path: blobPath,
			SHA:  blob.SHA,
		})
	}

	return blobResults, nil
}

type boshBlobsSchema map[string]boshBlobsSchemaBlob

type boshBlobsSchemaBlob struct {
	SHA string `yaml:"sha"`
}
