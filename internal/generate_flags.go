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
		&cli.StringFlag{
			Name:    "scope-file",
			Usage:   "Path to a scope file or devcontainer.json declaring repositories and permissions (Codespaces format) to downscope the token to",
			Aliases: []string{"f"},
		},
		&cli.StringSliceFlag{
			Name:    "repository",
			Usage:   "Repository to downscope the token to, as owner/repo (or a bare name with --installation-id). Repeatable",
			Aliases: []string{"r"},
		},
		&cli.StringSliceFlag{
			Name:    "permission",
			Usage:   "Permission for the token, as name=level (read, write or admin), or read-all / write-all. Repeatable",
			Aliases: []string{"p"},
		},
		&cli.BoolFlag{
			Name:  "merge-permissions",
			Usage: "Allow repositories with different permissions by giving the token the union of them. This widens access",
		},
		&cli.BoolFlag{
			Name:  "print-request",
			Usage: "Print the resolved installation ID and the access token request body, then exit without minting a token",
		},
		&cli.BoolFlag{
			Name:    "current-repo",
			Usage:   "Include the clone's own repository, read from its git remote (like Codespaces). Defaults to true for devcontainer-shaped scope files",
			Aliases: []string{"c"},
		},
		&cli.StringFlag{
			Name:  "git-dir",
			Usage: "Clone to read the current repository from",
			Value: ".",
		},
		&cli.StringFlag{
			Name:  "git-remote",
			Usage: "Git remote to read the current repository from",
			Value: "origin",
		},
		&cli.StringFlag{
			Name:  "output-file",
			Usage: "Write the token atomically to this file (mode 0600) instead of stdout, plus metadata to <file>.json. Mount its directory, not the file, into containers",
		},
		&cli.BoolFlag{
			Name:  "verify-installation",
			Usage: "With --installation-id and a scope, check that the ID matches the installation found for the scope's owner",
		},
	}
}
