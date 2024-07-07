// Package main is the entrypoint for the gh-token CLI
package main

import (
	"fmt"
	"os"

	"github.com/Link-/gh-token/internal"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:                 "gh-token",
		Usage:                "Manage GitHub App installation tokens",
		Version:              "2.0.2",
		EnableBashCompletion: true,
		Suggest:              true,
		Commands: []*cli.Command{
			{
				Name:   "generate",
				Usage:  "Generate a new GitHub App installation token",
				Flags:  internal.GenerateFlags(),
				Action: internal.Generate,
			},
			{
				Name:   "revoke",
				Usage:  "Revoke a GitHub App installation token",
				Flags:  internal.RevokeFlags(),
				Action: internal.Revoke,
			},
			{
				Name:   "installations",
				Usage:  "List GitHub App installations",
				Flags:  internal.InstallationsFlags(),
				Action: internal.Installations,
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
