package rolodex

import (
	"code.cloudfoundry.org/lager"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"cred-alert/metrics"
	"rolodex/rolodexpb"
)

type handler struct {
	teamRepository TeamRepository

	logger         lager.Logger
	successCounter metrics.Counter
	failureCounter metrics.Counter
}

func NewHandler(logger lager.Logger, repo TeamRepository, emitter metrics.Emitter) rolodexpb.RolodexServer {
	handlerLogger := logger.Session("handler")

	return &handler{
		teamRepository: repo,
		logger:         handlerLogger,
		successCounter: emitter.Counter("rolodex.handler.success"),
		failureCounter: emitter.Counter("rolodex.handler.failure"),
	}
}

func (h *handler) GetOwners(ctx context.Context, request *rolodexpb.GetOwnersRequest) (*rolodexpb.GetOwnersResponse, error) {
	owner := request.GetRepository().GetOwner()
	name := request.GetRepository().GetName()

	if owner == "" || name == "" {
		h.failureCounter.Inc(h.logger)
		return nil, grpc.Errorf(codes.InvalidArgument, "repository owner and/or name may not be empty")
	}

	teams, err := h.teamRepository.GetOwners(Repository{
		Owner: owner,
		Name:  name,
	})
	if err != nil {
		h.failureCounter.Inc(h.logger)
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

	h.successCounter.Inc(h.logger)

	return &rolodexpb.GetOwnersResponse{
		Teams: pbteams,
	}, nil
}
