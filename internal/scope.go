package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/tailscale/hujson"
)

// Permission shorthands supported by Codespaces.
const (
	shorthandReadAll  = "read-all"
	shorthandWriteAll = "write-all"
)

// Sources describing where a scope entry came from, used in messages.
const (
	sourceScopeFile  = "scope file"
	sourceRepository = "--repository flag"
)

var (
	scopeKeyPattern       = regexp.MustCompile(`^[A-Za-z0-9-]+/([A-Za-z0-9._-]+|\*)$`)
	permissionNamePattern = regexp.MustCompile(`^[a-z_]+$`)
	permissionLevelRank   = map[string]int{"read": 1, "write": 2, "admin": 3}
)

// Permissions is either a Codespaces shorthand ("read-all" or "write-all")
// or an explicit map of permission name to level.
type Permissions struct {
	Shorthand string
	Levels    map[string]string
}

// UnmarshalJSON accepts either a shorthand string or an object.
func (p *Permissions) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) > 0 && data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		*p = Permissions{Shorthand: s}
		return nil
	}

	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		return fmt.Errorf(`permissions must be "read-all", "write-all" or an object of permission name to "read", "write" or "admin"`)
	}
	*p = Permissions{Levels: m}

	return nil
}

func (p Permissions) validate() error {
	if p.Shorthand != "" {
		if p.Shorthand != shorthandReadAll && p.Shorthand != shorthandWriteAll {
			return fmt.Errorf(`invalid permissions shorthand %q: expected "read-all" or "write-all"`, p.Shorthand)
		}
		return nil
	}
	if len(p.Levels) == 0 {
		return fmt.Errorf("no permissions declared")
	}
	for _, name := range sortedKeys(p.Levels) {
		if !permissionNamePattern.MatchString(name) {
			return fmt.Errorf("invalid permission name %q: expected lowercase letters and underscores", name)
		}
		if _, ok := permissionLevelRank[p.Levels[name]]; !ok {
			return fmt.Errorf(`invalid level %q for permission %q: expected "read", "write" or "admin"`, p.Levels[name], name)
		}
	}

	return nil
}

// String renders the permissions in a stable form, also used to compare them.
func (p Permissions) String() string {
	if p.Shorthand != "" {
		return p.Shorthand
	}
	parts := make([]string, 0, len(p.Levels))
	for _, name := range sortedKeys(p.Levels) {
		parts = append(parts, name+"="+p.Levels[name])
	}

	return strings.Join(parts, ", ")
}

// ScopeEntry is one repository (or owner/*) with its declared permissions.
type ScopeEntry struct {
	Owner       string // as written; empty for bare repository names
	Repo        string // as written, or "*"
	Permissions Permissions
	Source      string // where the entry came from, for messages
}

// Name returns owner/repo, or the bare repository name when there is no owner.
func (e ScopeEntry) Name() string {
	if e.Owner == "" {
		return e.Repo
	}
	return e.Owner + "/" + e.Repo
}

func (e ScopeEntry) describe() string {
	if e.Source == "" || e.Source == sourceScopeFile {
		return e.Name()
	}
	return fmt.Sprintf("%s (from %s)", e.Name(), e.Source)
}

// ScopeFile is a parsed scope or devcontainer file, or the equivalent built
// from command line flags.
type ScopeFile struct {
	Path string
	// Devcontainer is true when the file has a top-level "customizations" key.
	Devcontainer bool
	Entries      []ScopeEntry
	// CurrentRepository holds customizations.gh-token.currentRepository.permissions.
	CurrentRepository *Permissions
}

type rawRepoScope struct {
	Permissions *Permissions `json:"permissions"`
}

type rawScopeFile struct {
	Repositories   *map[string]rawRepoScope `json:"repositories"`
	Customizations *struct {
		Codespaces *struct {
			Repositories *map[string]rawRepoScope `json:"repositories"`
		} `json:"codespaces"`
		GhToken *struct {
			CurrentRepository *rawRepoScope `json:"currentRepository"`
		} `json:"gh-token"`
	} `json:"customizations"`
}

// ReadScopeFile reads and parses a scope file or devcontainer.json.
func ReadScopeFile(path string) (*ScopeFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("unable to read scope file: %w", err)
	}

	return ParseScopeFile(path, data)
}

