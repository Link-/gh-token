package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/bradleyfalzon/ghinstallation/v2"
)

// Implementation of the "Generate" command
type GenerateCmd struct {

	// Arguments
	AppID     int64  `arg:"" help:"Github App ID." type:"int" aliases:"app_id" env:"GHTOKEN_APP_ID" required:"true"`
	InstallID int64  `arg:"" help:"Github App Installation ID." type:"int" aliases:"install_id" env:"GHTOKEN_INSTALL_ID" required:"true"`
	KeyFile   string `arg:"" help:"Path to the private key file (pem)." type:"existingfile" aliases:"key" env:"GHTOKEN_KEY_FILE" required:"true"`

	// Options
	GithubURL string `help:"Github API URL" default:"https://api.github.com" env:"GHTOKEN_GITHUB_URL"`
	Duration  int    `help:"Duration in minutes for the token to be valid for." default:"60" env:"GHTOKEN_DURATION"`
}

func (cmd *GenerateCmd) Run() error {
	// Build the logger and use it for any output
	logger := NewLogger(cli.Logging.Level, cli.Logging.Type)
	logger.Debugf("GenerateCmd: called with AppID %v, InstallID %v, KeyFile %v", cmd.AppID, cmd.InstallID, cmd.KeyFile)

	// Create the transport for the Github client. This contains the options required to authenticate with Github.
	githubTransport, err := ghinstallation.NewAppsTransportKeyFromFile(http.DefaultTransport, cmd.AppID, cmd.KeyFile)
	if err != nil {
		return err
	}

	logger.Debug("GenerateCmd: Building the Github client")
	githubTransport.BaseURL = cmd.GithubURL
	client, err := BuildGithubClient(githubTransport)
	if err != nil {
		return err
	}

	token, resp, err := client.Apps.CreateInstallationToken(context.Background(), cmd.InstallID, nil)
	if resp != nil {
		logger.Debug(resp.Status)
	}
	if err != nil {
		return err
	}

	t := token.GetToken()
	if t == "" {
		return fmt.Errorf("token is empty")
	}

	println(t)

	return nil
}
