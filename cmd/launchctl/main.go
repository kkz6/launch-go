package main

import (
	"fmt"
	"os"

	"github.com/kkz6/launch-go/cmd/launchctl/commands"
)

var version = "development"

func main() {
	root := commands.NewRootCmd(version)
	if err := root.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
