package internal

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
)

const testKey = "fixtures/test-private-key.test.pem"

// runGenerate runs "gh-token generate <args>" through the real CLI flag
// definitions and captures the scoped path's stdout and stderr.
func runGenerate(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	var out, errOut bytes.Buffer
	oldOut, oldErr := stdout, stderr
	stdout, stderr = &out, &errOut
	defer func() { stdout, stderr = oldOut, oldErr }()

	app := &cli.App{
		Name: "gh-token",
		Commands: []*cli.Command{
			{Name: "generate", Flags: GenerateFlags(), Action: Generate},
		},
	}
	err := app.Run(append([]string{"gh-token", "generate"}, args...))

	return out.String(), errOut.String(), err
}

func writeTempFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

// recordTokenRequests registers an access token responder that records the
// request bodies it receives.
func recordTokenRequests(endpoint, response string) *[]string {
	var bodies []string
	httpmock.RegisterResponder("POST", endpoint, func(req *http.Request) (*http.Response, error) {
		b := []byte{}
		if req.Body != nil {
			b, _ = io.ReadAll(req.Body)
		}
		bodies = append(bodies, string(b))
		return httpmock.NewStringResponse(201, response), nil
	})
	return &bodies
}

const scopedTokenResponse = `{
  "token": "ghs_scoped",
  "expires_at": "2030-01-01T00:00:00Z",
  "permissions": {"actions": "write", "checks": "read", "contents": "write", "workflows": "write", "metadata": "read"},
  "repository_selection": "selected",
  "repositories": [{"name": "some-fork", "full_name": "my-user/some-fork"}]
}`

func TestGenerateWithScopeFile(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder("GET", "https://api.github.com/repos/my-user/some-fork/installation",
		httpmock.NewStringResponder(200, `{"id": 12345}`))
	bodies := recordTokenRequests("https://api.github.com/app/installations/12345/access_tokens", scopedTokenResponse)

	out, errOut, err := runGenerate(t, "-i", "123", "-k", testKey, "-f", "fixtures/scope-plain.json", "--token-only")
	require.NoError(t, err)
	assert.Equal(t, "ghs_scoped\n", out)
	assert.Empty(t, errOut)
	require.Len(t, *bodies, 1)
	assert.JSONEq(t, `{"repositories":["some-fork"],"permissions":{"actions":"write","checks":"read","contents":"write","workflows":"write"}}`, (*bodies)[0])
}

func TestGenerateWithScopeFileJSONOutput(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder("GET", "https://api.github.com/repos/my-user/some-fork/installation",
		httpmock.NewStringResponder(200, `{"id": 12345}`))
	recordTokenRequests("https://api.github.com/app/installations/12345/access_tokens", scopedTokenResponse)

	out, _, err := runGenerate(t, "-i", "123", "-k", testKey, "-f", "fixtures/scope-plain.json")
	require.NoError(t, err)
	assert.Contains(t, out, `"token": "ghs_scoped"`)
	assert.Contains(t, out, `"expires_at": "2030-01-01T00:00:00Z"`)
}

func TestGenerateScopedPrintRequest(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder("GET", "https://api.github.com/repos/my-user/some-fork/installation",
		httpmock.NewStringResponder(200, `{"id": 12345}`))

	out, _, err := runGenerate(t, "-i", "123", "-k", testKey, "-f", "fixtures/scope-plain.json", "--print-request")
	require.NoError(t, err)
	assert.Equal(t, "# installation: 12345 (looked up for my-user)\n"+
		"# repository: my-user/some-fork (scope file)\n"+
		`{"repositories":["some-fork"],"permissions":{"actions":"write","checks":"read","contents":"write","workflows":"write"}}`+"\n", out)

	info := httpmock.GetCallCountInfo()
	assert.Equal(t, 0, info["POST https://api.github.com/app/installations/12345/access_tokens"])
}

