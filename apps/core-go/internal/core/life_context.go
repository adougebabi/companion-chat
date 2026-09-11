package core

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

var ErrLifeContextStale = errors.New("life_context_stale")
var ErrFoundationRevisionStale = errors.New("foundation_revision_stale")
var ErrCurrentStateRevisionStale = errors.New("current_state_revision_stale")
var ErrContextProjectionUnstable = errors.New("context_projection_unstable")

type lifeContextQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func (a *App) readLifeContextSnapshotAt(ctx context.Context, fluctlightID string, at time.Time) (map[string]any, map[string]any, error) {
	if at.IsZero() {
		at = time.Now().UTC()
	}
	tx, err := a.DB.Pool().BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback(ctx)
	schedule, life, err := resolveLifeContextSnapshotWith(ctx, tx, fluctlightID, at.UTC())
	if err != nil {
		return nil, nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, nil, err
	}
	return schedule, life, nil
}

func (a *App) readFoundationLifeSnapshotAt(ctx context.Context, fluctlightID, ownerActorID string, at time.Time) (Fluctlight, map[string]any, map[string]any, error) {
	if at.IsZero() {
		at = time.Now().UTC()
	}
	tx, err := a.DB.Pool().BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return Fluctlight{}, nil, nil, err
	}
	defer tx.Rollback(ctx)
	row := tx.QueryRow(ctx, `
		SELECT f.id,f.core_persona,f.identity,f.personality,f.behavioral_policy,f.life_profile,f.provenance,f.status,f.current_revision,
		       0,NULL::timestamptz
		FROM public.fluctlights f
		WHERE f.id=$1 AND f.created_by_actor_id=$2`, fluctlightID, ownerActorID)
	fluctlight, err := scanFluctlight(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Fluctlight{}, nil, nil, ErrNotFound
	}
	if err != nil {
		return Fluctlight{}, nil, nil, err
	}
	schedule, life, err := resolveLifeContextSnapshotWith(ctx, tx, fluctlightID, at.UTC())
	if err != nil {
		return Fluctlight{}, nil, nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Fluctlight{}, nil, nil, err
	}
	return fluctlight, schedule, life, nil
}

func resolveLifeContextSnapshotWith(ctx context.Context, query lifeContextQuerier, fluctlightID string, at time.Time) (map[string]any, map[string]any, error) {
	timezone, err := readLifeContextTimezoneWith(ctx, query, fluctlightID)
	if err != nil {
		return nil, nil, err
	}
	schedule, err := readScheduleAtWith(ctx, query, fluctlightID, timezone, at)
	if err != nil {
		return nil, nil, err
	}
	life, err := resolveLifeContextAtWith(ctx, query, fluctlightID, schedule, timezone, at)
	if err != nil {
		return nil, nil, err
	}
	return schedule, life, nil
}

func readLifeContextTimezoneWith(ctx context.Context, query lifeContextQuerier, fluctlightID string) (string, error) {
	var identity []byte
	if err := query.QueryRow(ctx, `SELECT identity FROM public.fluctlights WHERE id=$1`, fluctlightID).Scan(&identity); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", err
	}
	timezone := canonicalTimezone(stringValue(decodeObject(identity)["timezone"]))
	if timezone == "" {
		timezone = "Asia/Shanghai"
	}
	if _, err := time.LoadLocation(timezone); err != nil {
		return "", fmt.Errorf("schedule_timezone_invalid: %w", err)
	}
	return timezone, nil
}

