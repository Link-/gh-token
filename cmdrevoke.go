package main

import (
	"context"

	"github.com/google/go-github/v49/github"
	"golang.org/x/oauth2"
)

// Implementation of the "Revoke" command
type RevokeCmd struct {
	// Arguments
	// AppID int64  `arg:"" help:"Github App ID." type:"int" aliases:"app_id" env:"GHTOKEN_APP_ID" required:"true"`
	Token string `arg:"" help:"The token to auth as for revocation." type:"string" aliases:"token" env:"GHTOKEN_TOKEN" required:"true"`

	// Options
	GithubURL string `help:"Github API URL" default:"https://api.github.com" env:"GHTOKEN_GITHUB_URL"`
}

func (cmd *RevokeCmd) Run() error {
	// Build the logger and use it for any output
	logger := NewLogger(cli.Logging.Level, cli.Logging.Type)
	logger.Debug("RevokeCmd called")

	// Create the transport for the Github client. This contains the options required to authenticate with Github.
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: cmd.Token},
	)
	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)
	resp, err := client.Apps.RevokeInstallationToken(ctx) // TODO: This is not working with Github Enterprise yet.
	if err != nil {
		return err
	}
	if resp.StatusCode == 204 {
		logger.Info("Token revoked successfully")
	} else {
		logger.Errorf("Token revocation failed with status code %v", resp.StatusCode)
	}

	return nil
}
