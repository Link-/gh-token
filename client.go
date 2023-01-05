package main

import (
	"net/http"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v49/github"
)

// Determines which client to build
func BuildGithubClient(appsTransport *ghinstallation.AppsTransport) (*github.Client, error) {

	// Create a placeholder client to return
	var client *github.Client
	var err error

	// Build the default Github client if we're using the default URL
	if appsTransport.BaseURL == "https://api.github.com" {
		client = github.NewClient(&http.Client{Transport: appsTransport})
	} else {
		// Build the Github Enterprise client if we're using a custom URL
		client, err = github.NewEnterpriseClient(appsTransport.BaseURL, appsTransport.BaseURL, &http.Client{Transport: appsTransport})
		if err != nil {
			return nil, err // Only Github Enterprise clients return an error on create for some reason.
		}
	}
	return client, nil
}