func readScheduleAtWith(ctx context.Context, query lifeContextQuerier, fluctlightID, timezone string, at time.Time) (map[string]any, error) {
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, fmt.Errorf("schedule_timezone_invalid: %w", err)
	}
	localToday := at.In(location).Format("2006-01-02")
	var id string
	var localDate time.Time
	var storedTimezone, status string
	var reschedulePolicy []byte
	var revision int
	err = query.QueryRow(ctx, `SELECT id,local_date,timezone,status,revision,reschedule_policy FROM public.life_schedules WHERE fluctlight_id=$1 AND status='accepted' AND local_date=$2 ORDER BY revision DESC LIMIT 1`, fluctlightID, localToday).Scan(&id, &localDate, &storedTimezone, &status, &revision, &reschedulePolicy)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	rows, err := query.Query(ctx, `SELECT id,start_at,end_at,activity,scene,location,item_type,status,priority,flexibility,interruption_cost FROM public.life_schedule_items WHERE schedule_id=$1 ORDER BY start_at`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]map[string]any, 0)
	for rows.Next() {
		var itemID, activity, scene, itemType, itemStatus, priority, flexibility, interruptionCost string
		var itemLocation *string
		var start, end time.Time
		if err := rows.Scan(&itemID, &start, &end, &activity, &scene, &itemLocation, &itemType, &itemStatus, &priority, &flexibility, &interruptionCost); err != nil {
			return nil, err
		}
		items = append(items, map[string]any{
			"id": itemID, "start_at": start.UTC().Format(time.RFC3339Nano), "end_at": end.UTC().Format(time.RFC3339Nano),
			"activity": activity, "scene": scene, "location": nullablePointerValue(itemLocation), "item_type": itemType, "status": itemStatus,
			"priority": scheduleContextNumber(priority), "flexibility": scheduleContextNumber(flexibility), "interruption_cost": scheduleContextNumber(interruptionCost),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	schedule := map[string]any{
		"id": id, "local_date": localDate.Format("2006-01-02"), "timezone": canonicalTimezone(storedTimezone),
		"revision": revision, "status": status, "completed_before": at.UTC().Format(time.RFC3339Nano),
		"reschedule_policy": decodeJSONValue(reschedulePolicy), "items": items,
	}
	if item := scheduleItemAt(schedule, at); len(item) > 0 {
		schedule["current_item_id"] = item["id"]
		schedule["current_item_effective_at"] = item["start_at"]
		schedule["current_item_expires_at"] = item["end_at"]
	}
	return schedule, nil
}

func resolveLifeContextAtWith(ctx context.Context, query lifeContextQuerier, fluctlightID string, schedule map[string]any, timezone string, at time.Time) (map[string]any, error) {
	result := pendingLifeContext(timezone, at)
	attachScheduleAuthorityTuple(result, schedule, at)
	var scene, activity, location *string
	var eventID, eventKind, eventStatus string
	var eventRevision int
	var eventStart, eventEnd time.Time
	var eventExpires *time.Time
	if err := query.QueryRow(ctx, `SELECT id,kind,status,revision,scene,activity,location,start_at,end_at,expires_at FROM public.life_events WHERE fluctlight_id=$1 AND status IN ('confirmed','inferred') AND start_at <= $2 AND end_at > $2 AND (expires_at IS NULL OR expires_at > $2) ORDER BY CASE WHEN status='confirmed' THEN 0 ELSE 1 END,start_at DESC,id DESC LIMIT 1`, fluctlightID, at).Scan(&eventID, &eventKind, &eventStatus, &eventRevision, &scene, &activity, &location, &eventStart, &eventEnd, &eventExpires); err == nil {
		result["source"] = "event"
		result["authority_status"] = eventStatus
		result["event_kind"] = eventKind
		result["event_id"] = eventID
		result["event_revision"] = eventRevision
		result["effective_at"] = eventStart.UTC().Format(time.RFC3339Nano)
		result["expires_at"] = eventEnd.UTC().Format(time.RFC3339Nano)
		if eventExpires != nil && eventExpires.Before(eventEnd) {
			result["expires_at"] = eventExpires.UTC().Format(time.RFC3339Nano)
		}
		result["scene"], result["activity"], result["location"] = nullablePointerValue(scene), nullablePointerValue(activity), nullablePointerValue(location)
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	} else {
		contextFromScheduleAt(result, schedule, at)
	}
	var presenceID, presenceActorID, presenceStatus string
	var currentTask, userPresence *string
	var presenceRevision int
	var presenceCreated time.Time
	var presenceExpires *time.Time
	if err := query.QueryRow(ctx, `SELECT id,actor_id,status,revision,current_task,user_presence,created_at,expires_at FROM public.life_presence_overlays WHERE fluctlight_id=$1 AND status='active' AND (expires_at IS NULL OR expires_at > $2) ORDER BY created_at DESC,id DESC LIMIT 1`, fluctlightID, at).Scan(&presenceID, &presenceActorID, &presenceStatus, &presenceRevision, &currentTask, &userPresence, &presenceCreated, &presenceExpires); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	} else if err == nil {
		presence := map[string]any{
			"id": presenceID, "actor_id": presenceActorID, "status": presenceStatus, "revision": presenceRevision,
			"current_task": nullablePointerValue(currentTask), "user_presence": nullablePointerValue(userPresence), "effective_at": presenceCreated.UTC().Format(time.RFC3339Nano),
		}
		if presenceExpires != nil {
			presence["expires_at"] = presenceExpires.UTC().Format(time.RFC3339Nano)
		}
		result["presence"] = presence
		result["presence_overlay"] = true
	}
	stampLifeContextRevision(result, schedule, timezone)
	return result, nil
}

func nullablePointerValue(value *string) any {
	if value == nil {
		return nil
	}
	return strings.TrimSpace(*value)
}

func pendingLifeContext(timezone string, at time.Time) map[string]any {
	result := map[string]any{
		"source": "pending", "authority_status": "pending", "scene": nil, "activity": nil, "location": nil,
		"instant": at.UTC().Format(time.RFC3339Nano), "timezone": timezone,
	}
	if location, err := time.LoadLocation(timezone); err == nil {
		local := at.In(location)
		start := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
		result["local_date"] = start.Format("2006-01-02")
		result["effective_at"] = start.UTC().Format(time.RFC3339Nano)
		result["expires_at"] = start.AddDate(0, 0, 1).UTC().Format(time.RFC3339Nano)
	}
	return result
}

func contextFromScheduleAt(result map[string]any, schedule map[string]any, at time.Time) {
	item := scheduleItemAt(schedule, at)
	if len(item) == 0 {
		return
	}
	result["source"] = "schedule"
	result["authority_status"] = "accepted"
	result["effective_at"] = item["start_at"]
	result["expires_at"] = item["end_at"]
	result["scene"] = item["scene"]
	result["activity"] = item["activity"]
	result["location"] = item["location"]
}

func scheduleItemAt(schedule map[string]any, at time.Time) map[string]any {
	if schedule == nil {
		return nil
	}
	for _, raw := range arrayValue(schedule["items"]) {
		item := mapValue(raw)
		start, startErr := time.Parse(time.RFC3339Nano, stringValue(item["start_at"]))
		end, endErr := time.Parse(time.RFC3339Nano, stringValue(item["end_at"]))
		if startErr == nil && endErr == nil && !at.Before(start) && at.Before(end) {
			return item
		}
	}
	return nil
}

func attachScheduleAuthorityTuple(result, schedule map[string]any, at time.Time) {
	if schedule == nil {
		return
	}
	result["schedule_id"] = schedule["id"]
	result["schedule_revision"] = schedule["revision"]
	if item := scheduleItemAt(schedule, at); len(item) > 0 {
		result["schedule_item_id"] = item["id"]
		result["schedule_item_effective_at"] = item["start_at"]
		result["schedule_item_expires_at"] = item["end_at"]
	}
}

func stampLifeContextRevision(life, schedule map[string]any, timezone string) {
	authority := map[string]any{
		"source": life["source"], "authority_status": life["authority_status"], "timezone": timezone,
		"local_date": life["local_date"], "effective_at": life["effective_at"], "expires_at": life["expires_at"],
		"event_id": life["event_id"], "event_revision": life["event_revision"],
		"schedule_id": life["schedule_id"], "schedule_revision": life["schedule_revision"], "schedule_item_id": life["schedule_item_id"],
		"schedule_item_effective_at": life["schedule_item_effective_at"], "schedule_item_expires_at": life["schedule_item_expires_at"],
	}
	if schedule != nil && authority["local_date"] == nil {
		authority["local_date"] = schedule["local_date"]
	}
	if presence := mapValue(life["presence"]); len(presence) > 0 {
		authority["presence_id"] = presence["id"]
		authority["presence_revision"] = presence["revision"]
		authority["presence_effective_at"] = presence["effective_at"]
		authority["presence_expires_at"] = presence["expires_at"]
	}
	digest := stableDigest(jsonString(authority))
	life["context_revision"] = "life_ctx_" + digest
	parsed, _ := strconv.ParseUint(digest[:8], 16, 32)
	revision := int(parsed & 0x7fffffff)
	if revision == 0 {
		revision = 1
	}
	life["revision"] = revision
}

func lockLifeContextTx(ctx context.Context, tx pgx.Tx, fluctlightID string) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "life-context:"+strings.TrimSpace(fluctlightID))
	return err
}

