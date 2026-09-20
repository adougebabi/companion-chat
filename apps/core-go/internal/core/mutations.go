package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func (a *App) conversationRuntime() ConversationRuntime { return newConversationRuntime(a) }

type TurnResult struct {
	UserMessage   map[string]any
	Assistant     map[string]any
	MediaIntentID string
	TurnID        string
	CorrelationID string
}

type turnCallbacks struct {
	onActionResult func(map[string]any) error
	onChunk        func(string) error
}

var errCognitionTurnSuperseded = errors.New("cognition_turn_superseded")

func (a *App) AcceptSchedule(ctx context.Context, actorID, fluctlightID string, payload map[string]any) (map[string]any, error) {
	return a.acceptScheduleWithTx(ctx, nil, actorID, fluctlightID, payload)
}

func (a *App) acceptScheduleTx(ctx context.Context, tx pgx.Tx, actorID, fluctlightID string, payload map[string]any) (map[string]any, error) {
	return a.acceptScheduleWithTx(ctx, tx, actorID, fluctlightID, payload)
}

func (a *App) acceptScheduleWithTx(ctx context.Context, callerTx pgx.Tx, actorID, fluctlightID string, payload map[string]any) (map[string]any, error) {
	if callerTx != nil {
		var authorized bool
		if err := callerTx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.fluctlights WHERE id=$1 AND created_by_actor_id=$2)`, fluctlightID, actorID).Scan(&authorized); err != nil {
			return nil, err
		}
		if !authorized {
			return nil, ErrUnauthorized
		}
	} else if _, err := a.DB.GetFluctlight(ctx, fluctlightID, actorID); err != nil {
		return nil, err
	}
	localDate, err := time.Parse("2006-01-02", stringValue(payload["local_date"]))
	if err != nil {
		return nil, errors.New("schedule_accept_failed")
	}
	timezone := canonicalTimezone(stringValue(payload["timezone"]))
	if timezone == "" {
		return nil, errors.New("schedule_accept_failed")
	}
	if _, err := time.LoadLocation(timezone); err != nil {
		return nil, errors.New("schedule_timezone_invalid")
	}
	items := arrayValue(payload["items"])
	if len(items) == 0 {
		return nil, errors.New("schedule_accept_failed")
	}
	expectedRevision, expectedOK := nonNegativeRevision(payload["expected_revision"])
	if !expectedOK {
		return nil, errors.New("schedule_expected_revision_required")
	}
	expectedLifeRevision := strings.TrimSpace(stringValue(payload["expected_life_context_revision"]))
	if expectedLifeRevision == "" {
		return nil, errors.New("life_context_revision_required")
	}
	revision := 1
	location, _ := time.LoadLocation(timezone)
	type scheduleEntry struct {
		item       map[string]any
		start, end time.Time
	}
	entries := make([]scheduleEntry, 0, len(items))
	for _, raw := range items {
		item := mapValue(raw)
		if len(item) == 0 {
			return nil, errors.New("schedule item invalid")
		}
		start, e1 := parseScheduleTime(stringValue(item["start_at"]))
		end, e2 := parseScheduleTime(stringValue(item["end_at"]))
		if e1 != nil || e2 != nil || !end.After(start) {
			return nil, errors.New("schedule item time is invalid")
		}
		if strings.TrimSpace(stringValue(item["activity"])) == "" || strings.TrimSpace(stringValue(item["scene"])) == "" {
			return nil, errors.New("schedule item activity and scene are required")
		}
		if len([]rune(strings.TrimSpace(stringValue(item["location"])))) > 512 {
			return nil, errors.New("schedule item location is too long")
		}
		for _, field := range []string{"priority", "flexibility", "interruption_cost"} {
			if rawValue, ok := item[field]; ok && rawValue != nil {
				normalizedValue := normalizeScheduleScalar(rawValue)
				item[field] = normalizedValue
				value, ok := numberFloat(normalizedValue)
				if !ok || value < 0 || value > 1 {
					return nil, errors.New("schedule item numeric value invalid")
				}
			}
		}
		entries = append(entries, scheduleEntry{item: item, start: start, end: end})
	}
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].start.Before(entries[j].start) })
	dayStart := time.Date(localDate.Year(), localDate.Month(), localDate.Day(), 0, 0, 0, 0, location)
	nextDay := dayStart.AddDate(0, 0, 1)
	if !entries[0].start.In(location).Equal(dayStart) || !entries[len(entries)-1].end.In(location).Equal(nextDay) {
		return nil, errors.New("schedule must cover complete local day")
	}
	for index := range entries {
		if index > 0 && !entries[index].start.Equal(entries[index-1].end) {
			return nil, errors.New("schedule items must be contiguous")
		}
		if entries[index].start.In(location).Before(dayStart) || entries[index].end.In(location).After(nextDay) {
			return nil, errors.New("schedule item outside local day")
		}
	}
	idempotencyKey := strings.TrimSpace(stringValue(payload["idempotency_key"]))
	if idempotencyKey == "" || len([]rune(idempotencyKey)) > 256 {
		return nil, errors.New("schedule_idempotency_key_required")
	}
	evidence := arrayValue(payload["evidence_refs"])
	if len(evidence) == 0 {
		evidence = []any{"owner:" + actorID}
	}
	if sourceFactID := stringValue(payload["source_fact_id"]); sourceFactID != "" && !containsStringValue(evidence, sourceFactID) {
		evidence = append(evidence, sourceFactID)
	}
	requestDigest := scheduleAcceptanceRequestDigest(payload, evidence)
	scheduleID := "schedule_" + stableDigest(fluctlightID+":"+idempotencyKey)
	var result map[string]any
	apply := func(tx pgx.Tx) error {
		if err := lockLifeContextTx(ctx, tx, fluctlightID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, fluctlightID+":"+localDate.Format("2006-01-02")); err != nil {
			return err
		}
		var existingID, existingDigest string
		var existingResult []byte
		replayErr := tx.QueryRow(ctx, `SELECT id,COALESCE(request_digest,''),result FROM public.life_schedules WHERE fluctlight_id=$1 AND idempotency_key=$2 FOR UPDATE`, fluctlightID, idempotencyKey).Scan(&existingID, &existingDigest, &existingResult)
		if replayErr == nil {
			if existingDigest == "" || existingDigest != requestDigest {
				return errors.New("schedule_idempotency_conflict")
			}
			result = decodeObject(existingResult)
			if len(result) == 0 {
				return errors.New("schedule_replay_result_invalid")
			}
			scheduleID = existingID
			result["replayed"] = true
			return nil
		}
		if !errors.Is(replayErr, pgx.ErrNoRows) {
			return replayErr
		}
		applyAt := time.Now().UTC()
		if _, err := a.requireLifeContextRevisionTx(ctx, tx, fluctlightID, expectedLifeRevision, applyAt); err != nil {
			return err
		}
		currentTimezone, err := readLifeContextTimezoneWith(ctx, tx, fluctlightID)
		if err != nil {
			return err
		}
		if currentTimezone != timezone {
			return errors.New("schedule_timezone_stale")
		}
		var currentID string
		var current int
		err = tx.QueryRow(ctx, `SELECT id,revision FROM public.life_schedules WHERE fluctlight_id=$1 AND local_date=$2 AND status='accepted' ORDER BY revision DESC LIMIT 1`, fluctlightID, localDate).Scan(&currentID, &current)
		found := err == nil
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if errors.Is(err, pgx.ErrNoRows) {
			current = 0
		}
		if expectedRevision != current {
			return ErrConflict
		}
		var latestRevision int
		if err := tx.QueryRow(ctx, `SELECT COALESCE(MAX(revision),0) FROM public.life_schedules WHERE fluctlight_id=$1 AND local_date=$2`, fluctlightID, localDate).Scan(&latestRevision); err != nil {
			return err
		}
		revision = latestRevision + 1
		if _, err := tx.Exec(ctx, `UPDATE public.life_schedules SET status='superseded' WHERE fluctlight_id=$1 AND local_date=$2 AND status='accepted'`, fluctlightID, localDate); err != nil {
			return err
		}
		generatedFrom := firstString(payload["generated_from"], "owner")
		if _, err := tx.Exec(ctx, `INSERT INTO public.life_schedules (id,fluctlight_id,local_date,timezone,status,generated_from,evidence_refs,revision,generated_at,reschedule_policy,idempotency_key,request_digest,result,updated_at) VALUES ($1,$2,$3,$4,'accepted',$5,$6,$7,$8,$9,$10,$11,'{}',$8)`, scheduleID, fluctlightID, localDate, timezone, generatedFrom, jsonBytes(evidence), revision, applyAt, jsonBytes(payload["reschedule_policy"]), idempotencyKey, requestDigest); err != nil {
			return err
		}
		if found {
			if _, err := tx.Exec(ctx, `UPDATE public.life_schedules SET previous_version_id=$2 WHERE id=$1`, scheduleID, currentID); err != nil {
				return err
			}
		}
		var previousEnd *time.Time
		for _, entry := range entries {
			item := entry.item
			start, end := entry.start, entry.end
			if previousEnd != nil && !start.Equal(*previousEnd) {
				return errors.New("schedule items must be contiguous")
			}
			if _, err := tx.Exec(ctx, `INSERT INTO public.life_schedule_items (id,schedule_id,start_at,end_at,activity,scene,location,item_type,status,priority,flexibility,interruption_cost) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, randomID("schedule_item_"), scheduleID, start, end, strings.TrimSpace(stringValue(item["activity"])), strings.TrimSpace(stringValue(item["scene"])), nullableString(stringValue(item["location"])), firstString(item["item_type"], "planned"), firstString(item["status"], "planned"), numberString(item["priority"], 0.5), numberString(item["flexibility"], 0.5), numberString(item["interruption_cost"], 0.5)); err != nil {
				return err
			}
			previousEnd = &end
		}
		if previousEnd == nil {
			return errors.New("schedule item time is invalid")
		}
		if err := insertPostScheduleLifecycleIntentsTx(ctx, tx, fluctlightID, localDate, timezone); err != nil {
			return err
		}
		_, resultingLife, err := resolveLifeContextSnapshotWith(ctx, tx, fluctlightID, applyAt)
		if err != nil {
			return err
		}
		result = map[string]any{
			"id": scheduleID, "local_date": localDate.Format("2006-01-02"), "timezone": timezone,
			"revision": revision, "status": "accepted", "reschedule_policy": payload["reschedule_policy"],
			"expected_context_revision": expectedLifeRevision, "resulting_context_revision": resultingLife["context_revision"],
			"idempotency_key": idempotencyKey, "replayed": false,
		}
		if _, err := tx.Exec(ctx, `UPDATE public.life_schedules SET result=$2 WHERE id=$1 AND revision=$3`, scheduleID, jsonBytes(result), revision); err != nil {
			return err
		}
		outboxKind := "schedule.accepted"
		if generatedFrom == "model_replan" {
			outboxKind = "schedule.replanned"
		}
		causationID := firstString(payload["source_fact_id"], scheduleID)
		correlationID := firstString(payload["correlation_id"], "schedule:"+scheduleID)
		return appendOutboxTx(ctx, tx, outboxKind, "fluctlight", fluctlightID, causationID, scheduleID, correlationID, "schedule-outbox:"+scheduleID, map[string]any{
			"schedule_id": scheduleID, "local_date": localDate.Format("2006-01-02"), "revision": revision,
			"generated_from": generatedFrom, "source_fact_id": payload["source_fact_id"], "conversation_id": payload["conversation_id"],
			"trigger": payload["trigger"], "reason": payload["reason"], "completed_before": payload["completed_before"], "evidence_refs": evidence,
		})
	}
	if callerTx != nil {
		err = apply(callerTx)
	} else {
		err = withTransaction(ctx, a.DB.Pool(), apply)
	}
	if err != nil {
		return nil, err
	}
	return result, nil
}

