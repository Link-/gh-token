package main

import (
	"context"
	"fmt"
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
	logger.Debugf("InstallationsCmd: called with AppID %v, KeyFile %v", cmd.AppID, cmd.KeyFile)

	// Create a new Github app client
	logger.Debug("InstallationsCmd: Creating a Github transport from Key file")
	githubTransport, err := ghinstallation.NewAppsTransportKeyFromFile(http.DefaultTransport, cmd.AppID, cmd.KeyFile)
	if err != nil {
		return err
	}

	logger.Debug("InstallationsCmd: Checking to see whether we're targeting Github Enterprise or Github.com")
	githubTransport.BaseURL = cmd.GithubURL
	var client *github.Client
	if githubTransport.BaseURL == "https://api.github.com" {
		// Build the default Github client if we're using the default URL
		logger.Debug("InstallationsCmd: BaseURL is default (https://api.github.com), using default client")
		client = github.NewClient(&http.Client{Transport: githubTransport})
	} else {
		// Build the Github Enterprise client if we're using a custom URL
		logger.Debugf("InstallationsCmd: BaseURL is not default (%v), using the Github Enterprise client", githubTransport.BaseURL)
		client, err = github.NewEnterpriseClient(githubTransport.BaseURL, githubTransport.BaseURL, &http.Client{Transport: githubTransport})
		if err != nil {
			return err // Only Github Enterprise clients return an error on create for some reason.
		}
	}

	// List all installations for the provided AppId
	logger.Debug("InstallationsCmd: Listing all installations")
	// TODO: Handle pagination as an edge case, for now we'll just return a single page of 100.
	installations, resp, err := client.Apps.ListInstallations(context.Background(), &github.ListOptions{
		PerPage: 100,
	})
	if resp != nil {
		logger.Debug(resp.Status)
		if resp.StatusCode != 200 {
			return fmt.Errorf("InstallationsCmd: Got status code %v", resp.StatusCode)
		}
	}
	if err != nil {
		return err
	}

	logger.Debugf("InstallationsCmd: Got %v installations", len(installations))
	logger.Info(installations)

	return nil
}
