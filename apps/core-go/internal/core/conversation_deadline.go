package core

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// A durable turn owns a total reply budget, independent of any HTTP observer or
// physical Provider attempt. Only an explicit retry admits a fresh budget.
func conversationTurnBudget(now time.Time, seconds int, version int64, attempt int) map[string]any {
	if seconds < 1 {
		seconds = defaultConversationReplySettings().TimeoutSeconds
	}
	if seconds > 3600 {
		seconds = 3600
	}
	return map[string]any{"admitted_at": now.UTC().Format(time.RFC3339Nano), "deadline_at": now.Add(time.Duration(seconds) * time.Second).UTC().Format(time.RFC3339Nano), "timeout_seconds": seconds, "config_version": version, "attempt": attempt}
}

func (a *App) newConversationTurnBudget(ctx context.Context, attempt int) (map[string]any, error) {
	settings, version, err := a.conversationReplySettings(ctx)
	if err != nil {
		return nil, err
	}
	return conversationTurnBudget(a.now(), settings.TimeoutSeconds, version, attempt), nil
}

func (a *App) cognitionTurnDeadline(ctx context.Context, inboxID string) (time.Time, map[string]any, bool, error) {
	var raw []byte
	var occurred time.Time
	var status string
	err := a.DB.Pool().QueryRow(ctx, `SELECT payload,occurred_at,status FROM public.cognition_inbox WHERE id=$1`, inboxID).Scan(&raw, &occurred, &status)
	if err != nil {
		return time.Time{}, nil, false, err
	}
	payload := decodeObject(raw)
	if stringValue(payload["conversation_id"]) == "" || stringValue(payload["turn_id"]) == "" || status == "processed" || status == "failed" {
		return time.Time{}, nil, false, nil
	}
	budget := mapValue(payload["reply_budget"])
	if deadline, err := time.Parse(time.RFC3339Nano, stringValue(budget["deadline_at"])); err == nil {
		return deadline, budget, true, nil
	}
	// Pre-upgrade accepted facts have no frozen budget. Use their original
	// admission time, never a fresh full timeout on each Worker activity retry.
	settings, _, err := a.conversationReplySettings(ctx)
	if err != nil {
		return time.Time{}, nil, false, err
	}
	return occurred.Add(time.Duration(settings.TimeoutSeconds) * time.Second), budget, true, nil
}

func (a *App) ProcessCognitionInbox(ctx context.Context, inboxID string) (map[string]any, error) {
	if inboxID == "" {
		return nil, errors.New("cognition_inbox_id_required")
	}
	deadline, budget, limited, err := a.cognitionTurnDeadline(ctx, inboxID)
	if err != nil {
		return nil, err
	}
	if !limited {
		return a.processCognitionInbox(ctx, inboxID)
	}
	remaining := deadline.Sub(a.now())
	if remaining <= 0 {
		return a.expireConversationTurn(ctx, inboxID, budget)
	}
	// Translate the application clock into a duration so injected clocks and real
	// context deadlines agree. The persisted absolute budget remains authoritative.
	timeoutAt := time.Now().Add(remaining)
	ctx = context.WithValue(ctx, conversationBudgetKey{}, conversationBudgetScope{inboxID, budget})
	runCtx, cancel := context.WithTimeout(ctx, remaining)
	defer cancel()
	result, runErr := a.processCognitionInbox(runCtx, inboxID)
	if errors.Is(runCtx.Err(), context.DeadlineExceeded) && !time.Now().Before(timeoutAt) {
		return a.expireConversationTurn(context.WithoutCancel(ctx), inboxID, budget)
	}
	return result, runErr
}

