package internal

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-github/v55/github"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeClock advances only when wait is called.
type fakeClock struct {
	now   time.Time
	waits []time.Duration
	// onWait runs after each wait; returning an error stops the refresher.
	onWait func() error
}

func (c *fakeClock) Now() time.Time { return c.now }

func (c *fakeClock) wait(ctx context.Context, d time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.waits = append(c.waits, d)
	c.now = c.now.Add(d)
	if c.onWait != nil {
		return c.onWait()
	}
	return nil
}

func tokenExpiring(name string, at time.Time) *mintedToken {
	return &mintedToken{
		InstallationToken: &github.InstallationToken{Token: github.String(name), ExpiresAt: &github.Timestamp{Time: at}},
		details:           &tokenDetails{ExpiresAt: at},
	}
}

func newTestRefresher(clock *fakeClock, mint func() (*mintedToken, error), written *[]string) *refresher {
	return &refresher{
		mint: mint,
		write: func(t *mintedToken) error {
			*written = append(*written, t.GetToken())
			return nil
		},
		now:           clock.Now,
		wait:          clock.wait,
		jitter:        func(max time.Duration) time.Duration { return max }, // deterministic: full delay
		refreshBefore: 10 * time.Minute,
		log:           func(string, ...any) {},
	}
}

func TestRefresherRemintsBeforeExpiry(t *testing.T) {
	start := time.Date(2030, 1, 1, 12, 0, 0, 0, time.UTC)
	clock := &fakeClock{now: start}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var mintTimes []time.Time
	var written []string
	r := newTestRefresher(clock, func() (*mintedToken, error) {
		mintTimes = append(mintTimes, clock.now)
		if len(mintTimes) == 2 {
			cancel()
		}
		return tokenExpiring(fmt.Sprintf("ghs_%d", len(mintTimes)), clock.now.Add(time.Hour)), nil
	}, &written)

	err := r.run(ctx, tokenExpiring("ghs_0", start.Add(time.Hour)))
	require.NoError(t, err)

	assert.Equal(t, []time.Time{start.Add(50 * time.Minute), start.Add(100 * time.Minute)}, mintTimes)
	assert.Equal(t, []string{"ghs_1", "ghs_2"}, written)
	for _, d := range clock.waits {
		assert.LessOrEqual(t, d, maxWaitChunk, "waits are chunked so the wall clock is re-checked")
	}
}

func TestRefresherWallClockJump(t *testing.T) {
	// A suspended laptop: the wall clock jumps past the refresh time while
	// the process sleeps. The next chunk boundary must mint immediately.
	start := time.Date(2030, 1, 1, 12, 0, 0, 0, time.UTC)
	clock := &fakeClock{now: start}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	clock.onWait = func() error {
		if len(clock.waits) == 1 {
			clock.now = start.Add(2 * time.Hour)
		}
		return nil
	}

	var mintTimes []time.Time
	var written []string
	r := newTestRefresher(clock, func() (*mintedToken, error) {
		mintTimes = append(mintTimes, clock.now)
		cancel()
		return tokenExpiring("ghs_1", clock.now.Add(time.Hour)), nil
	}, &written)

	require.NoError(t, r.run(ctx, tokenExpiring("ghs_0", start.Add(time.Hour))))
	assert.Equal(t, []time.Time{start.Add(2 * time.Hour)}, mintTimes)
	assert.Len(t, clock.waits, 1)
}

func TestRefresherBacksOffAndKeepsOldToken(t *testing.T) {
	start := time.Date(2030, 1, 1, 12, 0, 0, 0, time.UTC)
	clock := &fakeClock{now: start}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var mintTimes []time.Time
	var written []string
	var logs []string
	r := newTestRefresher(clock, func() (*mintedToken, error) {
		mintTimes = append(mintTimes, clock.now)
		if len(mintTimes) <= 9 {
			return nil, errors.New("boom")
		}
		cancel()
		return tokenExpiring("ghs_new", clock.now.Add(time.Hour)), nil
	}, &written)
	r.log = func(format string, args ...any) { logs = append(logs, fmt.Sprintf(format, args...)) }

	require.NoError(t, r.run(ctx, tokenExpiring("ghs_old", start.Add(time.Hour))))

	// Exponential from 5s with full jitter, capped at 5m.
	var gaps []time.Duration
	for i := 1; i < len(mintTimes); i++ {
		gaps = append(gaps, mintTimes[i].Sub(mintTimes[i-1]))
	}
	assert.Equal(t, []time.Duration{
		5 * time.Second, 10 * time.Second, 20 * time.Second, 40 * time.Second, 80 * time.Second,
		160 * time.Second, 5 * time.Minute, 5 * time.Minute, 5 * time.Minute,
	}, gaps)

	// The old token stays in place until a new one is written.
	assert.Equal(t, []string{"ghs_new"}, written)
	assert.Contains(t, logs[0], "refresh failed (attempt 1), keeping the current token: boom")
	assert.Contains(t, logs[len(logs)-2], "token refreshed, expires at")
}

