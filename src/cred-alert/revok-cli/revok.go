package main

import (
	"cred-alert/gitclient"
	"cred-alert/kolsch"
	"cred-alert/revok"
	"cred-alert/sniff"
	"fmt"
	"log"
	"os"

	git "github.com/libgit2/git2go"
)

func main() {
	path := os.Args[1]

	client := gitclient.New("", "")

	repo, err := git.OpenRepository(path)
	if err != nil {
		log.Fatal(err.Error())
	}
	defer repo.Free()

	referenceIterator, err := repo.NewReferenceIterator()
	if err != nil {
		log.Fatal(err.Error())
	}
	defer referenceIterator.Free()

	sniffer := sniff.NewDefaultSniffer()
	scanner := revok.NewScanner(
		client,
		nil,
		nil,
		nil,
		sniffer,
		nil,
		nil,
	)

	var startSHAs []string
	for {
		ref, err := referenceIterator.Next()
		if git.IsErrorCode(err, git.ErrIterOver) {
			break
		}
		if err != nil {
			log.Fatal(err.Error())
		}
		defer ref.Free()

		if ref.IsBranch() {
			startSHAs = append(startSHAs, ref.Target().String())
		}
	}

	quietLogger := kolsch.NewLogger()
	credentials, err := scanner.ScanMultiple(quietLogger, "", "", startSHAs, path)
	if err != nil {
		log.Fatal(fmt.Sprintf("failed-to-scan-multiple: %s", err.Error()))
	}

	for i := range credentials {
		fmt.Printf("%s - %s:%d '%s'\n", credentials[i].SHA, credentials[i].Path, credentials[i].LineNumber, credentials[i].Content)
	}

	os.Exit(0)
}
