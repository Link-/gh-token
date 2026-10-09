package internal

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"regexp"
	"strings"
)

// scpLikeRemote matches git's scp-like syntax: [user@]host:owner/repo
var scpLikeRemote = regexp.MustCompile(`^(?:[^@/]+@)?([^:/]+):(.+)$`)

// parseGitRemoteURL extracts host, owner and repository from a git remote
// URL: https://host/owner/repo(.git), git@host:owner/repo(.git) or
// ssh://git@host(:port)/owner/repo(.git). The host is lowercased and has no
// port.
func parseGitRemoteURL(raw string) (host, owner, repo string, err error) {
	raw = strings.TrimSpace(raw)
	var path string

	if strings.Contains(raw, "://") {
		u, err := url.Parse(raw)
		if err != nil {
			return "", "", "", fmt.Errorf("unable to parse git remote URL %q: %w", raw, err)
		}
		switch u.Scheme {
		case "https", "http", "ssh", "git", "git+ssh", "ssh+git":
		default:
			return "", "", "", fmt.Errorf("unsupported git remote URL %q: scheme %q", raw, u.Scheme)
		}
		host, path = u.Hostname(), u.Path
	} else if m := scpLikeRemote.FindStringSubmatch(raw); m != nil {
		host, path = m[1], m[2]
	} else {
		return "", "", "", fmt.Errorf("unsupported git remote URL %q", raw)
	}

	path = strings.TrimSuffix(strings.Trim(path, "/"), ".git")
	parts := strings.Split(path, "/")
	if host == "" || len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", "", fmt.Errorf("git remote URL %q is not of the form host/owner/repo", raw)
	}

	host = strings.ToLower(host)
	// GitHub's SSH-over-HTTPS-port endpoint serves the same repositories.
	if host == "ssh.github.com" {
		host = "github.com"
	}

	return host, parts[0], parts[1], nil
}

// webHostForAPI returns the git host matching an API hostname as normalised
// by Generate: api.github.com -> github.com, ghes.example.com/api/v3 ->
// ghes.example.com. Ports are dropped, as in parseGitRemoteURL.
func webHostForAPI(apiHostname string) string {
	if apiHostname == "api.github.com" {
		return "github.com"
	}
	host := strings.TrimSuffix(strings.TrimSuffix(apiHostname, "/"), "/api/v3")
	if u, err := url.Parse("https://" + host); err == nil && u.Hostname() != "" {
		return strings.ToLower(u.Hostname())
	}
	return host
}

// gitRemoteURL returns the URL of a remote of the clone at gitDir. It shells
// out to git so worktrees, includeIf and insteadOf are honoured.
func gitRemoteURL(gitDir, remote string) (string, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return "", fmt.Errorf("git is not installed or not on PATH; it is needed to find the current repository (or pass --current-repo=false)")
	}

	cmd := exec.Command("git", "-C", gitDir, "remote", "get-url", remote)
	var errOut bytes.Buffer
	cmd.Stderr = &errOut
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(errOut.String())
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && msg != "" {
			return "", fmt.Errorf("unable to read git remote %q in %s: %s", remote, gitDir, msg)
		}
		return "", fmt.Errorf("unable to read git remote %q in %s: %w", remote, gitDir, err)
	}

	return strings.TrimSpace(string(out)), nil
}

// currentRepository resolves owner/repo of the clone at gitDir from its
// remote, checking that the remote's host matches the API host.
func currentRepository(apiHostname, gitDir, remote string) (owner, repo string, err error) {
	raw, err := gitRemoteURL(gitDir, remote)
	if err != nil {
		return "", "", err
	}
	host, owner, repo, err := parseGitRemoteURL(raw)
	if err != nil {
		return "", "", err
	}
	if want := webHostForAPI(apiHostname); host != want {
		return "", "", fmt.Errorf("git remote %q points at %s, but tokens are generated for %s; pass --hostname or --git-remote, or --current-repo=false", remote, host, want)
	}

	return owner, repo, nil
}