func scheduleAcceptanceRequestDigest(payload map[string]any, evidence []any) string {
	return stableDigest(jsonString(map[string]any{
		"local_date":                     payload["local_date"],
		"timezone":                       canonicalTimezone(stringValue(payload["timezone"])),
		"expected_revision":              payload["expected_revision"],
		"items":                          payload["items"],
		"reschedule_policy":              payload["reschedule_policy"],
		"completed_before":               payload["completed_before"],
		"generated_from":                 payload["generated_from"],
		"source_fact_id":                 payload["source_fact_id"],
		"conversation_id":                payload["conversation_id"],
		"trigger":                        payload["trigger"],
		"reason":                         payload["reason"],
		"evidence_refs":                  evidence,
		"expected_life_context_revision": payload["expected_life_context_revision"],
		"capability_call_hint":           payload["idempotency_key"],
	}))
}

// ReplanSchedule shares the immutable acceptance/CAS path but refuses to
// rewrite a completed interval. Callers provide the completed boundary in
// RFC3339; completed items must be carried forward unchanged by the planner.
func (a *App) ReplanSchedule(ctx context.Context, actorID, fluctlightID string, payload map[string]any) (map[string]any, error) {
	return a.replanScheduleWithTx(ctx, nil, actorID, fluctlightID, payload)
}

func (a *App) replanScheduleTx(ctx context.Context, tx pgx.Tx, actorID, fluctlightID string, payload map[string]any) (map[string]any, error) {
	return a.replanScheduleWithTx(ctx, tx, actorID, fluctlightID, payload)
}

func (a *App) replanScheduleWithTx(ctx context.Context, callerTx pgx.Tx, actorID, fluctlightID string, payload map[string]any) (map[string]any, error) {
	boundaryValue := stringValue(payload["completed_before"])
	if boundaryValue == "" {
		return nil, errors.New("completed_before_required")
	}
	boundary, err := time.Parse(time.RFC3339, boundaryValue)
	if err != nil {
		return nil, errors.New("completed_before_invalid")
	}
	var current map[string]any
	if callerTx != nil {
		current, err = a.currentAcceptedScheduleTx(ctx, callerTx, fluctlightID)
	} else {
		current, err = a.currentAcceptedSchedule(ctx, fluctlightID)
	}
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("schedule_replan_schedule_missing")
		}
		return nil, err
	}
	if stringValue(payload["local_date"]) != stringValue(current["local_date"]) {
		return nil, errors.New("schedule_replan_local_date_invalid")
	}
	if canonicalTimezone(stringValue(payload["timezone"])) != canonicalTimezone(stringValue(current["timezone"])) {
		return nil, errors.New("schedule_replan_timezone_invalid")
	}
	if intValue(payload["expected_revision"]) <= 0 || intValue(payload["expected_revision"]) != intValue(current["revision"]) {
		return nil, ErrConflict
	}
	if err := validateScheduleReplanItems(arrayValue(payload["items"])); err != nil {
		return nil, err
	}
	location, err := time.LoadLocation(canonicalTimezone(stringValue(current["timezone"])))
	if err != nil {
		return nil, fmt.Errorf("schedule_timezone_invalid: %w", err)
	}
	day, err := time.ParseInLocation("2006-01-02", stringValue(current["local_date"]), location)
	if err != nil {
		return nil, errors.New("schedule_replan_local_date_invalid")
	}
	dayStart := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, location)
	dayEnd := dayStart.AddDate(0, 0, 1)
	if boundary.Before(dayStart) || boundary.After(dayEnd) {
		return nil, errors.New("schedule_replan_completed_before_out_of_day")
	}
	if err := validateScheduleReplanBoundary(boundary, time.Now().In(location)); err != nil {
		return nil, err
	}
	if err := validateCompletedScheduleHistory(current, payload, boundary); err != nil {
		return nil, err
	}
	for _, raw := range arrayValue(payload["items"]) {
		item := mapValue(raw)
		start, startErr := parseScheduleTime(stringValue(item["start_at"]))
		end, endErr := parseScheduleTime(stringValue(item["end_at"]))
		if startErr != nil || endErr != nil {
			return nil, errors.New("schedule item time is invalid")
		}
		if start.Before(boundary) && end.After(boundary) {
			return nil, errors.New("schedule replan crosses completed boundary")
		}
	}
	if callerTx != nil {
		return a.acceptScheduleTx(ctx, callerTx, actorID, fluctlightID, payload)
	}
	return a.AcceptSchedule(ctx, actorID, fluctlightID, payload)
}

// validateScheduleReplanBoundary prevents a stale model response from moving
// the immutable-history boundary backwards. A provider may take a little time
// to finish, so the guard allows a small clock/request skew, but it must still
// describe approximately "everything completed up to now". The boundary is
// also not allowed to be in the future, because that would make an interval
// that is currently running look completed and therefore immutable.
func validateScheduleReplanBoundary(boundary, now time.Time) error {
	const (
		maxBoundaryAge    = 5 * time.Minute
		maxBoundaryFuture = 30 * time.Second
	)
	if boundary.Before(now.Add(-maxBoundaryAge)) {
		return errors.New("schedule_replan_completed_before_stale")
	}
	if boundary.After(now.Add(maxBoundaryFuture)) {
		return errors.New("schedule_replan_completed_before_future")
	}
	return nil
}

// validateCompletedScheduleHistory makes the immutable accepted schedule the
// authority for the part of the day that has already happened. A replan may
// replace only the current/future portion; if the active item crosses the
// boundary, the replacement must first truncate it at that boundary with the
// same semantic fields.
func validateCompletedScheduleHistory(current, proposal map[string]any, boundary time.Time) error {
	currentItems := arrayValue(current["items"])
	proposalItems := arrayValue(proposal["items"])
	for _, raw := range currentItems {
		old := mapValue(raw)
		oldStart, startErr := parseScheduleTime(stringValue(old["start_at"]))
		oldEnd, endErr := parseScheduleTime(stringValue(old["end_at"]))
		if startErr != nil || endErr != nil || !oldEnd.After(oldStart) {
			return errors.New("schedule_replan_current_history_invalid")
		}
		if oldEnd.After(boundary) && !oldStart.Before(boundary) {
			continue
		}
		wantEnd := oldEnd
		if oldStart.Before(boundary) && oldEnd.After(boundary) {
			wantEnd = boundary
		}
		if !findScheduleItemWithFields(proposalItems, old, oldStart, wantEnd) {
			return errors.New("schedule_replan_completed_history_changed")
		}
	}
	return nil
}

func findScheduleItemWithFields(items []any, want map[string]any, wantStart, wantEnd time.Time) bool {
	for _, raw := range items {
		item := mapValue(raw)
		start, startErr := parseScheduleTime(stringValue(item["start_at"]))
		end, endErr := parseScheduleTime(stringValue(item["end_at"]))
		if startErr != nil || endErr != nil || !start.Equal(wantStart) || !end.Equal(wantEnd) {
			continue
		}
		if !sameScheduleItemSemantics(want, item) {
			continue
		}
		return true
	}
	return false
}

func sameScheduleItemSemantics(left, right map[string]any) bool {
	for _, key := range []string{"activity", "scene", "location", "item_type", "status"} {
		if strings.TrimSpace(stringValue(left[key])) != strings.TrimSpace(stringValue(right[key])) {
			return false
		}
	}
	for _, key := range []string{"priority", "flexibility", "interruption_cost"} {
		leftValue, leftOK := numberFloat(normalizeScheduleScalar(left[key]))
		rightValue, rightOK := numberFloat(normalizeScheduleScalar(right[key]))
		if !leftOK || !rightOK || math.Abs(leftValue-rightValue) > 0.000001 {
			return false
		}
	}
	return true
}

// insertPostScheduleLifecycleIntentsTx releases the cognition-producing
// lifecycle intents only after the current local day's Schedule is accepted.
// Keeping these inserts in the same transaction as schedule acceptance makes
// schedule-first ordering durable across Worker restarts and dispatcher
// retries; an accepted future schedule does not wake the Fluctlight early.
func insertPostScheduleLifecycleIntentsTx(ctx context.Context, tx pgx.Tx, fluctlightID string, localDate time.Time, timezone string) error {
	location, err := time.LoadLocation(canonicalTimezone(timezone))
	if err != nil {
		return fmt.Errorf("schedule_timezone_invalid: %w", err)
	}
	if localDate.Format("2006-01-02") != time.Now().In(location).Format("2006-01-02") {
		return nil
	}
	dateValue := localDate.Format("2006-01-02")
	if _, err := tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents (intent_id,workflow_id,task_queue,intent_type,payload) VALUES ($1,$2,'lifecycle','daily_review.current_day',$3) ON CONFLICT DO NOTHING`, "daily_review_intent:"+fluctlightID+":"+dateValue, "daily_review:"+fluctlightID+":"+dateValue, jsonBytes(map[string]any{"fluctlight_id": fluctlightID, "local_date": dateValue})); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents (intent_id,workflow_id,task_queue,intent_type,payload) VALUES ($1,$2,'lifecycle','wake_up.current',$3) ON CONFLICT DO NOTHING`, "wake_up_intent:"+fluctlightID, "wake_up:"+fluctlightID, jsonBytes(map[string]any{"fluctlight_id": fluctlightID, "cycle": 0}))
	return err
}

func parseScheduleTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if strings.Contains(value, "T24:") {
		value = strings.Replace(value, "T24:", "T00:", 1)
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			return time.Time{}, err
		}
		return parsed.Add(24 * time.Hour), nil
	}
	return time.Parse(time.RFC3339, value)
}

func canonicalTimezone(value string) string {
	normalized := strings.TrimSpace(value)
	switch strings.ToLower(normalized) {
	case "utc+8", "utc+08:00", "gmt+8", "gmt+08:00", "china standard time", "cst":
		return "Asia/Shanghai"
	case "utc", "gmt", "z":
		return "UTC"
	default:
		return normalized
	}
}

func (a *App) HandleTurn(ctx context.Context, actorID, conversationID string, payload map[string]any) (TurnResult, error) {
	return a.handleTurn(ctx, actorID, conversationID, payload, turnCallbacks{}, false)
}

// HandleActorTurn is the internal/group-chat entry point. Authorization stays
// anchored to the Owner Human while the persisted message and cognition
// speaker use the actual Actor (including another Fluctlight).
func (a *App) HandleActorTurn(ctx context.Context, ownerActorID, senderActorID, conversationID string, payload map[string]any) (TurnResult, error) {
	copyPayload := cloneMap(payload)
	copyPayload["authorization_actor_id"] = ownerActorID
	return a.handleTurn(ctx, senderActorID, conversationID, copyPayload, turnCallbacks{}, false)
}

