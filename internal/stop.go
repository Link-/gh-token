package internal

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/urfave/cli/v2"
)

// StopFlags returns the CLI flags for the stop command
func StopFlags() []cli.Flag {
	return []cli.Flag{
		&cli.DurationFlag{
			Name:  "timeout",
			Usage: "How long to wait for the refresher to exit",
			Value: 10 * time.Second,
		},
		&cli.BoolFlag{
			Name:    "silent",
			Usage:   "Do not print to stdout",
			Aliases: []string{"s"},
		},
	}
}

// Stop is the entrypoint for the stop command: it stops the refresher that
// keeps a token directory fresh (see generate --refresh --pid-file).
func Stop(c *cli.Context) error {
	target := c.Args().First()
	if target == "" || c.NArg() > 1 {
		return fmt.Errorf("usage: gh token stop <token directory or pid file>")
	}
	say := func(format string, args ...any) {
		if !c.Bool("silent") {
			_, _ = fmt.Fprintf(stdout, format+"\n", args...)
		}
	}

	pidFile := target
	if st, err := os.Stat(target); err == nil && st.IsDir() {
		pidFile = filepath.Join(target, "pid")
	}

	pid, _, err := readPIDFile(pidFile)
	if errors.Is(err, os.ErrNotExist) {
		say("no refresher is running for %s", target)
		return nil
	}
	if err != nil {
		return fmt.Errorf("unable to read pid file %s: %w", pidFile, err)
	}
	if !processAlive(pid) {
		_ = os.Remove(pidFile)
		say("no refresher is running for %s (removed a stale pid file)", target)
		return nil
	}

	if err := terminate(pid); err != nil {
		return fmt.Errorf("unable to stop refresher %d: %w", pid, err)
	}
	deadline := time.Now().Add(c.Duration("timeout"))
	for processAlive(pid) {
		if time.Now().After(deadline) {
			return fmt.Errorf("refresher %d did not exit within %s", pid, c.Duration("timeout"))
		}
		time.Sleep(100 * time.Millisecond)
	}
	// The refresher removes its pid file on a clean exit; clean up otherwise.
	if p, _, err := readPIDFile(pidFile); err == nil && p == pid {
		_ = os.Remove(pidFile)
	}
	say("stopped refresher %d", pid)

	return nil
}

// terminate asks a process to exit: SIGTERM, so a refresher started with
// --revoke-on-exit revokes its token; Windows has no SIGTERM.
func terminate(pid int) error {
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		return p.Kill()
	}
	return p.Signal(syscall.SIGTERM)
}
