package internal

import (
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/go-github/v55/github"
	"github.com/urfave/cli/v2"
)

// Installations is the entrypoint for the installations command
func Installations(c *cli.Context) error {
	appID := c.String("app-id")
	keyPath := c.String("key")
	keyBase64 := c.String("key-base64")
	hostname := strings.ToLower(c.String("hostname"))

	if keyPath == "" && keyBase64 == "" {
		return fmt.Errorf("either --key or --key-base64 must be specified")
	}

	if keyPath != "" && keyBase64 != "" {
		return fmt.Errorf("only one of --key or --key-base64 may be specified")
	}

	if hostname != "api.github.com" && !strings.Contains(hostname, "/api/v3") {
		endpoint := fmt.Sprintf("%s/api/v3", hostname)
		hostname = strings.TrimSuffix(endpoint, "/")
	}

	var err error
	var privateKey *rsa.PrivateKey
	if keyPath != "" {
		privateKey, err = readKey(keyPath)
		if err != nil {
			return err
		}
	} else {
		privateKey, err = readKeyBase64(keyBase64)
		if err != nil {
			return err
		}
	}

	jsonWebToken, err := generateJWT(appID, 1, privateKey)
	if err != nil {
		return fmt.Errorf("failed generating JWT: %w", err)
	}

	installations, err := listInstallations(hostname, jsonWebToken)
	if err != nil {
		return fmt.Errorf("failed listing installations: %w", err)
	}

	bytes, err := json.MarshalIndent(installations, "", "  ")
	if err != nil {
		return fmt.Errorf("failed marshalling installations to JSON: %w", err)
	}

	fmt.Println(string(bytes))

	return nil
}

func listInstallations(hostname, jwt string) (*[]github.Installation, error) {
	page := 0
	var responses []github.Installation
	for {
		endpoint := fmt.Sprintf("https://%s/app/installations?per_page=100&page=%d", hostname, page)
		req, err := http.NewRequest("GET", endpoint, nil)
		if err != nil {
			return nil, fmt.Errorf("unable to create GET request to %s: %w", endpoint, err)
		}
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", jwt))
		req.Header.Add("Accept", "application/vnd.github+json")
		req.Header.Add("X-GitHub-Api-Version", "2022-11-28")
		req.Header.Add("User-Agent", "Link-/gh-token")

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("unable to POST to %s: %w", endpoint, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
		}

		var response *[]github.Installation
		bytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("unable to read response body: %w", err)
		}

		err = json.Unmarshal(bytes, &response)
		if err != nil {
			return nil, fmt.Errorf("unable to unmarshal response body: %w", err)
		}
		responses = append(responses, *response...)

		if len(*response) < 100 {
			break
		}
		page++

		time.Sleep(1 * time.Second)
	}

	return &responses, nil
}