func TestGenerateScopedWildcardFallsBackToUser(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder("GET", "https://api.github.com/orgs/my-user/installation", httpmock.NewStringResponder(404, `{}`))
	httpmock.RegisterResponder("GET", "https://api.github.com/users/my-user/installation", httpmock.NewStringResponder(200, `{"id": 77}`))
	bodies := recordTokenRequests("https://api.github.com/app/installations/77/access_tokens", `{"token": "ghs_x", "permissions": {"contents": "read", "metadata": "read"}, "repository_selection": "all"}`)

	scope := writeTempFile(t, "scope.json", `{"repositories": {"my-user/*": {"permissions": {"contents": "read"}}}}`)
	out, errOut, err := runGenerate(t, "-i", "123", "-k", testKey, "-f", scope, "-t")
	require.NoError(t, err)
	assert.Equal(t, "ghs_x\n", out)
	assert.Empty(t, errOut)
	assert.Equal(t, []string{`{"permissions":{"contents":"read"}}`}, *bodies)
}

func TestGenerateScopedReadAll(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder("GET", "https://api.github.com/repos/my-org/app/installation", httpmock.NewStringResponder(200, `{"id": 5}`))
	httpmock.RegisterResponder("GET", "https://api.github.com/app/installations/5",
		httpmock.NewStringResponder(200, `{"id": 5, "permissions": {"contents": "write", "issues": "write", "metadata": "read"}}`))

	out, _, err := runGenerate(t, "-i", "123", "-k", testKey, "-r", "my-org/app", "-p", "read-all", "--print-request")
	require.NoError(t, err)
	assert.Contains(t, out, "# repository: my-org/app (--repository flag)\n")
	assert.Contains(t, out, `{"repositories":["app"],"permissions":{"contents":"read","issues":"read","metadata":"read"}}`)
}

func TestGenerateScopedAdHocFlags(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	bodies := recordTokenRequests("https://api.github.com/app/installations/9/access_tokens",
		`{"token": "ghs_y", "permissions": {"contents": "write", "metadata": "read"}, "repository_selection": "selected"}`)

	out, errOut, err := runGenerate(t, "-i", "123", "-k", testKey, "-l", "9", "-r", "app", "-r", "lib", "-p", "contents=write", "-t")
	require.NoError(t, err)
	assert.Equal(t, "ghs_y\n", out)
	assert.Empty(t, errOut)
	assert.Equal(t, []string{`{"repositories":["app","lib"],"permissions":{"contents":"write"}}`}, *bodies)
}

func TestGenerateScopedVerificationWarnings(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	recordTokenRequests("https://api.github.com/app/installations/9/access_tokens",
		`{"token": "ghs_y", "permissions": {"contents": "read", "issues": "write", "metadata": "read"}, "repository_selection": "all"}`)

	_, errOut, err := runGenerate(t, "-i", "123", "-k", testKey, "-l", "9", "-r", "my-org/app", "-p", "contents=write", "-p", "pull_requests=read", "-t")
	require.NoError(t, err)
	assert.Equal(t, "Warning: requested contents=write but the token has contents=read\n"+
		"Warning: requested pull_requests=read but the token does not have it\n"+
		"Warning: the token has issues=write, which was not requested\n"+
		"Warning: requested specific repositories but the token's repository_selection is \"all\"\n", errOut)

	_, errOut, err = runGenerate(t, "-i", "123", "-k", testKey, "-l", "9", "-r", "my-org/app", "-p", "contents=write", "-t", "--silent")
	require.NoError(t, err)
	assert.Empty(t, errOut)
}

func TestGenerateScopedVerifyInstallation(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder("GET", "https://api.github.com/repos/my-org/app/installation", httpmock.NewStringResponder(200, `{"id": 5}`))

	_, _, err := runGenerate(t, "-i", "123", "-k", testKey, "-l", "6", "-r", "my-org/app", "-p", "contents=read", "--verify-installation", "--print-request")
	assert.EqualError(t, err, "--installation-id 6 does not match the installation 5 found for my-org")

	out, _, err := runGenerate(t, "-i", "123", "-k", testKey, "-l", "5", "-r", "my-org/app", "-p", "contents=read", "--verify-installation", "--print-request")
	require.NoError(t, err)
	assert.Contains(t, out, "# installation: 5 (--installation-id, verified)\n")
}

func TestGenerateScopedGHES(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder("GET", "https://github.example.com/api/v3/repos/my-user/some-fork/installation",
		httpmock.NewStringResponder(200, `{"id": 3}`))
	bodies := recordTokenRequests("https://github.example.com/api/v3/app/installations/3/access_tokens", scopedTokenResponse)

	_, _, err := runGenerate(t, "-i", "123", "-k", testKey, "-o", "github.example.com", "-f", "fixtures/scope-plain.json", "-t")
	require.NoError(t, err)
	assert.Len(t, *bodies, 1)
}

