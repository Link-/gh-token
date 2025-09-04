package internal

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/urfave/cli/v2"
)

// Revoke is the entrypoint for the revoke command
func Revoke(c *cli.Context) error {
	token := c.String("token")
	hostname := strings.ToLower(c.String("hostname"))
	silent := c.Bool("silent")

	if hostname != "api.github.com" && !strings.Contains(hostname, "/api/v3") {
		endpoint := fmt.Sprintf("%s/api/v3", hostname)
		hostname = strings.TrimSuffix(endpoint, "/")
	}

	err := revokeToken(hostname, token)
	if err != nil {
		return fmt.Errorf("failed revoking installation token: %w", err)
	}
	if !silent {
		fmt.Println("Successfully revoked installation token")
	}

	return nil
}

func revokeToken(hostname, token string) error {
	endpoint := fmt.Sprintf("https://%s/installation/token", hostname)
	req, err := http.NewRequest("DELETE", endpoint, nil)
	if err != nil {
		return fmt.Errorf("unable to create DELETE request to %s: %w", endpoint, err)
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Add("Accept", "application/vnd.github+json")
	req.Header.Add("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Add("User-Agent", "Link-/gh-token")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("unable to DELETE to %s: %w", endpoint, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != 204 {
		return fmt.Errorf("token might be invalid or not properly formatted. Unexpected status code: %d", resp.StatusCode)
	}

	return nil
}