func (a *App) handleTurn(ctx context.Context, actorID, conversationID string, payload map[string]any, callbacks turnCallbacks, claimStream bool) (TurnResult, error) {
	// ─────────────────────────────────────────────────────────────────────────
	// STAGE 1: Parameter Validation & Actor Authorization
	// ─────────────────────────────────────────────────────────────────────────
	authorizationActorID := firstString(payload["authorization_actor_id"], actorID)
	fluctlightID := stringValue(payload["fluctlight_id"])
	text := stringValue(payload["text"])
	idempotency := stringValue(payload["idempotency_key"])
	turnID := stringValue(payload["turn_id"])
	if turnID == "" {
		turnID = "turn_" + stableDigest(conversationID+":"+idempotency)
	}
	if fluctlightID == "" || text == "" || idempotency == "" {
		return TurnResult{}, errors.New("conversation_turn_invalid")
	}
	if _, err := a.DB.GetFluctlight(ctx, fluctlightID, authorizationActorID); err != nil {
		return TurnResult{}, err
	}
	if authorizationActorID != actorID {
		if err := a.authorizeActorTurn(ctx, authorizationActorID, actorID, fluctlightID, conversationID); err != nil {
			return TurnResult{}, err
		}
	}
	claimOwner := ""
	if claimStream {
		claimOwner = "go-stream:" + randomID("claim_")
	}
	claimSettled := !claimStream

	// ─────────────────────────────────────────────────────────────────────────
	// STAGE 2: Atomic Turn Message & Cognition Fact Enqueue (Transaction 1)
	// ─────────────────────────────────────────────────────────────────────────
	var user map[string]any
	var inboxID string
	var supersededInboxIDs []string
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var existingID string
		var existingText string
		var existingSequence int
		var existingAuthor string
		var existingTurnID string
		var existingSourceFactID string
		var existingCorrelationID string
		var existingAttachments []byte
		var existingCreatedAt time.Time
		err := tx.QueryRow(ctx, `SELECT id,sequence,text,author_actor_id,attachment_refs,created_at,COALESCE(turn_id,''),COALESCE(source_fact_id,''),COALESCE(correlation_id,'') FROM public.conversation_messages WHERE conversation_id=$1 AND idempotency_key=$2`, conversationID, idempotency).Scan(&existingID, &existingSequence, &existingText, &existingAuthor, &existingAttachments, &existingCreatedAt, &existingTurnID, &existingSourceFactID, &existingCorrelationID)
		messageExists := err == nil
		if err == nil {
			if existingAuthor != actorID || existingText != text || !jsonEqual(existingAttachments, payload["attachment_refs"]) {
				return ErrConflict
			}
			if existingTurnID != "" && existingTurnID != turnID {
				return ErrConflict
			}
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if !messageExists {
			var participantCount int
			if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM public.conversation_participants WHERE conversation_id=$1 AND actor_id IN ($2,$3) AND status='active'`, conversationID, actorID, fluctlightID).Scan(&participantCount); err != nil {
				return err
			}
			if participantCount != 2 {
				return errors.New("conversation_not_found")
			}
		}
		var enqueueErr error
		inboxID, supersededInboxIDs, enqueueErr = a.enqueueTurnFactTx(ctx, tx, actorID, fluctlightID, conversationID, turnID, idempotency, text, payload["attachment_refs"], claimOwner)
		if enqueueErr != nil {
			return enqueueErr
		}
		correlationID := "turn:" + turnID
		if messageExists {
			if existingSourceFactID != "" && existingSourceFactID != inboxID {
				return ErrConflict
			}
			if existingCorrelationID != "" && existingCorrelationID != correlationID {
				return ErrConflict
			}
			// A retry can close the old message-before-fact crash window only when
			// the full actor/conversation/text/idempotency tuple has been verified.
			if existingSourceFactID == "" || existingTurnID == "" || existingCorrelationID == "" {
				if _, err := tx.Exec(ctx, `UPDATE public.conversation_messages SET turn_id=COALESCE(turn_id,$2),source_fact_id=COALESCE(source_fact_id,$3),correlation_id=COALESCE(correlation_id,$4) WHERE id=$1`, existingID, turnID, inboxID, correlationID); err != nil {
					return err
				}
				existingTurnID, existingSourceFactID, existingCorrelationID = turnID, inboxID, correlationID
			}
			user = map[string]any{"id": existingID, "conversation_id": conversationID, "sequence": existingSequence, "author_actor_id": existingAuthor, "kind": "user", "text": existingText, "attachment_refs": decodeArray(existingAttachments), "created_at": existingCreatedAt.UTC().Format(time.RFC3339Nano)}
			return nil
		}
		var seq int
		if err := tx.QueryRow(ctx, `SELECT next_sequence FROM public.conversation_heads WHERE conversation_id=$1 FOR UPDATE`, conversationID).Scan(&seq); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE public.conversation_heads SET next_sequence=$2 WHERE conversation_id=$1`, conversationID, seq+1); err != nil {
			return err
		}
		messageID := turnID
		if !strings.HasPrefix(messageID, "message_") {
			messageID = randomID("message_")
		}
		attachments := payload["attachment_refs"]
		if attachments == nil {
			attachments = []any{}
		}
		var createdAt time.Time
		if err := tx.QueryRow(ctx, `INSERT INTO public.conversation_messages (id,conversation_id,sequence,author_actor_id,kind,text,attachment_refs,idempotency_key,turn_id,source_fact_id,correlation_id) VALUES ($1,$2,$3,$4,'user',$5,$6,$7,$8,$9,$10) RETURNING created_at`, messageID, conversationID, seq, actorID, text, jsonBytes(attachments), idempotency, turnID, inboxID, correlationID).Scan(&createdAt); err != nil {
			return err
		}
		user = map[string]any{"id": messageID, "conversation_id": conversationID, "sequence": seq, "author_actor_id": actorID, "kind": "user", "text": text, "attachment_refs": attachments, "created_at": createdAt.UTC().Format(time.RFC3339Nano)}
		return nil
	})
	if err != nil {
		return TurnResult{}, err
	}
	// ─────────────────────────────────────────────────────────────────────────
	// STAGE 3: Lifecycle Preemption & Stream Setup
	// ─────────────────────────────────────────────────────────────────────────
	a.cancelSupersededCognitionFacts(ctx, supersededInboxIDs)
	// A synchronous turn starts its Provider call in this process instead of
	// waiting for Dispatcher.DispatchOnce.  Apply the same lifecycle preemption
	// boundary here so an in-flight/pending Wake-up or Reflection cannot win a
	// race with the newly accepted cognition fact.
	if preemptErr := a.CancelLifecycleForCognition(ctx, fluctlightID, "cognition:"+inboxID); preemptErr != nil {
		slog.Warn("Go Core lifecycle preemption before synchronous cognition failed", "fluctlight_id", fluctlightID, "inbox_id", inboxID, "error_type", fmt.Sprintf("%T", preemptErr))
	}
	ctx = WithProviderCancellationKey(ctx, inboxID)
	if claimStream {
		defer func() {
			if claimSettled || claimOwner == "" {
				return
			}
			if releaseErr := a.releaseCognitionClaim(ctx, inboxID, claimOwner); releaseErr != nil {
				slog.Default().Warn("Go Core streaming cognition claim cleanup failed", "inbox_id", inboxID, "turn_id", turnID, "claim_owner", claimOwner, "error", releaseErr)
			}
		}()
	}
	userFrameEmitted := false
	emitUserFrame := func() error {
		if userFrameEmitted || callbacks.onActionResult == nil {
			return nil
		}
		if err := callbacks.onActionResult(map[string]any{"message": user, "correlation_id": "turn:" + turnID}); err != nil {
			return err
		}
		userFrameEmitted = true
		return nil
	}
	emitAssistantFrame := func(message map[string]any) error {
		if callbacks.onActionResult == nil || len(message) == 0 {
			return nil
		}
		return callbacks.onActionResult(map[string]any{"message": message, "correlation_id": "turn:" + turnID})
	}
	// The user message, claimed cognition fact, workflow intent, and outbox are
	// committed together before cognition starts. Emit the authoritative message
	// immediately so the browser does not depend on an in-memory optimistic bubble
	// while the Provider is thinking.
	if err := emitUserFrame(); err != nil {
		return TurnResult{}, err
	}
	if a.cognitionFactSuperseded(ctx, inboxID) {
		return TurnResult{}, errCognitionTurnSuperseded
	}

	// ─────────────────────────────────────────────────────────────────────────
	// STAGE 4: Replay Fast-Path & Crash Recovery Check
	// ─────────────────────────────────────────────────────────────────────────
	var replayed map[string]any
	var replayedID, replayedText string
	var replayedSequence int
	var replayedCreatedAt time.Time
	if err := a.DB.Pool().QueryRow(ctx, `SELECT id,sequence,text,created_at FROM public.conversation_messages WHERE conversation_id=$1 AND idempotency_key=$2`, conversationID, "assistant:"+turnID).Scan(&replayedID, &replayedSequence, &replayedText, &replayedCreatedAt); err == nil {
		replayed = map[string]any{"id": replayedID, "conversation_id": conversationID, "sequence": replayedSequence, "author_actor_id": fluctlightID, "kind": "assistant", "text": replayedText, "attachment_refs": []any{}, "created_at": replayedCreatedAt.UTC().Format(time.RFC3339Nano)}
		if mediaIntent, recovered, recoveryErr := a.recoverFrozenTurnAfterAssistant(ctx, inboxID, fluctlightID, conversationID, replayedID, replayedText); recoveryErr != nil {
			return TurnResult{}, recoveryErr
		} else if recovered {
			if err := emitUserFrame(); err != nil {
				return TurnResult{}, err
			}
			if callbacks.onChunk != nil {
				if err := callbacks.onChunk(replayedText); err != nil {
					return TurnResult{}, err
				}
			}
			if err := emitAssistantFrame(replayed); err != nil {
				return TurnResult{}, err
			}
			return TurnResult{UserMessage: user, Assistant: replayed, MediaIntentID: mediaIntent, TurnID: turnID, CorrelationID: "turn:" + turnID}, nil
		}
		if err := emitUserFrame(); err != nil {
			return TurnResult{}, err
		}
		if callbacks.onChunk != nil {
			if err := callbacks.onChunk(replayedText); err != nil {
				return TurnResult{}, err
			}
		}
		if err := emitAssistantFrame(replayed); err != nil {
			return TurnResult{}, err
		}
		return TurnResult{UserMessage: user, Assistant: replayed, TurnID: turnID, CorrelationID: "turn:" + turnID}, nil
	}

	// ─────────────────────────────────────────────────────────────────────────
	// STAGE 5: Context Projection & Persona Authority Derivation
	// ─────────────────────────────────────────────────────────────────────────
	var decision map[string]any
	var action string
	var capabilityInvocations []CapabilityInvocation
	var capabilityResults []CapabilityResult
	var responsePlan map[string]any
	var composite CompositeActionV1
	var personalityPlan *personalityDecisionPlan
	responseMode := "final"
	var continuationBaseMessages []map[string]any
	structuredFallback := false
	frozen, frozenFound, err := a.LoadFrozenTurn(ctx, inboxID)
	if err != nil {
		return TurnResult{}, err
	}
	if frozenFound && frozen.Status == "failed" {
		return TurnResult{}, fmt.Errorf("cognition turn is quarantined: %s", frozen.ErrorCode)
	}
	if frozenFound && frozen.Status == "completed" {
		return TurnResult{}, errors.New("completed cognition action is missing its assistant output")
	}
	var projection ContextProjection
	var personaScope turnPersonaScope
	var personaSwitch personaSwitchNormalization
	personaGrant := persistentSwitchGrant{}
	if frozenFound && frozen.Status == "frozen" {
		frozenDecision := mapValue(frozen.Payload["decision"])
		if savedProjection, ok := contextProjectionFromValue(frozenDecision["context_projection"]); ok {
			projection = savedProjection
		} else {
			return TurnResult{}, errors.New("frozen_context_projection_missing")
		}
		// The frozen scope is the persona view that generation actually saw.
		// Recovery never re-derives the owner from the current runtime row.
		if savedScope, ok := turnPersonaScopeFromPayload(frozen.Payload); ok {
			personaScope = savedScope
		}
	} else {
		projection, err = a.buildTurnProjection(ctx, authorizationActorID, actorID, fluctlightID, conversationID, inboxID, text)
		if err != nil {
			return TurnResult{}, err
		}
		personaScope = resolveTurnPersonaScope(projection)
	}
	// The normalized persona-switch view and its authorization are derived once
	// per turn from the same projection the generation consumes.
	personaSwitch = normalizePersonaSwitchRules(projection.CorePersona, projection.PersonalityRuntime, personaScope.ActiveProfileID)
	personaGrant = resolvePersistentSwitchGrant(personaScope, persistentSwitchGrantScenarioMain, personaSwitch.Rules)
	personaAuthority := turnDecisionAuthority{Grant: personaGrant, Scope: personaScope}
	// ─────────────────────────────────────────────────────────────────────────
	// STAGE 6: Agent Decision Generation & Normalization
	// ─────────────────────────────────────────────────────────────────────────
	if frozenFound && frozen.Status == "frozen" {
		hydrated, hydrateErr := a.hydrateFrozenTurn(frozen, fluctlightID)
		if hydrateErr != nil {
			return TurnResult{}, hydrateErr
		}
		action = hydrated.Action
		decision = hydrated.Decision
		personalityPlan = hydrated.PersonalityPlan
		capabilityInvocations = hydrated.Invocations
		composite = hydrated.Composite
		capabilityResults = hydrated.Results
		responsePlan = hydrated.ResponsePlan
		responseMode = hydrated.ResponseMode
		continuationBaseMessages = hydrated.ContinuationBaseMessages
		if len(hydrated.Projection.FluctlightID) > 0 {
			projection = hydrated.Projection
		}
	} else {
		// Moment publication is a Wake-up/autonomy output, not an ordinary
		// interactive reply capability. Keep it registered globally for the
		// Runtime while withholding it from the conversation tool catalog.
		definitions := capabilityCatalog(a.capabilityRegistry(), CapabilitySurfaceConversation)
		// The response schema is conditional on the authorization of THIS call:
		// a scenario that may not propose a persistent switch is not even offered
		// the field, so the constraint does not depend on post-hoc filtering.
		schema := cognitiveTurnResponseSchemaForGrant(personaGrant)
		assembly, assembledProjection, assemblyErr := a.assembleProjectionPromptForSurface(ctx, ProviderContextSurfaceConversationMain, projection, "cognitive_assessment", []string{providerContextAuthorityRule, capabilityConversationPolicyInstruction}, text, definitions, "conversation_turn_response", schema)
		if assemblyErr != nil {
			return TurnResult{}, assemblyErr
		}
		projection = assembledProjection
		continuationBaseMessages = cloneMapSlice(assembly.Messages)
		providerCtx := WithPromptDiagnostics(WithProviderScenario(ctx, "cognitive_assessment"), assembly.Diagnostics)
		providerCtx = WithProviderCorrelation(providerCtx, "turn:"+turnID)
		// Main cognition is the semantic decision boundary for persona, action and
		// reply. Allow the configured Provider to use its thinking channel; the
		// adapter still parses reasoning_content as a structured candidate and Core
		// validates the resulting decision before any side effect.
		run, completionErr := a.conversationRuntime().RunMain(providerCtx, ConversationMainInput{
			Role: "cognitive_assessment", Messages: assembly.Messages, Definitions: definitions,
			SchemaName: "conversation_turn_response", Schema: schema,
			EnableThinking: structuredThinkingEnabledForSchema("conversation_turn_response"),
			Capability: &ConversationCapabilityContext{
				FluctlightID: fluctlightID, ConversationID: conversationID, SourceFactID: inboxID,
				ActionID: "frozen_" + stableDigest(inboxID), Projection: projection,
			},
		})
		if completionErr != nil {
			if a.cognitionFactSuperseded(ctx, inboxID) {
				return TurnResult{}, errCognitionTurnSuperseded
			}
			return TurnResult{}, completionErr
		}
		if a.cognitionFactSuperseded(ctx, inboxID) {
			return TurnResult{}, errCognitionTurnSuperseded
		}
		completion := run.Completion
		decision = completion.Structured
		structuredFallback = completion.StructuredFallback
		capabilityInvocations = append([]CapabilityInvocation(nil), completion.ToolCalls...)
		if run.Trace != nil {
			capabilityInvocations = mergeADKTraceInvocations(capabilityInvocations, run.Trace)
			capabilityResults = append(capabilityResults, run.Trace.Results...)
		}
		// Both the Main generation and the takeover reply run through this one
		// normalizer so a takeover candidate cannot bypass a single validation
		// step that the Main candidate passed (design.md 4.8).
		normalized, normalizeErr := a.normalizeTurnDecision(ctx, turnDecisionNormalizationInput{
			InboxID: inboxID, FluctlightID: fluctlightID, ConversationID: conversationID, TurnID: turnID,
			Projection: projection, Grant: personaGrant, Decision: decision, Invocations: capabilityInvocations,
			Definitions: definitions, StructuredFallback: structuredFallback,
			ContinuationBaseMessages: continuationBaseMessages,
		})
		if normalizeErr != nil {
			return TurnResult{}, normalizeErr
		}
		decision = normalized.Decision
		capabilityInvocations = normalized.Invocations
		responsePlan = normalized.ResponsePlan
		responseMode = normalized.ResponseMode
		action = normalized.Action
		composite = normalized.Composite
		personalityPlan = nil
		delete(decision, "personality_transition")
		// The Main response is already a valid candidate. Only now, after its
		// visible reply has been determined, ask the small tool-free assessment
		// whether the persistent profile should change for the next turn.
		if persistentSwitchAssessmentRequired(personaGrant, personaSwitch) {
			assessmentPlan, assessmentDecision, assessmentErr := a.assessPersistentSwitchAfterCandidate(ctx, persistentSwitchPostAssessmentInput{
				InboxID: inboxID, FluctlightID: fluctlightID, CurrentText: text,
				CandidateReply: normalized.Canonical.Text, ResponseIntent: stringValue(decision["response_intent"]), Projection: projection,
			})
			if assessmentErr != nil {
				slog.Warn("Go Core post-cognition persona switch assessment degraded", "fluctlight_id", fluctlightID, "inbox_id", inboxID, "error_type", fmt.Sprintf("%T", assessmentErr))
			} else {
				decision["persistent_switch_assessment"] = map[string]any{
					"decision":          stringValue(assessmentDecision["decision"]),
					"target_profile_id": stringValue(assessmentDecision["target_profile_id"]),
					"trigger_id":        stringValue(assessmentDecision["trigger_id"]),
				}
				if assessmentPlan != nil && assessmentPlan.TargetProfile != assessmentPlan.CurrentProfile {
					decision["personality_transition"] = assessmentPlan
					personalityPlan = assessmentPlan
					policyInvocation, policyResult, policyErr := a.executePersonaPolicyAction(ctx, personaSwitchCapabilityName, fluctlightID, conversationID, inboxID, "frozen_"+stableDigest(inboxID), map[string]any{
						"decision": "switch", "source_profile_id": assessmentPlan.CurrentProfile,
						"target_profile_id": assessmentPlan.TargetProfile, "reason": assessmentPlan.Reason,
					})
					if policyErr != nil {
						return TurnResult{}, policyErr
					}
					decision["personality_action"] = map[string]any{"invocation": policyInvocation, "result": policyResult}
				}
			}
		}
		if a.cognitionFactSuperseded(ctx, inboxID) {
			return TurnResult{}, errCognitionTurnSuperseded
		}
		frozen, err = a.PersistTurnDecision(ctx, inboxID, fluctlightID, conversationID, turnID, action, decision, personaAuthority)
		if err != nil {
			return TurnResult{}, err
		}
	}

	// ─────────────────────────────────────────────────────────────────────────
	// STAGE 7: Side-Effect-Free Candidate Capability Validation Gate
	// ─────────────────────────────────────────────────────────────────────────
	// Candidate validation happens BEFORE the Judge (F02/F05). The cheap,
	// side-effect-free validator runs the same schema check the WakeUp worker
	// uses plus the deterministic authorization gate F-02 requires (declared
	// surface, frozen identity ownership, declared target kinds). A candidate
	// that is structurally valid but unauthorized must fail here so a takeover
	// by B cannot mask A's illegal invocation.
	if len(capabilityInvocations) > 0 {
		if validateErr := a.validateCandidateCapabilityInvocations(capabilityInvocations, candidateValidationContext{
			FluctlightID: fluctlightID, ConversationID: conversationID, SourceFactID: inboxID, ActionID: frozen.ID,
			Surface: CapabilitySurfaceConversation, ContextSnapshot: mapValue(frozen.Payload["capability_context_snapshot"]), Context: ctx,
		}); validateErr != nil {
			if failures := capabilityBatchFailures(validateErr); len(failures) > 0 {
				// Candidate validation is a Judge precondition. A malformed or
				// unauthorized sibling cannot be hidden by a valid reply or by a
				// takeover; retain its bounded result, quarantine the frozen turn,
				// and stop before any Judge/Prepare/Execute path.
				capabilityResults = mergeCapabilityResults(capabilityResults, failures)
				if persistErr := a.PersistCapabilityResults(ctx, frozen.ID, capabilityResults); persistErr != nil {
					return TurnResult{}, persistErr
				}
			}
			_ = a.FailTurnCognition(ctx, inboxID, frozen.ID, "candidate_invalid")
			return TurnResult{}, validateErr
		}
	}

	// ─────────────────────────────────────────────────────────────────────────
	// STAGE 8: Turn Takeover Arbitration & Execution Window Entry
	// ─────────────────────────────────────────────────────────────────────────
	// Exactly one insertion point exists between the A generation and the
	// Prepare/execution window. Only turn_stage == winner_ready may execute
	// (F03), so a rejected candidate can never reach a side effect.
	handled, takeoverErr := a.applyTurnTakeover(ctx, turnTakeoverInput{
		InboxID: inboxID, FluctlightID: fluctlightID, ConversationID: conversationID, TurnID: turnID,
		Projection: projection, Scope: personaScope, Switch: personaSwitch,
		ResponseMode: responseMode, Action: action, Decision: decision,
		Invocations: capabilityInvocations, Frozen: frozen,
	})
	if takeoverErr != nil {
		// A superseded turn belongs to another worker: its frozen row and stage
		// are left untouched so that worker can finish. Every other arbitration
		// failure is this turn's, so it is quarantined with an exact code.
		//
		// F06: a takeover reply that asks for yet another result-dependent
		// continuation is a controlled failure. Leaving it resumable would let a
		// retry spend an unbounded number of generations on a candidate that is
		// structurally forbidden from succeeding, so it fails closed instead.
		if errors.Is(takeoverErr, errCognitionTurnSuperseded) {
			return TurnResult{}, takeoverErr
		}
		_ = a.FailTurnCognition(ctx, inboxID, frozen.ID, takeoverFailureCode(takeoverErr))
		return TurnResult{}, takeoverErr
	}
	if handled {
		// Read the winning candidate and every derived variable back from the
		// payload; A's local variables are the rejected candidate (F03).
		reloaded, reloadFound, reloadErr := a.LoadFrozenTurn(ctx, inboxID)
		if reloadErr != nil {
			return TurnResult{}, reloadErr
		}
		if !reloadFound {
			return TurnResult{}, errors.New("takeover_frozen_turn_missing")
		}
		frozen = reloaded
		hydrated, hydrateErr := a.hydrateFrozenTurn(frozen, fluctlightID)
		if hydrateErr != nil {
			return TurnResult{}, hydrateErr
		}
		action = hydrated.Action
		decision = hydrated.Decision
		personalityPlan = hydrated.PersonalityPlan
		composite = hydrated.Composite
		capabilityResults = hydrated.Results
		responsePlan = hydrated.ResponsePlan
		responseMode = hydrated.ResponseMode
		continuationBaseMessages = hydrated.ContinuationBaseMessages
		if len(hydrated.Projection.FluctlightID) > 0 {
			projection = hydrated.Projection
		}
	}
	stage := turnStageOf(frozen.Payload)
	if !turnStageExecutable(frozen.Payload) {
		// A concurrent worker moved (or never reached) the eligible stage. Fail
		// closed instead of executing a candidate that was never authorized.
		_ = a.FailTurnCognition(ctx, inboxID, frozen.ID, "turn_stage_not_executable")
		return TurnResult{}, errors.New("turn_stage_not_executable")
	}
	if stage == turnStageWinnerReady {
		// The winner is admitted, so the side-effect window begins now. The
		// marker is written before any Prepare work, making a crash inside the
		// window distinguishable from "arbitration never finished": a later
		// recovery reads executing and resumes instead of re-deciding (F03).
		if err := a.BeginTurnExecution(ctx, frozen.ID); err != nil {
			return TurnResult{}, err
		}
		frozen.Payload[turnStagePayloadKey] = turnStageExecuting
	}

	// ─────────────────────────────────────────────────────────────────────────
	// STAGE 9: Query Continuation & Capability Preparation
	// ─────────────────────────────────────────────────────────────────────────
	var continuationState QueryContinuationState
	if responseMode == "query_continuation" {
		if raw := frozen.Payload["query_continuation"]; raw != nil {
			continuationState, err = queryContinuationStateFromValue(raw)
			if err != nil {
				return TurnResult{}, err
			}
		} else {
			continuationState = newQueryContinuationState(continuationBaseMessages, capabilityInvocations)
			if err := a.persistQueryContinuationState(ctx, frozen.ID, continuationState); err != nil {
				return TurnResult{}, err
			}
		}
		if expected := newQueryContinuationState(continuationState.BaseMessages, capabilityInvocations); expected.RequestDigest != continuationState.RequestDigest {
			return TurnResult{}, errors.New("query_continuation_digest_invalid")
		}
	}
	if action != "reply" && action != "no_op" {
		return TurnResult{}, errors.New("decision_effect_invalid")
	}
	if action == "no_op" && decision["tool_only"] != true {
		_ = a.FailTurnCognition(ctx, inboxID, frozen.ID, "cognition_visible_text_missing")
		return TurnResult{}, errors.New("cognition_visible_text_missing")
	}
	capabilityInvocations, err = capabilityInvocationsFromValue(frozen.Payload["capability_invocations"])
	if err != nil {
		return TurnResult{}, err
	}
	for index := range capabilityInvocations {
		capabilityInvocations[index].ActionID = frozen.ID
	}
	if len(capabilityInvocations) > 0 {
		capabilityInvocations, err = a.prepareCapabilityInvocations(ctx, fluctlightID, conversationID, inboxID, capabilityInvocations, capabilityResults)
		if err != nil {
			if failures := capabilityBatchFailures(err); len(failures) > 0 {
				capabilityResults = mergeCapabilityResults(capabilityResults, failures)
				slog.Warn("Go Core capability preparation degraded per call", "turn_id", turnID, "failed_call_count", len(failures))
			} else {
				code, _ := capabilityErrorInfo(err, "capability_prepare_failed", true)
				_ = a.FailTurnCognition(ctx, inboxID, frozen.ID, code)
				return TurnResult{}, newCapabilityError(code, true, err)
			}
		}
		// The prepared invocation is the crash/replay boundary. No Capability may
		// execute until its runtime-owned plan and context snapshot are durable.
		if err := a.persistFrozenCapabilityInvocations(ctx, frozen.ID, capabilityInvocations); err != nil {
			return TurnResult{}, err
		}
	}

	// ─────────────────────────────────────────────────────────────────────────
	// STAGE 10: Capability Planning & Turn Settlement (Transaction 2)
	// ─────────────────────────────────────────────────────────────────────────
	if action == "no_op" {
		toolOnlyBinding := OutputBindingV1{}
		if decision["tool_only"] == true {
			// Tool-only direct turns have no assistant message yet. Deferred
			// output capabilities such as image generation bind to the durable
			// conversation itself; completion may later create a media_reference
			// message without requiring conversation.reply.
			toolOnlyBinding = OutputBindingV1{TargetKind: "conversation", TargetRef: conversationID}
		}
		if len(capabilityInvocations) > 0 {
			capabilityResults, err = a.planCapabilitiesForTransaction(ctx, fluctlightID, conversationID, inboxID, capabilityInvocations, capabilityResults)
			if err != nil {
				// Each capability failure remains a per-call diagnostic. Only the
				// capability that owns a concrete visible output target is checked as
				// a settlement gate; unrelated siblings continue independently.
				slog.Default().Warn("Go Core capability failed during no-op turn", "turn_id", turnID, "error", err, "capability_results", capabilityResults)
			}
		}
		reflectionDelay := a.reflectionDelay(ctx)
		nextReflectionAt := a.now().UTC().Add(reflectionDelay)
		// Appraisal/Current State, native mutations, claims, action result and
		// inbox settlement share this one transaction. A failure leaves only the
		// immutable frozen plan for deterministic retry/quarantine.
		settleErr := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
			if err := a.requireCognitionAuthorityRevisionsTx(ctx, tx, fluctlightID, projection.ContextRevision, projection.CurrentStateRevision, projection.LifeContextRevision, a.now().UTC()); err != nil {
				return err
			}
			if _, err := a.applyPersistentSwitchIfAuthorizedTx(ctx, tx, fluctlightID, frozen.Payload, personalityPlan); err != nil {
				return err
			}
			if err := a.applyFrozenCognitiveStagesTx(ctx, tx, fluctlightID, inboxID, decision, action, frozen.ID, frozen.StateRev); err != nil {
				return err
			}
			if len(capabilityInvocations) > 0 {
				settled, settleErr := a.settleDeferredCapabilitiesTx(ctx, tx, fluctlightID, inboxID, inboxID, capabilityInvocations, capabilityResults, toolOnlyBinding)
				if settleErr != nil {
					return settleErr
				}
				capabilityResults = settled
				if err := a.persistFrozenCapabilityInvocationsTx(ctx, tx, frozen.ID, capabilityInvocations); err != nil {
					return err
				}
				command, err := tx.Exec(ctx, `UPDATE public.cognition_frozen_actions SET payload=jsonb_set(payload,'{capability_results}',$2::jsonb,true) WHERE id=$1 AND status='frozen'`, frozen.ID, jsonBytes(capabilityResults))
				if err != nil {
					return err
				}
				if command.RowsAffected() != 1 {
					return ErrConflict
				}
				if requiredErr := requiredVisibleOutputCapabilityFailure(capabilityResults, capabilityInvocations, a.capabilityRegistry(), false); requiredErr != nil {
					return requiredErr
				}
			}
			if err := persistClaimsTx(ctx, tx, fluctlightID, inboxID, responsePlan); err != nil {
				return err
			}
			_, err := a.completeTurnCognitionTx(ctx, tx, inboxID, frozen.ID, map[string]any{"status": "no_op", "response_intent": stringValue(responsePlan["response_intent"]), "capability_results": capabilityResults}, nextReflectionAt)
			return err
		})
		if settleErr != nil {
			if len(capabilityInvocations) > 0 {
				capabilityResults = capabilityResultsAfterSettlementFailure(capabilityResults, capabilityInvocations, a.capabilityRegistry(), "capability_settlement_failed")
				_ = a.PersistCapabilityResults(ctx, frozen.ID, capabilityResults)
			}
			code := "structured_turn_settlement_failed"
			if errors.Is(settleErr, ErrLifeContextStale) {
				code = "life_context_stale"
			}
			_ = a.FailTurnCognition(ctx, inboxID, frozen.ID, code)
			return TurnResult{}, newCapabilityError(code, true, settleErr)
		}
		if a.cognitionFactSuperseded(ctx, inboxID) {
			return TurnResult{}, errCognitionTurnSuperseded
		}
		if followupErr := a.scheduleCognitionFollowups(ctx, fluctlightID); followupErr != nil {
			// Follow-up scheduling is a best-effort lifecycle hint. The user
			// message and cognition result have already committed; a Redis or
			// clock maintenance failure must not turn this successful turn into
			// a browser retry or suppress its authoritative frame.
			slog.Warn("Go Core cognition follow-up scheduling degraded after no-op turn", "fluctlight_id", fluctlightID, "inbox_id", inboxID, "error_type", fmt.Sprintf("%T", followupErr))
		}
		if err := emitUserFrame(); err != nil {
			return TurnResult{}, err
		}
		claimSettled = true
		return TurnResult{UserMessage: user, Assistant: map[string]any{}, TurnID: turnID, CorrelationID: "turn:" + turnID}, nil
	}
	mediaIntent := ""
	if len(capabilityInvocations) > 0 {
		capabilityResults, err = a.planCapabilitiesForTransaction(ctx, fluctlightID, conversationID, inboxID, capabilityInvocations, capabilityResults)
		if err != nil {
			if len(capabilityBatchFailures(err)) == 0 {
				// Preserve the structured failure before settling the frozen action.
				// This keeps native capability diagnostics replayable instead of
				// reducing every executor error to `tool_call_failed`.
				if len(capabilityResults) > 0 {
					_ = a.PersistCapabilityResults(ctx, frozen.ID, capabilityResults)
				}
				_ = a.FailTurnCognition(ctx, inboxID, frozen.ID, "tool_call_failed")
				return TurnResult{}, newCapabilityError("tool_call_failed", true, err)
			}
			slog.Warn("Go Core capability planning degraded per call", "turn_id", turnID, "failed_call_count", len(capabilityBatchFailures(err)))
		}
		if err := a.PersistCapabilityResults(ctx, frozen.ID, capabilityResults); err != nil {
			return TurnResult{}, err
		}
		if requiredErr := requiredVisibleOutputCapabilityFailure(capabilityResults, capabilityInvocations, a.capabilityRegistry(), false); requiredErr != nil {
			_ = a.FailTurnCognition(ctx, inboxID, frozen.ID, "required_capability_failed")
			return TurnResult{}, requiredErr
		}
		if hasDeferredOutputCapabilities(capabilityInvocations, a.capabilityRegistry()) {
			// External async tools are deliberately deferred until the assistant
			// message exists, so their intent can bind to that concrete output.
			// A replay may already have a completed result; otherwise settlement
			// happens in the message transaction below.
			mediaIntent = mediaIntentIDFromCapabilityResults(capabilityResults)
		}
	}
	var visible string
	// The conversation cognition call is the single semantic pass. Its
	// visible_text is selected together with the active personality, action and
	// response plan, so it must be sent directly instead of being replaced by a
	// second action_realization request.
	if responseMode == "query_continuation" {
		if err := validatePureQueryContinuation(capabilityInvocations, a.capabilityRegistry()); err != nil {
			return TurnResult{}, err
		}
		if continuationState.Phase == "requested" {
			messages, messageErr := queryContinuationMessages(continuationState, capabilityInvocations, capabilityResults)
			if messageErr != nil {
				_ = a.FailTurnCognition(ctx, inboxID, frozen.ID, "query_continuation_query_failed")
				return TurnResult{}, messageErr
			}
			continuationState.Phase = "queries_completed"
			continuationState.Results = append([]CapabilityResult(nil), capabilityResults...)
			if err := a.persistQueryContinuationState(ctx, frozen.ID, continuationState); err != nil {
				return TurnResult{}, err
			}
			if a.cognitionFactSuperseded(ctx, inboxID) {
				return TurnResult{}, errCognitionTurnSuperseded
			}
			continuationCtx := WithProviderCorrelation(WithProviderScenario(ctx, "query_continuation"), "query-continuation:"+frozen.ID)
			run, continuationErr := a.conversationRuntime().RunQueryContinuation(continuationCtx, QueryContinuationInput{Role: "cognitive_assessment", Messages: messages, SchemaName: "query_continuation_response", Schema: queryContinuationResponseSchema()})
			completion := run.Completion
			if continuationErr != nil || len(completion.ToolCalls) > 0 {
				if continuationErr == nil {
					continuationErr = errors.New("query_continuation_tool_call_forbidden")
				}
				return TurnResult{}, continuationErr
			}
			visible, err = continuationVisibleText(completion.Structured)
			if err != nil {
				return TurnResult{}, err
			}
			if a.cognitionFactSuperseded(ctx, inboxID) {
				return TurnResult{}, errCognitionTurnSuperseded
			}
			continuationState.Phase = "provider_completed"
			continuationState.VisibleText = visible
			if err := a.persistQueryContinuationState(ctx, frozen.ID, continuationState); err != nil {
				return TurnResult{}, err
			}
		} else if continuationState.Phase == "queries_completed" {
			messages, messageErr := queryContinuationMessages(continuationState, capabilityInvocations, continuationState.Results)
			if messageErr != nil {
				return TurnResult{}, messageErr
			}
			if a.cognitionFactSuperseded(ctx, inboxID) {
				return TurnResult{}, errCognitionTurnSuperseded
			}
			continuationCtx := WithProviderCorrelation(WithProviderScenario(ctx, "query_continuation"), "query-continuation:"+frozen.ID)
			run, continuationErr := a.conversationRuntime().RunQueryContinuation(continuationCtx, QueryContinuationInput{Role: "cognitive_assessment", Messages: messages, SchemaName: "query_continuation_response", Schema: queryContinuationResponseSchema()})
			completion := run.Completion
			if continuationErr != nil || len(completion.ToolCalls) > 0 {
				return TurnResult{}, firstError(continuationErr, errors.New("query_continuation_tool_call_forbidden"))
			}
			visible, err = continuationVisibleText(completion.Structured)
			if err != nil {
				return TurnResult{}, err
			}
			if a.cognitionFactSuperseded(ctx, inboxID) {
				return TurnResult{}, errCognitionTurnSuperseded
			}
			continuationState.Phase, continuationState.VisibleText = "provider_completed", visible
			if err := a.persistQueryContinuationState(ctx, frozen.ID, continuationState); err != nil {
				return TurnResult{}, err
			}
		} else {
			visible = continuationState.VisibleText
		}
	} else {
		// R05/F02: the frozen decision already carries the single visible-text
		// authority resolved at generation time. Settlement reads it verbatim
		// instead of re-deriving the precedence (which is how the Judge or a
		// preview could audit one text while a different text was sent). The
		// reply-capability argument is deliberately NOT consulted here; the
		// shared normalizer must freeze it before this settlement boundary.
		visible = normalizeVisibleReply(stringValue(decision["visible_text"]))
		if strings.TrimSpace(visible) == "" {
			visible = normalizeVisibleReply(stringValue(responsePlan["visible_text"]))
		}
	}
	if strings.TrimSpace(visible) == "" {
		if frozenFound || frozen.ID != "" {
			_ = a.FailTurnCognition(ctx, inboxID, frozen.ID, "cognition_visible_text_missing")
		}
		return TurnResult{}, errors.New("cognition_visible_text_missing")
	}
	assistantID := randomID("message_")
	var assistant map[string]any
	reflectionDelay := a.reflectionDelay(ctx)
	nextReflectionAt := a.now().UTC().Add(reflectionDelay)
	err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if err := a.requireCognitionAuthorityRevisionsTx(ctx, tx, fluctlightID, projection.ContextRevision, projection.CurrentStateRevision, projection.LifeContextRevision, a.now().UTC()); err != nil {
			return err
		}
		if _, err := a.applyPersistentSwitchIfAuthorizedTx(ctx, tx, fluctlightID, frozen.Payload, personalityPlan); err != nil {
			return err
		}
		if err := a.applyFrozenCognitiveStagesTx(ctx, tx, fluctlightID, inboxID, decision, action, frozen.ID, frozen.StateRev); err != nil {
			return err
		}
		var existingID, existingText string
		var existingSequence int
		var existingCreatedAt time.Time
		if err := tx.QueryRow(ctx, `SELECT id,sequence,text,created_at FROM public.conversation_messages WHERE conversation_id=$1 AND idempotency_key=$2`, conversationID, "assistant:"+turnID).Scan(&existingID, &existingSequence, &existingText, &existingCreatedAt); err == nil {
			assistantID = existingID
			assistant = map[string]any{"id": existingID, "conversation_id": conversationID, "sequence": existingSequence, "author_actor_id": fluctlightID, "kind": "assistant", "text": existingText, "attachment_refs": []any{}, "created_at": existingCreatedAt.UTC().Format(time.RFC3339Nano)}
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		} else {
			var seq int
			if err := tx.QueryRow(ctx, `SELECT next_sequence FROM public.conversation_heads WHERE conversation_id=$1 FOR UPDATE`, conversationID).Scan(&seq); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE public.conversation_heads SET next_sequence=$2 WHERE conversation_id=$1`, conversationID, seq+1); err != nil {
				return err
			}
			var createdAt time.Time
			if err := tx.QueryRow(ctx, `INSERT INTO public.conversation_messages (id,conversation_id,sequence,author_actor_id,kind,text,attachment_refs,idempotency_key,turn_id,source_fact_id,correlation_id) VALUES ($1,$2,$3,$4,'assistant',$5,'[]',$6,$7,$8,$9) RETURNING created_at`, assistantID, conversationID, seq, fluctlightID, visible, "assistant:"+turnID, turnID, inboxID, "turn:"+turnID).Scan(&createdAt); err != nil {
				return err
			}
			assistant = map[string]any{"id": assistantID, "conversation_id": conversationID, "sequence": seq, "author_actor_id": fluctlightID, "kind": "assistant", "text": visible, "attachment_refs": []any{}, "created_at": createdAt.UTC().Format(time.RFC3339Nano)}
		}
		if len(capabilityInvocations) > 0 {
			composite = bindCompositeActionOutput(composite, "conversation_message", assistantID)
			bound := OutputBindingV1{ToolCallID: "", TargetKind: "conversation_message", TargetRef: assistantID}
			settled, settleErr := a.settleDeferredCapabilitiesTx(ctx, tx, fluctlightID, inboxID, inboxID, capabilityInvocations, capabilityResults, bound)
			if settleErr != nil {
				return settleErr
			}
			capabilityResults = settled
			if requiredErr := requiredVisibleOutputCapabilityFailure(capabilityResults, capabilityInvocations, a.capabilityRegistry(), true); requiredErr != nil {
				return requiredErr
			}
			if err := a.persistFrozenCapabilityInvocationsTx(ctx, tx, frozen.ID, capabilityInvocations); err != nil {
				return err
			}
			mediaIntent = mediaIntentIDFromCapabilityResults(capabilityResults)
			if _, err := tx.Exec(ctx, `UPDATE public.cognition_frozen_actions SET payload=jsonb_set(jsonb_set(jsonb_set(payload,'{capability_results}',$2::jsonb,true),'{decision,composite_action}',$3::jsonb,true),'{capability_invocations}',$4::jsonb,true) WHERE id=$1 AND status='frozen'`, frozen.ID, jsonBytes(capabilityResults), jsonBytes(composite), jsonBytes(capabilityInvocations)); err != nil {
				return err
			}
		}
		if len(capabilityInvocations) == 0 {
			composite = bindCompositeActionOutput(composite, "conversation_message", assistantID)
			if _, err := tx.Exec(ctx, `UPDATE public.cognition_frozen_actions SET payload=jsonb_set(payload,'{decision,composite_action}',$2::jsonb,true) WHERE id=$1 AND status='frozen'`, frozen.ID, jsonBytes(composite)); err != nil {
				return err
			}
		}
		if err := persistClaimsTx(ctx, tx, fluctlightID, inboxID, responsePlan); err != nil {
			return err
		}
		_, err := a.completeTurnCognitionTx(ctx, tx, inboxID, frozen.ID, map[string]any{"message_id": assistantID, "text": visible, "media_intent_id": mediaIntent, "capability_results": capabilityResults}, nextReflectionAt)
		return err
	})
	if err != nil {
		if errors.Is(err, errCognitionTurnSuperseded) {
			return TurnResult{}, err
		}
		// The output transaction rolled back. Any deferred target result is no
		// longer durable (including a capability that returned completed before
		// the rollback), so record an explicit bounded failure for replay and
		// quarantine the frozen turn. This is also required when the transaction
		// failed before the deferred settlement callback was reached.
		capabilityResults = capabilityResultsAfterSettlementFailure(capabilityResults, capabilityInvocations, a.capabilityRegistry(), "capability_settlement_failed")
		_ = a.PersistCapabilityResults(ctx, frozen.ID, capabilityResults)
		code := "capability_settlement_failed"
		if errors.Is(err, ErrLifeContextStale) {
			code = "life_context_stale"
		}
		_ = a.FailTurnCognition(ctx, inboxID, frozen.ID, code)
		return TurnResult{}, newCapabilityError(code, true, err)
	}
	if followupErr := a.scheduleCognitionFollowups(ctx, fluctlightID); followupErr != nil {
		// The assistant row and capability effects are already committed. Keep
		// delivery authoritative even when optional reflection/WakeUp hints
		// cannot be updated in this request.
		slog.Warn("Go Core cognition follow-up scheduling degraded after reply turn", "fluctlight_id", fluctlightID, "inbox_id", inboxID, "error_type", fmt.Sprintf("%T", followupErr))
	}
	if err := emitUserFrame(); err != nil {
		return TurnResult{}, err
	}
	if callbacks.onChunk != nil {
		if err := callbacks.onChunk(visible); err != nil {
			return TurnResult{}, err
		}
	}
	if err := emitAssistantFrame(assistant); err != nil {
		return TurnResult{}, err
	}
	if a.cognitionFactSuperseded(ctx, inboxID) {
		return TurnResult{}, errCognitionTurnSuperseded
	}
	claimSettled = true
	return TurnResult{UserMessage: user, Assistant: assistant, MediaIntentID: mediaIntent, TurnID: turnID, CorrelationID: "turn:" + turnID}, nil
}

func (a *App) authorizeActorTurn(ctx context.Context, ownerActorID, senderActorID, fluctlightID, conversationID string) error {
	var actorType, status string
	if err := a.DB.Pool().QueryRow(ctx, `SELECT actor_type,status FROM public.actors WHERE id=$1`, senderActorID).Scan(&actorType, &status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrUnauthorized
		}
		return err
	}
	if status != "active" || (actorType != "human" && actorType != "fluctlight") {
		return ErrUnauthorized
	}
	if actorType == "fluctlight" {
		var createdBy string
		if err := a.DB.Pool().QueryRow(ctx, `SELECT created_by_actor_id FROM public.fluctlights WHERE id=$1`, senderActorID).Scan(&createdBy); err != nil || createdBy != ownerActorID {
			return ErrUnauthorized
		}
	}
	var participants int
	if err := a.DB.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM public.conversation_participants WHERE conversation_id=$1 AND actor_id IN ($2,$3) AND status='active'`, conversationID, senderActorID, fluctlightID).Scan(&participants); err != nil {
		return err
	}
	if participants != 2 {
		return errors.New("conversation_not_found")
	}
	return nil
}

