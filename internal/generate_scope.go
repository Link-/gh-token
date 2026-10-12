package internal

import (
	"crypto/rsa"
	"crypto/sha256"
	"encoding/hex"
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

const (
	defaultRefreshBefore = 10 * time.Minute
	maxRefreshBefore     = 50 * time.Minute
)

// scopeOptions holds the flags that downscope the token or change where it
// is written. None of them set means the original generate behaviour.
type scopeOptions struct {
	scopeFile          string
	repositories       []string
	permissions        []string
	mergePermissions   bool
	printRequest       bool
	verifyInstallation bool
	currentRepo        bool
	currentRepoSet     bool
	gitDir             string
	gitRemote          string
	outputFile         string
	refresh            bool
	refreshBefore      time.Duration
	revokeOnExit       bool
	pidFile            string
}

func readScopeOptions(c *cli.Context) *scopeOptions {
	return &scopeOptions{
		scopeFile:          c.String("scope-file"),
		repositories:       c.StringSlice("repository"),
		permissions:        c.StringSlice("permission"),
		mergePermissions:   c.Bool("merge-permissions"),
		printRequest:       c.Bool("print-request"),
		verifyInstallation: c.Bool("verify-installation"),
		currentRepo:        c.Bool("current-repo"),
		currentRepoSet:     c.IsSet("current-repo"),
		gitDir:             c.String("git-dir"),
		gitRemote:          c.String("git-remote"),
		outputFile:         c.String("output-file"),
		refresh:            c.Bool("refresh"),
		refreshBefore:      c.Duration("refresh-before"),
		revokeOnExit:       c.Bool("revoke-on-exit"),
		pidFile:            c.String("pid-file"),
	}
}

// active reports whether any of the new flags is in use.
func (o *scopeOptions) active() bool {
	return o.scopeFile != "" || len(o.repositories) > 0 || len(o.permissions) > 0 ||
		o.mergePermissions || o.printRequest || o.verifyInstallation || o.currentRepo ||
		o.outputFile != "" || o.refresh || o.revokeOnExit || o.pidFile != ""
}

// validate checks flag combinations that don't need any I/O.
func (o *scopeOptions) validate(installationID string, tokenOnly bool) error {
	if o.refreshBefore == 0 {
		o.refreshBefore = defaultRefreshBefore
	}
	if o.refresh && o.outputFile == "" {
		return fmt.Errorf("--refresh requires --output-file")
	}
	if (o.revokeOnExit || o.pidFile != "") && !o.refresh {
		return fmt.Errorf("--revoke-on-exit and --pid-file require --refresh")
	}
	if o.refreshBefore < 0 || o.refreshBefore > maxRefreshBefore {
		return fmt.Errorf("--refresh-before must be between 0 and %s; tokens live for one hour", maxRefreshBefore)
	}
	if o.outputFile != "" && tokenOnly {
		return fmt.Errorf("--output-file cannot be combined with --token-only; the file always holds just the token")
	}
	if o.scopeFile != "" && len(o.repositories) > 0 {
		return fmt.Errorf("--scope-file and --repository cannot be combined; list the repositories in the scope file")
	}
	if len(o.repositories) > 0 && len(o.permissions) == 0 {
		return fmt.Errorf("--repository requires at least one --permission (name=level, read-all or write-all)")
	}
	if len(o.permissions) > 0 && o.scopeFile == "" && len(o.repositories) == 0 && !o.currentRepo {
		return fmt.Errorf("--permission requires --repository, --current-repo or --scope-file")
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

// buildScope assembles the scope from a file or the ad hoc flags, adding the
// clone's own repository when --current-repo is in effect. It returns nil
// when no scope was requested (the token covers the whole installation).
func (o *scopeOptions) buildScope(apiHostname string) (*ScopeFile, error) {
	flagPerms, err := parsePermissionFlags(o.permissions)
	if err != nil {
		return nil, err
	}

	var s *ScopeFile
	switch {
	case o.scopeFile != "":
		s, err = ReadScopeFile(o.scopeFile)
		if err != nil {
			return nil, err
		}
	case len(o.repositories) > 0:
		s = &ScopeFile{}
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
	case o.currentRepo:
		s = &ScopeFile{}
	default:
		return nil, nil
	}

	// Like Codespaces, a devcontainer file implies the clone's own repo.
	currentRepo := o.currentRepo
	if !o.currentRepoSet && o.scopeFile != "" {
		currentRepo = s.Devcontainer
	}

	if !currentRepo {
		if o.scopeFile != "" && flagPerms != nil {
			return nil, fmt.Errorf("--permission with --scope-file only sets the current repository's permissions, but --current-repo is off")
		}
		return s, nil
	}

	owner, repo, err := currentRepository(apiHostname, o.gitDir, o.gitRemote)
	if err != nil {
		return nil, fmt.Errorf("failed resolving the current repository: %w", err)
	}
	source := fmt.Sprintf("git remote %q", o.gitRemote)

	// With ad hoc flags the --permission set applies to every repository.
	var fileFlagPerms *Permissions
	if o.scopeFile != "" {
		fileFlagPerms = flagPerms
	}
	if err := addCurrentRepository(s, owner, repo, source, fileFlagPerms, flagPerms); err != nil {
		return nil, err
	}

	return s, nil
}

// addCurrentRepository adds the clone's repository to the scope. Its
// permissions come from, in order: an explicit entry in the scope,
// customizations.gh-token.currentRepository, --permission flags, or an
// owner/* entry covering it. There is deliberately no default: an implicit
// default is how scopes silently grow.
//
// conflictPerms is --permission given together with a scope file, which may
// only supply permissions the file doesn't declare; perms is the
// --permission set to use for the current repository.
func addCurrentRepository(s *ScopeFile, owner, repo, source string, conflictPerms, perms *Permissions) error {
	name := owner + "/" + repo

	for i, e := range s.Entries {
		if strings.EqualFold(e.Owner, owner) && strings.EqualFold(e.Repo, repo) {
			if conflictPerms != nil {
				return fmt.Errorf("--permission conflicts with the scope file, which already declares permissions for the current repository %s", name)
			}
			s.Entries[i].Source = fmt.Sprintf("%s; current repository from %s", e.Source, source)
			return nil
		}
	}

	entry := ScopeEntry{Owner: owner, Repo: repo, Source: "current repository from " + source}
	switch {
	case s.CurrentRepository != nil:
		if conflictPerms != nil {
			return fmt.Errorf("--permission conflicts with customizations.gh-token.currentRepository in the scope file")
		}
		entry.Permissions = *s.CurrentRepository
	case perms != nil:
		entry.Permissions = *perms
	default:
		for i, e := range s.Entries {
			if e.Repo == "*" && strings.EqualFold(e.Owner, owner) {
				s.Entries[i].Source = fmt.Sprintf("%s; covers the current repository %s from %s", e.Source, name, source)
				return nil
			}
		}
		return fmt.Errorf("no permissions declared for current repository %s (from %s); add customizations.gh-token.currentRepository or an entry under repositories", name, source)
	}

	s.Entries = append(s.Entries, entry)

	return nil
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

func (g *scopedGeneration) log(format string, args ...any) {
	if !g.silent {
		_, _ = fmt.Fprintf(stderr, "%s "+format+"\n", append([]any{time.Now().UTC().Format(time.RFC3339)}, args...)...)
	}
}

func (g *scopedGeneration) run() error {
	scopeFile, err := g.opts.buildScope(g.hostname)
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

	if g.opts.refresh && g.opts.pidFile != "" && !g.opts.printRequest {
		running, release, err := acquirePIDFile(g.opts.pidFile, g.scopeHash(scope))
		if err != nil {
			return err
		}
		if running {
			g.log("a refresher with the same scope is already running for %s; nothing to do", g.opts.pidFile)
			return nil
		}
		defer release()
	}

	installationID, installationSource, err := g.resolveInstallation(scope)
	if err != nil {
		return err
	}

	var body *accessTokenRequest
	var granted map[string]string
	var missing []string
	if scope != nil {
		if scope.ReadAll || scope.Permissions != nil {
			granted, err = installationPermissions(g.hostname, g.jwt, installationID)
			if err != nil {
				if scope.ReadAll {
					return fmt.Errorf("failed expanding read-all: %w", err)
				}
				g.warn("could not compare the requested permissions with the installation's: %v", err)
			}
		}
		if scope.ReadAll {
			expandReadAll(scope, granted)
		}
		body = &accessTokenRequest{Repositories: scope.Repositories, Permissions: scope.Permissions}
		if granted != nil {
			missing = missingPermissions(body.Permissions, granted)
		}
	}
	missingErr := missingPermissionsError(installationID, missing)

	if g.opts.printRequest {
		if err := printScopeRequest(installationID, installationSource, scope, body, granted, missing); err != nil {
			return err
		}
		return missingErr
	}
	// GitHub would answer 422 without saying which permission is missing.
	if missingErr != nil {
		return missingErr
	}

	token, err := mintToken(g.hostname, g.jwt, installationID, body)
	if err != nil {
		return fmt.Errorf("failed generating installation token: %w", err)
	}
	for _, w := range verifyMintedToken(body, token.details) {
		g.warn("%s", w)
	}

	if g.opts.outputFile != "" {
		if err := writeTokenFiles(g.opts.outputFile, token); err != nil {
			return err
		}
		g.log("token written to %s, expires at %s", g.opts.outputFile, token.GetExpiresAt().UTC().Format(time.RFC3339))
		if g.opts.refresh {
			return g.refreshLoop(installationID, body, token)
		}
		return nil
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

// refreshLoop keeps the output file fresh until interrupted. body is fixed
// here: the scope file and git remote are not re-read, so changes inside the
// workspace cannot widen later tokens.
func (g *scopedGeneration) refreshLoop(installationID string, body *accessTokenRequest, first *mintedToken) error {
	ctx, cancel := newRefreshContext()
	defer cancel()

	r := &refresher{
		mint: func() (*mintedToken, error) {
			jwt, err := generateJWT(g.appID, g.jwtExpiry, g.privateKey)
			if err != nil {
				return nil, fmt.Errorf("failed generating JWT: %w", err)
			}
			return mintToken(g.hostname, jwt, installationID, body)
		},
		write:         func(t *mintedToken) error { return writeTokenFiles(g.opts.outputFile, t) },
		now:           refreshNow,
		wait:          refreshWait,
		jitter:        randomJitter,
		refreshBefore: g.opts.refreshBefore,
		log:           g.log,
	}
	if g.opts.revokeOnExit {
		r.revoke = func(token string) error { return revokeToken(g.hostname, token) }
	}
	g.log("refreshing %s %s before expiry; stop with SIGINT or SIGTERM", g.opts.outputFile, g.opts.refreshBefore)

	return r.run(ctx, first)
}

// scopeHash identifies what a refresher mints, so a second refresher for
// the same pid file can tell whether the running one is equivalent.
func (g *scopedGeneration) scopeHash(scope *ScopeRequest) string {
	type hashed struct {
		Hostname       string
		AppID          string
		InstallationID string
		OutputFile     string
		Owner          string
		Repositories   []string
		Permissions    map[string]string
		ReadAll        bool
	}
	h := hashed{Hostname: g.hostname, AppID: g.appID, InstallationID: g.installationID, OutputFile: g.opts.outputFile}
	if scope != nil {
		h.Owner, h.Repositories, h.Permissions, h.ReadAll = strings.ToLower(scope.Owner), scope.Repositories, scope.Permissions, scope.ReadAll
	}
	b, _ := json.Marshal(h)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
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
func printScopeRequest(installationID, installationSource string, scope *ScopeRequest, body *accessTokenRequest, granted map[string]string, missing []string) error {
	_, _ = fmt.Fprintf(stdout, "# installation: %s (%s)\n", installationID, installationSource)
	if scope != nil {
		for _, e := range scope.Entries {
			_, _ = fmt.Fprintf(stdout, "# repository: %s (%s)\n", e.Name(), e.Source)
		}
	}
	if granted != nil {
		_, _ = fmt.Fprintf(stdout, "# granted: %s\n", Permissions{Levels: granted})
	}
	for _, m := range missing {
		_, _ = fmt.Fprintf(stdout, "# missing: %s\n", m)
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

// missingPermissions lists every requested permission the installation
// doesn't have, or has at a lower level.
func missingPermissions(requested, granted map[string]string) []string {
	var missing []string
	for _, name := range sortedKeys(requested) {
		want := requested[name]
		have, ok := granted[name]
		switch {
		case !ok:
			missing = append(missing, fmt.Sprintf("%s: requested %s, the installation has none", name, want))
		case permissionLevelRank[have] < permissionLevelRank[want]:
			missing = append(missing, fmt.Sprintf("%s: requested %s, the installation has %s", name, want, have))
		}
	}
	return missing
}

func missingPermissionsError(installationID string, missing []string) error {
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf("the scope asks for permissions installation %s doesn't have:\n  %s\nadd them to the App's permissions; the installation owner must then accept the change",
		installationID, strings.Join(missing, "\n  "))
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
