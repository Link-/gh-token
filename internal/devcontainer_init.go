package internal

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/urfave/cli/v2"
)

// DevcontainerInitFlags returns the CLI flags for the devcontainer-init command
func DevcontainerInitFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:    "app-id",
			Usage:   "GitHub App ID [$GH_TOKEN_APP_ID, or the env file]",
			Aliases: []string{"i"},
		},
		&cli.StringFlag{
			Name:    "key",
			Usage:   "Path to the App's private key [$GH_TOKEN_KEY, or the env file; default: app.pem next to the env file]",
			Aliases: []string{"k"},
		},
		&cli.StringFlag{
			Name:    "hostname",
			Usage:   "GitHub Enterprise Server hostname [$GH_TOKEN_HOSTNAME, or the env file]",
			Aliases: []string{"o"},
		},
		&cli.StringFlag{
			Name:    "scope-file",
			Usage:   "Scope file, relative to the workspace [$GH_TOKEN_SCOPE_FILE, or the env file] (default: .devcontainer/devcontainer.json)",
			Aliases: []string{"f"},
		},
		&cli.StringFlag{
			Name:  "env-file",
			Usage: "File with KEY=value settings [$GH_TOKEN_ENV_FILE] (default: ${XDG_CONFIG_HOME:-~/.config}/gh-token/devcontainer.env)",
		},
		&cli.BoolFlag{
			Name:  "optional",
			Usage: "When the host isn't set up or minting fails, warn and let the container start without a token [$GH_TOKEN_OPTIONAL=1]",
		},
		&cli.DurationFlag{
			Name:  "timeout",
			Usage: "How long to wait for the first token [$GH_TOKEN_TIMEOUT]",
			Value: 30 * time.Second,
		},
		&cli.BoolFlag{
			Name:  "merge-permissions",
			Usage: "Pass --merge-permissions to the refresher: accept differing per-repository permissions by merging them (widens access) [$GH_TOKEN_MERGE_PERMISSIONS=1]",
		},
		&cli.DurationFlag{
			Name:  "max-lifetime",
			Usage: "Stop the refresher after this long, e.g. 12h [$GH_TOKEN_MAX_LIFETIME] (default: run until stopped)",
		},
	}
}

// Replaceable in tests: the command that runs the refresher, normally this
// same binary.
var refresherCommand = func(args []string) (*exec.Cmd, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("unable to find the gh-token executable: %w", err)
	}
	return exec.Command(exe, args...), nil
}

// errNotConfigured marks problems that --optional turns into a warning.
type errNotConfigured struct{ msg string }

func (e errNotConfigured) Error() string { return e.msg }

func notConfigured(format string, args ...any) error {
	return errNotConfigured{msg: fmt.Sprintf(format, args...)}
}

// DevcontainerInit is the entrypoint for the devcontainer-init command. It
// runs on the host from initializeCommand: it starts a refresher that keeps
// a downscoped token in the token directory, which devcontainer.json mounts
// into the container, and waits for the first token.
//
// It is part of the installed extension, so initializeCommand runs no code
// from the (agent-writable) workspace; only the scope is read from there.
func DevcontainerInit(c *cli.Context) error {
	if c.NArg() > 1 {
		return fmt.Errorf("usage: gh token devcontainer-init [options] <token directory>")
	}
	say := func(format string, args ...any) {
		_, _ = fmt.Fprintf(stderr, "gh-token devcontainer-init: "+format+"\n", args...)
	}

	workspace, err := os.Getwd()
	if err != nil {
		return err
	}
	if resolved, err := filepath.EvalSymlinks(workspace); err == nil {
		workspace = resolved
	}

	envFile := c.String("env-file")
	if envFile == "" {
		envFile = defaultEnvFile()
	}
	fileSettings, err := readEnvFile(envFile)
	if err != nil {
		return err
	}
	setting := func(flag, env string) string {
		if c.IsSet(flag) {
			return c.String(flag)
		}
		if v := os.Getenv(env); v != "" {
			return v
		}
		return fileSettings[env]
	}
	optional := c.Bool("optional") || isTrue(setting("", "GH_TOKEN_OPTIONAL"))

	// The token directory is the bind mount's source: create it before
	// anything can fail, or Docker refuses to start the container.
	tokenDir, err := prepareTokenDir(c.Args().First(), workspace)
	if err != nil {
		return err
	}
	say("token directory %s", tokenDir)

	err = startRefresher(c, tokenDir, workspace, envFile, setting, say)
	var nc errNotConfigured
	if err != nil && optional && (errors.As(err, &nc) || errors.Is(err, errRefresherFailed)) {
		say("WARNING: %v", err)
		say("starting the container without a GitHub token (--optional)")
		return nil
	}

	return err
}

