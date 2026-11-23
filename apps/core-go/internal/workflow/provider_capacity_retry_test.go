package workflow

import (
	"fmt"
	"testing"
	"time"
)

func TestProviderCapacityRetryPreservesBusinessAttemptsAndBoundsWait(t *testing.T) {
	delays := []time.Duration{30 * time.Second, time.Minute, 2 * time.Minute, 4 * time.Minute, 5 * time.Minute, 5 * time.Minute, 5 * time.Minute, 5 * time.Minute}
	for _, priorAttempts := range []int{0, 1, 2} {
		for retries, delay := range delays {
			payload := []byte(fmt.Sprintf(`{"provider_capacity_retries":%d}`, retries))
			// Dispatch reserves one business attempt; capacity must refund it,
			// even at the schedule's final business attempt.
			got := providerCapacityRetryDecision(payload, priorAttempts+1)
			if got.status != "retry" || got.attempts != priorAttempts || got.retries != retries+1 || got.delay != delay {
				t.Fatalf("prior=%d capacity=%d: %+v", priorAttempts, retries, got)
			}
		}
		got := providerCapacityRetryDecision([]byte(`{"provider_capacity_retries":8}`), priorAttempts+1)
		if got.status != "dead_letter" || got.delay != 0 || got.attempts != priorAttempts || got.retries != 9 {
			t.Fatalf("unbounded capacity retry: %+v", got)
		}
		if !providerCapacityExhausted("wake_up.current", []byte(fmt.Sprintf(`{"provider_capacity_retries":%d}`, got.retries))) {
			t.Fatal("automatic wake-up repair can bypass capacity exhaustion")
		}
	}
	first := providerCapacityRetryDecision([]byte(`{"fluctlight_id":"role"}`), 1)
	if first.status != "retry" || first.retries != 1 || first.attempts != 0 || first.delay != 30*time.Second {
		t.Fatalf("first capacity wait: %+v", first)
	}
}

func TestProviderCapacityMalformedRetryStateCannotResetAllowance(t *testing.T) {
	for _, payload := range []string{`{`, `{"provider_capacity_retries":-1}`, `{"provider_capacity_retries":100}`, `{"provider_capacity_retries":"1"}`} {
		got := providerCapacityRetryDecision([]byte(payload), 0)
		if got.status != "dead_letter" || got.attempts != 0 || got.delay != 0 {
			t.Fatalf("invalid state granted more attempts: %s %+v", payload, got)
		}
	}
}

func TestVisualIdentitySerializedCapacityFailureIsNotTerminal(t *testing.T) {
	if visualIdentityFailureIsTerminal(capacityFailureFixture) {
		t.Fatal("serialized capacity failure permanently closes visual identity workflow")
	}
	if !visualIdentityFailureIsTerminal("provider HTTP 400: invalid request") {
		t.Fatal("ordinary invalid requests must keep their terminal policy")
	}
}