func (a *App) buildTurnProjection(ctx context.Context, authorizationActorID, speakerActorID, fluctlightID, conversationID, sourceFactID, userText string) (ContextProjection, error) {
	return a.BuildContextProjectionFor(ctx, ContextProjectionRequest{
		AuthorizationActorID:   authorizationActorID,
		SpeakerActorID:         speakerActorID,
		FluctlightID:           fluctlightID,
		ConversationID:         conversationID,
		SourceFactID:           sourceFactID,
		CurrentUserText:        userText,
		MemoryOperation:        MemoryForConversation,
		MemoryConversationMode: MemoryConversationExact,
	})
}

func (a *App) recoverFrozenTurnAfterAssistant(ctx context.Context, inboxID, fluctlightID, conversationID, assistantID, visible string) (string, bool, error) {
	frozen, found, err := a.LoadFrozenTurn(ctx, inboxID)
	if err != nil || !found {
		return "", false, err
	}
	if frozen.Status == "failed" {
		return "", false, fmt.Errorf("cognition turn is quarantined: %s", frozen.ErrorCode)
	}
	if frozen.Status != "frozen" {
		return "", false, nil
	}
	decision := mapValue(frozen.Payload["decision"])
	var personalityPlan *personalityDecisionPlan
	if raw, exists := decision["personality_transition"]; exists {
		personalityPlan, err = personalityDecisionPlanFromValue(raw)
		if err != nil || personalityPlan == nil || personalityPlan.FluctlightID != fluctlightID {
			return "", true, errors.New("personality_decision_plan_invalid")
		}
	}
	invocations, err := capabilityInvocationsFromValue(frozen.Payload["capability_invocations"])
	if err != nil {
		return "", false, err
	}
	results, err := capabilityResultsFromValue(frozen.Payload["capability_results"])
	if err != nil {
		return "", false, err
	}
	action := frozen.ActionType
	composite, ok := compositeActionFromValue(decision["composite_action"])
	if !ok {
		composite, err = normalizeCompositeAction(decision, invocations, inboxID, action)
		if err != nil {
			return "", true, err
		}
	} else {
		composite.ToolCalls = invocations
	}
	mediaIntent := mediaIntentIDFromCapabilityResults(results)
	reflectionDelay := a.reflectionDelay(ctx)
	nextReflectionAt := a.now().UTC().Add(reflectionDelay)
	err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if _, err := a.applyPersistentSwitchIfAuthorizedTx(ctx, tx, fluctlightID, frozen.Payload, personalityPlan); err != nil {
			return err
		}
		if err := a.applyFrozenCognitiveStagesTx(ctx, tx, fluctlightID, inboxID, decision, action, frozen.ID, frozen.StateRev); err != nil {
			return err
		}
		if len(invocations) > 0 {
			composite = bindCompositeActionOutput(composite, "conversation_message", assistantID)
			settled, settleErr := a.settleDeferredCapabilitiesTx(ctx, tx, fluctlightID, inboxID, inboxID, invocations, results, OutputBindingV1{TargetKind: "conversation_message", TargetRef: assistantID})
			if settleErr != nil {
				return settleErr
			}
			results = settled
			mediaIntent = mediaIntentIDFromCapabilityResults(results)
		}
		if requiredErr := requiredVisibleOutputCapabilityFailure(results, invocations, a.capabilityRegistry(), true); requiredErr != nil {
			return requiredErr
		}
		payloadUpdate := `UPDATE public.cognition_frozen_actions SET payload=jsonb_set(jsonb_set(jsonb_set(payload,'{capability_results}',$2::jsonb,true),'{capability_invocations}',$3::jsonb,true),'{decision,composite_action}',$4::jsonb,true) WHERE id=$1 AND status='frozen'`
		if _, err := tx.Exec(ctx, payloadUpdate, frozen.ID, jsonBytes(results), jsonBytes(invocations), jsonBytes(composite)); err != nil {
			return err
		}
		if err := persistClaimsTx(ctx, tx, fluctlightID, inboxID, mapValue(decision["response_plan"])); err != nil {
			return err
		}
		_, err := a.completeTurnCognitionTx(ctx, tx, inboxID, frozen.ID, map[string]any{"message_id": assistantID, "text": visible, "media_intent_id": mediaIntent, "capability_results": results}, nextReflectionAt)
		return err
	})
	if err != nil {
		if errors.Is(err, errCognitionTurnSuperseded) {
			return "", true, err
		}
		results = capabilityResultsAfterSettlementFailure(results, invocations, a.capabilityRegistry(), "capability_settlement_failed")
		_ = a.PersistCapabilityResults(ctx, frozen.ID, results)
		_ = a.FailTurnCognition(ctx, inboxID, frozen.ID, "capability_settlement_failed")
		return "", true, newCapabilityError("capability_settlement_failed", true, err)
	}
	if followupErr := a.scheduleCognitionFollowups(ctx, fluctlightID); followupErr != nil {
		// Recovery found the assistant row and completed its settlement. A
		// best-effort follow-up failure must not make an already delivered reply
		// appear failed to the browser.
		slog.Warn("Go Core cognition follow-up scheduling degraded during recovery", "fluctlight_id", fluctlightID, "inbox_id", inboxID, "error_type", fmt.Sprintf("%T", followupErr))
	}
	return mediaIntent, true, nil
}

