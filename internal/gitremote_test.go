package internal

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseGitRemoteURL(t *testing.T) {
	tests := []struct {
		raw           string
		host          string
		owner         string
		repo          string
		expectedError string
	}{
		{raw: "https://github.com/my-user/some-fork.git", host: "github.com", owner: "my-user", repo: "some-fork"},
		{raw: "https://github.com/my-user/some-fork", host: "github.com", owner: "my-user", repo: "some-fork"},
		{raw: "https://github.com/my-user/some-fork/", host: "github.com", owner: "my-user", repo: "some-fork"},
		{raw: "https://x-access-token:abc@GitHub.com/My-Org/Repo.git", host: "github.com", owner: "My-Org", repo: "Repo"},
		{raw: "git@github.com:my-user/some-fork.git", host: "github.com", owner: "my-user", repo: "some-fork"},
		{raw: "git@github.com:my-user/some-fork", host: "github.com", owner: "my-user", repo: "some-fork"},
		{raw: "github.com:my-user/some-fork.git", host: "github.com", owner: "my-user", repo: "some-fork"},
		{raw: "ssh://git@github.com/my-user/some-fork.git", host: "github.com", owner: "my-user", repo: "some-fork"},
		{raw: "ssh://git@ssh.github.com:443/my-user/some-fork.git", host: "github.com", owner: "my-user", repo: "some-fork"},
		{raw: "ssh://git@ghes.example.com:2222/team/app", host: "ghes.example.com", owner: "team", repo: "app"},
		{raw: "https://ghes.example.com/team/app.git", host: "ghes.example.com", owner: "team", repo: "app"},
		{raw: "git@ghes.example.com:team/app.git\n", host: "ghes.example.com", owner: "team", repo: "app"},
		{raw: "https://github.com/my-user", expectedError: "is not of the form host/owner/repo"},
		{raw: "https://github.com/a/b/c", expectedError: "is not of the form host/owner/repo"},
		{raw: "git@github.com:repo.git", expectedError: "is not of the form host/owner/repo"},
		{raw: "/srv/git/repo.git", expectedError: "unsupported git remote URL"},
		{raw: "file:///srv/git/o/r.git", expectedError: `scheme "file"`},
		{raw: "", expectedError: "unsupported git remote URL"},
		{raw: "https://%zz/o/r", expectedError: "unable to parse git remote URL"},
	}

	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			host, owner, repo, err := parseGitRemoteURL(tt.raw)
			if tt.expectedError != "" {
				assert.ErrorContains(t, err, tt.expectedError)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.host, host)
			assert.Equal(t, tt.owner, owner)
			assert.Equal(t, tt.repo, repo)
		})
	}
}

func TestWebHostForAPI(t *testing.T) {
	assert.Equal(t, "github.com", webHostForAPI("api.github.com"))
	assert.Equal(t, "ghes.example.com", webHostForAPI("ghes.example.com/api/v3"))
	assert.Equal(t, "ghes.example.com", webHostForAPI("ghes.example.com/api/v3/"))
}

// initGitRepo creates a git repository with the given origin URL and
// returns its path. Skips the test when git is unavailable.
func initGitRepo(t *testing.T, remotes map[string]string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	run := func(args ...string) {
		out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
		require.NoError(t, err, string(out))
	}
	run("init", "-q")
	for name, url := range remotes {
		run("remote", "add", name, url)
	}
	return dir
}

func TestCurrentRepository(t *testing.T) {
	dir := initGitRepo(t, map[string]string{
		"origin":   "git@github.com:my-user/some-fork.git",
		"upstream": "https://github.com/upstream-org/some-repo.git",
		"ghes":     "https://ghes.example.com/team/app.git",
	})

	owner, repo, err := currentRepository("api.github.com", dir, "origin")
	require.NoError(t, err)
	assert.Equal(t, "my-user", owner)
	assert.Equal(t, "some-fork", repo)

	owner, repo, err = currentRepository("api.github.com", dir, "upstream")
	require.NoError(t, err)
	assert.Equal(t, "upstream-org/some-repo", owner+"/"+repo)

	owner, repo, err = currentRepository("ghes.example.com/api/v3", dir, "ghes")
	require.NoError(t, err)
	assert.Equal(t, "team/app", owner+"/"+repo)

	_, _, err = currentRepository("api.github.com", dir, "ghes")
	assert.ErrorContains(t, err, `git remote "ghes" points at ghes.example.com, but tokens are generated for github.com`)

	_, _, err = currentRepository("api.github.com", dir, "missing")
	assert.ErrorContains(t, err, `unable to read git remote "missing"`)

	_, _, err = currentRepository("api.github.com", t.TempDir(), "origin")
	assert.ErrorContains(t, err, `unable to read git remote "origin"`)
}
