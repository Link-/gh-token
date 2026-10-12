package internal

import (
	"context"
	"math/rand/v2"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	// minRefreshWait keeps a token that is already close to expiry from
	// causing a busy loop.
	minRefreshWait = 30 * time.Second
	initialBackoff = 5 * time.Second
	maxBackoff     = 5 * time.Minute
	// maxWaitChunk bounds each sleep so the wall clock is re-checked, for
	// example after a laptop resumes from suspend (monotonic timers don't
	// advance while suspended).
	maxWaitChunk = time.Minute
)

// Replaceable in tests.
var (
	newRefreshContext = func() (context.Context, context.CancelFunc) {
		return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	}
	refreshWait = sleepContext
	refreshNow  = time.Now
)

func sleepContext(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// refresher keeps a token file fresh. The scope is captured in mint when the
// refresher is created and never re-read, so edits to the workspace (which
// an agent can make) cannot widen later tokens.
type refresher struct {
	mint          func() (*mintedToken, error)
	write         func(*mintedToken) error
	revoke        func(token string) error // nil unless --revoke-on-exit
	now           func() time.Time
	wait          func(ctx context.Context, d time.Duration) error
	jitter        func(max time.Duration) time.Duration
	refreshBefore time.Duration
	// stopAt, when set, ends the refresher (as if signalled) at that time.
	stopAt time.Time
	log    func(format string, args ...any)
}

// run re-mints the token refreshBefore its expiry until ctx is cancelled.
// On failure it keeps the current token in place and retries with
// exponential backoff and jitter, capped at maxBackoff.
func (r *refresher) run(ctx context.Context, current *mintedToken) error {
	failures := 0
	for {
		var deadline time.Time
		if failures == 0 {
			deadline = r.refreshAt(current)
		} else {
			deadline = r.now().Add(r.backoff(failures))
		}

		if !r.stopAt.IsZero() && r.stopAt.Before(deadline) {
			deadline = r.stopAt
		}
		if err := r.waitUntil(ctx, deadline); err != nil {
			return r.shutdown(current)
		}
		if !r.stopAt.IsZero() && !r.now().Before(r.stopAt) {
			r.log("maximum lifetime reached")
			return r.shutdown(current)
		}

		token, err := r.mint()
		if err == nil {
			err = r.write(token)
		}
		if err != nil {
			failures++
			r.log("refresh failed (attempt %d), keeping the current token: %v", failures, err)
			continue
		}

		failures = 0
		current = token
		r.log("token refreshed, expires at %s", expiresAt(current, r.now()).UTC().Format(time.RFC3339))
	}
}

// refreshAt returns when to re-mint: refreshBefore the token's expiry, but
// at least minRefreshWait from now.
func (r *refresher) refreshAt(token *mintedToken) time.Time {
	now := r.now()
	at := expiresAt(token, now).Add(-r.refreshBefore)
	if earliest := now.Add(minRefreshWait); at.Before(earliest) {
		return earliest
	}
	return at
}

// expiresAt returns the token's expiry, assuming the documented one hour
// lifetime when the response didn't include one.
func expiresAt(token *mintedToken, now time.Time) time.Time {
	if t := token.GetExpiresAt().Time; !t.IsZero() {
		return t
	}
	return now.Add(time.Hour)
}

// backoff returns the delay before retry number n (1-based): exponential
// from initialBackoff, capped at maxBackoff, with the upper half jittered.
func (r *refresher) backoff(n int) time.Duration {
	d := maxBackoff
	if n < 10 {
		d = min(initialBackoff<<(n-1), maxBackoff)
	}
	return d/2 + r.jitter(d/2)
}

func (r *refresher) waitUntil(ctx context.Context, deadline time.Time) error {
	for {
		remaining := deadline.Sub(r.now())
		if remaining <= 0 {
			return ctx.Err()
		}
		if err := r.wait(ctx, min(remaining, maxWaitChunk)); err != nil {
			return err
		}
	}
}

func (r *refresher) shutdown(current *mintedToken) error {
	if r.revoke == nil {
		r.log("stopping")
		return nil
	}
	if err := r.revoke(current.GetToken()); err != nil {
		r.log("stopping; failed revoking the current token: %v", err)
		return err
	}
	r.log("stopping; current token revoked")
	return nil
}

func randomJitter(max time.Duration) time.Duration {
	if max <= 0 {
		return 0
	}
	return rand.N(max + 1)
}