func (a *App) createMediaIntentTargetTx(ctx context.Context, tx pgx.Tx, fluctlightID string, concept map[string]any, id, workflowID, requestID, conversationID, messageID, momentID string) error {
	if conversationID != "" && messageID != "" {
		return errors.New("media_intent_target_ambiguous")
	}
	if messageID != "" && momentID != "" {
		return errors.New("media_intent_target_ambiguous")
	}
	if messageID != "" {
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.conversation_messages WHERE id=$1)`, messageID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return ErrNotFound
		}
	}
	if momentID != "" {
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.moments WHERE id=$1)`, momentID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return ErrNotFound
		}
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.media_intents (id,owner_fluctlight_id,kind,mime_type,prompt,provider_request_id,workflow_id,conversation_id,message_id,moment_id,status,revision) VALUES ($1,$2,'image','image/png',$3,$4,$5,$6,$7,$8,'pending',0) ON CONFLICT (id) DO NOTHING`, id, fluctlightID, jsonString(concept), requestID, workflowID, nullableString(conversationID), nullableString(messageID), nullableString(momentID)); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents (intent_id,workflow_id,task_queue,intent_type,payload) VALUES ($1,$2,'media','media.generation',$3) ON CONFLICT DO NOTHING`, "media_workflow_intent:"+id, workflowID, jsonBytes(map[string]any{"intent_id": id, "provider_request_id": requestID, "fluctlight_id": fluctlightID})); err != nil {
		return err
	}
	return appendOutboxTx(ctx, tx, "media.intent.created", "media_intent", id, fluctlightID, requestID, "media:"+id, "media-intent:"+id, map[string]any{"intent_id": id, "workflow_id": workflowID, "moment_id": nullableString(momentID), "message_id": nullableString(messageID), "conversation_id": nullableString(conversationID)})
}