func TestGenerateScopedErrors(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	scope := writeTempFile(t, "scope.json", `{"repositories": {"o/r": {"permissions": "read-all"}}}`)

	tests := []struct {
		name          string
		args          []string
		mocks         func()
		expectedError string
	}{
		{
			name:          "scope_file_and_repository",
			args:          []string{"-f", scope, "-r", "o/r"},
			expectedError: "--scope-file and --repository cannot be combined",
		},
		{
			name:          "jwt_with_scope_flags",
			args:          []string{"-j", "-f", scope},
			expectedError: "--jwt cannot be combined with scope or output flags",
		},
		{
			name:          "repository_without_permission",
			args:          []string{"-r", "o/r"},
			expectedError: "--repository requires at least one --permission",
		},
		{
			name:          "permission_alone",
			args:          []string{"-p", "contents=read"},
			expectedError: "--permission requires --repository or --scope-file",
		},
		{
			name:          "bare_repository_without_installation_id",
			args:          []string{"-r", "app", "-p", "contents=read"},
			expectedError: "--repository app has no owner; use owner/repo or pass --installation-id",
		},
		{
			name:          "invalid_permission_flag",
			args:          []string{"-r", "o/r", "-p", "contents"},
			expectedError: `invalid --permission "contents": expected name=level`,
		},
		{
			name:          "invalid_permission_level",
			args:          []string{"-r", "o/r", "-p", "contents=all"},
			expectedError: `invalid --permission: invalid level "all"`,
		},
		{
			name:          "verify_without_installation_id",
			args:          []string{"-f", scope, "--verify-installation"},
			expectedError: "--verify-installation requires --installation-id",
		},
		{
			name:          "multi_owner_file",
			args:          []string{"-f", "fixtures/scope-multi-owner.json"},
			expectedError: "repositories span multiple owners",
		},
		{
			name:          "mixed_permissions_file",
			args:          []string{"-f", "fixtures/scope-mixed-perms.json"},
			expectedError: "repositories declare different permissions",
		},
		{
			name: "app_not_installed",
			args: []string{"-f", scope},
			mocks: func() {
				httpmock.RegisterResponder("GET", "https://api.github.com/repos/o/r/installation", httpmock.NewStringResponder(404, `{"message":"Not Found"}`))
			},
			expectedError: "failed resolving installation ID: the GitHub App is not installed on o/r",
		},
		{
			name: "token_request_rejected",
			args: []string{"-l", "1", "-r", "o/r", "-p", "bogus=read"},
			mocks: func() {
				httpmock.RegisterResponder("POST", "https://api.github.com/app/installations/1/access_tokens",
					httpmock.NewStringResponder(422, `{"message":"The permissions requested are not granted to this installation."}`))
			},
			expectedError: `failed generating installation token: unexpected status code: 422: {"message":"The permissions requested are not granted to this installation."}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpmock.Reset()
			if tt.mocks != nil {
				tt.mocks()
			}
			_, _, err := runGenerate(t, append([]string{"-i", "123", "-k", testKey}, tt.args...)...)
			assert.ErrorContains(t, err, tt.expectedError)
		})
	}
}

func TestGenerateMergePermissionsWarns(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder("GET", "https://api.github.com/repos/my-org/app/installation", httpmock.NewStringResponder(200, `{"id": 5}`))

	out, errOut, err := runGenerate(t, "-i", "123", "-k", testKey, "-f", "fixtures/scope-mixed-perms.json", "--merge-permissions", "--print-request")
	require.NoError(t, err)
	assert.Contains(t, out, `{"repositories":["app","shared-charts"],"permissions":{"contents":"write","issues":"read","pull_requests":"write"}}`)
	assert.Contains(t, errOut, "Warning: --merge-permissions: my-org/app receives permissions it did not declare: issues=read")
	assert.Contains(t, errOut, "Warning: --merge-permissions: my-org/shared-charts receives permissions it did not declare: contents=write (declared read), pull_requests=write")
}
