package internal

import (
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/google/go-github/v55/github"
	"github.com/urfave/cli/v2"
)

// Output streams, replaceable in tests.
var (
	stdout io.Writer = os.Stdout
	stderr io.Writer = os.Stderr
)

var bareRepoPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// scopeOptions holds the flags that downscope the token or change where it
// is written. None of them set means the original generate behaviour.
type scopeOptions struct {
	scopeFile          string
	repositories       []string
	permissions        []string
	mergePermissions   bool
	printRequest       bool
	verifyInstallation bool
}

func readScopeOptions(c *cli.Context) *scopeOptions {
	return &scopeOptions{
		scopeFile:          c.String("scope-file"),
		repositories:       c.StringSlice("repository"),
		permissions:        c.StringSlice("permission"),
		mergePermissions:   c.Bool("merge-permissions"),
		printRequest:       c.Bool("print-request"),
		verifyInstallation: c.Bool("verify-installation"),
	}
}

// active reports whether any of the new flags is in use.
func (o *scopeOptions) active() bool {
	return o.scopeFile != "" || len(o.repositories) > 0 || len(o.permissions) > 0 ||
		o.mergePermissions || o.printRequest || o.verifyInstallation
}

// validate checks flag combinations that don't need any I/O.
func (o *scopeOptions) validate(installationID string) error {
	if o.scopeFile != "" && len(o.repositories) > 0 {
		return fmt.Errorf("--scope-file and --repository cannot be combined; list the repositories in the scope file")
	}
	if len(o.repositories) > 0 && len(o.permissions) == 0 {
		return fmt.Errorf("--repository requires at least one --permission (name=level, read-all or write-all)")
	}
	if len(o.permissions) > 0 && o.scopeFile == "" && len(o.repositories) == 0 {
		return fmt.Errorf("--permission requires --repository or --scope-file")
	}
	if o.scopeFile != "" && len(o.permissions) > 0 {
		return fmt.Errorf("--permission cannot be combined with --scope-file; declare permissions in the scope file")
	}
	for _, r := range o.repositories {
		if !strings.Contains(r, "/") && installationID == "" {
			return fmt.Errorf("--repository %s has no owner; use owner/repo or pass --installation-id", r)
		}
	}
	if o.verifyInstallation && installationID == "" {
		return fmt.Errorf("--verify-installation requires --installation-id")
	}

	return nil
}

// parsePermissionFlags turns --permission values into Permissions: either a
// single read-all / write-all shorthand or name=level pairs.
func parsePermissionFlags(values []string) (*Permissions, error) {
	if len(values) == 0 {
		return nil, nil
	}
	if len(values) == 1 && (values[0] == shorthandReadAll || values[0] == shorthandWriteAll) {
		return &Permissions{Shorthand: values[0]}, nil
	}

	levels := map[string]string{}
	for _, v := range values {
		name, level, ok := strings.Cut(v, "=")
		if !ok {
			return nil, fmt.Errorf("invalid --permission %q: expected name=level, read-all or write-all", v)
		}
		if _, dup := levels[name]; dup {
			return nil, fmt.Errorf("--permission %s given more than once", name)
		}
		levels[name] = level
	}
	p := &Permissions{Levels: levels}
	if err := p.validate(); err != nil {
		return nil, fmt.Errorf("invalid --permission: %w", err)
	}

	return p, nil
}

// buildScope assembles the scope from a file or the ad hoc flags. It returns
// nil when no scope was requested (the token covers the whole installation).
func (o *scopeOptions) buildScope() (*ScopeFile, error) {
	flagPerms, err := parsePermissionFlags(o.permissions)
	if err != nil {
		return nil, err
	}

	switch {
	case o.scopeFile != "":
		return ReadScopeFile(o.scopeFile)
	case len(o.repositories) > 0:
		s := &ScopeFile{}
		for _, r := range o.repositories {
			var entry ScopeEntry
			if strings.Contains(r, "/") {
				entry, err = newScopeEntry(r, flagPerms, sourceRepository)
				if err != nil {
					return nil, fmt.Errorf("invalid --repository: %w", err)
				}
			} else {
				if !bareRepoPattern.MatchString(r) {
					return nil, fmt.Errorf("invalid --repository %q", r)
				}
				entry = ScopeEntry{Repo: r, Permissions: *flagPerms, Source: sourceRepository}
			}
			s.Entries = append(s.Entries, entry)
		}
		return s, nil
	default:
		return nil, nil
	}
}

