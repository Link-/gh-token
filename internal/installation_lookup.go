package internal

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
)

// errNotInstalled is returned when a lookup gets a 404.
var errNotInstalled = errors.New("not installed")

type installationResponse struct {
	ID          int64             `json:"id"`
	Permissions map[string]string `json:"permissions"`
}

// getInstallation GETs an installation resource authenticated with the JWT.
// A 404 is reported as errNotInstalled.
func getInstallation(hostname, jwt, path string) (*installationResponse, error) {
	endpoint := fmt.Sprintf("https://%s%s", hostname, path)
	req, err := newAPIRequest("GET", endpoint, jwt, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("unable to GET %s: %w", endpoint, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("%w: %w", errNotInstalled, unexpectedStatusError(resp))
	}
	if resp.StatusCode != http.StatusOK {
		return nil, unexpectedStatusError(resp)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("unable to read response body: %w", err)
	}
	var installation installationResponse
	if err := json.Unmarshal(b, &installation); err != nil {
		return nil, fmt.Errorf("unable to unmarshal response body: %w", err)
	}

	return &installation, nil
}

// lookupInstallationID finds the App's installation for the scope's owner:
// through the first concrete repository if there is one, otherwise through
// the organization, falling back to the user account.
func lookupInstallationID(hostname, jwt string, scope *ScopeRequest) (string, error) {
	if scope.Owner == "" {
		return "", fmt.Errorf("cannot look up the installation without an owner; use owner/repo or pass --installation-id")
	}

	for _, e := range scope.Entries {
		if e.Owner == "" || e.Repo == "*" {
			continue
		}
		installation, err := getInstallation(hostname, jwt, fmt.Sprintf("/repos/%s/%s/installation", e.Owner, e.Repo))
		if errors.Is(err, errNotInstalled) {
			return "", fmt.Errorf("the GitHub App is not installed on %s, or the repository is not among the installation's selected repositories; check the installation's repository access (%w)", e.Name(), err)
		}
		if err != nil {
			return "", fmt.Errorf("unable to look up the installation for %s: %w", e.Name(), err)
		}
		return strconv.FormatInt(installation.ID, 10), nil
	}

	installation, err := getInstallation(hostname, jwt, fmt.Sprintf("/orgs/%s/installation", scope.Owner))
	if errors.Is(err, errNotInstalled) {
		installation, err = getInstallation(hostname, jwt, fmt.Sprintf("/users/%s/installation", scope.Owner))
	}
	if errors.Is(err, errNotInstalled) {
		return "", fmt.Errorf("the GitHub App is not installed on the %s organization or user account (%w)", scope.Owner, err)
	}
	if err != nil {
		return "", fmt.Errorf("unable to look up the installation for %s: %w", scope.Owner, err)
	}

	return strconv.FormatInt(installation.ID, 10), nil
}

// installationPermissions returns the permissions granted to an installation.
func installationPermissions(hostname, jwt, installationID string) (map[string]string, error) {
	installation, err := getInstallation(hostname, jwt, fmt.Sprintf("/app/installations/%s", installationID))
	if err != nil {
		return nil, fmt.Errorf("unable to read installation %s: %w", installationID, err)
	}

	return installation.Permissions, nil
}

// writeOnlyPermissions have no read level, so read-all cannot include them.
var writeOnlyPermissions = map[string]bool{"workflows": true}

// expandReadAll turns read-all into an explicit map: every permission the
// installation has, at read level, unless an explicit (merged) permission
// already asks for more.
func expandReadAll(scope *ScopeRequest, installationPerms map[string]string) {
	if !scope.ReadAll {
		return
	}
	expanded := map[string]string{}
	for name := range installationPerms {
		if !writeOnlyPermissions[name] {
			expanded[name] = "read"
		}
	}
	for name, level := range scope.Permissions {
		expanded[name] = level
	}
	scope.Permissions = expanded
	scope.ReadAll = false
}
