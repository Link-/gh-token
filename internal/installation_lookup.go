package internal

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// errNotInstalled is returned when a lookup gets a 404.
var errNotInstalled = errors.New("not installed")

type installationResponse struct {
	ID          int64             `json:"id"`
	Permissions map[string]string `json:"permissions"`
	Account     struct {
		Login string `json:"login"`
	} `json:"account"`
	// redirected is true when GitHub redirected the request, as it does for
	// renamed or transferred repositories.
	redirected bool
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
	installation.redirected = resp.Request != nil && resp.Request.URL.String() != endpoint

	return &installation, nil
}

// repositoryMoved checks whether owner/repo, as the git remote names it, was
// renamed or transferred. GitHub redirects the installation lookup for a
// moved repository, and the installation's account is its current owner.
// It returns the current owner (unchanged when not moved) and a note for
// the user, or empty values when the lookup fails; the installation lookup
// later reports real problems.
func repositoryMoved(hostname, jwt, owner, repo string) (string, string) {
	installation, err := getInstallation(hostname, jwt, fmt.Sprintf("/repos/%s/%s/installation", owner, repo))
	if err != nil || !installation.redirected {
		return owner, ""
	}
	current := installation.Account.Login
	if current == "" || strings.EqualFold(current, owner) {
		return owner, fmt.Sprintf("GitHub redirects %s/%s, so the repository was probably renamed; update the remote with git remote set-url", owner, repo)
	}
	return current, fmt.Sprintf("%s/%s has moved to the %s account; using %s/%s. Update the remote with git remote set-url", owner, repo, current, current, repo)
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
