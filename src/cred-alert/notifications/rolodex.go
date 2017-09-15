package notifications

import (
	"context"
	"time"

	"code.cloudfoundry.org/lager"
	netcontext "golang.org/x/net/context"
	"google.golang.org/grpc"

	"red/redpb"
	"rolodex/rolodexpb"
)

//go:generate counterfeiter . RolodexClient

type RolodexClient interface {
	GetOwners(ctx netcontext.Context, in *rolodexpb.GetOwnersRequest, opts ...grpc.CallOption) (*rolodexpb.GetOwnersResponse, error)
}

type Address struct {
	URL     string
	Channel string
}

type TeamURLs struct {
	slackTeamURLs         map[string]string
	defaultPublicAddress  Address
	defaultPrivateAddress Address
}

func NewTeamURLs(defaultURL string, defaultPublicChannel, defaultPrivateChannel string, mapping map[string]string) TeamURLs {
	return TeamURLs{
		slackTeamURLs: mapping,
		defaultPublicAddress: Address{
			URL:     defaultURL,
			Channel: defaultPublicChannel,
		},
		defaultPrivateAddress: Address{
			URL:     defaultURL,
			Channel: defaultPrivateChannel,
		},
	}
}

// TODO add isPrivate
func (t TeamURLs) Default(isPrivate bool) Address {
	if isPrivate {
		return t.defaultPrivateAddress
	}
	return t.defaultPublicAddress
}

func (t TeamURLs) Lookup(logger lager.Logger, isPrivate bool, teamName, channelName string) Address {
	url, found := t.slackTeamURLs[teamName]
	if !found {
		logger.Info("unknown-slack-team", lager.Data{
			"team-name": teamName,
		})
		return t.Default(isPrivate)
	}

	return Address{
		URL:     url,
		Channel: channelName,
	}
}

//go:generate counterfeiter . AddressBook

type AddressBook interface {
	AddressForRepo(ctx context.Context, logger lager.Logger, isPrivate bool, owner, name string) []Address
}

type rolodex struct {
	client   RolodexClient
	teamURLs TeamURLs
}

func NewRolodex(client RolodexClient, teamURLs TeamURLs) AddressBook {
	return &rolodex{
		client:   client,
		teamURLs: teamURLs,
	}
}

func (r *rolodex) AddressForRepo(ctx context.Context, logger lager.Logger, isPrivate bool, owner, name string) []Address {
	logger = logger.Session("rolodex", lager.Data{
		"owner":      owner,
		"repository": name,
	})

	ctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()

	response, err := r.client.GetOwners(ctx, &rolodexpb.GetOwnersRequest{
		Repository: &redpb.Repository{
			Owner: owner,
			Name:  name,
		},
	})

	if err != nil {
		logger.Error("getting-owners-failed", err)
		return []Address{r.teamURLs.Default(isPrivate)}
	}

	return r.addressesForTeam(logger, isPrivate, response.GetTeams())
}

func (r *rolodex) addressesForTeam(logger lager.Logger, isPrivate bool, teams []*rolodexpb.Team) []Address {
	if len(teams) == 0 {
		logger.Info("no-owners-found")
		return []Address{r.teamURLs.Default(isPrivate)}
	}

	addresses := []Address{}

	for _, team := range teams {
		channel := team.GetSlackChannel()
		address := r.teamURLs.Lookup(logger, isPrivate, channel.GetTeam(), channel.GetName())
		addresses = append(addresses, address)
	}

	return addresses
}
