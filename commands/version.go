package commands

import "fmt"

type VersionCommand struct{}

// overridden in CI
var version = "dev"

func (command *VersionCommand) Execute(args []string) error {
	fmt.Println(version)

	return nil
}
