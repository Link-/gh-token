package internal

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
)

// TestHelperRefresher stands in for `gh-token generate --refresh` when
// devcontainer-init starts it in tests. It is not a real test.
func TestHelperRefresher(t *testing.T) {
	mode := os.Getenv("GH_TOKEN_TEST_REFRESHER")
	if mode == "" {
		t.Skip("helper process")
	}
	args := os.Args
	for i, a := range args {
		if a == "--" {
			args = args[i+1:]
			break
		}
	}
	_ = os.WriteFile(os.Getenv("GH_TOKEN_TEST_ARGS"), []byte(strings.Join(args, "\n")+"\n"), 0o600)
	out := ""
	for i, a := range args {
		if a == "--output-file" && i+1 < len(args) {
			out = args[i+1]
		}
	}
	switch mode {
	case "ok":
		_ = os.WriteFile(out, []byte(fmt.Sprintf("ghs_%d\n", time.Now().UnixNano())), 0o600)
		time.Sleep(2 * time.Second)
		os.Exit(0)
	case "fail":
		fmt.Fprintln(os.Stderr, "Error: failed resolving installation ID: unexpected status code: 404")
		os.Exit(1)
	case "running":
		os.Exit(0)
	case "hang":
		time.Sleep(3 * time.Second)
		os.Exit(1)
	}
}

type initEnv struct {
	workspace string
	home      string
	argsFile  string
	envFile   string
	key       string
}

