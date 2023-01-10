package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v49/github"
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

// Validation hook
// https://github.com/alecthomas/kong#hooks-beforereset-beforeresolve-beforeapply-afterapply-and-the-bind-option
func (I GenerateCmd) Validate() (err error) {
	if cli.TlsConfig.InsecureSkipVerify {
		println("WARNING: InsecureSkipVerify is enabled. This is not recommended in production.")
	}
	return nil
}

func (cmd *GenerateCmd) Run() error {
	// Build the logger and use it for any output
	logger := NewLogger(cli.Logging.Level, cli.Logging.Type)
	logger.Debugf("GenerateCmd: called with AppID %v, InstallID %v, KeyFile %v", cmd.AppID, cmd.InstallID, cmd.KeyFile)

	customTransport := &http.Transport{
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: cli.TlsConfig.InsecureSkipVerify},
		Proxy:               http.ProxyFromEnvironment,
		TLSHandshakeTimeout: time.Duration(cli.TlsConfig.HandshakeTimeout) * time.Second,
	}

	// Create the transport for the Github client. This contains the options required to authenticate with Github.
	githubTransport, err := ghinstallation.NewAppsTransportKeyFromFile(customTransport, cmd.AppID, cmd.KeyFile)
	if err != nil {
		return err
	}

	logger.Debug("GenerateCmd: Building the Github client")
	githubTransport.BaseURL = cmd.GithubURL

	var client *github.Client

	// Build the default Github client if we're using the default URL
	if githubTransport.BaseURL == "https://api.github.com" {
		client = github.NewClient(&http.Client{Transport: githubTransport, Timeout: time.Duration(cli.TlsConfig.HandshakeTimeout) * time.Second})
	} else {
		// Build the Github Enterprise client if we're using a custom URL
		client, err = github.NewEnterpriseClient(githubTransport.BaseURL, githubTransport.BaseURL, &http.Client{Transport: githubTransport})
		if err != nil {
			return err
		}
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