// scopedGeneration carries everything needed to mint a token for the new
// flags.
type scopedGeneration struct {
	opts           *scopeOptions
	hostname       string
	appID          string
	installationID string
	jwtExpiry      int
	privateKey     *rsa.PrivateKey
	jwt            string
	tokenOnly      bool
	silent         bool
}

func (g *scopedGeneration) warn(format string, args ...any) {
	if !g.silent {
		_, _ = fmt.Fprintf(stderr, "Warning: "+format+"\n", args...)
	}
}

func (g *scopedGeneration) run() error {
	scopeFile, err := g.opts.buildScope()
	if err != nil {
		return err
	}

	var scope *ScopeRequest
	if scopeFile != nil {
		scope, err = BuildScopeRequest(scopeFile, g.opts.mergePermissions)
		if err != nil {
			return err
		}
		for _, w := range scope.Warnings {
			g.warn("%s", w)
		}
	}

	installationID, installationSource, err := g.resolveInstallation(scope)
	if err != nil {
		return err
	}

	var body *accessTokenRequest
	if scope != nil {
		if scope.ReadAll {
			perms, err := installationPermissions(g.hostname, g.jwt, installationID)
			if err != nil {
				return fmt.Errorf("failed expanding read-all: %w", err)
			}
			expandReadAll(scope, perms)
		}
		body = &accessTokenRequest{Repositories: scope.Repositories, Permissions: scope.Permissions}
	}

	if g.opts.printRequest {
		return printScopeRequest(installationID, installationSource, scope, body)
	}

	token, err := mintToken(g.hostname, g.jwt, installationID, body)
	if err != nil {
		return fmt.Errorf("failed generating installation token: %w", err)
	}
	for _, w := range verifyMintedToken(body, token.details) {
		g.warn("%s", w)
	}

	return g.printToken(token.InstallationToken)
}

// tokenDetails is decoded from the raw access token response. Unlike
// go-github's InstallationToken it keeps repository_selection and permission
// names the library doesn't know about.
type tokenDetails struct {
	ExpiresAt           time.Time         `json:"expires_at"`
	Permissions         map[string]string `json:"permissions"`
	RepositorySelection string            `json:"repository_selection"`
	Repositories        []struct {
		Name     string `json:"name"`
		FullName string `json:"full_name"`
	} `json:"repositories"`
}

// mintedToken is a token as printed (go-github's type, unchanged output)
// plus the details used for verification and the sidecar file.
type mintedToken struct {
	*github.InstallationToken
	details *tokenDetails
}

func mintToken(hostname, jwt, installationID string, body *accessTokenRequest) (*mintedToken, error) {
	raw, err := requestAccessToken(hostname, jwt, installationID, body)
	if err != nil {
		return nil, err
	}
	var token github.InstallationToken
	if err := json.Unmarshal(raw, &token); err != nil {
		return nil, fmt.Errorf("unable to unmarshal response body: %w", err)
	}
	var details tokenDetails
	if err := json.Unmarshal(raw, &details); err != nil {
		return nil, fmt.Errorf("unable to unmarshal response body: %w", err)
	}

	return &mintedToken{InstallationToken: &token, details: &details}, nil
}

