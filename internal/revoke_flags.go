package internal

import "github.com/urfave/cli/v2"

// RevokeFlags returns the CLI flags for the revoke command
func RevokeFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:     "token",
			Usage:    "GitHub App installation Token",
			Required: true,
			Aliases:  []string{"t"},
		},
		&cli.StringFlag{
			Name:     "hostname",
			Usage:    "GitHub Enterprise Server API endpoint, example: github.example.com",
			Required: false,
			Aliases:  []string{"o"},
			Value:    "api.github.com",
		},
		&cli.BoolFlag{
			Name:    "silent",
			Usage:   "Do not print to stdout",
			Aliases: []string{"s"},
			Value:   false,
		},
	}
}
