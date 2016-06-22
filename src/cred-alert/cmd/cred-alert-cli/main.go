package main

import (
	"fmt"
	"net/http"
	"os"

	"cred-alert/git"
	"cred-alert/github"
)

func main() {
	owner := os.Args[1]
	repo := os.Args[2]
	base := os.Args[3]
	head := os.Args[4]

	httpClient := &http.Client{}

	githubClient := github.NewClient("https://api.github.com/", httpClient)

	input, err := githubClient.CompareRefs(owner, repo, base, head)
	if err != nil {
		fmt.Fprintln(os.Stderr, "request error: ", err)
		os.Exit(1)
	}

	matchingLines := git.Scan(input)
	for _, line := range matchingLines {
		fmt.Printf("Line matches pattern! File: %s, Line Number: %d, Content: %s\n", line.Path, line.LineNumber, line.Content)
	}
}
