package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadScopeFile(t *testing.T) {
	t.Run("plain", func(t *testing.T) {
		s, err := ReadScopeFile("fixtures/scope-plain.json")
		require.NoError(t, err)
		assert.False(t, s.Devcontainer)
		require.Len(t, s.Entries, 1)
		assert.Equal(t, "my-user", s.Entries[0].Owner)
		assert.Equal(t, "some-fork", s.Entries[0].Repo)
		assert.Equal(t, map[string]string{"contents": "write", "workflows": "write", "actions": "write", "checks": "read"}, s.Entries[0].Permissions.Levels)
	})

	t.Run("devcontainer_with_comments_and_trailing_commas", func(t *testing.T) {
		s, err := ReadScopeFile("fixtures/scope-devcontainer.jsonc")
		require.NoError(t, err)
		assert.True(t, s.Devcontainer)
		require.Len(t, s.Entries, 2)
		assert.Equal(t, "my-org/app", s.Entries[0].Name())
		assert.Equal(t, "my-org/shared-charts", s.Entries[1].Name())
		assert.Nil(t, s.CurrentRepository)
	})

	t.Run("devcontainer_with_current_repository", func(t *testing.T) {
		s, err := ReadScopeFile("fixtures/scope-devcontainer-current-repo.jsonc")
		require.NoError(t, err)
		require.NotNil(t, s.CurrentRepository)
		assert.Equal(t, "write", s.CurrentRepository.Levels["pull_requests"])
	})

	t.Run("missing_file", func(t *testing.T) {
		_, err := ReadScopeFile("fixtures/nope.json")
		assert.ErrorContains(t, err, "unable to read scope file")
	})
}

