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
	a.cancelSupersededCognitionFacts(ctx, supersededInboxIDs)
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
	var decision map[string]any
	var action string
	var capabilityInvocations []CapabilityInvocation
	var capabilityResults []CapabilityResult
	var responsePlan map[string]any
	var composite CompositeActionV1
	var personalityPlan *personalityDecisionPlan
	responseMode := "final"
	var continuationBaseMessages []map[string]any
	toolOnlyNoReply := false
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
	if frozenFound && frozen.Status == "frozen" {
		frozenDecision := mapValue(frozen.Payload["decision"])
		if savedProjection, ok := contextProjectionFromValue(frozenDecision["context_projection"]); ok {
			projection = savedProjection
		} else {
			return TurnResult{}, errors.New("frozen_context_projection_missing")
		}
	} else {
		projection, err = a.buildTurnProjection(ctx, authorizationActorID, actorID, fluctlightID, conversationID, inboxID, text)
		if err != nil {
			return TurnResult{}, err
		}
	}
	if frozenFound && frozen.Status == "frozen" {
		action = frozen.ActionType
		decision = mapValue(frozen.Payload["decision"])
		if err := validateFrozenDecisionInfluences(decision); err != nil {
			return TurnResult{}, err
		}
		if raw, exists := decision["personality_transition"]; exists {
			personalityPlan, err = personalityDecisionPlanFromValue(raw)
			if err != nil || personalityPlan == nil || personalityPlan.FluctlightID != fluctlightID {
				return TurnResult{}, errors.New("personality_decision_plan_invalid")
			}
		}
		if savedProjection, ok := contextProjectionFromValue(decision["context_projection"]); ok {
			projection = savedProjection
		}
		capabilityInvocations, err = capabilityInvocationsFromValue(frozen.Payload["capability_invocations"])
		if err != nil {
			return TurnResult{}, err
		}
		if loaded, ok := compositeActionFromValue(decision["composite_action"]); ok {
			composite = loaded
			composite.ToolCalls = capabilityInvocations
			if len(composite.CapabilityCallIDs) == 0 {
				composite.CapabilityCallIDs = capabilityCallIDs(capabilityInvocations)
			}
		} else {
			return TurnResult{}, errors.New("frozen_composite_action_missing")
		}
		capabilityResults, err = capabilityResultsFromValue(frozen.Payload["capability_results"])
		if err != nil {
			return TurnResult{}, err
		}
		responsePlan = mapValue(decision["response_plan"])
		if len(responsePlan) == 0 {
			return TurnResult{}, errors.New("frozen_response_plan_missing")
		}
		responseMode = firstString(responsePlan["response_mode"], firstString(decision["response_mode"], "final"))
		continuationBaseMessages, _ = decision["continuation_base_messages"].([]map[string]any)
		if continuationBaseMessages == nil {
			continuationBaseMessages = cloneMapSliceFromAny(decision["continuation_base_messages"])
		}
	} else {
		// Moment publication is a Wake-up/autonomy output, not an ordinary
		// interactive reply capability. Keep it registered globally for the
		// Runtime while withholding it from the conversation tool catalog.
		definitions := capabilityCatalog(a.capabilityRegistry(), CapabilitySurfaceConversation)
		schema := cognitiveTurnResponseSchema()
		assembly, assembledProjection, assemblyErr := a.assembleProjectionPrompt(ctx, projection, "cognitive_assessment", []string{providerContextAuthorityRule, capabilityConversationPolicyInstruction}, text, definitions, "conversation_turn_response", schema)
		if assemblyErr != nil {
			return TurnResult{}, assemblyErr
		}
		projection = assembledProjection
		continuationBaseMessages = cloneMapSlice(assembly.Messages)
		providerCtx := WithPromptDiagnostics(WithProviderScenario(ctx, "cognitive_assessment"), assembly.Diagnostics)
		completion, completionErr := a.Provider.StructuredAssembledWithToolsSchema(providerCtx, "cognitive_assessment", assembly.Messages, definitions, "conversation_turn_response", schema, true)
		if completionErr != nil {
			if a.cognitionFactSuperseded(ctx, inboxID) {
				return TurnResult{}, errCognitionTurnSuperseded
			}
			return TurnResult{}, completionErr
		}
		if a.cognitionFactSuperseded(ctx, inboxID) {
			return TurnResult{}, errCognitionTurnSuperseded
		}
		decision = completion.Structured
		structuredFallback = completion.StructuredFallback
		capabilityInvocations = append([]CapabilityInvocation(nil), completion.ToolCalls...)
		for index := range capabilityInvocations {
			capabilityInvocations[index] = normalizeCapabilityInvocationMetadata(capabilityInvocations[index], fluctlightID, conversationID, inboxID, inboxID, index)
		}
		if decision == nil {
			decision = map[string]any{}
		}
		// A thinking-enabled Provider may return only an immediate native tool
		// call (for example scene_event) and no JSON sidecar. This is a valid
		// capability-only outcome, but it is not evidence for a synthetic affect
		// appraisal.
		toolOnlyNoReply = completion.StructuredFallback && len(capabilityInvocations) > 0 && !hasConversationReplyCapability(capabilityInvocations, a.capabilityRegistry()) && !hasDeferredOutputCapabilities(capabilityInvocations, a.capabilityRegistry())
		skipCognitiveStateTransition := toolOnlyNoReply && len(mapValue(decision["appraisal"])) == 0
		if toolOnlyNoReply {
			decision["action_type"] = "no_op"
			decision["response_intent"] = ""
		}
		// Influences are validated against exactly the Core-owned projection that
		// the Provider saw. The mapping is frozen before a personality switch or
		// any other state transition can refresh execution context.
		if _, err := freezeDecisionInfluences(decision, projection, false); err != nil {
			return TurnResult{}, err
		}
		if skipCognitiveStateTransition {
			decision["cognitive_state_transition"] = "not_proposed"
		}
		if personalityDecision := mapValue(decision["personality_decision"]); len(personalityDecision) > 0 {
			personalityPlan, err = a.preparePersonalityDecision(ctx, fluctlightID, personalityDecision)
			if err != nil {
				return TurnResult{}, err
			}
			if personalityPlan != nil {
				decision["personality_transition"] = personalityPlan
			}
		}
		// Normalize the root sidecar once; the response plan never receives a
		// second nested tool_calls copy.
		if len(capabilityInvocations) > 0 {
			decision["capability_invocations"] = capabilityInvocations
		}
		responsePlan, err = normalizeResponsePlan(decision, inboxID, projection)
		if err != nil {
			return TurnResult{}, err
		}
		// Root tool_calls is the sole provider codec sidecar. Keep it on the
		// frozen decision; response_plan is a visible-plan projection only.
		decision["response_plan"] = responsePlan
		decision["context_projection"] = projection
		visibleCandidate := normalizeVisibleReply(firstString(responsePlan["visible_text"], stringValue(decision["visible_text"])))
		responseMode = normalizeConversationResponseMode(stringValue(decision["response_mode"]), structuredFallback, visibleCandidate, capabilityInvocations, a.capabilityRegistry())
		responsePlan["response_mode"] = responseMode
		if len(capabilityInvocations) > 0 {
			definitionMap := make(map[string]CapabilityDefinition, len(definitions))
			for _, definition := range definitions {
				definitionMap[definition.Name] = definition
			}
			action, err = resolveCapabilityAction(capabilityInvocations, definitionMap)
			if err != nil {
				return TurnResult{}, err
			}
			if toolOnlyNoReply {
				action = "no_op"
			}
		} else {
			action = normalizeConversationActionType(stringValue(decision["action_type"]))
		}
		// A direct user turn has exactly one terminal product contract: either the
		// same Main cognition supplies visible text, or the turn fails explicitly
		// and remains retryable. A successful no-op makes the user's double-check
		// message look delivered while producing no assistant row.
		if responseMode == "query_continuation" {
			if visibleCandidate != "" || validatePureQueryContinuation(capabilityInvocations, a.capabilityRegistry()) != nil {
				return TurnResult{}, errors.New("query_continuation_contract_invalid")
			}
			decision["continuation_base_messages"] = continuationBaseMessages
			delete(responsePlan, "visible_text")
			delete(decision, "visible_text")
		} else {
			if responseMode != "final" {
				return TurnResult{}, errors.New("response_mode_invalid")
			}
			if visibleCandidate == "" {
				visibleCandidate = replyTextFromCapabilityInvocations(capabilityInvocations, a.capabilityRegistry())
			}
			if visibleCandidate == "" {
				return TurnResult{}, errors.New("cognition_visible_text_missing")
			}
			responsePlan["visible_text"] = visibleCandidate
			decision["visible_text"] = visibleCandidate
		}
		action = "reply"
		decision["action_type"] = "reply"
		if preferenceDecision := mapValue(responsePlan["output_preference_decision"]); len(preferenceDecision) > 0 {
			responsePlan["output_preference_decision"] = evaluateOutputPreferenceAction(preferenceDecision, action, capabilityInvocations, a.capabilityRegistry())
		}
		composite, err = normalizeCompositeAction(decision, capabilityInvocations, inboxID, action)
		if err != nil {
			return TurnResult{}, err
		}
		decision["composite_action"] = composite
		if action != "reply" && action != "no_op" {
			return TurnResult{}, errors.New("decision_effect_invalid")
		}
		if a.cognitionFactSuperseded(ctx, inboxID) {
			return TurnResult{}, errCognitionTurnSuperseded
		}
		frozen, err = a.PersistTurnDecision(ctx, inboxID, fluctlightID, conversationID, turnID, action, decision)
		if err != nil {
			return TurnResult{}, err
		}
	}
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
	if action == "no_op" {
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
			code, _ := capabilityErrorInfo(err, "capability_prepare_failed", true)
			_ = a.FailTurnCognition(ctx, inboxID, frozen.ID, code)
			return TurnResult{}, err
		}
		// The prepared invocation is the crash/replay boundary. No Capability may
		// execute until its runtime-owned plan and context snapshot are durable.
		if err := a.persistFrozenCapabilityInvocations(ctx, frozen.ID, capabilityInvocations); err != nil {
			return TurnResult{}, err
		}
	}
	if action == "no_op" {
		if len(capabilityInvocations) > 0 {
			capabilityResults, err = a.planCapabilitiesForTransaction(ctx, fluctlightID, conversationID, inboxID, capabilityInvocations, capabilityResults)
			if err != nil {
				// Optional capability failures remain structured diagnostics. Required
				// state-changing failures are checked immediately below and quarantine
				// the frozen turn before any visible output exists.
				slog.Default().Warn("Go Core capability failed during no-op turn", "turn_id", turnID, "error", err, "capability_results", capabilityResults)
			}
		}
		reflectionDelay := a.reflectionDelay(ctx)
		nextReflectionAt := time.Now().UTC().Add(reflectionDelay)
		// Appraisal/Current State, native mutations, claims, action result and
		// inbox settlement share this one transaction. A failure leaves only the
		// immutable frozen plan for deterministic retry/quarantine.
		settleErr := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
			if err := a.requireCognitionAuthorityRevisionsTx(ctx, tx, fluctlightID, projection.ContextRevision, projection.CurrentStateRevision, projection.LifeContextRevision, time.Now().UTC()); err != nil {
				return err
			}
			if _, err := a.applyPersonalityDecisionPlanTx(ctx, tx, fluctlightID, personalityPlan); err != nil {
				return err
			}
			if err := a.applyFrozenCognitiveStagesTx(ctx, tx, fluctlightID, inboxID, decision, action, frozen.ID, frozen.StateRev); err != nil {
				return err
			}
			if len(capabilityInvocations) > 0 {
				settled, settleErr := a.settleDeferredCapabilitiesTx(ctx, tx, fluctlightID, inboxID, inboxID, capabilityInvocations, capabilityResults, OutputBindingV1{})
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
				if requiredErr := requiredCapabilityFailureCanonical(capabilityResults, capabilityInvocations, a.capabilityRegistry(), false); requiredErr != nil {
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
			return TurnResult{}, settleErr
		}
		if a.cognitionFactSuperseded(ctx, inboxID) {
			return TurnResult{}, errCognitionTurnSuperseded
		}
		a.scheduleReflectionTrigger(ctx, "reflection_intent:"+inboxID, reflectionDelay)
		a.scheduleWakeUpTrigger(ctx, fluctlightID, int(reflectionDelay/time.Second))
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
			// Preserve the structured failure before settling the frozen action.
			// This keeps native capability diagnostics replayable instead of
			// reducing every executor error to `tool_call_failed`.
			if len(capabilityResults) > 0 {
				_ = a.PersistCapabilityResults(ctx, frozen.ID, capabilityResults)
			}
			_ = a.FailTurnCognition(ctx, inboxID, frozen.ID, "tool_call_failed")
			return TurnResult{}, err
		}
		if err := a.PersistCapabilityResults(ctx, frozen.ID, capabilityResults); err != nil {
			return TurnResult{}, err
		}
		if requiredErr := requiredCapabilityFailureCanonical(capabilityResults, capabilityInvocations, a.capabilityRegistry(), false); requiredErr != nil {
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
			completion, continuationErr := a.Provider.StructuredQueryContinuation(WithProviderCorrelation(WithProviderScenario(ctx, "query_continuation"), "query-continuation:"+frozen.ID), "cognitive_assessment", messages, "query_continuation_response", queryContinuationResponseSchema())
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
			completion, continuationErr := a.Provider.StructuredQueryContinuation(WithProviderCorrelation(WithProviderScenario(ctx, "query_continuation"), "query-continuation:"+frozen.ID), "cognitive_assessment", messages, "query_continuation_response", queryContinuationResponseSchema())
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
		visible = normalizeVisibleReply(firstString(responsePlan["visible_text"], stringValue(decision["visible_text"])))
		if strings.TrimSpace(visible) == "" {
			visible = replyTextFromCapabilityInvocations(capabilityInvocations, a.capabilityRegistry())
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
	nextReflectionAt := time.Now().UTC().Add(reflectionDelay)
	err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if err := a.requireCognitionAuthorityRevisionsTx(ctx, tx, fluctlightID, projection.ContextRevision, projection.CurrentStateRevision, projection.LifeContextRevision, time.Now().UTC()); err != nil {
			return err
		}
		if _, err := a.applyPersonalityDecisionPlanTx(ctx, tx, fluctlightID, personalityPlan); err != nil {
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
			if requiredErr := requiredCapabilityFailureCanonical(capabilityResults, capabilityInvocations, a.capabilityRegistry(), true); requiredErr != nil {
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
		return TurnResult{}, err
	}
	a.scheduleReflectionTrigger(ctx, "reflection_intent:"+inboxID, reflectionDelay)
	a.scheduleWakeUpTrigger(ctx, fluctlightID, int(reflectionDelay/time.Second))
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
	nextReflectionAt := time.Now().UTC().Add(reflectionDelay)
	err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if _, err := a.applyPersonalityDecisionPlanTx(ctx, tx, fluctlightID, personalityPlan); err != nil {
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
		if requiredErr := requiredCapabilityFailureCanonical(results, invocations, a.capabilityRegistry(), true); requiredErr != nil {
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
		return "", true, err
	}
	a.scheduleReflectionTrigger(ctx, "reflection_intent:"+inboxID, reflectionDelay)
	a.scheduleWakeUpTrigger(ctx, fluctlightID, int(reflectionDelay/time.Second))
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
		slog.Default().Error("Go Core conversation turn lifecycle settlement failed after visible output", "error", err, "turn_id", turnID)
		return writeFrame("error", map[string]any{"status": "failed", "error": "conversation_settlement_failed"})
	}
	messageIDs := make([]string, 0, 1)
	if messageID := stringValue(result.Assistant["id"]); messageID != "" {
		messageIDs = append(messageIDs, messageID)
	}
	return writeFrame("completed", map[string]any{"message_ids": messageIDs})
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
