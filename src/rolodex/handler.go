package rolodex

import (
	"rolodex/rolodexpb"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

type handler struct {
	teamRepository TeamRepository
}

func NewHandler(repo TeamRepository) rolodexpb.RolodexServer {
	return &handler{
		teamRepository: repo,
	}
}

func (h *handler) GetOwners(ctx context.Context, request *rolodexpb.GetOwnersRequest) (*rolodexpb.GetOwnersResponse, error) {
	owner := request.GetRepository().GetOwner()
	name := request.GetRepository().GetName()

	if owner == "" || name == "" {
		return nil, grpc.Errorf(codes.InvalidArgument, "repository owner and/or name may not be empty")
	}

	teams, err := h.teamRepository.GetOwners(Repository{
		Owner: owner,
		Name:  name,
	})
	if err != nil {
		return nil, err
	}

	pbteams := []*rolodexpb.Team{}

	for _, team := range teams {
		pbteam := &rolodexpb.Team{
			Name: team.Name,
		}

		if team.SlackChannel.Name != "" {
			pbteam.SlackChannel = &rolodexpb.SlackChannel{
				Team: team.SlackChannel.Team,
				Name: team.SlackChannel.Name,
			}
		}

		pbteams = append(pbteams, pbteam)
	}

	return &rolodexpb.GetOwnersResponse{
		Teams: pbteams,
	}, nil
}