func (a *App) requireLifeContextRevisionTx(ctx context.Context, tx pgx.Tx, fluctlightID, expectedRevision string, at time.Time) (map[string]any, error) {
	if strings.TrimSpace(expectedRevision) == "" {
		return nil, errors.New("life_context_revision_required")
	}
	if err := lockLifeContextTx(ctx, tx, fluctlightID); err != nil {
		return nil, err
	}
	_, life, err := resolveLifeContextSnapshotWith(ctx, tx, fluctlightID, at.UTC())
	if err != nil {
		return nil, err
	}
	if stringValue(life["context_revision"]) != strings.TrimSpace(expectedRevision) {
		return life, ErrLifeContextStale
	}
	return life, nil
}

func (a *App) validateLifeContextRevision(ctx context.Context, fluctlightID, expectedRevision string, at time.Time) error {
	if strings.TrimSpace(expectedRevision) == "" {
		return errors.New("life_context_revision_required")
	}
	_, life, err := a.readLifeContextSnapshotAt(ctx, fluctlightID, at)
	if err != nil {
		return err
	}
	if stringValue(life["context_revision"]) != strings.TrimSpace(expectedRevision) {
		return ErrLifeContextStale
	}
	return nil
}

func readCognitionAuthorityRevisionsWith(ctx context.Context, query lifeContextQuerier, fluctlightID string, at time.Time) (int, int, string, error) {
	var foundationRevision int
	if err := query.QueryRow(ctx, `SELECT current_revision FROM public.fluctlights WHERE id=$1`, fluctlightID).Scan(&foundationRevision); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, 0, "", ErrNotFound
		}
		return 0, 0, "", err
	}
	currentStateRevision := 0
	if err := query.QueryRow(ctx, `SELECT revision FROM public.fluctlight_inner_states WHERE fluctlight_id=$1`, fluctlightID).Scan(&currentStateRevision); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return 0, 0, "", err
	}
	_, life, err := resolveLifeContextSnapshotWith(ctx, query, fluctlightID, at.UTC())
	if err != nil {
		return 0, 0, "", err
	}
	return foundationRevision, currentStateRevision, stringValue(life["context_revision"]), nil
}