func (a *App) StreamTurn(ctx context.Context, writer http.ResponseWriter, actorID, conversationID string, payload map[string]any) error {
	turnID := stringValue(payload["turn_id"])
	if turnID == "" {
		turnID = "turn_" + stableDigest(conversationID+":"+stringValue(payload["idempotency_key"]))
	}
	sequence := 0
	started := false
	writeFrame := func(kind string, framePayload map[string]any) error {
		if err := json.NewEncoder(writer).Encode(map[string]any{"type": kind, "turn_id": turnID, "sequence": sequence, "payload": framePayload}); err != nil {
			return err
		}
		sequence++
		started = true
		if flusher, ok := writer.(http.Flusher); ok {
			flusher.Flush()
		}
		return nil
	}
	turnActorID := actorID
	turnPayload := payload
	if sender := strings.TrimSpace(stringValue(payload["sender_actor_id"])); sender != "" && sender != actorID {
		turnActorID = sender
		turnPayload = cloneMap(payload)
		turnPayload["authorization_actor_id"] = actorID
	}
	result, err := a.handleTurn(ctx, turnActorID, conversationID, turnPayload, turnCallbacks{
		onActionResult: func(framePayload map[string]any) error {
			return writeFrame("action_result", framePayload)
		},
		onChunk: func(chunk string) error {
			return writeFrame("token", map[string]any{"text": chunk})
		},
	}, true)
	if err != nil {
		if errors.Is(err, errCognitionTurnSuperseded) {
			return writeFrame("completed", map[string]any{"message_ids": []string{}})
		}
		if !started || ctx.Err() != nil {
			return err
		}
		// A callback may have emitted a previously committed frame, but a later
		// required settlement/lifecycle failure is never presented as success.
		errorCode := streamTurnFailureCode(err)
		slog.Default().Error("Go Core conversation turn lifecycle settlement failed after visible output", "error_type", fmt.Sprintf("%T", err), "error_code", errorCode, "turn_id", turnID)
		return writeFrame("error", map[string]any{"status": "failed", "code": errorCode})
	}
	messageIDs := make([]string, 0, 1)
	if messageID := stringValue(result.Assistant["id"]); messageID != "" {
		messageIDs = append(messageIDs, messageID)
	}
	return writeFrame("completed", map[string]any{"message_ids": messageIDs})
}

