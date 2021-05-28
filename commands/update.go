package commands

import (
	"errors"
	"fmt"
	"net/http"
	"runtime"

	"github.com/pivotal-cf/cred-alert/apply"
)

const s3Path = "https://s3.amazonaws.com/cred-alert/cli/current-release"

type UpdateCommand struct{}

func (command *UpdateCommand) Execute(args []string) error {
	var url string
	switch {
	case runtime.GOOS == "darwin":
		url = s3Path + "/cred-alert-cli_darwin"
	case runtime.GOOS == "linux":
		url = s3Path + "/cred-alert-cli_linux"
	default:
		return errors.New("unable to update cred-alert for this OS")
	}

	fmt.Print("Downloading new cred-alert...")
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	err = apply.Apply(resp.Body)
	if err != nil {
		fmt.Println("failed :(")
		return err
	}

	fmt.Println("done!")

	return nil
}