var errRefresherFailed = errors.New("the refresher failed")

func startRefresher(c *cli.Context, tokenDir, workspace, envFile string, setting func(flag, env string) string, say func(string, ...any)) error {
	hostname := setting("hostname", "GH_TOKEN_HOSTNAME")
	host := webHostForAPI(normalizeHostname(hostname))
	// The gh shim and credential helper read this to know which host they serve.
	if err := writeFileAtomic(filepath.Join(tokenDir, "host"), []byte(host+"\n")); err != nil {
		return fmt.Errorf("unable to write the host file: %w", err)
	}

	appID := setting("app-id", "GH_TOKEN_APP_ID")
	if appID == "" {
		return notConfigured("no GitHub App ID: set GH_TOKEN_APP_ID, or add it to %s", envFile)
	}
	key := expandHome(setting("key", "GH_TOKEN_KEY"))
	if key == "" {
		key = filepath.Join(filepath.Dir(envFile), "app.pem")
		if _, err := os.Stat(key); err != nil {
			return notConfigured("no App private key: set GH_TOKEN_KEY, add it to %s, or put the key at %s", envFile, key)
		}
	}
	if f, err := os.Open(key); err != nil {
		return notConfigured("cannot read the App private key %s: %v", key, err)
	} else {
		_ = f.Close()
	}
	scopeFile := setting("scope-file", "GH_TOKEN_SCOPE_FILE")
	if scopeFile == "" {
		scopeFile = filepath.Join(".devcontainer", "devcontainer.json")
	}
	if _, err := os.Stat(scopeFile); err != nil {
		return notConfigured("cannot read the scope file %s: %v", scopeFile, err)
	}

	args := []string{"generate",
		"--app-id", appID,
		"--key", key,
		"--scope-file", scopeFile,
		"--git-dir", workspace,
		"--output-file", filepath.Join(tokenDir, "token"),
		"--refresh",
		"--pid-file", filepath.Join(tokenDir, "pid"),
		// gh token stop then revokes the token too.
		"--revoke-on-exit",
	}
	if c.Bool("merge-permissions") || isTrue(setting("", "GH_TOKEN_MERGE_PERMISSIONS")) {
		args = append(args, "--merge-permissions")
	}
	if hostname != "" {
		args = append(args, "--hostname", hostname)
	}
	maxLifetime := c.Duration("max-lifetime")
	if !c.IsSet("max-lifetime") {
		if v := setting("", "GH_TOKEN_MAX_LIFETIME"); v != "" {
			d, err := time.ParseDuration(v)
			if err != nil {
				return fmt.Errorf("invalid GH_TOKEN_MAX_LIFETIME %q: %w", v, err)
			}
			maxLifetime = d
		}
	}
	if maxLifetime > 0 {
		args = append(args, "--max-lifetime", maxLifetime.String())
	}
	timeout := c.Duration("timeout")
	if !c.IsSet("timeout") {
		if v := setting("", "GH_TOKEN_TIMEOUT"); v != "" {
			d, err := time.ParseDuration(v)
			if err != nil {
				// Plain seconds.
				d, err = time.ParseDuration(v + "s")
			}
			if err != nil {
				return fmt.Errorf("invalid GH_TOKEN_TIMEOUT %q: %w", v, err)
			}
			timeout = d
		}
	}

	tokenPath := filepath.Join(tokenDir, "token")
	before, _ := os.ReadFile(tokenPath)

	logPath := filepath.Join(tokenDir, "refresh.log")
	logFile, err := os.OpenFile(logPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("unable to open %s: %w", logPath, err)
	}
	defer func() { _ = logFile.Close() }()

	cmd, err := refresherCommand(args)
	if err != nil {
		return err
	}
	cmd.Stdin = nil
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	detach(cmd)
	say("starting the refresher (log: %s)", logPath)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("unable to start the refresher: %w", err)
	}
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if now, err := os.ReadFile(tokenPath); err == nil && len(now) > 0 && !bytes.Equal(now, before) {
			say("token ready")
			return nil
		}
		select {
		case err := <-exited:
			if err == nil {
				// Exit 0 without a new token: a refresher with the same
				// scope is already running and keeps the token fresh.
				if len(before) == 0 {
					return fmt.Errorf("%w: it exited without a token; see %s", errRefresherFailed, logPath)
				}
				say("a refresher with the same scope is already running; reusing its token")
				return nil
			}
			return fmt.Errorf("%w (%v); last lines of %s:\n%s\nif the App is not installed on this repository (404), install it on the repository or its owner",
				errRefresherFailed, err, logPath, logTail(logPath, 20))
		case <-time.After(200 * time.Millisecond):
		}
	}

	return fmt.Errorf("%w: no token after %s; last lines of %s:\n%s", errRefresherFailed, timeout, logPath, logTail(logPath, 20))
}