// ParseScopeFile parses a plain scope file ({"repositories": ...}) or a
// devcontainer.json (customizations.codespaces.repositories). Comments and
// trailing commas are allowed.
func ParseScopeFile(path string, data []byte) (*ScopeFile, error) {
	std, err := hujson.Standardize(data)
	if err != nil {
		return nil, fmt.Errorf("unable to parse scope file %s: %w", path, err)
	}

	var raw rawScopeFile
	if err := json.Unmarshal(std, &raw); err != nil {
		return nil, fmt.Errorf("unable to parse scope file %s: %w", path, err)
	}

	s := &ScopeFile{Path: path, Devcontainer: raw.Customizations != nil}

	var repos *map[string]rawRepoScope
	if raw.Customizations != nil {
		if raw.Customizations.Codespaces != nil {
			repos = raw.Customizations.Codespaces.Repositories
		}
		if gt := raw.Customizations.GhToken; gt != nil && gt.CurrentRepository != nil {
			if gt.CurrentRepository.Permissions == nil {
				return nil, fmt.Errorf("scope file %s: customizations.gh-token.currentRepository has no permissions", path)
			}
			if err := gt.CurrentRepository.Permissions.validate(); err != nil {
				return nil, fmt.Errorf("scope file %s: customizations.gh-token.currentRepository: %w", path, err)
			}
			s.CurrentRepository = gt.CurrentRepository.Permissions
		}
	} else {
		repos = raw.Repositories
	}

	if repos == nil && s.CurrentRepository == nil {
		return nil, fmt.Errorf(`scope file %s has no "repositories" or "customizations.codespaces.repositories"`, path)
	}

	if repos != nil {
		for _, key := range sortedKeys(*repos) {
			entry, err := newScopeEntry(key, (*repos)[key].Permissions, sourceScopeFile)
			if err != nil {
				return nil, fmt.Errorf("scope file %s: %w", path, err)
			}
			s.Entries = append(s.Entries, entry)
		}
	}

	return s, nil
}

func newScopeEntry(key string, perms *Permissions, source string) (ScopeEntry, error) {
	if !scopeKeyPattern.MatchString(key) {
		return ScopeEntry{}, fmt.Errorf("invalid repository %q: expected owner/repo or owner/*", key)
	}
	if perms == nil {
		return ScopeEntry{}, fmt.Errorf("repository %q: no permissions declared", key)
	}
	if err := perms.validate(); err != nil {
		return ScopeEntry{}, fmt.Errorf("repository %q: %w", key, err)
	}
	owner, repo, _ := strings.Cut(key, "/")

	return ScopeEntry{Owner: owner, Repo: repo, Permissions: *perms, Source: source}, nil
}

// ScopeRequest is a scope translated into a single access token request.
type ScopeRequest struct {
	Owner        string            // single owner for installation lookup; empty for bare names
	Repositories []string          // bare repo names; empty means all repos in installation
	Permissions  map[string]string // nil means inherit installation permissions (unless ReadAll)
	ReadAll      bool              // needs installation lookup to expand
	Warnings     []string          // printed to stderr unless --silent
	Entries      []ScopeEntry      // the entries the request was built from
}

