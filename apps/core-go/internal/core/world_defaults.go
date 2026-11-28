package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Base NPCs used to be seeded with position_mode='unknown' and an empty
// routine, so they were never anywhere and no encounter could ever happen.
// They now get a daily routine through the world's shops and public places
// (never the character's home) and no home place: outside the routine they
// are simply not in any modelled place.
const seedNPCBackground = "这个世界的独立居民；尚无与角色相识或实际在场的记录。"

var seedNPCNames = []string{"周予安", "林晚晴"}

// Preferred default-framework places, in routine order. Custom frameworks fall
// back to any active non-home place.
var seedNPCPlaceKeys = []string{`["shops","grocer"]`, `["shops","food"]`, `["public","square"]`, `["public","meeting"]`}

func worldNPCRoutinePlacesTx(ctx context.Context, tx pgx.Tx, world string) ([]string, error) {
	rows, err := tx.Query(ctx, `SELECT id,COALESCE(framework_key,'') FROM public.world_places WHERE world_id=$1 AND status='active' AND (framework_key IS NULL OR framework_key NOT LIKE '["home",%') ORDER BY framework_key NULLS LAST,id`, world)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byKey := map[string]string{}
	others := []string{}
	for rows.Next() {
		var id, key string
		if err := rows.Scan(&id, &key); err != nil {
			return nil, err
		}
		byKey[key] = id
		others = append(others, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	places := []string{}
	used := map[string]bool{}
	for _, key := range seedNPCPlaceKeys {
		if id := byKey[key]; id != "" {
			places, used[id] = append(places, id), true
		}
	}
	for _, id := range others {
		if !used[id] {
			places = append(places, id)
		}
	}
	return places, nil
}

// defaultNPCRoutine gives each base NPC a distinct, non-overlapping daily
// routine over the given public places. An empty list yields no routine.
func defaultNPCRoutine(places []string, index int) []any {
	if len(places) == 0 {
		return []any{}
	}
	at := func(i int) string { return places[i%len(places)] }
	type slot struct{ start, end, activity string }
	var slots []slot
	var picks []int
	if index%2 == 0 {
		slots = []slot{{"09:00", "12:00", "看店、整理货架"}, {"12:00", "13:30", "吃午饭"}, {"13:30", "18:30", "看店、和熟客聊天"}, {"19:00", "21:00", "饭后散步"}}
		picks = []int{0, 1, 0, 2}
	} else {
		slots = []slot{{"10:00", "12:00", "晨间散步、晒太阳"}, {"12:00", "13:30", "吃午饭"}, {"14:00", "17:30", "在会客处看书、和人聊天"}, {"18:00", "20:00", "傍晚散步"}}
		picks = []int{2, 1, 3, 2}
	}
	routine := make([]any, 0, len(slots))
	for i, s := range slots {
		routine = append(routine, map[string]any{"day": "daily", "start": s.start, "end": s.end, "place_id": at(picks[i]), "activity": s.activity})
	}
	return routine
}

// ensureSeedNPCRoutinesTx gives untouched base NPCs (seed background, unknown
// position, empty routine) their default routine. Owner-edited NPCs are left
// alone because any edit changes the background, routine or position mode.
func (a *App) ensureSeedNPCRoutinesTx(ctx context.Context, tx pgx.Tx, world string) (int, error) {
	places, err := worldNPCRoutinePlacesTx(ctx, tx, world)
	if err != nil || len(places) == 0 {
		return 0, err
	}
	updated := 0
	for index, name := range seedNPCNames {
		var id string
		err := tx.QueryRow(ctx, `SELECT id FROM public.world_npcs WHERE world_id=$1 AND name=$2 AND background=$3 AND position_mode='unknown' AND routine='[]'::jsonb FOR UPDATE`, world, name, seedNPCBackground).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return updated, err
		}
		if _, err := tx.Exec(ctx, `UPDATE public.world_npcs SET routine=$2,position_mode='routine',home_place_id=NULL,revision=revision+1,updated_at=now() WHERE id=$1`, id, jsonBytes(defaultNPCRoutine(places, index))); err != nil {
			return updated, err
		}
		if err := invalidateNPCPresenceTx(ctx, tx, id, a.now().UTC(), "npc_definition_changed"); err != nil {
			return updated, err
		}
		updated++
	}
	return updated, nil
}

// Built-in starter events were seeded manual and disabled. Untouched ones
// (revision 1, still manual, disabled) become automatic with a modest chance,
// so a world has some weather/atmosphere without the Owner wiring it up. An
// Owner who toggled a starter bumped its revision and keeps their choice.
func (a *App) enableStarterWorldEventsTx(ctx context.Context, tx pgx.Tx, world string) (int, error) {
	tag, err := tx.Exec(ctx, `UPDATE public.world_event_templates SET enabled=true,
 definition=jsonb_set(jsonb_set(jsonb_set(definition,'{trigger,mode}','"automatic"'),'{trigger,interval_minutes}','180'),'{trigger,probability}','0.35'),
 revision=revision+1,next_check_at=$2,error_code=NULL,updated_at=now()
 WHERE world_id=$1 AND id LIKE 'world_template_starter_%' AND revision=1 AND enabled=false AND definition->'trigger'->>'mode'='manual'`, world, a.now().UTC())
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

// EnsureWorldDefaults backfills existing worlds created before NPC routines
// and automatic starter events existed. Both steps only touch untouched seed
// rows, so it is idempotent and cheap once a world is done.
func (a *App) EnsureWorldDefaults(ctx context.Context, limit int) (int, error) {
	if limit < 1 || limit > 100 {
		return 0, ErrInvalidArguments
	}
	worlds := []string{}
	rows, err := a.DB.Pool().Query(ctx, `SELECT DISTINCT world_id FROM (
 SELECT world_id FROM public.world_npcs WHERE background=$1 AND position_mode='unknown' AND routine='[]'::jsonb
 UNION SELECT world_id FROM public.world_event_templates WHERE id LIKE 'world_template_starter_%' AND revision=1 AND enabled=false AND definition->'trigger'->>'mode'='manual'
) w ORDER BY world_id LIMIT $2`, seedNPCBackground, limit)
	if err != nil {
		return 0, err
	}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		worlds = append(worlds, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	count := 0
	failures := []error{}
	for _, world := range worlds {
		err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
			if err := lockWorldRuntimeTx(ctx, tx, world); err != nil {
				return err
			}
			npcs, err := a.ensureSeedNPCRoutinesTx(ctx, tx, world)
			if err != nil {
				return err
			}
			starters, err := a.enableStarterWorldEventsTx(ctx, tx, world)
			count += npcs + starters
			return err
		})
		if err != nil {
			failures = append(failures, fmt.Errorf("world defaults %s: %w", world, err))
		}
	}
	return count, errors.Join(failures...)
}

const scheduleArrivalEventKind = "schedule_arrival"

// EnsureScheduledArrivals turns "the schedule says I am at place X now" into
// an inferred life event at X for the rest of that item. Presence at a place
// needs an event (the schedule alone is only a plan), so without this a
// character living by its schedule never counted as anywhere: no NPC
// opportunity, no nearby people, no place-scoped world event. The event mirrors
// the schedule item exactly, so the world display does not change, and being
// inferred it never outranks a confirmed event or a newer scene change.
func (a *App) EnsureScheduledArrivals(ctx context.Context, limit int) (int, error) {
	if limit < 1 || limit > 500 {
		return 0, ErrInvalidArguments
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT f.id FROM public.fluctlight_world_bindings b JOIN public.fluctlights f ON f.id=b.fluctlight_id WHERE f.status='active' ORDER BY f.id LIMIT $1`, limit)
	if err != nil {
		return 0, err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	count := 0
	failures := []error{}
	for _, fid := range ids {
		err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
			if err := lockLifeContextTx(ctx, tx, fid); err != nil {
				return err
			}
			created, err := a.arriveAtScheduledPlaceTx(ctx, tx, fid)
			if created {
				count++
			}
			return err
		})
		if err != nil {
			failures = append(failures, fmt.Errorf("scheduled arrival %s: %w", fid, err))
		}
	}
	return count, errors.Join(failures...)
}

// arriveAtScheduledPlaceTx writes the arrival event when the current life
// comes from an awake schedule item that has a place. The caller holds the
// life-context lock.
func (a *App) arriveAtScheduledPlaceTx(ctx context.Context, tx pgx.Tx, fid string) (bool, error) {
	at := a.now().UTC()
	schedule, life, err := resolveLifeContextSnapshotWith(ctx, tx, fid, at)
	if err != nil {
		return false, err
	}
	if life["source"] == "event" && life["event_kind"] == scheduleArrivalEventKind {
		// An arrival mirrors one schedule revision. A replanned or edited
		// schedule supersedes it, and the character arrives from the new item.
		var revision any
		if err := tx.QueryRow(ctx, `SELECT result->'schedule_revision' FROM public.life_events WHERE id=$1`, life["event_id"]).Scan(&revision); err != nil {
			return false, err
		}
		if fmt.Sprint(revision) == fmt.Sprint(schedule["revision"]) {
			return false, nil
		}
		if _, err := cancelScheduleArrivalTx(ctx, tx, fid, life, at); err != nil {
			return false, err
		}
		if schedule, life, err = resolveLifeContextSnapshotWith(ctx, tx, fid, at); err != nil {
			return false, err
		}
	}
	if life["source"] != "schedule" || life["behavior_state"] == "sleep" || stringValue(life["place_id"]) == "" {
		return false, nil
	}
	item := scheduleItemAt(schedule, at)
	if len(item) == 0 || item["id"] != life["schedule_item_id"] {
		return false, nil
	}
	end, err := time.Parse(time.RFC3339Nano, stringValue(item["end_at"]))
	if err != nil || !end.After(at.Add(time.Minute)) {
		return false, err
	}
	place := stringValue(life["place_id"])
	key := "schedule-arrival:" + stringValue(item["id"]) + ":" + fmt.Sprint(schedule["revision"]) + ":" + at.Truncate(time.Minute).Format(time.RFC3339)
	id := "event_" + stableDigest(fid+":"+key)
	result := map[string]any{"place_id": place, "schedule_item_id": item["id"], "schedule_revision": schedule["revision"], "source": "schedule_arrival"}
	tag, err := tx.Exec(ctx, `INSERT INTO public.life_events(id,fluctlight_id,kind,start_at,end_at,scene,activity,location,status,revision,evidence_refs,idempotency_key,request_digest,result,expires_at,context_scope) VALUES($1,$2,$3,$4,$5,$6,$7,$8,'inferred',1,$9,$10,$11,$12,$5,'life') ON CONFLICT DO NOTHING`,
		id, fid, scheduleArrivalEventKind, at, end, nullableString(strings.TrimSpace(stringValue(item["scene"]))), nullableString(strings.TrimSpace(stringValue(item["activity"]))), nullableString(strings.TrimSpace(stringValue(item["location"]))),
		jsonBytes([]any{"schedule_item:" + stringValue(item["id"])}), key, stableDigest(key+":"+place), jsonBytes(result))
	if err != nil || tag.RowsAffected() != 1 {
		return false, err
	}
	// Life authority rows must carry a replayable receipt (checked at commit).
	_, resulting, err := resolveLifeContextSnapshotWith(ctx, tx, fid, at)
	if err != nil {
		return false, err
	}
	result["id"], result["revision"], result["status"], result["replayed"] = id, 1, "inferred", false
	result["expected_context_revision"], result["resulting_context_revision"] = life["context_revision"], resulting["context_revision"]
	if _, err := tx.Exec(ctx, `UPDATE public.life_events SET result=$2 WHERE id=$1 AND revision=1`, id, jsonBytes(result)); err != nil {
		return false, err
	}
	return true, nil
}

// cancelScheduleArrivalTx ends a schedule arrival event that no longer matches
// the schedule (the item was edited). The next arrival pass, or the caller,
// re-arrives from the edited item.
func cancelScheduleArrivalTx(ctx context.Context, tx pgx.Tx, fid string, life map[string]any, at time.Time) (bool, error) {
	if life["source"] != "event" || life["event_kind"] != scheduleArrivalEventKind {
		return false, nil
	}
	tag, err := tx.Exec(ctx, `UPDATE public.life_events SET end_at=LEAST(end_at,GREATEST($3,start_at+interval '1 millisecond')),expires_at=$3,status='cancelled',revision=revision+1,updated_at=$3 WHERE id=$1 AND fluctlight_id=$2 AND kind=$4 AND status='inferred'`, life["event_id"], fid, at, scheduleArrivalEventKind)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// recordWorldEventMemoryTx leaves the character a short episodic memory of a
// world event it actually experienced, so the event outlives its effect
// window (the prompt only shows events while they are active).
func (a *App) recordWorldEventMemoryTx(ctx context.Context, tx pgx.Tx, fid, runID, description, placeName string, at time.Time) error {
	description = strings.TrimSpace(description)
	if description == "" || runID == "" {
		return nil
	}
	var owner string
	if err := tx.QueryRow(ctx, `SELECT created_by_actor_id FROM public.fluctlights WHERE id=$1 AND status='active'`, fid).Scan(&owner); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	content := "经历了世界里发生的事：" + description
	if placeName = strings.TrimSpace(placeName); placeName != "" {
		content = "在" + placeName + "经历了世界里发生的事：" + description
	}
	ref := "world-event-run:" + runID
	command := PreparedMemoryMutation{SchemaVersion: memoryLifecycleSchemaVersion, Operation: MemoryCreate, OwnerFluctlightID: fid, OwnerActorID: owner, ActorID: fid, ActorRefs: []string{fid}, EventRefs: []string{ref}, EvidenceRefs: []string{ref}, Semantic: &MemorySemanticInput{Type: "episodic", Content: content, Confidence: 1, Importance: 0.3}, Visibility: "private", OccurredAt: at, SourceFactID: ref, SemanticReason: "虚拟世界事件实际生效时的经历记录", IdempotencyKey: "world-event-memory:" + runID + ":" + fid}
	command.RequestDigest = memoryCommandDigest(command)
	_, err := a.applyMemoryCommandTx(ctx, tx, command)
	return err
}

// worldEventMemberEffects selects the effects a member experiences: world
// effects reach everyone in the world, place effects only those at the place.
func worldEventMemberEffects(definition, life map[string]any) []any {
	effects := []any{}
	place := stringValue(life["place_id"])
	for _, raw := range arrayValue(definition["effects"]) {
		effect := mapValue(raw)
		if effect["scope"] == "world" || (effect["scope"] == "place" && place != "" && effect["place_id"] == place) {
			effects = append(effects, copyPromptData(effect))
		}
	}
	return effects
}