func TestRefresherWriteFailureIsRetried(t *testing.T) {
	start := time.Date(2030, 1, 1, 12, 0, 0, 0, time.UTC)
	clock := &fakeClock{now: start}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mints := 0
	r := newTestRefresher(clock, func() (*mintedToken, error) {
		mints++
		return tokenExpiring("ghs", clock.now.Add(time.Hour)), nil
	}, nil)
	writes := 0
	r.write = func(*mintedToken) error {
		writes++
		if writes == 1 {
			return errors.New("disk full")
		}
		cancel()
		return nil
	}

	require.NoError(t, r.run(ctx, tokenExpiring("ghs_old", start.Add(time.Hour))))
	assert.Equal(t, 2, mints)
}

func TestRefresherShortLivedToken(t *testing.T) {
	start := time.Date(2030, 1, 1, 12, 0, 0, 0, time.UTC)
	clock := &fakeClock{now: start}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var mintTimes []time.Time
	r := newTestRefresher(clock, func() (*mintedToken, error) {
		mintTimes = append(mintTimes, clock.now)
		cancel()
		return tokenExpiring("ghs", clock.now.Add(time.Hour)), nil
	}, &[]string{})

	// Expires in 5 minutes, less than refreshBefore: wait the minimum.
	require.NoError(t, r.run(ctx, tokenExpiring("ghs_old", start.Add(5*time.Minute))))
	assert.Equal(t, []time.Time{start.Add(minRefreshWait)}, mintTimes)
}

func TestRefresherRevokeOnExit(t *testing.T) {
	clock := &fakeClock{now: time.Date(2030, 1, 1, 12, 0, 0, 0, time.UTC)}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var revoked []string
	r := newTestRefresher(clock, func() (*mintedToken, error) {
		t.Fatal("must not mint after cancellation")
		return nil, nil
	}, &[]string{})
	r.wait = sleepContext
	r.revoke = func(token string) error {
		revoked = append(revoked, token)
		return nil
	}

	require.NoError(t, r.run(ctx, tokenExpiring("ghs_current", clock.now.Add(time.Hour))))
	assert.Equal(t, []string{"ghs_current"}, revoked)

	r.revoke = func(string) error { return errors.New("nope") }
	assert.EqualError(t, r.run(ctx, tokenExpiring("ghs_current", clock.now.Add(time.Hour))), "nope")
}

func TestSleepContext(t *testing.T) {
	assert.NoError(t, sleepContext(context.Background(), time.Millisecond))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	assert.ErrorIs(t, sleepContext(ctx, time.Hour), context.Canceled)
}

func TestAcquirePIDFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "pid")

	running, release, err := acquirePIDFile(path, "hash-a")
	require.NoError(t, err)
	assert.False(t, running)
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("%d\nhash-a\n", os.Getpid()), string(b))

	release()
	_, err = os.Stat(path)
	assert.True(t, os.IsNotExist(err), "release removes the pid file")

	t.Run("stale_pid_is_replaced", func(t *testing.T) {
		require.NoError(t, os.WriteFile(path, []byte("999999999\nhash-a\n"), 0o600))
		running, release, err := acquirePIDFile(path, "hash-a")
		require.NoError(t, err)
		assert.False(t, running)
		defer release()
		pid, _, err := readPIDFile(path)
		require.NoError(t, err)
		assert.Equal(t, os.Getpid(), pid)
	})

	t.Run("garbage_is_replaced", func(t *testing.T) {
		require.NoError(t, os.WriteFile(path, []byte("not a pid"), 0o600))
		running, release, err := acquirePIDFile(path, "hash-a")
		require.NoError(t, err)
		assert.False(t, running)
		release()
	})

	t.Run("live_refresher", func(t *testing.T) {
		// A live process whose command line contains --refresh.
		cmd := exec.Command("sh", "-c", "sleep 30", "--refresh")
		if err := cmd.Start(); err != nil {
			t.Skip("cannot start helper process")
		}
		defer func() {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}()
		require.NoError(t, os.WriteFile(path, []byte(fmt.Sprintf("%d\nhash-a\n", cmd.Process.Pid)), 0o600))

		running, release, err := acquirePIDFile(path, "hash-a")
		require.NoError(t, err)
		assert.True(t, running, "same scope: second instance should exit 0")
		assert.Nil(t, release)

		_, _, err = acquirePIDFile(path, "hash-b")
		assert.ErrorContains(t, err, fmt.Sprintf("another refresher (pid %d) is running for this pid file with a different scope", cmd.Process.Pid))
	})

	t.Run("live_unrelated_process_is_stale_on_linux", func(t *testing.T) {
		if _, err := os.Stat("/proc/self/cmdline"); err != nil {
			t.Skip("no /proc")
		}
		cmd := exec.Command("sleep", "30")
		require.NoError(t, cmd.Start())
		defer func() {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}()
		require.NoError(t, os.WriteFile(path, []byte(fmt.Sprintf("%d\nhash-a\n", cmd.Process.Pid)), 0o600))

		running, release, err := acquirePIDFile(path, "hash-a")
		require.NoError(t, err)
		assert.False(t, running, "a recycled PID is not a refresher")
		release()
	})
}

