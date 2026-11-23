package workflow

import (
	"encoding/json"
	"time"
)

const providerCapacityMaximumRetries = 8

const providerCapacityExhaustedSQL = `UPDATE public.platform_workflow_intents
SET status='dead_letter',next_attempt_at=NULL,started_at=NULL,completed_at=now(),last_error='provider_capacity_wait_exhausted'
WHERE intent_id=$1 AND payload=$2::jsonb AND status IN ('pending','retry')`

// Payload CAS and the started/failed guard refund each dispatched attempt only
// once, including when multiple reconcilers observe the same terminal Run.
const providerCapacityRetrySQL = `UPDATE public.platform_workflow_intents
SET status=$4::varchar,attempt_count=$5,
    payload=payload||jsonb_build_object('provider_capacity_retries',$6::int),
    next_attempt_at=CASE WHEN $4::varchar='retry' THEN now()+($7 * interval '1 second') ELSE NULL END,
    started_at=NULL,completed_at=CASE WHEN $4::varchar='dead_letter' THEN now() ELSE NULL END,
    last_error=CASE WHEN $4::varchar='dead_letter' THEN 'provider_capacity_wait_exhausted: '||$8::text ELSE $8::text END
WHERE intent_id=$1 AND payload=$2::jsonb AND attempt_count=$3 AND status IN ('started','failed')`

func providerCapacityIntent(kind string) bool {
	switch kind {
	case "schedule.current_day", "wardrobe.initialize", "visual_identity.initialize", "wake_up.current":
		return true
	}
	return false
}

func providerCapacityQueued(kind string, payload []byte) bool {
	if !providerCapacityIntent(kind) {
		return false
	}
	var state struct {
		Retries int `json:"provider_capacity_retries"`
	}
	return json.Unmarshal(payload, &state) == nil && state.Retries > 0
}

func providerCapacityExhausted(kind string, payload []byte) bool {
	if !providerCapacityIntent(kind) {
		return false
	}
	var state struct {
		Retries int `json:"provider_capacity_retries"`
	}
	return json.Unmarshal(payload, &state) == nil && state.Retries > providerCapacityMaximumRetries
}

type capacityRetryDecision struct {
	status   string
	attempts int
	retries  int
	delay    time.Duration
}

func providerCapacityRetryDecision(payload []byte, attemptCount int) capacityRetryDecision {
	var state struct {
		Retries int `json:"provider_capacity_retries"`
	}
	if err := json.Unmarshal(payload, &state); err != nil || state.Retries < 0 || state.Retries > providerCapacityMaximumRetries {
		// Corrupt retry state cannot grant an unbounded new waiting allowance.
		state.Retries = providerCapacityMaximumRetries
	}
	decision := capacityRetryDecision{status: "dead_letter", attempts: max(0, attemptCount-1), retries: state.Retries + 1}
	if state.Retries >= providerCapacityMaximumRetries {
		return decision
	}
	decision.status = "retry"
	decision.delay = min(30*time.Second*time.Duration(1<<state.Retries), 5*time.Minute)
	return decision
}