// BuildScopeRequest translates a scope into one access token request. It
// fails rather than silently widening access: tokens belong to one
// installation (one owner) and carry one permission set.
func BuildScopeRequest(s *ScopeFile, mergePermissions bool) (*ScopeRequest, error) {
	if s == nil || len(s.Entries) == 0 {
		return nil, fmt.Errorf("no repositories in scope")
	}

	req := &ScopeRequest{Entries: s.Entries}

	// One owner per token.
	owners := map[string][]string{}
	var ownerOrder []string
	for _, e := range s.Entries {
		if e.Owner == "" {
			continue
		}
		key := strings.ToLower(e.Owner)
		if _, ok := owners[key]; !ok {
			ownerOrder = append(ownerOrder, key)
			req.Owner = e.Owner
		}
		owners[key] = append(owners[key], e.describe())
	}
	if len(owners) > 1 {
		lines := make([]string, 0, len(ownerOrder))
		for _, o := range ownerOrder {
			lines = append(lines, fmt.Sprintf("  %s: %s", o, strings.Join(owners[o], ", ")))
		}
		return nil, fmt.Errorf("repositories span multiple owners:\n%s\na token belongs to a single installation, so all repositories must share one owner; split the scope file or run once per owner",
			strings.Join(lines, "\n"))
	}

	// Repositories, honouring owner/* and rejecting duplicates.
	wildcard := false
	seen := map[string]bool{}
	for _, e := range s.Entries {
		if e.Repo == "*" {
			wildcard = true
			continue
		}
		key := strings.ToLower(e.Name())
		if seen[key] {
			return nil, fmt.Errorf("repository %s is listed more than once", e.Name())
		}
		seen[key] = true
	}
	if wildcard {
		for _, e := range s.Entries {
			if e.Repo != "*" {
				req.Warnings = append(req.Warnings, fmt.Sprintf("%s/* already includes %s; it only contributes permissions", e.Owner, e.describe()))
			}
		}
	} else {
		for _, e := range s.Entries {
			req.Repositories = append(req.Repositories, e.Repo)
		}
	}

	// One permission set per token.
	identical := true
	for _, e := range s.Entries[1:] {
		if e.Permissions.String() != s.Entries[0].Permissions.String() {
			identical = false
			break
		}
	}

	if identical {
		applyPermissions(req, s.Entries[0].Permissions)
		return req, nil
	}

	if !mergePermissions {
		lines := make([]string, 0, len(s.Entries))
		for _, e := range s.Entries {
			lines = append(lines, fmt.Sprintf("  %s: %s", e.describe(), e.Permissions))
		}
		return nil, fmt.Errorf("repositories declare different permissions:\n%s\na token carries one permission set for all its repositories, so merging would give every repository the union of these permissions; align them, split the scope, or pass --merge-permissions to accept the wider access",
			strings.Join(lines, "\n"))
	}

	mergeScopePermissions(req, s.Entries)

	return req, nil
}

func applyPermissions(req *ScopeRequest, p Permissions) {
	switch p.Shorthand {
	case shorthandWriteAll:
		req.Permissions = nil
	case shorthandReadAll:
		req.ReadAll = true
	default:
		req.Permissions = copyLevels(p.Levels)
	}
}

// mergeScopePermissions takes the union of the entries' permissions, highest
// level winning, and warns about every repository that receives more than it
// declared.
func mergeScopePermissions(req *ScopeRequest, entries []ScopeEntry) {
	for _, e := range entries {
		if e.Permissions.Shorthand == shorthandWriteAll {
			req.Permissions = nil
			req.ReadAll = false
			for _, other := range entries {
				if other.Permissions.Shorthand != shorthandWriteAll {
					req.Warnings = append(req.Warnings, fmt.Sprintf("--merge-permissions: %s receives all installation permissions (write-all) instead of %s", other.describe(), other.Permissions))
				}
			}
			return
		}
	}

	merged := map[string]string{}
	for _, e := range entries {
		if e.Permissions.Shorthand == shorthandReadAll {
			req.ReadAll = true
			continue
		}
		for name, level := range e.Permissions.Levels {
			if permissionLevelRank[level] > permissionLevelRank[merged[name]] {
				merged[name] = level
			}
		}
	}
	if len(merged) > 0 {
		req.Permissions = merged
	}

	for _, e := range entries {
		var extra []string
		if e.Permissions.Shorthand == shorthandReadAll {
			for _, name := range sortedKeys(merged) {
				if permissionLevelRank[merged[name]] > permissionLevelRank["read"] {
					extra = append(extra, fmt.Sprintf("%s=%s (declared read)", name, merged[name]))
				}
			}
		} else {
			if req.ReadAll {
				extra = append(extra, "read access to all installation permissions (read-all)")
			}
			for _, name := range sortedKeys(merged) {
				declared, ok := e.Permissions.Levels[name]
				switch {
				case !ok:
					extra = append(extra, fmt.Sprintf("%s=%s", name, merged[name]))
				case declared != merged[name]:
					extra = append(extra, fmt.Sprintf("%s=%s (declared %s)", name, merged[name], declared))
				}
			}
		}
		if len(extra) > 0 {
			req.Warnings = append(req.Warnings, fmt.Sprintf("--merge-permissions: %s receives permissions it did not declare: %s", e.describe(), strings.Join(extra, ", ")))
		}
	}
}

func copyLevels(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
