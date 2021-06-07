package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"runtime"

	"github.com/pivotal-cf/cred-alert/apply"
)

type UpdateCommand struct{}

func (command *UpdateCommand) Execute(args []string) error {
	type GitHubAsset struct {
        Name string `json:"name"`
        BrowserDownloadUrl string `json:"browser_download_url"`
	}

	type GitHubRelease struct {
		TagName string `json:"tag_name"`
		TargetCommitish string `json:"target_commitish"`
        Assets []GitHubAsset `json:"assets"`
	}

	apiResponse, err := http.Get("https://api.github.com/repos/pivotal-cf/cred-alert/releases/latest")
	if err != nil {
		return err
	}
	if apiResponse.StatusCode != 200 {
		return errors.New("Error fetching latest release: " + apiResponse.Status)
	}

	defer apiResponse.Body.Close()
	decoder := json.NewDecoder(apiResponse.Body)

	var release GitHubRelease
    err = decoder.Decode(&release)
	if err != nil {
		return err
	}

	latestVersion := fmt.Sprintf("%s (%s)", release.TagName, release.TargetCommitish)

	if version == latestVersion {
		fmt.Println("Already up to date.")
		return nil
	}

	assetName := fmt.Sprintf("cred-alert-cli_%s", runtime.GOOS)

	var downloadUrl string
	for _, asset := range release.Assets {
		if asset.Name == assetName {
			downloadUrl = asset.BrowserDownloadUrl
			break
		}
	}
    if downloadUrl == "" {
		return errors.New("unable to update cred-alert for this OS")
	}

	fmt.Println("Downloading new cred-alert...")
	downloadResponse, err := http.Get(downloadUrl)
	if err != nil {
		return err
	}
	if downloadResponse.StatusCode != 200 {
		return errors.New("Error downloading latest release: " + downloadResponse.Status)
	}

	defer downloadResponse.Body.Close()
	err = apply.Apply(downloadResponse.Body)
	if err != nil {
		return err
	}

	fmt.Printf("Upgraded from %s to %s.\n", version, latestVersion)

	return nil
}
