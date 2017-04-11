package notifications

import (
	"context"
	"time"

	"code.cloudfoundry.org/lager"
	netcontext "golang.org/x/net/context"
	"google.golang.org/grpc"

	"cloud.google.com/go/trace"
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
	slackTeamURLs  map[string]string
	defaultAddress Address
}

func NewTeamURLs(defaultURL string, defaultChannel string, mapping map[string]string) TeamURLs {
	return TeamURLs{
		slackTeamURLs: mapping,
		defaultAddress: Address{
			URL:     defaultURL,
			Channel: defaultChannel,
		},
	}
}

func (t TeamURLs) Default() Address {
	return t.defaultAddress
}

func (t TeamURLs) Lookup(logger lager.Logger, teamName string, channelName string) Address {
	url, found := t.slackTeamURLs[teamName]
	if !found {
		logger.Info("unknown-slack-team", lager.Data{
			"team-name": teamName,
		})
		return t.defaultAddress
	}

	return Address{
		URL:     url,
		Channel: channelName,
	}
}

//go:generate counterfeiter . AddressBook

type AddressBook interface {
	AddressForRepo(ctx context.Context, logger lager.Logger, owner, name string) []Address
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

func (r *rolodex) AddressForRepo(ctx context.Context, logger lager.Logger, owner, name string) []Address {
	logger = logger.Session("rolodex", lager.Data{
		"owner":      owner,
		"repository": name,
	})

	ctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()

	span := trace.FromContext(ctx).NewChild("/rolodex")
	defer span.Finish()

	response, err := r.client.GetOwners(ctx, &rolodexpb.GetOwnersRequest{
		Repository: &redpb.Repository{
			Owner: owner,
			Name:  name,
		},
	})

	if err != nil {
		logger.Error("getting-owners-failed", err)

		return []Address{r.teamURLs.Default()}
	}

	return r.addressesFor(logger, response.GetTeams())
}

func (r *rolodex) addressesFor(logger lager.Logger, teams []*rolodexpb.Team) []Address {
	if len(teams) == 0 {
		logger.Info("no-owners-found")
		return []Address{r.teamURLs.Default()}
	}

	addresses := []Address{}

	for _, team := range teams {
		channel := team.GetSlackChannel()
		address := r.teamURLs.Lookup(logger, channel.GetTeam(), channel.GetName())
		addresses = append(addresses, address)
	}

	return addresses
}
