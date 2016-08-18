package main

import (
	"cred-alert/commands"
	"fmt"
	"os"

	"github.com/jessevdk/go-flags"
)

func main() {
	parser := flags.NewParser(&commands.CredAlert, flags.HelpFlag)

	_, err := parser.Parse()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
}