// prepareTokenDir creates the token directory with mode 0700 and refuses
// one inside the workspace, where the agent could read or commit it. An
// empty or unsubstituted ${devcontainerId} path falls back to a directory
// named after a hash of the workspace path.
func prepareTokenDir(dir, workspace string) (string, error) {
	if dir == "" || strings.Contains(dir, "${") {
		base := os.Getenv("XDG_RUNTIME_DIR")
		if base == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			base = filepath.Join(home, ".cache")
		}
		sum := sha256.Sum256([]byte(workspace))
		dir = filepath.Join(base, "gh-token", hex.EncodeToString(sum[:])[:16])
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("unable to create token directory %s: %w", dir, err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return "", err
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
	}
	if rel, err := filepath.Rel(workspace, abs); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("token directory %s is inside the workspace %s; choose a path outside it", abs, workspace)
	}
	return abs, nil
}

func defaultEnvFile() string {
	if f := os.Getenv("GH_TOKEN_ENV_FILE"); f != "" {
		return expandHome(f)
	}
	config := os.Getenv("XDG_CONFIG_HOME")
	if config == "" {
		home, _ := os.UserHomeDir()
		config = filepath.Join(home, ".config")
	}
	return filepath.Join(config, "gh-token", "devcontainer.env")
}

// readEnvFile reads simple KEY=value lines; blank lines, # comments and an
// "export " prefix are allowed, and values may be quoted. It is parsed, not
// executed. A missing file is not an error.
func readEnvFile(path string) (map[string]string, error) {
	settings := map[string]string{}
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return settings, nil
	}
	if err != nil {
		return nil, fmt.Errorf("unable to read %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	for n := 1; scanner.Scan(); n++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("%s:%d: expected KEY=value", path, n)
		}
		value = strings.TrimSpace(value)
		if len(value) >= 2 && (value[0] == '"' || value[0] == '\'') && value[len(value)-1] == value[0] {
			value = value[1 : len(value)-1]
		}
		settings[strings.TrimSpace(key)] = value
	}
	return settings, scanner.Err()
}

func expandHome(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(path, "~"))
		}
	}
	return path
}

func isTrue(v string) bool {
	switch strings.ToLower(v) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// normalizeHostname turns a --hostname value into the API host, as
// Generate does.
func normalizeHostname(hostname string) string {
	hostname = strings.ToLower(strings.TrimPrefix(hostname, "https://"))
	if hostname == "" || hostname == "github.com" || hostname == "api.github.com" {
		return "api.github.com"
	}
	if !strings.Contains(hostname, "/api/v3") {
		hostname = strings.TrimSuffix(hostname, "/") + "/api/v3"
	}
	return hostname
}

func logTail(path string, lines int) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	all := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	if len(all) > lines {
		all = all[len(all)-lines:]
	}
	return strings.Join(all, "\n")
}
