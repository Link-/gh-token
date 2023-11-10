package internal

import (
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/go-github/v55/github"
	"github.com/urfave/cli/v2"
)

// Generate is the entrypoint for the generate command
func Generate(c *cli.Context) error {
	appID := c.String("app-id")
	installationID := c.String("installation-id")
	keyPath := c.String("key")
	keyBase64 := c.String("base64-key")
	printJWT := c.Bool("jwt")
	jwtExpiry := c.Int("jwt-expiry")
	hostname := strings.ToLower(c.String("hostname"))
	tokenOnly := c.Bool("token-only")
	silent := c.Bool("silent")

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

	if jwtExpiry < 1 || jwtExpiry > 10 {
		jwtExpiry = 10
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

	jsonWebToken, err := generateJWT(appID, jwtExpiry, privateKey)
	if err != nil {
		return fmt.Errorf("failed generating JWT: %w", err)
	}

	if printJWT {
		if !silent {
			fmt.Println(jsonWebToken)
		}

		return nil
	}

	if installationID == "" {
		installationID, err = retrieveDefaultInstallationID(hostname, jsonWebToken)
		if err != nil {
			return fmt.Errorf("failed retrieving default installation ID: %w", err)
		}
	}

	token, err := generateToken(hostname, jsonWebToken, installationID)
	if err != nil {
		return fmt.Errorf("failed generating installation token: %w", err)
	}

	if !silent {
		bytes, err := json.MarshalIndent(token, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal token to JSON: %w", err)
		}

		if tokenOnly {
			fmt.Println(*token.Token)
		} else {
			fmt.Println(string(bytes))
		}
	}

	return nil
}

func retrieveDefaultInstallationID(hostname, jwt string) (string, error) {
	endpoint := fmt.Sprintf("https://%s/app/installations?per_page=1", hostname)
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("unable to create GET request to %s: %w", endpoint, err)
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", jwt))
	req.Header.Add("Accept", "application/vnd.github+json")
	req.Header.Add("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Add("User-Agent", "Link-/gh-token")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("unable to POST to %s: %w", endpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var response []github.Installation
	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("unable to read response body: %w", err)
	}

	err = json.Unmarshal(bytes, &response)
	if err != nil {
		return "", fmt.Errorf("unable to unmarshal response body: %w", err)
	}

	return strconv.FormatInt(*response[0].ID, 10), nil
}

func generateToken(hostname, jwt, installationID string) (*github.InstallationToken, error) {
	endpoint := fmt.Sprintf("https://%s/app/installations/%s/access_tokens", hostname, installationID)
	req, err := http.NewRequest("POST", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to create POST request to %s: %w", endpoint, err)
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

	if resp.StatusCode != 201 {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var response *github.InstallationToken
	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("unable to read response body: %w", err)
	}

	err = json.Unmarshal(bytes, &response)
	if err != nil {
		return nil, fmt.Errorf("unable to unmarshal response body: %w", err)
	}

	return response, nil
}
