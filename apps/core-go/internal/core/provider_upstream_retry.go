package core

import (
	"context"
	"errors"
	"math/rand/v2"
	"strings"
	"time"
)

const (
	// upstreamProviderMaxRetries is the shared cap for one physical model call.
	// The first request plus two retries is the upper end of the 2–3 retry budget
	// only when every attempt fails; a successful attempt stops immediately.
	upstreamProviderMaxRetries = 2
	upstreamRetryBaseDelay     = 200 * time.Millisecond
	upstreamRetryMaxDelay      = 2 * time.Second
)

type upstreamModelRetryContextKey struct{}

// withUpstreamModelRetry marks one call whose formal scenario is shared with
// other work. Owner life requests use it so native cognition stays unretriable.
func withUpstreamModelRetry(ctx context.Context) context.Context {
	return context.WithValue(ctx, upstreamModelRetryContextKey{}, true)
}

func upstreamModelRetryScenario(scenario string) bool {
	switch strings.TrimSpace(scenario) {
	case "fact_only_reply", "cognitive_assessment", "initialization", "persona_compilation", "studio_assistant", "studio_text_preview":
		return true
	default:
		return false
	}
}

func upstreamModelRetryEnabled(ctx context.Context, scenario string) bool {
	if upstreamModelRetryScenario(scenario) {
		return true
	}
	if ctx == nil {
		return false
	}
	enabled, _ := ctx.Value(upstreamModelRetryContextKey{}).(bool)
	return enabled
}

func providerUpstreamRetryable(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var response *providerHTTPResponseError
	if errors.As(err, &response) {
		return providerHTTPUpstreamRetryable(response)
	}
	text := strings.ToLower(err.Error())
	if strings.Contains(text, "insufficient_user_quota") || strings.Contains(text, "insufficient_quota") {
		return false
	}
	return strings.Contains(text, "get_channel_failed") && !strings.Contains(text, "provider http 4")
}

// modelTransportRetryable is the shared 5xx policy. Reply also keeps its
// existing reconnect retry for a dropped socket; that stays on the model call
// and does not extend to cognition or other scenarios.
func modelTransportRetryable(scenario string, err error) bool {
	if providerUpstreamRetryable(err) {
		return true
	}
	return strings.TrimSpace(scenario) == "fact_only_reply" && replyNetworkRetryable(err)
}

func upstreamRetryBackoff(retryIndex int) time.Duration {
	if retryIndex < 1 {
		retryIndex = 1
	}
	delay := upstreamRetryBaseDelay
	for step := 1; step < retryIndex; step++ {
		if delay >= upstreamRetryMaxDelay/2 {
			delay = upstreamRetryMaxDelay
			break
		}
		delay *= 2
	}
	if delay > upstreamRetryMaxDelay {
		delay = upstreamRetryMaxDelay
	}
	if delay <= 0 {
		return 0
	}
	return time.Duration(rand.Int64N(int64(delay) + 1))
}

var upstreamRetrySleep = func(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// retryUpstreamModelCall repeats only the closure. Callers must put tool
// execution and other side effects outside it and keep the idempotency key
// that was captured before the loop.
func retryUpstreamModelCall[T any](ctx context.Context, scenario string, fn func() (T, error)) (T, error) {
	var zero T
	var last error
	for attempt := 0; attempt <= upstreamProviderMaxRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return zero, err
		}
		value, err := fn()
		if err == nil || !modelTransportRetryable(scenario, err) || attempt == upstreamProviderMaxRetries {
			return value, err
		}
		last = err
		if sleepErr := upstreamRetrySleep(ctx, upstreamRetryBackoff(attempt+1)); sleepErr != nil {
			return zero, sleepErr
		}
	}
	return zero, last
}
