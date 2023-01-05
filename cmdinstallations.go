package main

import (
	"context"
	"net/http"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v49/github"
)

// Implementation of the "installations" command
type InstallationsCmd struct {

	// Arguments
	AppID   int64  `arg:"" help:"Github App ID." type:"int" aliases:"app_id" env:"GHTOKEN_APP_ID" required:"true"`
	KeyFile string `arg:"" help:"Path to the private key file (pem)." type:"existingfile" aliases:"key" env:"GHTOKEN_KEY_FILE" required:"true"`

	// Options
	GithubURL string `help:"Github API URL" default:"https://api.github.com" env:"GHTOKEN_GITHUB_URL"`
}

func (cmd *InstallationsCmd) Run() error {
	// Build the logger and use it for any output
	logger := NewLogger(cli.Logging.Level, cli.Logging.Type)
	logger.Debugf("InstallationsCmd: called with AppID %v, KeyFile %v, GithubURL %v", cmd.AppID, cmd.KeyFile, cmd.GithubURL)

	// Create a new Github app client
	logger.Debug("InstallationsCmd: Creating a Github transport from Key file")
	githubTransport, err := ghinstallation.NewAppsTransportKeyFromFile(http.DefaultTransport, cmd.AppID, cmd.KeyFile)
	if err != nil {
		return err
	}

	logger.Debug("InstallationsCmd: Building the Github client")
	githubTransport.BaseURL = cmd.GithubURL
	client, err := BuildGithubClient(githubTransport)
	if err != nil {
		return err
	}

	// List all installations for the provided AppId
	logger.Debug("InstallationsCmd: Listing all installations")
	// TODO: Potentially handle pagination as an edge case, for now we'll just return a single page of 100.
	installations, resp, err := client.Apps.ListInstallations(context.Background(), &github.ListOptions{
		PerPage: 100,
	})
	if resp != nil {
		logger.Debug(resp.Status)
	}
	if err != nil {
		return err
	}

	logger.Debugf("InstallationsCmd: Got %v installations", len(installations))
	logger.Info(installations)

	return nil
}
