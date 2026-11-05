package internal

import (
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLookupInstallationID(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	repoScope := &ScopeRequest{Owner: "my-org", Entries: []ScopeEntry{
		{Owner: "my-org", Repo: "*"},
		{Owner: "my-org", Repo: "app"},
	}}
	wildcardScope := &ScopeRequest{Owner: "my-user", Entries: []ScopeEntry{{Owner: "my-user", Repo: "*"}}}

	tests := []struct {
		name          string
		hostname      string
		scope         *ScopeRequest
		mocks         map[string]httpmock.Responder
		expectedID    string
		expectedError string
		expectedCalls []string
	}{
		{
			name:          "first_concrete_repo",
			hostname:      "api.github.com",
			scope:         repoScope,
			mocks:         map[string]httpmock.Responder{"https://api.github.com/repos/my-org/app/installation": httpmock.NewStringResponder(200, `{"id": 42}`)},
			expectedID:    "42",
			expectedCalls: []string{"GET https://api.github.com/repos/my-org/app/installation"},
		},
		{
			name:          "repo_not_installed",
			hostname:      "api.github.com",
			scope:         repoScope,
			mocks:         map[string]httpmock.Responder{"https://api.github.com/repos/my-org/app/installation": httpmock.NewStringResponder(404, `{"message":"Not Found"}`)},
			expectedError: `the GitHub App is not installed on my-org/app, or the repository is not among the installation's selected repositories; check the installation's repository access (not installed: unexpected status code: 404: {"message":"Not Found"})`,
		},
		{
			name:     "org_installation",
			hostname: "api.github.com",
			scope:    wildcardScope,
			mocks: map[string]httpmock.Responder{
				"https://api.github.com/orgs/my-user/installation": httpmock.NewStringResponder(200, `{"id": 7}`),
			},
			expectedID:    "7",
			expectedCalls: []string{"GET https://api.github.com/orgs/my-user/installation"},
		},
		{
			name:     "user_fallback_on_org_404",
			hostname: "api.github.com",
			scope:    wildcardScope,
			mocks: map[string]httpmock.Responder{
				"https://api.github.com/orgs/my-user/installation":  httpmock.NewStringResponder(404, `{}`),
				"https://api.github.com/users/my-user/installation": httpmock.NewStringResponder(200, `{"id": 8}`),
			},
			expectedID: "8",
			expectedCalls: []string{
				"GET https://api.github.com/orgs/my-user/installation",
				"GET https://api.github.com/users/my-user/installation",
			},
		},
		{
			name:     "neither_org_nor_user",
			hostname: "api.github.com",
			scope:    wildcardScope,
			mocks: map[string]httpmock.Responder{
				"https://api.github.com/orgs/my-user/installation":  httpmock.NewStringResponder(404, `{}`),
				"https://api.github.com/users/my-user/installation": httpmock.NewStringResponder(404, `{}`),
			},
			expectedError: "the GitHub App is not installed on the my-user organization or user account",
		},
		{
			name:          "server_error_is_not_a_fallback",
			hostname:      "api.github.com",
			scope:         wildcardScope,
			mocks:         map[string]httpmock.Responder{"https://api.github.com/orgs/my-user/installation": httpmock.NewStringResponder(500, `boom`)},
			expectedError: "unable to look up the installation for my-user: unexpected status code: 500: boom",
		},
		{
			name:          "ghes_hostname",
			hostname:      "github.example.com/api/v3",
			scope:         repoScope,
			mocks:         map[string]httpmock.Responder{"https://github.example.com/api/v3/repos/my-org/app/installation": httpmock.NewStringResponder(200, `{"id": 9}`)},
			expectedID:    "9",
			expectedCalls: []string{"GET https://github.example.com/api/v3/repos/my-org/app/installation"},
		},
		{
			name:          "no_owner",
			hostname:      "api.github.com",
			scope:         &ScopeRequest{Entries: []ScopeEntry{{Repo: "app"}}},
			expectedError: "cannot look up the installation without an owner",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpmock.Reset()
			for url, r := range tt.mocks {
				httpmock.RegisterResponder("GET", url, r)
			}

			id, err := lookupInstallationID(tt.hostname, "test.jwt.token", tt.scope)
			if tt.expectedError != "" {
				assert.ErrorContains(t, err, tt.expectedError)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expectedID, id)
			info := httpmock.GetCallCountInfo()
			for _, call := range tt.expectedCalls {
				assert.Equal(t, 1, info[call], call)
			}
			assert.Equal(t, len(tt.expectedCalls), httpmock.GetTotalCallCount())
		})
	}
}

func TestInstallationPermissionsAndExpandReadAll(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder("GET", "https://api.github.com/app/installations/42",
		httpmock.NewStringResponder(200, `{"id": 42, "permissions": {"contents": "write", "issues": "read", "metadata": "read", "workflows": "write"}}`))

	perms, err := installationPermissions("api.github.com", "test.jwt.token", "42")
	require.NoError(t, err)

	scope := &ScopeRequest{ReadAll: true, Permissions: map[string]string{"issues": "write"}}
	expandReadAll(scope, perms)
	assert.False(t, scope.ReadAll)
	assert.Equal(t, map[string]string{"contents": "read", "issues": "write", "metadata": "read"}, scope.Permissions)

	unchanged := &ScopeRequest{Permissions: map[string]string{"contents": "write"}}
	expandReadAll(unchanged, perms)
	assert.Equal(t, map[string]string{"contents": "write"}, unchanged.Permissions)

	httpmock.RegisterResponder("GET", "https://api.github.com/app/installations/43", httpmock.NewStringResponder(404, ``))
	_, err = installationPermissions("api.github.com", "test.jwt.token", "43")
	assert.ErrorContains(t, err, "unable to read installation 43")
}
