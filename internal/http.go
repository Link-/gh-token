package internal

import (
	"fmt"
	"io"
	"net/http"
	"strings"
)

// maxErrorBodyBytes caps how much of an error response body is included in
// error messages.
const maxErrorBodyBytes = 4096

// newAPIRequest builds a request to the GitHub API authenticated with the
// given bearer token (a JWT or an installation token).
func newAPIRequest(method, endpoint, bearer string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, endpoint, body)
	if err != nil {
		return nil, fmt.Errorf("unable to create %s request to %s: %w", method, endpoint, err)
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", bearer))
	req.Header.Add("Accept", "application/vnd.github+json")
	req.Header.Add("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Add("User-Agent", "Link-/gh-token")

	return req, nil
}

// unexpectedStatusError returns an error carrying the status code and up to
// maxErrorBodyBytes of the response body, which usually holds GitHub's
// explanation (for example the rejected repository or permission).
func unexpectedStatusError(resp *http.Response) error {
	b, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
	msg := strings.TrimSpace(string(b))
	if msg == "" {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return fmt.Errorf("unexpected status code: %d: %s", resp.StatusCode, msg)
}
