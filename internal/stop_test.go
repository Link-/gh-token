package internal

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
)

func runStop(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	old := stdout
	stdout = &out
	defer func() { stdout = old }()

	app := &cli.App{Commands: []*cli.Command{{Name: "stop", Flags: StopFlags(), Action: Stop}}}
	err := app.Run(append([]string{"gh-token", "stop"}, args...))
	return out.String(), err
}

func TestStop(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "pid")

	t.Run("nothing_running", func(t *testing.T) {
		out, err := runStop(t, dir)
		require.NoError(t, err)
		assert.Equal(t, "no refresher is running for "+dir+"\n", out)
	})

	t.Run("stale_pid_file", func(t *testing.T) {
		require.NoError(t, os.WriteFile(pidFile, []byte("999999999\nhash\n"), 0o600))
		out, err := runStop(t, dir)
		require.NoError(t, err)
		assert.Contains(t, out, "removed a stale pid file")
		_, err = os.Stat(pidFile)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("unrelated_process_is_not_killed", func(t *testing.T) {
		if _, err := exec.LookPath("sleep"); err != nil {
			t.Skip("no sleep")
		}
		cmd := exec.Command("sleep", "30")
		require.NoError(t, cmd.Start())
		defer func() {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}()
		require.NoError(t, os.WriteFile(pidFile, []byte(fmt.Sprintf("%d\nhash\n", cmd.Process.Pid)), 0o600))

		out, err := runStop(t, pidFile)
		require.NoError(t, err)
		assert.Contains(t, out, "removed a stale pid file")
		assert.Nil(t, cmd.ProcessState, "sleep must still be running")
	})

	t.Run("stops_a_refresher", func(t *testing.T) {
		// A stand-in whose command line contains --refresh and that exits on SIGTERM.
		cmd := exec.Command("sh", "-c", `trap "exit 0" TERM; while :; do sleep 0.1; done`, "--refresh")
		if err := cmd.Start(); err != nil {
			t.Skip("cannot start helper process")
		}
		exited := make(chan error, 1)
		go func() { exited <- cmd.Wait() }()
		defer func() { _ = cmd.Process.Kill() }()
		require.NoError(t, os.WriteFile(pidFile, []byte(fmt.Sprintf("%d\nhash\n", cmd.Process.Pid)), 0o600))

		out, err := runStop(t, dir)
		require.NoError(t, err)
		assert.Equal(t, fmt.Sprintf("stopped refresher %d\n", cmd.Process.Pid), out)
		select {
		case <-exited:
		case <-time.After(5 * time.Second):
			t.Fatal("the refresher is still running")
		}
		_, err = os.Stat(pidFile)
		assert.True(t, os.IsNotExist(err), "pid file removed")
	})

	t.Run("usage", func(t *testing.T) {
		_, err := runStop(t)
		assert.EqualError(t, err, "usage: gh token stop <token directory or pid file>")
	})
}
