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
			if ctx.Err() == nil {
				slog.Default().Error("Go Core conversation turn failed before visible output", "error_type", fmt.Sprintf("%T", err), "turn_id", turnID)
			}
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
	return streamTurnFailureCodeWithFallback(err, "conversation_settlement_failed")
}

func streamTurnFailureCodeWithFallback(err error, fallback string) string {
	if err == nil {
		return fallback
	}
	if code := ProviderErrorCode(err); code == "tool_call_invalid" {
		return code
	}
	if errors.Is(err, context.Canceled) {
		return "request_cancelled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "request_timeout"
	}
	if errors.Is(err, errProviderRequestFailed) {
		if code := safeStreamTurnErrorCode(providerRunErrorCode(err)); code != "" {
			return code
		}
	}
	if errors.Is(err, errAgentFinalContractInvalid) {
		return "agent_final_contract_invalid"
	}
	var capabilityErr *CapabilityError
	if errors.As(err, &capabilityErr) && capabilityErr != nil {
		if code := safeStreamTurnErrorCode(capabilityErr.Code); code != "" {
			return code
		}
	}
	errStr := strings.TrimSpace(err.Error())
	if colon := strings.Index(errStr, ":"); colon > 0 {
		prefix := strings.TrimSpace(errStr[:colon])
		if code := safeStreamTurnErrorCode(prefix); code != "" {
			return code
		}
	}
	if code := safeStreamTurnErrorCode(errStr); code != "" {
		return code
	}
	if strings.HasPrefix(errStr, "provider request failed:") {
		return "provider_request_failed"
	}
	// Tool adapters include the bounded result code after the human-readable
	// execution prefix. Recover only that closed code set; never forward the
	// dependency error or model-controlled text.
	if code := embeddedToolExecutionErrorCode(err); code != "" {
		return code
	}
	if strings.HasPrefix(errStr, "adk_run:") {
		return "adk_run_failed"
	}
	return fallback
}

// ConversationTurnFailureCode returns only a stable, public-safe code for a
// conversation turn failure. It is used by the in-process browser boundary
// when a turn fails before Core can emit its first NDJSON frame.
func ConversationTurnFailureCode(err error) string {
	return streamTurnFailureCodeWithFallback(err, "")
}

func safeStreamTurnErrorCode(value string) string {
	code := strings.TrimSpace(value)
	switch code {
	case "adk_final_message_missing", "adk_final_output_invalid", "adk_final_output_missing", "adk_final_text_missing",
		"adk_run_failed", "agent_run_failed", "agent_turn_failed", "agent_final_contract_invalid", "agent_output_publication_failed", "agent_cognition_settlement_failed", "capability_prepare_failed", "capability_settlement_failed",
		"fluctlight_inactive", "fluctlight_paused",
		"cognition_visible_text_missing", "conversation_not_found", "conversation_settlement_failed",
		"conversation_turn_conflict", "conversation_turn_failed", "conversation_turn_invalid",
		"conversation_unauthorized", "decision_effect_invalid", "frozen_context_projection_missing",
		"life_context_stale", "media_arguments_invalid", "media_capability_unavailable",
		"media_context_stale", "media_intent_failed", "media_intent_invalid", "media_prepare_required",
		"personality_decision_plan_invalid", "request_cancelled", "request_timeout",
		"required_capability_failed", "structured_turn_settlement_failed", "takeover_failed",
		"takeover_frozen_turn_missing", "takeover_reply_budget_exhausted", "takeover_resume_decision_invalid",
		"takeover_resume_rule_missing", "takeover_target_profile_missing", "tool_call_failed",
		"tool_call_invalid", "tool_execution_failed", "turn_stage_invalid", "turn_stage_not_executable", "visible_text_source_conflict":
		return code
	default:
		return ""
	}
}

func embeddedToolExecutionErrorCode(err error) string {
	if err == nil {
		return ""
	}
	errText := strings.TrimSpace(err.Error())
	marker := "tool execution "
	markerIndex := strings.Index(errText, marker)
	if markerIndex < 0 {
		return ""
	}
	toolFailure := errText[markerIndex+len(marker):]
	if colon := strings.Index(toolFailure, ":"); colon > 0 {
		return safeStreamTurnErrorCode(strings.TrimSpace(toolFailure[:colon]))
	}
	return safeStreamTurnErrorCode(toolFailure)
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