// resolveInstallation returns the installation ID and a description of how
// it was found.
func (g *scopedGeneration) resolveInstallation(scope *ScopeRequest) (string, string, error) {
	if g.installationID != "" {
		if g.opts.verifyInstallation && scope != nil {
			looked, err := lookupInstallationID(g.hostname, g.jwt, scope)
			if err != nil {
				return "", "", fmt.Errorf("failed verifying installation ID: %w", err)
			}
			if looked != g.installationID {
				return "", "", fmt.Errorf("--installation-id %s does not match the installation %s found for %s", g.installationID, looked, scope.Owner)
			}
			return g.installationID, "--installation-id, verified", nil
		}
		return g.installationID, "--installation-id", nil
	}

	if scope != nil {
		id, err := lookupInstallationID(g.hostname, g.jwt, scope)
		if err != nil {
			return "", "", fmt.Errorf("failed resolving installation ID: %w", err)
		}
		return id, "looked up for " + scope.Owner, nil
	}

	id, err := retrieveDefaultInstallationID(g.hostname, g.jwt)
	if err != nil {
		return "", "", fmt.Errorf("failed retrieving default installation ID: %w", err)
	}

	return id, "first installation of the App", nil
}

func (g *scopedGeneration) printToken(token *github.InstallationToken) error {
	if g.silent {
		return nil
	}
	if g.tokenOnly {
		_, _ = fmt.Fprintln(stdout, token.GetToken())
		return nil
	}
	b, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal token to JSON: %w", err)
	}
	_, _ = fmt.Fprintln(stdout, string(b))

	return nil
}

// printScopeRequest prints what would be requested: comment lines describing
// the installation and where each repository came from, then the JSON body
// on the last line.
func printScopeRequest(installationID, installationSource string, scope *ScopeRequest, body *accessTokenRequest) error {
	_, _ = fmt.Fprintf(stdout, "# installation: %s (%s)\n", installationID, installationSource)
	if scope != nil {
		for _, e := range scope.Entries {
			_, _ = fmt.Fprintf(stdout, "# repository: %s (%s)\n", e.Name(), e.Source)
		}
	}
	if body == nil {
		body = &accessTokenRequest{}
	}
	b, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("unable to marshal request body: %w", err)
	}
	_, _ = fmt.Fprintln(stdout, string(b))

	return nil
}

// verifyMintedToken compares the minted token with the request and returns a
// warning for every difference, as a guard against silent API changes.
func verifyMintedToken(body *accessTokenRequest, token *tokenDetails) []string {
	if body.isEmpty() || token == nil {
		return nil
	}
	var warnings []string

	if body.Permissions != nil {
		got := token.Permissions
		for _, name := range sortedKeys(body.Permissions) {
			want := body.Permissions[name]
			switch have, ok := got[name]; {
			case !ok:
				warnings = append(warnings, fmt.Sprintf("requested %s=%s but the token does not have it", name, want))
			case have != want:
				warnings = append(warnings, fmt.Sprintf("requested %s=%s but the token has %s=%s", name, want, name, have))
			}
		}
		for _, name := range sortedKeys(got) {
			// GitHub always adds metadata=read.
			if _, ok := body.Permissions[name]; !ok && name != "metadata" {
				warnings = append(warnings, fmt.Sprintf("the token has %s=%s, which was not requested", name, got[name]))
			}
		}
	}

	if len(body.Repositories) > 0 {
		if token.RepositorySelection != "selected" {
			warnings = append(warnings, fmt.Sprintf(`requested specific repositories but the token's repository_selection is %q`, token.RepositorySelection))
		}
		if len(token.Repositories) > 0 {
			want := map[string]bool{}
			for _, r := range body.Repositories {
				want[strings.ToLower(r)] = true
			}
			have := map[string]bool{}
			for _, r := range token.Repositories {
				name := strings.ToLower(r.Name)
				have[name] = true
				if !want[name] {
					warnings = append(warnings, fmt.Sprintf("the token includes repository %s, which was not requested", r.Name))
				}
			}
			for _, r := range body.Repositories {
				if !have[strings.ToLower(r)] {
					warnings = append(warnings, fmt.Sprintf("requested repository %s but the token does not include it", r))
				}
			}
		}
	}

	return warnings
}