// streamTurnFailureCode preserves a small, browser-safe subset of the error
// taxonomy when the user frame has already been emitted. A failure after that
// point used to be collapsed unconditionally to conversation_settlement_failed,
// which hid whether the candidate had no visible reply, a malformed tool call,
// or a required capability settlement failure. Unknown/internal errors retain
// the generic outer code.
func streamTurnFailureCode(err error) string {
	const fallback = "conversation_settlement_failed"
	if err == nil {
		return fallback
	}
	if code := ProviderErrorCode(err); code == "tool_call_invalid" {
		return code
	}
	var capabilityErr *CapabilityError
	if errors.As(err, &capabilityErr) && capabilityErr != nil {
		if code := safeStreamTurnErrorCode(capabilityErr.Code); code != "" {
			return code
		}
	}
	if code := safeStreamTurnErrorCode(err.Error()); code != "" {
		return code
	}
	return fallback
}

func safeStreamTurnErrorCode(value string) string {
	code := strings.TrimSpace(value)
	switch code {
	case "capability_prepare_failed", "capability_settlement_failed", "cognition_visible_text_missing",
		"conversation_not_found", "conversation_settlement_failed", "conversation_turn_conflict",
		"conversation_turn_failed", "conversation_turn_invalid", "conversation_unauthorized",
		"decision_effect_invalid", "frozen_context_projection_missing", "life_context_stale",
		"media_arguments_invalid", "media_capability_unavailable", "media_context_stale",
		"media_intent_failed", "media_intent_invalid", "media_prepare_required",
		"personality_decision_plan_invalid", "query_continuation_contract_invalid",
		"query_continuation_digest_invalid", "query_continuation_query_failed",
		"query_continuation_tool_call_forbidden", "required_capability_failed",
		"structured_turn_settlement_failed", "takeover_failed", "takeover_frozen_turn_missing",
		"takeover_reply_budget_exhausted", "takeover_resume_decision_invalid",
		"takeover_resume_rule_missing", "takeover_target_profile_missing", "tool_call_failed",
		"tool_call_invalid", "turn_stage_invalid", "turn_stage_not_executable", "visible_text_source_conflict":
		return code
	default:
		return ""
	}
}

func jsonString(value any) string { data, _ := json.Marshal(value); return string(data) }
func intValue(value any) int {
	switch v := value.(type) {
	case float64:
		return int(v)
	case int:
		return v
	case json.Number:
		i, _ := v.Int64()
		return int(i)
	}
	return 0
}
func numberString(value any, fallback float64) string {
	if v, ok := value.(float64); ok {
		return fmt.Sprintf("%g", v)
	}
	if v, ok := value.(int); ok {
		return fmt.Sprintf("%d", v)
	}
	return fmt.Sprintf("%g", fallback)
}

func numberFloat(value any) (float64, bool) {
	finite := func(value float64, ok bool) (float64, bool) {
		if !ok || math.IsNaN(value) || math.IsInf(value, 0) {
			return 0, false
		}
		return value, true
	}
	switch v := value.(type) {
	case float64:
		return finite(v, true)
	case int:
		return float64(v), true
	case json.Number:
		parsed, err := v.Float64()
		return finite(parsed, err == nil)
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		return finite(parsed, err == nil)
	default:
		return 0, false
	}
}

func normalizeScheduleScalar(value any) any {
	if numeric, ok := numberFloat(value); ok {
		return numeric
	}
	switch strings.ToLower(strings.TrimSpace(stringValue(value))) {
	case "very_high", "very high", "urgent":
		return 1.0
	case "high":
		return 0.9
	case "medium", "normal":
		return 0.5
	case "low":
		return 0.1
	case "very_low", "very low":
		return 0.0
	case "none":
		return 0.0
	default:
		return value
	}
}
func firstString(value any, fallback string) string {
	if v, ok := value.(string); ok && strings.TrimSpace(v) != "" {
		return v
	}
	return fallback
}

func jsonEqual(raw []byte, value any) bool {
	var left any
	if len(raw) == 0 {
		raw = []byte("[]")
	}
	if json.Unmarshal(raw, &left) != nil {
		return false
	}
	right := value
	if right == nil {
		right = []any{}
	}
	return jsonString(left) == jsonString(right)
}
