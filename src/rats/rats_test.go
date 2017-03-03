package rats_test

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"

	"github.com/google/go-github/github"
	"github.com/nlopes/slack"
	"golang.org/x/oauth2"
)

var _ = Describe("Revok", func() {
	var (
		githubClient   *github.Client
		messageHistory *slackHistory
	)

	BeforeEach(func() {
		rand.Seed(GinkgoRandomSeed())

		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: mustGetEnv("RATS_GITHUB_TOKEN")},
		)
		tc := oauth2.NewClient(oauth2.NoContext, ts)
		githubClient = github.NewClient(tc)

		messageHistory = newSlackHistory(mustGetEnv("RATS_SLACK_TOKEN"), mustGetEnv("RATS_SLACK_CHANNEL"))
	})

	It("posts a message to Slack when a credential is committed to GitHub", func() {
		By("making a commit")

		owner := mustGetEnv("RATS_GITHUB_OWNER")
		repo := mustGetEnv("RATS_GITHUB_REPO")

		sha := makeCommit(githubClient, owner, repo)

		By("checking Slack")
		AtSomePoint(messageHistory.recentMessages).Should(ContainAMessageAlertingAboutCredentialsIn(sha))
	})
})

func mustGetEnv(name string) string {
	value := os.Getenv(name)

	Expect(value).NotTo(BeEmpty(), name+" was not found in the environment! please set it")

	return value
}

var letters = []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZ")

func AtSomePoint(fn func() []string) GomegaAsyncAssertion {
	return Eventually(fn, 15*time.Second, 1*time.Second)
}

func ContainAMessageAlertingAboutCredentialsIn(sha string) types.GomegaMatcher {
	return ContainElement(ContainSubstring(sha))
}

func randomCredential() string {
	b := make([]rune, 16)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return "AKIA" + string(b)
}

func makeCommit(githubClient *github.Client, owner string, repo string) string {
	commits, _, err := githubClient.Repositories.ListCommits(context.TODO(), owner, repo, &github.CommitsListOptions{
		SHA: "master",
		ListOptions: github.ListOptions{
			PerPage: 1,
		},
	})

	Expect(err).NotTo(HaveOccurred())
	Expect(commits).To(HaveLen(1))

	headCommit, _, err := githubClient.Git.GetCommit(context.TODO(), owner, repo, *commits[0].SHA)
	Expect(err).NotTo(HaveOccurred())

	tree, _, err := githubClient.Git.CreateTree(context.TODO(), owner, repo, *headCommit.Tree.SHA, []github.TreeEntry{
		{
			Path:    github.String("system-test.txt"),
			Mode:    github.String("100644"),
			Type:    github.String("blob"),
			Content: github.String(fmt.Sprintf(`password = "%s"`, randomCredential())),
		},
	})
	Expect(err).NotTo(HaveOccurred())

	author := &github.CommitAuthor{
		Name:  github.String("system tester"),
		Email: github.String("pcf-security-enablement+revok-system-test@pivotal.io"),
	}

	commit, _, err := githubClient.Git.CreateCommit(context.TODO(), owner, repo, &github.Commit{
		Message:   github.String("system test commit"),
		Author:    author,
		Committer: author,
		Tree:      tree,
		Parents:   []github.Commit{*headCommit},
	})

	Expect(err).NotTo(HaveOccurred())

	_, _, err = githubClient.Git.UpdateRef(context.TODO(), owner, repo, &github.Reference{
		Ref: github.String("refs/heads/master"),
		Object: &github.GitObject{
			SHA: commit.SHA,
		},
	}, false)

	Expect(err).NotTo(HaveOccurred())

	return *commit.SHA
}

type slackHistory struct {
	client  *slack.Client
	channel *slack.Channel
}

func newSlackHistory(token string, channelName string) *slackHistory {
	api := slack.New(token)

	channels, err := api.GetChannels(true)
	Expect(err).NotTo(HaveOccurred())

	var channel *slack.Channel

	for _, ch := range channels {
		if ch.Name == channelName {
			channel = &ch
			break
		}
	}

	Expect(channel).ToNot(BeNil(), "channel could not be found")

	return &slackHistory{
		client:  api,
		channel: channel,
	}
}

func (s *slackHistory) recentMessages() []string {
	history, err := s.client.GetChannelHistory(s.channel.ID, slack.HistoryParameters{
		Oldest: fmt.Sprintf("%d", time.Now().Add(-1*time.Minute).Unix()),
		Count:  10,
	})
	Expect(err).NotTo(HaveOccurred())

	messages := []string{}

	for _, message := range history.Messages {
		if len(message.Attachments) == 0 {
			continue
		}

		attachment := message.Attachments[0]

		messages = append(messages, attachment.Text)
	}

	return messages
}
