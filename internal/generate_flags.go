package internal

import "github.com/urfave/cli/v2"

// GenerateFlags returns the CLI flags for the generate command
func GenerateFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:     "app-id",
			Usage:    "GitHub App ID",
			Required: true,
			Aliases:  []string{"a"},
		},
		&cli.StringFlag{
			Name:     "installation-id",
			Usage:    "GitHub App installation ID. Defaults to the first installation returned by the GitHub API if not specified",
			Required: false,
			Aliases:  []string{"i"},
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
		&cli.BoolFlag{
			Name:    "token-only",
			Usage:   "Only print the token to stdout, not the full JSON response, useful for piping to other commands",
			Aliases: []string{"t"},
			Value:   false,
		},
		&cli.BoolFlag{
			Name:     "jwt",
			Usage:    "Return the JWT instead of generating an installation token, useful for calling API's requiring a JWT",
			Required: false,
			Aliases:  []string{"j"},
			Value:    false,
		},
		&cli.IntFlag{
			Name:     "jwt-expiry",
			Usage:    "The expiry time of the JWT in minutes up to a maximum value of 10, useful when using the --jwt flag",
			Required: false,
			Aliases:  []string{"e"},
			Value:    1,
		},
		&cli.BoolFlag{
			Name:    "silent",
			Usage:   "Do not print token to stdout",
			Aliases: []string{"s"},
			Value:   false,
		},
	}
}