func compareCognitionAuthorityRevisions(expectedFoundation, expectedCurrentState int, expectedLife string, actualFoundation, actualCurrentState int, actualLife string) error {
	if expectedFoundation != actualFoundation {
		return ErrFoundationRevisionStale
	}
	if expectedCurrentState != actualCurrentState {
		return ErrCurrentStateRevisionStale
	}
	if strings.TrimSpace(expectedLife) == "" || strings.TrimSpace(expectedLife) != actualLife {
		return ErrLifeContextStale
	}
	return nil
}

func cognitionAuthorityRevisionsFromValue(value any) (int, int, string, error) {
	revisions := mapValue(value)
	foundationRaw, foundationOK := revisions["foundation_revision"]
	currentStateRaw, currentStateOK := revisions["current_state_revision"]
	lifeRevision := strings.TrimSpace(stringValue(revisions["life_context_revision"]))
	if !foundationOK || !currentStateOK || lifeRevision == "" {
		return 0, 0, "", errors.New("cognition_authority_revisions_required")
	}
	foundationRevision, foundationValid := nonNegativeRevision(foundationRaw)
	currentStateRevision, currentStateValid := nonNegativeRevision(currentStateRaw)
	if !foundationValid || !currentStateValid {
		return 0, 0, "", errors.New("cognition_authority_revisions_invalid")
	}
	return foundationRevision, currentStateRevision, lifeRevision, nil
}

func cognitionAuthorityStaleCode(err error) string {
	switch {
	case errors.Is(err, ErrFoundationRevisionStale):
		return "foundation_revision_stale"
	case errors.Is(err, ErrCurrentStateRevisionStale):
		return "current_state_revision_stale"
	case errors.Is(err, ErrLifeContextStale):
		return "life_context_stale"
	default:
		return ""
	}
}

