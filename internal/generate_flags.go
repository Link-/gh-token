package internal

import "github.com/urfave/cli/v2"

// GenerateFlags returns the CLI flags for the generate command
func GenerateFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:     "app-id",
			Usage:    "GitHub App ID",
			Required: true,
			Aliases:  []string{"i", "app_id"},
		},
		&cli.StringFlag{
			Name:     "installation-id",
			Usage:    "GitHub App installation ID. Defaults to the first installation returned by the GitHub API if not specified",
			Required: false,
			Aliases:  []string{"l", "installation_id"},
		},
		&cli.StringFlag{
			Name:     "repository",
			Usage:    "GitHub repository, can be specified in place of GitHub App installation ID",
			Required: false,
			Aliases:  []string{"r"},
		},
		&cli.StringFlag{
			Name:     "key",
			Usage:    "Path to private key",
			Required: false,
			Aliases:  []string{"k"},
		},
		&cli.StringFlag{
			Name:     "base64-key",
			Usage:    "A base64 encoded private key",
			Required: false,
			Aliases:  []string{"b", "base64_key"},
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
			Name:     "duration",
			Usage:    "The expiry time of the JWT in minutes up to a maximum value of 10, useful when using the --jwt flag",
			Required: false,
			Aliases:  []string{"d"},
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
