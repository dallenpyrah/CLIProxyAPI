package auth

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v8/sdk/cliproxy/executor"
)

// EarliestWeeklyResetSelector spends the eligible account whose observed weekly
// allowance resets soonest. Static priority and session affinity remain authoritative.
// Unknown or expired observations use round-robin when no weekly reset is known.
type EarliestWeeklyResetSelector struct {
	fallback RoundRobinSelector
}

func (s *EarliestWeeklyResetSelector) Pick(ctx context.Context, provider, model string, opts cliproxyexecutor.Options, auths []*Auth) (*Auth, error) {
	now := time.Now()
	available, err := getSelectorAvailableAuths(ctx, auths, provider, model, now)
	if err != nil {
		return nil, err
	}
	available = preferCodexWebsocketAuths(ctx, provider, available)
	var selected *Auth
	var earliest time.Time
	for _, candidate := range available {
		reset := authWeeklyReset(candidate, now)
		if !reset.IsZero() && (selected == nil || reset.Before(earliest)) {
			selected, earliest = candidate, reset
		}
	}
	if selected != nil {
		return selected, nil
	}
	return s.fallback.Pick(ctx, provider, model, opts, available)
}

func authWeeklyReset(auth *Auth, now time.Time) time.Time {
	signals := auth.Quota.Signals
	value := func(name string) string { return strings.TrimSpace(signals[http.CanonicalHeaderKey(name)]) }
	var earliest time.Time
	consider := func(reset time.Time) {
		if reset.After(now) && !reset.After(now.Add(7*24*time.Hour+time.Hour)) && (earliest.IsZero() || reset.Before(earliest)) {
			earliest = reset
		}
	}
	switch strings.ToLower(strings.TrimSpace(auth.Provider)) {
	case "claude":
		if seconds, err := strconv.ParseInt(value("Anthropic-Ratelimit-Unified-7d-Reset"), 10, 64); err == nil {
			consider(time.Unix(seconds, 0))
		}
	case "codex":
		for _, window := range []string{"Primary", "Secondary"} {
			prefix := "X-Codex-" + window + "-"
			if minutes, err := strconv.ParseInt(value(prefix+"Window-Minutes"), 10, 64); err != nil || minutes != 7*24*60 {
				continue
			}
			if seconds, err := strconv.ParseInt(value(prefix+"Reset-At"), 10, 64); err == nil && seconds > 0 {
				consider(time.Unix(seconds, 0))
				continue
			}
			if seconds, err := strconv.ParseInt(value(prefix+"Reset-After-Seconds"), 10, 64); err == nil && seconds > 0 && seconds <= 7*24*60*60 && !auth.Quota.ObservedAt.IsZero() {
				consider(auth.Quota.ObservedAt.Add(time.Duration(seconds) * time.Second))
			}
		}
	}
	return earliest
}