func (a *App) validateCognitionAuthorityRevisions(ctx context.Context, fluctlightID string, expectedFoundation, expectedCurrentState int, expectedLife string, at time.Time) error {
	tx, err := a.DB.Pool().BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	actualFoundation, actualCurrentState, actualLife, err := readCognitionAuthorityRevisionsWith(ctx, tx, fluctlightID, at)
	if err != nil {
		return err
	}
	if err := compareCognitionAuthorityRevisions(expectedFoundation, expectedCurrentState, expectedLife, actualFoundation, actualCurrentState, actualLife); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (a *App) requireCognitionAuthorityRevisionsTx(ctx context.Context, tx pgx.Tx, fluctlightID string, expectedFoundation, expectedCurrentState int, expectedLife string, at time.Time) error {
	if err := lockLifeContextTx(ctx, tx, fluctlightID); err != nil {
		return err
	}
	actualFoundation, actualCurrentState, actualLife, err := readCognitionAuthorityRevisionsWith(ctx, tx, fluctlightID, at)
	if err != nil {
		return err
	}
	return compareCognitionAuthorityRevisions(expectedFoundation, expectedCurrentState, expectedLife, actualFoundation, actualCurrentState, actualLife)
}

// applyLifeContextTimezoneChangeTx closes the otherwise hidden Life Context
// write path through Foundation identity. Historical schedules stay intact;
// currently accepted schedules whose local day is no longer trustworthy are
// superseded, and the change re-enters cognition as a typed fact.
func (a *App) applyLifeContextTimezoneChangeTx(
	ctx context.Context,
	tx pgx.Tx,
	fluctlightID string,
	actorID string,
	sourceID string,
	resultingFoundationRevision int,
	previousIdentity map[string]any,
	resultingIdentity map[string]any,
) error {
	previousTimezone := canonicalTimezone(stringValue(previousIdentity["timezone"]))
	if previousTimezone == "" {
		previousTimezone = "Asia/Shanghai"
	}
	resultingTimezone := canonicalTimezone(stringValue(resultingIdentity["timezone"]))
	if resultingTimezone == "" {
		resultingTimezone = "Asia/Shanghai"
	}
	if previousTimezone == resultingTimezone {
		return nil
	}
	previousLocation, previousErr := time.LoadLocation(previousTimezone)
	resultingLocation, resultingErr := time.LoadLocation(resultingTimezone)
	if previousErr != nil || resultingErr != nil {
		return errors.New("schedule_timezone_invalid")
	}
	now := time.Now().UTC()
	cutoff := now.In(previousLocation).Format("2006-01-02")
	if resultingDate := now.In(resultingLocation).Format("2006-01-02"); resultingDate < cutoff {
		cutoff = resultingDate
	}
	if _, err := tx.Exec(ctx, `
		UPDATE public.life_schedules
		SET status='superseded',updated_at=$3
		WHERE fluctlight_id=$1 AND status='accepted' AND local_date >= $2::date`, fluctlightID, cutoff, now); err != nil {
		return err
	}
	factKey := "timezone:" + stableDigest(fluctlightID+"\x1f"+sourceID+"\x1f"+strconv.Itoa(resultingFoundationRevision))
	inboxID, err := a.enqueueNativeFactTx(ctx, tx, fluctlightID, "", sourceID, "life.timezone.changed", factKey, map[string]any{
		"previous_timezone": previousTimezone, "resulting_timezone": resultingTimezone,
		"foundation_revision": resultingFoundationRevision,
	})
	if err != nil {
		return err
	}
	return appendOutboxTx(ctx, tx, "life.timezone.changed", "fluctlight", fluctlightID, fluctlightID, sourceID, "timezone:"+fluctlightID, "timezone:"+fluctlightID+":"+stableDigest(sourceID), map[string]any{
		"previous_timezone": previousTimezone, "resulting_timezone": resultingTimezone,
		"foundation_revision": resultingFoundationRevision, "inbox_id": inboxID, "actor_id": actorID,
		"aggregate_sequence": resultingFoundationRevision,
	})
}
