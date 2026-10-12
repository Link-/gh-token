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
		Version:              Version,
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
				Name:      "devcontainer-init",
				Usage:     "Start a refresher for a devcontainer's token directory (run from initializeCommand)",
				ArgsUsage: "<token directory>",
				Flags:     internal.DevcontainerInitFlags(),
				Action:    internal.DevcontainerInit,
			},
			{
				Name:      "stop",
				Usage:     "Stop the refresher keeping a token directory fresh (started with generate --refresh --pid-file)",
				ArgsUsage: "<token directory or pid file>",
				Flags:     internal.StopFlags(),
				Action:    internal.Stop,
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