// TestGenerateRefreshEndToEnd runs generate --refresh with a fake clock and
// checks that the scope is fixed at start: editing the scope file after
// the first token doesn't change later requests.
func TestGenerateRefreshEndToEnd(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	dir := t.TempDir()
	scopePath := filepath.Join(dir, "scope.json")
	require.NoError(t, os.WriteFile(scopePath, []byte(`{"repositories": {"my-org/app": {"permissions": {"contents": "read"}}}}`), 0o600))
	outPath := filepath.Join(dir, "out", "token")
	pidPath := filepath.Join(dir, "out", "pid")

	start := time.Date(2030, 1, 1, 12, 0, 0, 0, time.UTC)
	clock := &fakeClock{now: start}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	oldCtx, oldWait, oldNow := newRefreshContext, refreshWait, refreshNow
	newRefreshContext = func() (context.Context, context.CancelFunc) { return ctx, cancel }
	refreshWait, refreshNow = clock.wait, clock.Now
	defer func() { newRefreshContext, refreshWait, refreshNow = oldCtx, oldWait, oldNow }()

	httpmock.RegisterResponder("GET", "https://api.github.com/repos/my-org/app/installation", httpmock.NewStringResponder(200, `{"id": 4}`))
	httpmock.RegisterResponder("DELETE", "https://api.github.com/installation/token", httpmock.NewStringResponder(204, ""))

	var bodies []string
	var tokensOnDisk []string
	httpmock.RegisterResponder("POST", "https://api.github.com/app/installations/4/access_tokens", func(req *http.Request) (*http.Response, error) {
		b, _ := io.ReadAll(req.Body)
		bodies = append(bodies, string(b))
		if len(bodies) == 1 {
			// Widen the scope file after the first token; must be ignored.
			_ = os.WriteFile(scopePath, []byte(`{"repositories": {"my-org/*": {"permissions": "write-all"}}}`), 0o600)
		} else {
			onDisk, _ := os.ReadFile(outPath)
			tokensOnDisk = append(tokensOnDisk, string(onDisk))
		}
		if len(bodies) == 3 {
			cancel()
		}
		expires := clock.now.Add(time.Hour).Format(time.RFC3339)
		return httpmock.NewStringResponse(201, fmt.Sprintf(`{"token": "ghs_%d", "expires_at": %q, "permissions": {"contents": "read"}, "repository_selection": "selected"}`, len(bodies), expires)), nil
	})

	_, errOut, err := runGenerate(t, "-i", "123", "-k", testKey, "-f", scopePath, "--output-file", outPath,
		"--refresh", "--refresh-before", "5m", "--pid-file", pidPath, "--revoke-on-exit")
	require.NoError(t, err)

	want := `{"repositories":["app"],"permissions":{"contents":"read"}}`
	assert.Equal(t, []string{want, want, want}, bodies)
	assert.Equal(t, []string{"ghs_1\n", "ghs_2\n"}, tokensOnDisk, "old token stays in place until the new one is written")
	onDisk, err := os.ReadFile(outPath)
	require.NoError(t, err)
	assert.Equal(t, "ghs_3\n", string(onDisk))

	info := httpmock.GetCallCountInfo()
	assert.Equal(t, 1, info["DELETE https://api.github.com/installation/token"], "revoke on exit")
	_, err = os.Stat(pidPath)
	assert.True(t, os.IsNotExist(err), "pid file removed on exit")
	assert.Contains(t, errOut, "refreshing "+outPath+" 5m0s before expiry")
	assert.Contains(t, errOut, "current token revoked")
}

func TestGenerateRefreshFlagErrors(t *testing.T) {
	tests := []struct {
		args          []string
		expectedError string
	}{
		{[]string{"--refresh"}, "--refresh requires --output-file"},
		{[]string{"--output-file", "x", "--pid-file", "p"}, "--revoke-on-exit and --pid-file require --refresh"},
		{[]string{"--output-file", "x", "--revoke-on-exit"}, "--revoke-on-exit and --pid-file require --refresh"},
		{[]string{"--output-file", "x", "--refresh", "--refresh-before", "55m"}, "--refresh-before must be between 0 and 50m0s; tokens live for one hour"},
	}
	for _, tt := range tests {
		t.Run(tt.expectedError, func(t *testing.T) {
			_, _, err := runGenerate(t, append([]string{"-i", "123", "-k", testKey, "-l", "1"}, tt.args...)...)
			assert.EqualError(t, err, tt.expectedError)
		})
	}
}
