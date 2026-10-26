package internal

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

// acquirePIDFile makes this process the single refresher for path. The file
// holds "<pid>\n<scope hash>\n". If a live refresher with the same scope
// hash holds it, alreadyRunning is true and the caller should exit
// successfully. A live refresher with a different scope is an error, and a
// stale file is replaced. release removes the file if it is still ours.
func acquirePIDFile(path, scopeHash string) (alreadyRunning bool, release func(), err error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return false, nil, fmt.Errorf("unable to create pid file directory: %w", err)
	}

	content := fmt.Sprintf("%d\n%s\n", os.Getpid(), scopeHash)
	for attempt := 0; attempt < 2; attempt++ {
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil {
			_, werr := f.WriteString(content)
			cerr := f.Close()
			if werr != nil || cerr != nil {
				_ = os.Remove(path)
				return false, nil, fmt.Errorf("unable to write pid file: %w", errors.Join(werr, cerr))
			}
			release := func() {
				if b, err := os.ReadFile(path); err == nil && string(b) == content {
					_ = os.Remove(path)
				}
			}
			return false, release, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return false, nil, fmt.Errorf("unable to create pid file: %w", err)
		}

		pid, hash, readErr := readPIDFile(path)
		if readErr == nil && pid != os.Getpid() && processAlive(pid) {
			if hash == scopeHash {
				return true, nil, nil
			}
			return false, nil, fmt.Errorf("another refresher (pid %d) is running for this pid file with a different scope; stop it first (kill %d)", pid, pid)
		}

		// Stale or unreadable: replace it.
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return false, nil, fmt.Errorf("unable to remove stale pid file: %w", err)
		}
	}

	return false, nil, fmt.Errorf("unable to acquire pid file %s", path)
}

func readPIDFile(path string) (int, string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, "", err
	}
	lines := strings.SplitN(strings.TrimSpace(string(b)), "\n", 2)
	pid, err := strconv.Atoi(strings.TrimSpace(lines[0]))
	if err != nil || pid <= 0 {
		return 0, "", fmt.Errorf("invalid pid file")
	}
	hash := ""
	if len(lines) == 2 {
		hash = strings.TrimSpace(lines[1])
	}
	return pid, hash, nil
}

// processAlive reports whether pid is a running gh-token refresher. Where
// the command line can be read (/proc on Linux, ps elsewhere on Unix) it
// must contain --refresh, so a recycled PID isn't mistaken for one.
func processAlive(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	if runtime.GOOS == "windows" {
		// FindProcess opens a handle and fails if the process doesn't exist.
		return true
	}
	if err := p.Signal(syscall.Signal(0)); err != nil && !errors.Is(err, os.ErrPermission) {
		return false
	}
	if cmdline, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid)); err == nil {
		return strings.Contains(string(cmdline), "--refresh")
	}
	if out, err := exec.Command("ps", "-o", "command=", "-p", strconv.Itoa(pid)).Output(); err == nil {
		return strings.Contains(string(out), "--refresh")
	}
	return true
}
