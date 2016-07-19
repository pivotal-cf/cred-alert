package main

import (
	"errors"
	"log"
	"os"

	"github.com/google/go-github/github"
	"github.com/jessevdk/go-flags"
	"golang.org/x/oauth2"

	"teamstr"
)

type Opts struct {
	Token        string `short:"t" long:"token" description:"github API token" value-name:"TOKEN" required:"true"`
	Team         string `short:"n" long:"team" description:"github team name" value-name:"NAME" required:"true"`
	Organization string `short:"o" long:"organization" description:"github organization name" value-name:"NAME" required:"true"`
}

func main() {
	var opts Opts

	_, err := flags.ParseArgs(&opts, os.Args)
	if err != nil {
		os.Exit(1)
	}

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: opts.Token},
	)
	tc := oauth2.NewClient(oauth2.NoContext, ts)
	client := github.NewClient(tc)

	teamID, err := fetchTeamID(client, opts.Organization, opts.Team)
	if err != nil {
		log.Fatalln(err)
	}

	syncer := teamstr.NewSyncer(client.Organizations, client.Repositories)
	err = syncer.Swim(opts.Organization, teamID)
	if err != nil {
		log.Fatalln(err)
	}
}

func fetchTeamID(client *github.Client, orgName string, teamName string) (int, error) {
	opts := &github.ListOptions{
		Page: 1,
	}

	for {
		teams, resp, err := client.Organizations.ListTeams(orgName, opts)
		if err != nil {
			return 0, err
		}

		for _, team := range teams {
			if *team.Name == teamName {
				return *team.ID, nil
			}
		}

		if resp.NextPage == 0 {
			break
		}

		opts.Page = resp.NextPage
	}

	return 0, errors.New("team name not found")
}
