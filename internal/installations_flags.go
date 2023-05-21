package internal

import "github.com/urfave/cli/v2"

// InstallationsFlags returns the CLI flags for the generate command
func InstallationsFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:     "app-id",
			Usage:    "GitHub App ID",
			Required: true,
			Aliases:  []string{"a"},
		},
		&cli.StringFlag{
			Name:     "key",
			Usage:    "Path to private key",
			Required: false,
			Aliases:  []string{"k"},
		},
		&cli.StringFlag{
			Name:     "key-base64",
			Usage:    "A base64 encoded private key",
			Required: false,
			Aliases:  []string{"b"},
		},
		&cli.StringFlag{
			Name:     "hostname",
			Usage:    "GitHub Enterprise Server API endpoint, example: github.example.com",
			Required: false,
			Aliases:  []string{"o"},
			Value:    "api.github.com",
		},
	}
}
