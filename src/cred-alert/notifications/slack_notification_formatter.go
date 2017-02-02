package notifications

import (
	"strconv"
	"strings"
	"fmt"
	"bytes"
	"net/url"
	"sort"
)

type slackNotificationFormatter struct{}

//go:generate counterfeiter . SlackNotificationFormatter

type SlackNotificationFormatter interface{
	FormatNotifications(batch []Notification) []SlackMessage
}

func NewSlackNotificationFormatter() SlackNotificationFormatter {
	return &slackNotificationFormatter{}
}

type commitGroup map[commit][]Notification
type fileGroup map[file][]Notification

type commit struct {
	Owner      string
	Repository string
	SHA        string
	Private    bool
}

func (r commit) RepoName() string {
	return fmt.Sprintf("%s/%s", r.Owner, r.Repository)
}

func (r commit) ShortSHA() string {
	return r.SHA[:7]
}

func (r commit) color() string {
	if r.Private {
		return "warning"
	}

	return "danger"
}

func (r commit) title() string {
	return fmt.Sprintf("Possible credentials found in %s!", slackLink{
		Text: fmt.Sprintf("%s / %s", r.RepoName(), r.ShortSHA()),
		Href: r.link(),
	})
}

func (r commit) fallbackMessage() string {
	return fmt.Sprintf("Possible credentials found in %s!", r.link())
}

func (r commit) link() string {
	return githubURL(r.Owner, r.Repository, "commit", r.SHA)
}

type file struct {
	path string
}

func (r file) linkIn(c commit) string {
	return githubURL(c.Owner, c.Repository, "blob", c.SHA, r.path)
}

func (r file) linkToLineInCommit(c commit, line int) string {
	return fmt.Sprintf("%s#L%d", r.linkIn(c), line)
}

func (s *slackNotificationFormatter) FormatNotifications(batch []Notification) []SlackMessage {
	messages := []SlackMessage{}

	commits := s.groupByCommit(batch)

	for commit, commitNotifications := range commits {
		files := s.groupByFile(commitNotifications)

		fileLines := []string{}

		for file, fileNotifications := range files {
			lineLinks := []string{}

			sort.Sort(byFileAndLineNumber(fileNotifications))

			for _, n := range fileNotifications {
				lineLinks = append(lineLinks, slackLink{
					Text: strconv.Itoa(n.LineNumber),
					Href: file.linkToLineInCommit(commit, n.LineNumber),
				}.String())
			}

			plurality := "line"
			if len(lineLinks) > 1 {
				plurality = "lines"
			}

			text := fmt.Sprintf("• %s on %s %s", slackLink{
				Text: file.path,
				Href: file.linkIn(commit),
			}, plurality, humanizeList(lineLinks))

			fileLines = append(fileLines, text)
		}

		messages = append(messages, SlackMessage{
			Attachments: []SlackAttachment{
				{
					Title:    commit.title(),
					Text:     strings.Join(fileLines, "\n"),
					Color:    commit.color(),
					Fallback: commit.fallbackMessage(),
				},
			},
		})
	}

	return messages
}

func (s *slackNotificationFormatter) groupByCommit(batch []Notification) commitGroup {
	group := make(commitGroup)

	for _, notification := range batch {
		repo := commit{
			Owner:      notification.Owner,
			Repository: notification.Repository,
			SHA:        notification.SHA,
			Private:    notification.Private,
		}

		_, found := group[repo]
		if !found {
			group[repo] = []Notification{}
		}

		group[repo] = append(group[repo], notification)
	}

	return group
}

func (s *slackNotificationFormatter) groupByFile(batch []Notification) fileGroup {
	group := make(fileGroup)

	for _, notification := range batch {
		file := file{
			path: notification.Path,
		}

		_, found := group[file]
		if !found {
			group[file] = []Notification{}
		}

		group[file] = append(group[file], notification)
	}

	return group
}


type slackLink struct {
	Text string
	Href string
}

func (l slackLink) String() string {
	return fmt.Sprintf("<%s|%s>", l.Href, l.Text)
}

func humanizeList(list []string) string {
	joinedLines := &bytes.Buffer{}

	if len(list) <= 1 {
		joinedLines.WriteString(list[0])
	} else if len(list) == 2 {
		joinedLines.WriteString(list[0])
		joinedLines.WriteString(" and ")
		joinedLines.WriteString(list[1])
	} else {
		for _, line := range list[:len(list)-1] {
			joinedLines.WriteString(line)
			joinedLines.WriteString(", ")
		}

		joinedLines.WriteString("and ")
		joinedLines.WriteString(list[len(list)-1])
	}

	return joinedLines.String()
}

func githubURL(components ...string) string {
	url := &url.URL{
		Scheme: "https",
		Host:   "github.com",
		Path:   strings.Join(components, "/"),
	}

	return url.String()
}

type byFileAndLineNumber []Notification

func (fn byFileAndLineNumber) Len() int {
	return len(fn)
}

func (fn byFileAndLineNumber) Swap(i, j int) {
	fn[i], fn[j] = fn[j], fn[i]
}

func (fn byFileAndLineNumber) Less(i, j int) bool {
	if fn[i].Path != fn[j].Path {
		return fn[i].Path < fn[j].Path
	}

	return fn[i].LineNumber < fn[j].LineNumber
}
