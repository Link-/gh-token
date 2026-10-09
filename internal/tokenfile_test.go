package internal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-github/v55/github"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testMintedToken(token string) *mintedToken {
	expires := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	details := &tokenDetails{
		ExpiresAt:           expires,
		Permissions:         map[string]string{"contents": "write", "metadata": "read"},
		RepositorySelection: "selected",
	}
	details.Repositories = append(details.Repositories, struct {
		Name     string `json:"name"`
		FullName string `json:"full_name"`
	}{Name: "app", FullName: "my-org/app"})

	return &mintedToken{
		InstallationToken: &github.InstallationToken{Token: github.String(token), ExpiresAt: &github.Timestamp{Time: expires}},
		details:           details,
	}
}

func TestWriteTokenFiles(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "gh-token")
	path := filepath.Join(dir, "token")

	require.NoError(t, writeTokenFiles(path, testMintedToken("ghs_one")))

	b, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "ghs_one\n", string(b))

	if runtime.GOOS != "windows" {
		st, err := os.Stat(path)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), st.Mode().Perm())
		st, err = os.Stat(dir)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o700), st.Mode().Perm())
		st, err = os.Stat(path + ".json")
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), st.Mode().Perm())
	}

	sidecar, err := os.ReadFile(path + ".json")
	require.NoError(t, err)
	assert.NotContains(t, string(sidecar), "ghs_one")
	assert.JSONEq(t, `{
		"expires_at": "2030-01-01T00:00:00Z",
		"permissions": {"contents": "write", "metadata": "read"},
		"repository_selection": "selected",
		"repositories": ["my-org/app"]
	}`, string(sidecar))

	// Overwrite, and leave no temp files behind.
	require.NoError(t, writeTokenFiles(path, testMintedToken("ghs_two")))
	b, err = os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "ghs_two\n", string(b))
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	assert.ElementsMatch(t, []string{"token", "token.json"}, names)
}

func TestWriteFileAtomicNeverExposesPartialContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token")
	tokens := []string{strings.Repeat("a", 64*1024) + "\n", strings.Repeat("b", 64*1024) + "\n"}

	var wg sync.WaitGroup
	done := make(chan struct{})
	var bad []string
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-done:
				return
			default:
			}
			b, err := os.ReadFile(path)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil || (string(b) != tokens[0] && string(b) != tokens[1]) {
				bad = append(bad, string(b[:min(len(b), 16)]))
				return
			}
		}
	}()

	for i := 0; i < 200; i++ {
		require.NoError(t, writeFileAtomic(path, []byte(tokens[i%2])))
	}
	close(done)
	wg.Wait()
	assert.Empty(t, bad, "reader observed a partial token")
}

func TestGenerateOutputFile(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	recordTokenRequests("https://api.github.com/app/installations/9/access_tokens",
		`{"token": "ghs_file", "expires_at": "2030-01-01T00:00:00Z", "permissions": {"contents": "read", "metadata": "read"}, "repository_selection": "selected", "repositories": [{"name": "app", "full_name": "my-org/app"}]}`)

	path := filepath.Join(t.TempDir(), "gh-token", "token")
	out, errOut, err := runGenerate(t, "-i", "123", "-k", testKey, "-l", "9", "-r", "my-org/app", "-p", "contents=read", "--output-file", path)
	require.NoError(t, err)
	assert.Empty(t, out, "the token must not be printed to stdout")
	assert.Contains(t, errOut, "token written to "+path+", expires at 2030-01-01T00:00:00Z")

	b, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "ghs_file\n", string(b))

	var sidecar tokenSidecar
	b, err = os.ReadFile(path + ".json")
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(b, &sidecar))
	assert.Equal(t, []string{"my-org/app"}, sidecar.Repositories)

	_, _, err = runGenerate(t, "-i", "123", "-k", testKey, "-l", "9", "--output-file", path, "-t")
	assert.EqualError(t, err, "--output-file cannot be combined with --token-only; the file always holds just the token")

	_, _, err = runGenerate(t, "-i", "123", "-k", testKey, "-j", "--output-file", path)
	assert.EqualError(t, err, "--jwt cannot be combined with scope or output flags")
}

func TestGenerateOutputFileWholeInstallation(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	bodies := recordTokenRequests("https://api.github.com/app/installations/9/access_tokens",
		`{"token": "ghs_all", "expires_at": "2030-01-01T00:00:00Z"}`)

	path := filepath.Join(t.TempDir(), "token")
	_, _, err := runGenerate(t, "-i", "123", "-k", testKey, "-l", "9", "--output-file", path, "--silent")
	require.NoError(t, err)
	assert.Equal(t, []string{""}, *bodies, "no scope means no request body")
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "ghs_all\n", string(b))
}
