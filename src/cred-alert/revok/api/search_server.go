package api

import (
	"code.cloudfoundry.org/lager"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"red/redpb"

	"cred-alert/revokpb"
	"cred-alert/search"
	"cred-alert/sniff/matchers"
)

type SearchServer struct {
	logger       lager.Logger
	searcher     search.Searcher
	blobSearcher search.BlobSearcher
}

func NewSearchServer(
	logger lager.Logger,
	searcher search.Searcher,
	blobSearcher search.BlobSearcher,
) *SearchServer {
	return &SearchServer{
		logger:       logger,
		searcher:     searcher,
		blobSearcher: blobSearcher,
	}
}

func (s *SearchServer) BoshBlobs(ctx context.Context, request *revokpb.BoshBlobsRequest) (*revokpb.BoshBlobsResponse, error) {
	logger := s.logger.Session("bosh-blobs-endpoint")

	repository := request.GetRepository()
	blobs, err := s.blobSearcher.ListBlobs(logger, repository.GetOwner(), repository.GetName())
	if err != nil {
		return nil, err
	}

	response := revokpb.BoshBlobsResponse{}

	for _, blob := range blobs {
		response.Blobs = append(response.Blobs, &revokpb.BoshBlob{
			Path: blob.Path,
			Sha:  blob.SHA,
		})
	}

	return &response, nil
}

func (s *SearchServer) Search(query *revokpb.SearchQuery, stream revokpb.RevokSearch_SearchServer) error {
	logger := s.logger.Session("search-endpoint")
	logger.Info("hit")

	regex := query.GetRegex()
	if regex == "" {
		return grpc.Errorf(codes.InvalidArgument, "query regular expression may not be empty")
	}

	matcher, err := matchers.TryFormat(regex)
	if err != nil {
		return grpc.Errorf(codes.InvalidArgument, "query regular expression is invalid: '%s'", regex)
	}

	results := s.searcher.SearchCurrent(stream.Context(), logger, matcher)

	for result := range results.C() {
		searchResult := &revokpb.SearchResult{
			Location: &revokpb.SourceLocation{
				Repository: &redpb.Repository{
					Owner: result.Owner,
					Name:  result.Repository,
				},
				Revision:   result.Revision,
				Path:       result.Path,
				LineNumber: uint32(result.LineNumber),
				Location:   uint32(result.Location),
				Length:     uint32(result.Length),
			},
			Content: result.Content,
		}

		if err := stream.Send(searchResult); err != nil {
			return err
		}
	}

	if err := results.Err(); err != nil {
		return err
	}

	return nil
}
