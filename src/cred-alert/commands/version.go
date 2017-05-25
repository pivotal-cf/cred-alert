package commands

import "fmt"

type VersionCommand struct {
}

var (
  // overridden in CI
  version = "dev"
)

func (command *VersionCommand) Execute(args []string) error {
  fmt.Println(version)

  return nil
}