// newInitEnv sets up a workspace with a scope file as the working
// directory, an empty environment and the helper refresher in `mode`.
func newInitEnv(t *testing.T, mode string) *initEnv {
	t.Helper()
	root := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	e := &initEnv{
		workspace: filepath.Join(root, "ws"),
		home:      filepath.Join(root, "home"),
		argsFile:  filepath.Join(root, "args"),
		envFile:   filepath.Join(root, "config", "gh-token", "devcontainer.env"),
		key:       filepath.Join(root, "app.pem"),
	}
	require.NoError(t, os.MkdirAll(filepath.Join(e.workspace, ".devcontainer"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(e.workspace, ".devcontainer", "devcontainer.json"), []byte(`{"customizations": {}}`), 0o600))
	require.NoError(t, os.MkdirAll(e.home, 0o700))
	require.NoError(t, os.WriteFile(e.key, []byte("key"), 0o600))
	t.Chdir(e.workspace)

	for _, k := range []string{"GH_TOKEN_APP_ID", "GH_TOKEN_KEY", "GH_TOKEN_HOSTNAME", "GH_TOKEN_SCOPE_FILE",
		"GH_TOKEN_OPTIONAL", "GH_TOKEN_TIMEOUT", "GH_TOKEN_MAX_LIFETIME", "GH_TOKEN_MERGE_PERMISSIONS", "XDG_RUNTIME_DIR"} {
		t.Setenv(k, "")
	}
	t.Setenv("GH_TOKEN_ENV_FILE", e.envFile)
	t.Setenv("HOME", e.home)
	t.Setenv("GH_TOKEN_TEST_REFRESHER", mode)
	t.Setenv("GH_TOKEN_TEST_ARGS", e.argsFile)

	old := refresherCommand
	refresherCommand = func(args []string) (*exec.Cmd, error) {
		return exec.Command(os.Args[0], append([]string{"-test.run=^TestHelperRefresher$", "--"}, args...)...), nil
	}
	t.Cleanup(func() { refresherCommand = old })
	return e
}

func (e *initEnv) args(t *testing.T) []string {
	t.Helper()
	b, err := os.ReadFile(e.argsFile)
	require.NoError(t, err)
	return strings.Split(strings.TrimSpace(string(b)), "\n")
}

func runDevcontainerInit(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var errOut bytes.Buffer
	old := stderr
	stderr = &errOut
	defer func() { stderr = old }()

	app := &cli.App{Commands: []*cli.Command{{Name: "devcontainer-init", Flags: DevcontainerInitFlags(), Action: DevcontainerInit}}}
	err := app.Run(append([]string{"gh-token", "devcontainer-init"}, args...))
	return errOut.String(), err
}

// argValue returns the value after flag in args.
func argValue(args []string, flag string) string {
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

func TestDevcontainerInit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses Unix paths and permissions")
	}

	t.Run("starts_the_refresher", func(t *testing.T) {
		e := newInitEnv(t, "ok")
		t.Setenv("GH_TOKEN_APP_ID", "123")
		t.Setenv("GH_TOKEN_KEY", e.key)
		dir := filepath.Join(e.home, ".cache", "gh-token", "abc")

		out, err := runDevcontainerInit(t, dir)
		require.NoError(t, err, out)
		assert.Contains(t, out, "token ready")

		st, err := os.Stat(dir)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o700), st.Mode().Perm())
		host, _ := os.ReadFile(filepath.Join(dir, "host"))
		assert.Equal(t, "github.com\n", string(host))

		args := e.args(t)
		assert.Equal(t, "generate", args[0])
		assert.Equal(t, "123", argValue(args, "--app-id"))
		assert.Equal(t, e.key, argValue(args, "--key"))
		assert.Equal(t, e.workspace, argValue(args, "--git-dir"))
		assert.Equal(t, filepath.Join(dir, "token"), argValue(args, "--output-file"))
		assert.Equal(t, filepath.Join(dir, "pid"), argValue(args, "--pid-file"))
		assert.Equal(t, filepath.Join(".devcontainer", "devcontainer.json"), argValue(args, "--scope-file"))
		assert.Contains(t, args, "--refresh")
		assert.Contains(t, args, "--revoke-on-exit")
		assert.NotContains(t, args, "--hostname")
		assert.NotContains(t, args, "--merge-permissions")
	})

	t.Run("reads_the_env_file", func(t *testing.T) {
		e := newInitEnv(t, "ok")
		require.NoError(t, os.MkdirAll(filepath.Dir(e.envFile), 0o700))
		require.NoError(t, os.WriteFile(e.envFile, []byte(
			"# settings for gh token devcontainer-init\n"+
				"export GH_TOKEN_APP_ID=456\n"+
				"GH_TOKEN_KEY=\"~/keys/app.pem\"\n"+
				"GH_TOKEN_HOSTNAME='ghes.example.com'\n"+
				"GH_TOKEN_MAX_LIFETIME=12h\n"+
				"GH_TOKEN_MERGE_PERMISSIONS=true\n"), 0o600))
		require.NoError(t, os.MkdirAll(filepath.Join(e.home, "keys"), 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(e.home, "keys", "app.pem"), []byte("key"), 0o600))
		dir := filepath.Join(e.home, "tokens")

		out, err := runDevcontainerInit(t, dir)
		require.NoError(t, err, out)
		args := e.args(t)
		assert.Equal(t, "456", argValue(args, "--app-id"))
		assert.Equal(t, filepath.Join(e.home, "keys", "app.pem"), argValue(args, "--key"))
		assert.Equal(t, "ghes.example.com", argValue(args, "--hostname"))
		assert.Equal(t, "12h0m0s", argValue(args, "--max-lifetime"))
		assert.Contains(t, args, "--merge-permissions")
		host, _ := os.ReadFile(filepath.Join(dir, "host"))
		assert.Equal(t, "ghes.example.com\n", string(host))
	})

	t.Run("environment_wins_over_the_env_file", func(t *testing.T) {
		e := newInitEnv(t, "ok")
		require.NoError(t, os.MkdirAll(filepath.Dir(e.envFile), 0o700))
		require.NoError(t, os.WriteFile(e.envFile, []byte("GH_TOKEN_APP_ID=456\nGH_TOKEN_KEY=/nope.pem\n"), 0o600))
		t.Setenv("GH_TOKEN_APP_ID", "789")
		t.Setenv("GH_TOKEN_KEY", e.key)

		out, err := runDevcontainerInit(t, filepath.Join(e.home, "tokens"))
		require.NoError(t, err, out)
		assert.Equal(t, "789", argValue(e.args(t), "--app-id"))
		assert.Equal(t, e.key, argValue(e.args(t), "--key"))
	})

	t.Run("default_key_next_to_the_env_file", func(t *testing.T) {
		e := newInitEnv(t, "ok")
		require.NoError(t, os.MkdirAll(filepath.Dir(e.envFile), 0o700))
		require.NoError(t, os.WriteFile(e.envFile, []byte("GH_TOKEN_APP_ID=1\n"), 0o600))
		defaultKey := filepath.Join(filepath.Dir(e.envFile), "app.pem")
		require.NoError(t, os.WriteFile(defaultKey, []byte("key"), 0o600))

		out, err := runDevcontainerInit(t, filepath.Join(e.home, "tokens"))
		require.NoError(t, err, out)
		assert.Equal(t, defaultKey, argValue(e.args(t), "--key"))
	})

	t.Run("not_configured_is_an_error", func(t *testing.T) {
		e := newInitEnv(t, "ok")
		dir := filepath.Join(e.home, "tokens")
		_, err := runDevcontainerInit(t, dir)
		assert.ErrorContains(t, err, "no GitHub App ID: set GH_TOKEN_APP_ID, or add it to "+e.envFile)
		_, statErr := os.Stat(dir)
		assert.NoError(t, statErr, "the token directory exists anyway")
	})

	t.Run("optional_not_configured", func(t *testing.T) {
		for _, optional := range []func(t *testing.T) []string{
			func(t *testing.T) []string { return []string{"--optional"} },
			func(t *testing.T) []string { t.Setenv("GH_TOKEN_OPTIONAL", "1"); return nil },
		} {
			e := newInitEnv(t, "ok")
			dir := filepath.Join(e.home, "tokens")
			out, err := runDevcontainerInit(t, append(optional(t), dir)...)
			require.NoError(t, err)
			assert.Contains(t, out, "WARNING: no GitHub App ID")
			assert.Contains(t, out, "starting the container without a GitHub token (--optional)")
			st, statErr := os.Stat(dir)
			require.NoError(t, statErr, "the mount source must exist")
			assert.True(t, st.IsDir())
		}
	})

	t.Run("unreadable_key", func(t *testing.T) {
		e := newInitEnv(t, "ok")
		t.Setenv("GH_TOKEN_APP_ID", "1")
		t.Setenv("GH_TOKEN_KEY", filepath.Join(e.home, "missing.pem"))
		_, err := runDevcontainerInit(t, filepath.Join(e.home, "tokens"))
		assert.ErrorContains(t, err, "cannot read the App private key")
	})

	t.Run("refresher_fails", func(t *testing.T) {
		e := newInitEnv(t, "fail")
		t.Setenv("GH_TOKEN_APP_ID", "1")
		t.Setenv("GH_TOKEN_KEY", e.key)
		_, err := runDevcontainerInit(t, filepath.Join(e.home, "tokens"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "the refresher failed")
		assert.Contains(t, err.Error(), "unexpected status code: 404")
		assert.Contains(t, err.Error(), "install it on the repository or its owner")

		out, err := runDevcontainerInit(t, "--optional", filepath.Join(e.home, "tokens"))
		require.NoError(t, err)
		assert.Contains(t, out, "WARNING: the refresher failed")
	})

	t.Run("already_running", func(t *testing.T) {
		e := newInitEnv(t, "running")
		t.Setenv("GH_TOKEN_APP_ID", "1")
		t.Setenv("GH_TOKEN_KEY", e.key)
		dir := filepath.Join(e.home, "tokens")

		_, err := runDevcontainerInit(t, dir)
		assert.ErrorContains(t, err, "it exited without a token")

		require.NoError(t, os.WriteFile(filepath.Join(dir, "token"), []byte("ghs_existing\n"), 0o600))
		out, err := runDevcontainerInit(t, dir)
		require.NoError(t, err)
		assert.Contains(t, out, "already running; reusing its token")
	})

	t.Run("timeout", func(t *testing.T) {
		e := newInitEnv(t, "hang")
		t.Setenv("GH_TOKEN_APP_ID", "1")
		t.Setenv("GH_TOKEN_KEY", e.key)
		t.Setenv("GH_TOKEN_TIMEOUT", "1")
		_, err := runDevcontainerInit(t, filepath.Join(e.home, "tokens"))
		assert.ErrorContains(t, err, "no token after 1s")
	})

	t.Run("token_dir_inside_the_workspace", func(t *testing.T) {
		e := newInitEnv(t, "ok")
		t.Setenv("GH_TOKEN_APP_ID", "1")
		t.Setenv("GH_TOKEN_KEY", e.key)
		_, err := runDevcontainerInit(t, "--optional", filepath.Join(e.workspace, ".tokens"))
		assert.ErrorContains(t, err, "is inside the workspace")
	})

	t.Run("unsubstituted_devcontainer_id", func(t *testing.T) {
		e := newInitEnv(t, "ok")
		t.Setenv("GH_TOKEN_APP_ID", "1")
		t.Setenv("GH_TOKEN_KEY", e.key)
		runtimeDir := filepath.Join(e.home, "run")
		t.Setenv("XDG_RUNTIME_DIR", runtimeDir)

		out, err := runDevcontainerInit(t, "/x/${devcontainerId}")
		require.NoError(t, err, out)
		matches, _ := filepath.Glob(filepath.Join(runtimeDir, "gh-token", "*", "token"))
		assert.Len(t, matches, 1)
	})
}

func TestReadEnvFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "env")
	require.NoError(t, os.WriteFile(path, []byte("A=1\n\n# comment\nexport B = two words \nC=\"quoted\"\nD='single'\nE=\n"), 0o600))
	settings, err := readEnvFile(path)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"A": "1", "B": "two words", "C": "quoted", "D": "single", "E": ""}, settings)

	require.NoError(t, os.WriteFile(path, []byte("not a setting\n"), 0o600))
	_, err = readEnvFile(path)
	assert.ErrorContains(t, err, ":1: expected KEY=value")

	settings, err = readEnvFile(filepath.Join(t.TempDir(), "missing"))
	require.NoError(t, err)
	assert.Empty(t, settings)
}
