package main

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/url"
	"time"

	"github.com/google/go-github/v49/github"
	"golang.org/x/oauth2"
)

// Implementation of the "Revoke" command
type RevokeCmd struct {
	// Arguments
	Token string `arg:"" help:"The token to auth as for revocation." type:"string" aliases:"token" env:"GHTOKEN_TOKEN" required:"true"`

	// Options
	GithubURL string `help:"Github API URL" type:"url" default:"https://api.github.com" env:"GHTOKEN_GITHUB_URL"`
}

// Validation hook
// https://github.com/alecthomas/kong#hooks-beforereset-beforeresolve-beforeapply-afterapply-and-the-bind-option
func (I RevokeCmd) Validate() (err error) {
	if cli.TlsConfig.InsecureSkipVerify {
		println("WARNING: InsecureSkipVerify is enabled. This is not recommended in production.")
	}
	return nil
}

// Revokes a Github access token
func (cmd *RevokeCmd) Run() error {
	// Build the logger and use it for any output
	logger := NewLogger(cli.Logging.Level, cli.Logging.Type)
	logger.Debug("RevokeCmd called")

	// Create the transport for the Github client. This contains the options required to authenticate with Github.
	ctx := context.Background()
	tokenSource := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: cmd.Token},
	)

	httpClient := oauth2.NewClient(ctx, tokenSource)
	httpClient.Timeout = time.Duration(cli.TlsConfig.HandshakeTimeout) * time.Second
	httpClient.Transport = &http.Transport{
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: cli.TlsConfig.InsecureSkipVerify},
		Proxy:               http.ProxyFromEnvironment,
		TLSHandshakeTimeout: time.Duration(cli.TlsConfig.HandshakeTimeout) * time.Second,
	}

	client := github.NewClient(httpClient)
	u, err := url.Parse(cmd.GithubURL + "/")
	if err != nil {
		return err
	}
	client.BaseURL = u
	resp, err := client.Apps.RevokeInstallationToken(ctx)
	if err != nil && resp != nil {
		if resp.StatusCode == 401 {
			logger.Errorf("Token revocation failed with status code %v. This is likely due to the token being invalid, this could be due to expiration or perhaps it was already successfully revoked.", resp.StatusCode)
			return nil
		}
	}
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