func TestParseScopeFileErrors(t *testing.T) {
	tests := []struct {
		name          string
		data          string
		expectedError string
	}{
		{
			name:          "no_repositories",
			data:          `{"name": "x"}`,
			expectedError: `scope file f.json has no "repositories" or "customizations.codespaces.repositories"`,
		},
		{
			name:          "devcontainer_without_codespaces_repositories",
			data:          `{"customizations": {"vscode": {}}}`,
			expectedError: `has no "repositories" or "customizations.codespaces.repositories"`,
		},
		{
			name:          "invalid_json",
			data:          `{"repositories": `,
			expectedError: "unable to parse scope file f.json",
		},
		{
			name:          "invalid_key",
			data:          `{"repositories": {"not a repo": {"permissions": "read-all"}}}`,
			expectedError: `invalid repository "not a repo": expected owner/repo or owner/*`,
		},
		{
			name:          "key_without_owner",
			data:          `{"repositories": {"repo": {"permissions": "read-all"}}}`,
			expectedError: `invalid repository "repo"`,
		},
		{
			name:          "invalid_level",
			data:          `{"repositories": {"o/r": {"permissions": {"contents": "full"}}}}`,
			expectedError: `repository "o/r": invalid level "full" for permission "contents": expected "read", "write" or "admin"`,
		},
		{
			name:          "invalid_shorthand",
			data:          `{"repositories": {"o/r": {"permissions": "everything"}}}`,
			expectedError: `repository "o/r": invalid permissions shorthand "everything": expected "read-all" or "write-all"`,
		},
		{
			name:          "invalid_permission_name",
			data:          `{"repositories": {"o/r": {"permissions": {"Contents": "read"}}}}`,
			expectedError: `invalid permission name "Contents"`,
		},
		{
			name:          "missing_permissions",
			data:          `{"repositories": {"o/r": {}}}`,
			expectedError: `repository "o/r": no permissions declared`,
		},
		{
			name:          "empty_permissions",
			data:          `{"repositories": {"o/r": {"permissions": {}}}}`,
			expectedError: `repository "o/r": no permissions declared`,
		},
		{
			name:          "permissions_wrong_type",
			data:          `{"repositories": {"o/r": {"permissions": 1}}}`,
			expectedError: `permissions must be "read-all", "write-all" or an object`,
		},
		{
			name:          "current_repository_without_permissions",
			data:          `{"customizations": {"gh-token": {"currentRepository": {}}}}`,
			expectedError: "customizations.gh-token.currentRepository has no permissions",
		},
		{
			name:          "current_repository_invalid_level",
			data:          `{"customizations": {"gh-token": {"currentRepository": {"permissions": {"contents": "x"}}}}}`,
			expectedError: `customizations.gh-token.currentRepository: invalid level "x"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseScopeFile("f.json", []byte(tt.data))
			assert.ErrorContains(t, err, tt.expectedError)
		})
	}
}

func TestParseScopeFileCurrentRepositoryOnly(t *testing.T) {
	s, err := ParseScopeFile("f.json", []byte(`{"customizations": {"gh-token": {"currentRepository": {"permissions": "read-all"}}}}`))
	require.NoError(t, err)
	assert.True(t, s.Devcontainer)
	assert.Empty(t, s.Entries)
	assert.Equal(t, "read-all", s.CurrentRepository.Shorthand)
}

func mustParseScope(t *testing.T, data string) *ScopeFile {
	t.Helper()
	s, err := ParseScopeFile("f.json", []byte(data))
	require.NoError(t, err)
	return s
}

func TestBuildScopeRequest(t *testing.T) {
	tests := []struct {
		name          string
		data          string
		merge         bool
		expected      *ScopeRequest
		expectedError []string
		warnings      []string
	}{
		{
			name: "identical_permissions",
			data: `{"repositories": {
				"my-org/a": {"permissions": {"contents": "write", "issues": "read"}},
				"my-org/b": {"permissions": {"issues": "read", "contents": "write"}}}}`,
			expected: &ScopeRequest{
				Owner:        "my-org",
				Repositories: []string{"a", "b"},
				Permissions:  map[string]string{"contents": "write", "issues": "read"},
			},
		},
		{
			name:          "multiple_owners",
			data:          `{"repositories": {"my-org/a": {"permissions": "read-all"}, "my-user/b": {"permissions": "read-all"}}}`,
			expectedError: []string{"repositories span multiple owners", "my-org: my-org/a", "my-user: my-user/b", "split the scope file or run once per owner"},
		},
		{
			name: "case_insensitive_owner_grouping",
			data: `{"repositories": {"My-Org/A": {"permissions": "read-all"}, "my-org/b": {"permissions": "read-all"}}}`,
			expected: &ScopeRequest{
				Owner:        "My-Org",
				Repositories: []string{"A", "b"},
				ReadAll:      true,
			},
		},
		{
			name:          "duplicate_repository",
			data:          `{"repositories": {"o/Repo": {"permissions": "read-all"}, "o/repo": {"permissions": "read-all"}}}`,
			expectedError: []string{"is listed more than once"},
		},
		{
			name: "differing_permissions_without_merge",
			data: `{"repositories": {
				"my-org/a": {"permissions": {"contents": "write"}},
				"my-org/b": {"permissions": {"contents": "read"}}}}`,
			expectedError: []string{
				"repositories declare different permissions",
				"my-org/a: contents=write",
				"my-org/b: contents=read",
				"--merge-permissions",
			},
		},
		{
			name: "differing_permissions_with_merge",
			data: `{"repositories": {
				"my-org/a": {"permissions": {"contents": "write", "pull_requests": "write"}},
				"my-org/b": {"permissions": {"contents": "read", "issues": "admin"}}}}`,
			merge: true,
			expected: &ScopeRequest{
				Owner:        "my-org",
				Repositories: []string{"a", "b"},
				Permissions:  map[string]string{"contents": "write", "pull_requests": "write", "issues": "admin"},
			},
			warnings: []string{
				"--merge-permissions: my-org/a receives permissions it did not declare: issues=admin",
				"--merge-permissions: my-org/b receives permissions it did not declare: contents=write (declared read), pull_requests=write",
			},
		},
		{
			name: "wildcard",
			data: `{"repositories": {"my-org/*": {"permissions": {"contents": "read"}}}}`,
			expected: &ScopeRequest{
				Owner:       "my-org",
				Permissions: map[string]string{"contents": "read"},
			},
		},
		{
			name: "wildcard_plus_specific_repo",
			data: `{"repositories": {"my-org/*": {"permissions": {"contents": "read"}}, "my-org/a": {"permissions": {"contents": "read"}}}}`,
			expected: &ScopeRequest{
				Owner:       "my-org",
				Permissions: map[string]string{"contents": "read"},
			},
			warnings: []string{"my-org/* already includes my-org/a; it only contributes permissions"},
		},
		{
			name: "write_all",
			data: `{"repositories": {"my-org/a": {"permissions": "write-all"}}}`,
			expected: &ScopeRequest{
				Owner:        "my-org",
				Repositories: []string{"a"},
			},
		},
		{
			name: "read_all",
			data: `{"repositories": {"my-org/a": {"permissions": "read-all"}}}`,
			expected: &ScopeRequest{
				Owner:        "my-org",
				Repositories: []string{"a"},
				ReadAll:      true,
			},
		},
		{
			name:          "write_all_mixed_without_merge",
			data:          `{"repositories": {"my-org/a": {"permissions": "write-all"}, "my-org/b": {"permissions": {"contents": "read"}}}}`,
			expectedError: []string{"repositories declare different permissions", "my-org/a: write-all"},
		},
		{
			name:  "write_all_dominates_with_merge",
			data:  `{"repositories": {"my-org/a": {"permissions": "write-all"}, "my-org/b": {"permissions": {"contents": "read"}}}}`,
			merge: true,
			expected: &ScopeRequest{
				Owner:        "my-org",
				Repositories: []string{"a", "b"},
			},
			warnings: []string{"--merge-permissions: my-org/b receives all installation permissions (write-all) instead of contents=read"},
		},
		{
			name:  "read_all_merged_with_explicit",
			data:  `{"repositories": {"my-org/a": {"permissions": "read-all"}, "my-org/b": {"permissions": {"contents": "write"}}}}`,
			merge: true,
			expected: &ScopeRequest{
				Owner:        "my-org",
				Repositories: []string{"a", "b"},
				Permissions:  map[string]string{"contents": "write"},
				ReadAll:      true,
			},
			warnings: []string{
				"--merge-permissions: my-org/a receives permissions it did not declare: contents=write (declared read)",
				"--merge-permissions: my-org/b receives permissions it did not declare: read access to all installation permissions (read-all)",
			},
		},
		{
			name:          "empty_repositories",
			data:          `{"repositories": {}}`,
			expectedError: []string{"no repositories in scope"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := mustParseScope(t, tt.data)
			req, err := BuildScopeRequest(s, tt.merge)
			if len(tt.expectedError) > 0 {
				require.Error(t, err)
				for _, msg := range tt.expectedError {
					assert.Contains(t, err.Error(), msg)
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected.Owner, req.Owner)
			assert.Equal(t, tt.expected.Repositories, req.Repositories)
			assert.Equal(t, tt.expected.Permissions, req.Permissions)
			assert.Equal(t, tt.expected.ReadAll, req.ReadAll)
			assert.Equal(t, tt.warnings, req.Warnings)
		})
	}
}

func TestBuildScopeRequestNamesEntrySource(t *testing.T) {
	s := mustParseScope(t, `{"repositories": {"my-org/shared": {"permissions": {"contents": "read"}}}}`)
	s.Entries = append(s.Entries, ScopeEntry{
		Owner: "my-user", Repo: "x",
		Permissions: Permissions{Levels: map[string]string{"contents": "read"}},
		Source:      `git remote "origin"`,
	})
	_, err := BuildScopeRequest(s, false)
	assert.ErrorContains(t, err, `my-user: my-user/x (from git remote "origin")`)
}