func (a *App) expireConversationTurn(ctx context.Context, inboxID string, expectedBudget map[string]any) (map[string]any, error) {
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	status := "failed"
	code := "request_timeout"
	err := withTransaction(cleanup, a.DB.Pool(), func(tx pgx.Tx) error {
		var raw []byte
		if err := tx.QueryRow(cleanup, `SELECT payload,status,COALESCE(error_code,'') FROM public.cognition_inbox WHERE id=$1 FOR UPDATE`, inboxID).Scan(&raw, &status, &code); err != nil {
			return err
		}
		payload := decodeObject(raw)
		if status == "processed" || status == "failed" {
			return nil
		}
		if jsonString(mapValue(payload["reply_budget"])) != jsonString(expectedBudget) {
			status = "waiting_for_agent"
			return nil
		}
		// Preserve every committed effect and prevent model retry when part of the
		// original operation has already mutated domain state.
		code = "request_timeout"
		var assistant bool
		if err := tx.QueryRow(cleanup, `SELECT EXISTS(SELECT 1 FROM public.conversation_messages WHERE source_fact_id=$1 AND kind='assistant')`, inboxID).Scan(&assistant); err != nil {
			return err
		}
		ledger, err := a.turnLedgerHasCommittedEffectsTx(cleanup, tx, inboxID)
		if err != nil {
			return err
		}
		if assistant || ledger || turnHasCommittedEffects(payload, a.capabilityRegistry()) {
			code = "agent_partial_committed"
		}
		if _, err := tx.Exec(cleanup, `UPDATE public.cognition_inbox SET status='failed',error_code=$2,processed_at=now(),claimed_by=NULL,claimed_at=NULL WHERE id=$1 AND status IN ('pending','claimed','waiting_reply')`, inboxID, code); err != nil {
			return err
		}
		if _, err := tx.Exec(cleanup, `UPDATE public.platform_workflow_intents SET last_error=$2 WHERE intent_id=$1`, "cognition_intent:"+inboxID, code); err != nil {
			return err
		}
		if _, err := tx.Exec(cleanup, `UPDATE public.conversation_reply_drafts SET status='failed',last_error=$2,updated_at=now() WHERE source_fact_id=$1 AND status IN ('prepared','needs_action','waiting_media','ready')`, inboxID, code); err != nil {
			return err
		}
		status = "failed"
		return nil
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"inbox_id": inboxID, "status": status, "error_code": code}, nil
}

type conversationBudgetKey struct{}
type conversationBudgetScope struct {
	InboxID string
	Budget  map[string]any
}

func conversationBudgetMatches(ctx context.Context, inboxID string, payload map[string]any) bool {
	expected, _ := ctx.Value(conversationBudgetKey{}).(conversationBudgetScope)
	return expected.InboxID == "" || expected.InboxID != inboxID || jsonString(expected.Budget) == jsonString(mapValue(payload["reply_budget"]))
}
func requireConversationBudgetTx(ctx context.Context, tx pgx.Tx, inboxID string, now time.Time, deadline bool, force ...bool) error {
	expected, _ := ctx.Value(conversationBudgetKey{}).(conversationBudgetScope)
	if (expected.InboxID == "" || expected.InboxID != inboxID) && (len(force) == 0 || !force[0]) {
		return nil
	}
	if inboxID == "" {
		return nil
	}
	var raw []byte
	var status string
	var occurred time.Time
	err := tx.QueryRow(ctx, `SELECT payload,status,occurred_at FROM public.cognition_inbox WHERE id=$1 FOR UPDATE`, inboxID).Scan(&raw, &status, &occurred)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	payload := decodeObject(raw)
	if !conversationBudgetMatches(ctx, inboxID, payload) {
		return ErrConflict
	}
	if !deadline {
		return nil
	}
	if stringValue(payload["conversation_id"]) == "" || stringValue(payload["turn_id"]) == "" {
		return nil
	}
	if status != "pending" && status != "claimed" && status != "waiting_reply" {
		return ErrConflict
	}
	end, parseErr := time.Parse(time.RFC3339Nano, stringValue(mapValue(payload["reply_budget"])["deadline_at"]))
	if parseErr != nil {
		settings := defaultConversationReplySettings()
		var rawSetting string
		err := tx.QueryRow(ctx, `SELECT value_json FROM public.runtime_settings WHERE key=$1`, conversationReplySettingKey).Scan(&rawSetting)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if err == nil {
			settings, err = decodeConversationReplySettings([]byte(rawSetting))
			if err != nil {
				return err
			}
		}
		end = occurred.Add(time.Duration(settings.TimeoutSeconds) * time.Second)
	}
	if !now.Before(end) {
		return newCapabilityError("request_timeout", false, context.DeadlineExceeded)
	}
	return nil
}
func turnHasCommittedEffects(payload map[string]any, registry *CapabilityRegistry) bool {
	return len(mapValue(payload["agent_result"])) > 0 || partialHasCommittedMutation(payload["agent_partial"], registry)
}
func (a *App) turnLedgerHasCommittedEffectsTx(ctx context.Context, tx pgx.Tx, inboxID string) (bool, error) {
	rows, err := tx.Query(ctx, `SELECT result FROM public.tool_executions WHERE invocation->>'source_fact_id'=$1`, inboxID)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return false, err
		}
		if partialHasCommittedMutation(map[string]any{"capability_results": []any{decodeObject(raw)}}, a.capabilityRegistry()) {
			return true, nil
		}
	}
	return false, rows.Err()
}

// Worker supervision owns queued-turn expiry even when its original activity
// has not been scheduled or has paused. It never starts a replacement turn.
func (a *App) ExpireDueConversationTurns(ctx context.Context, limit int) (int, error) {
	if limit < 1 || limit > 200 {
		limit = 50
	}
	settings, _, err := a.conversationReplySettings(ctx)
	if err != nil {
		return 0, err
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT id,payload FROM public.cognition_inbox WHERE event_type='conversation.turn' AND status IN ('pending','claimed','waiting_reply') AND COALESCE((payload->'reply_budget'->>'deadline_at')::timestamptz,occurred_at+($2*interval '1 second'))<=$1 ORDER BY occurred_at LIMIT $3`, a.now().UTC(), settings.TimeoutSeconds, limit)
	if err != nil {
		return 0, err
	}
	type candidate struct {
		id     string
		budget map[string]any
	}
	items := []candidate{}
	for rows.Next() {
		var id string
		var raw []byte
		if err := rows.Scan(&id, &raw); err != nil {
			rows.Close()
			return 0, err
		}
		items = append(items, candidate{id, mapValue(decodeObject(raw)["reply_budget"])})
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	count := 0
	for _, item := range items {
		if err := ctx.Err(); err != nil {
			return count, err
		}
		result, err := a.expireConversationTurn(ctx, item.id, item.budget)
		if err != nil {
			return count, err
		}
		if result["status"] == "failed" {
			count++
		}
	}
	return count, nil
}
