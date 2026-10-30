package core

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type ownerSchedulePromiseKey struct{}

func scheduleFailure(code, message string) *SurfaceError {
	return &SurfaceError{Status: 409, Code: code, Message: message}
}

func scheduleDay(value, zone string) (time.Time, error) {
	location, err := time.LoadLocation(zone)
	if err != nil {
		return time.Time{}, err
	}
	day, err := time.ParseInLocation("2006-01-02", value, location)
	if err != nil || day.Format("2006-01-02") != value {
		return time.Time{}, invalid("invalid local date")
	}
	return day, nil
}

// This snapshot fences a long Provider call against persona/world changes.
func ownerScheduleSnapshot(ctx context.Context, q lifeContextQuerier, owner, fid string) (map[string]any, error) {
	var raw []byte
	err := q.QueryRow(ctx, `SELECT jsonb_build_object('foundation_revision',f.current_revision,'profile_id',r.active_profile_id,'world_id',COALESCE(b.world_id,''),'world_revision',COALESCE(w.revision,0),'status',f.status)
 FROM public.fluctlights f JOIN public.fluctlight_personality_runtime r ON r.fluctlight_id=f.id LEFT JOIN public.fluctlight_world_bindings b ON b.fluctlight_id=f.id LEFT JOIN public.life_worlds w ON w.id=b.world_id WHERE f.id=$1 AND f.created_by_actor_id=$2`, fid, owner).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return decodeObject(raw), err
}

func (a *App) OwnerSchedules(ctx context.Context, owner, fid string, q url.Values) (map[string]any, error) {
	if _, err := a.DB.GetFluctlight(ctx, fid, owner); err != nil {
		return nil, err
	}
	zone, err := readLifeContextTimezoneWith(ctx, a.DB.Pool(), fid)
	if err != nil {
		return nil, err
	}
	location, _ := time.LoadLocation(zone)
	today := a.now().In(location).Format("2006-01-02")
	from := q.Get("from")
	if from == "" {
		from = q.Get("local_date")
	}
	if from == "" {
		from = today
	}
	to := q.Get("to")
	if to == "" {
		to = from
	}
	first, err := scheduleDay(from, zone)
	if err != nil {
		return nil, err
	}
	last, err := scheduleDay(to, zone)
	if err != nil || last.Before(first) || last.After(first.AddDate(0, 0, 30)) {
		return nil, invalid("date range must contain 1 to 31 local days")
	}
	_, life, err := a.readLifeContextSnapshotAt(ctx, fid, a.now())
	if err != nil {
		return nil, err
	}
	days := []any{}
	for day := first; !day.After(last); day = day.AddDate(0, 0, 1) {
		localDate := day.Format("2006-01-02")
		schedule, err := readScheduleAtWith(ctx, a.DB.Pool(), fid, zone, day.Add(12*time.Hour))
		if err != nil {
			return nil, err
		}
		if schedule != nil {
			for _, key := range []string{"completed_before", "current_item_id", "current_item_effective_at", "current_item_expires_at"} {
				delete(schedule, key)
			}
			for _, raw := range arrayValue(schedule["items"]) {
				delete(mapValue(raw), "action_plan")
			}
		}
		var request []byte
		err = a.DB.Pool().QueryRow(ctx, `SELECT `+ownerScheduleProjectionSQL+` FROM public.owner_schedule_requests r WHERE owner_actor_id=$1 AND fluctlight_id=$2 AND local_date=$3 ORDER BY created_at DESC,id DESC LIMIT 1`, owner, fid, localDate).Scan(&request)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		var scheduleValue any
		if schedule != nil {
			scheduleValue = schedule
		}
		entry := map[string]any{"local_date": localDate, "schedule": scheduleValue, "request": nil}
		if len(request) > 0 {
			entry["request"] = decodeObject(request)
		}
		days = append(days, entry)
	}
	return map[string]any{"timezone": zone, "today": today, "from": from, "to": to, "days": days, "context_revision": life["context_revision"]}, nil
}

const ownerScheduleProjectionSQL = `jsonb_build_object('request_id',r.id,'fluctlight_id',r.fluctlight_id,'kind',r.kind,'local_date',to_char(r.local_date,'YYYY-MM-DD'),'timezone',r.timezone,'status',CASE WHEN r.status IN ('queued','running') AND EXISTS(SELECT 1 FROM public.platform_workflow_intents w WHERE w.intent_id='owner-schedule:'||r.id AND w.status IN ('failed','cancelled','dead_letter')) THEN 'failed' ELSE r.status END,'result',r.result,'error_code',CASE WHEN r.status IN ('queued','running') AND EXISTS(SELECT 1 FROM public.platform_workflow_intents w WHERE w.intent_id='owner-schedule:'||r.id AND w.status IN ('failed','cancelled','dead_letter')) THEN 'schedule_task_stopped' ELSE r.error_code END,'task_id','owner-schedule:'||r.id,'created_at',r.created_at,'updated_at',r.updated_at)`

func (a *App) OwnerScheduleRequest(ctx context.Context, owner, fid, id string) (map[string]any, error) {
	var raw []byte
	err := a.DB.Pool().QueryRow(ctx, `SELECT `+ownerScheduleProjectionSQL+` FROM public.owner_schedule_requests r WHERE r.id=$1 AND r.owner_actor_id=$2 AND r.fluctlight_id=$3`, id, owner, fid).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return decodeObject(raw), nil
}

func (a *App) RequestOwnerSchedule(ctx context.Context, owner, fid string, body map[string]any) (map[string]any, error) {
	tx, ok := SurfaceDBTx(ctx)
	if !ok {
		return nil, ErrInvalidArguments
	}
	if !eventKeys(body, "local_date", "expected_revision", "expected_life_context_revision", "idempotency_key", "reason", "promise_id", "expected_promise_revision", "start_at", "end_at") {
		return nil, invalid("unsupported schedule request field")
	}
	if err := lockSharedWorldMembersTx(ctx, tx, fid); err != nil {
		return nil, err
	}
	key := strings.TrimSpace(stringValue(body["idempotency_key"]))
	reason := strings.TrimSpace(stringValue(body["reason"]))
	expected, valid := nonNegativeRevision(body["expected_revision"])
	if key == "" || len(key) > 256 || reason == "" || len([]rune(reason)) > 500 || !valid {
		return nil, invalid("request identity, reason and schedule revision are required")
	}
	// Replaying the exact request reads its durable result before stale guards.
	digest := stableDigest(jsonString(body))
	var previous, previousDigest string
	err := tx.QueryRow(ctx, `SELECT id,request_digest FROM public.owner_schedule_requests WHERE owner_actor_id=$1 AND idempotency_key=$2`, owner, key).Scan(&previous, &previousDigest)
	if err == nil {
		if previousDigest != digest {
			return nil, scheduleFailure("schedule_idempotency_conflict", "原提交身份对应不同内容")
		}
		return a.ownerScheduleRequestWith(ctx, tx, owner, fid, previous)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if err := requireOwnerMutableTx(ctx, tx, owner, fid); err != nil {
		return nil, err
	}
	zone, err := readLifeContextTimezoneWith(ctx, tx, fid)
	if err != nil {
		return nil, err
	}
	day, err := scheduleDay(stringValue(body["local_date"]), zone)
	if err != nil {
		return nil, err
	}
	today, _ := scheduleDay(a.now().In(day.Location()).Format("2006-01-02"), zone)
	if day.Before(today) || day.After(today.AddDate(0, 0, 30)) {
		return nil, invalid("generation requires today or a date within 30 days")
	}
	if _, err = a.requireLifeContextRevisionTx(ctx, tx, fid, stringValue(body["expected_life_context_revision"]), a.now()); err != nil {
		return nil, scheduleFailure("life_context_revision_conflict", "生活状态已变化，请保留输入并重新读取")
	}
	schedule, err := readScheduleAtWith(ctx, tx, fid, zone, day.Add(12*time.Hour))
	if err != nil {
		return nil, err
	}
	if intValue(schedule["revision"]) != expected {
		return nil, scheduleFailure("schedule_revision_conflict", "该日期的日程已变化，请重新读取")
	}
	// A terminated/lost activity is not an eternal in-flight request. Preserve
	// its identity/result and allow an explicit fresh command after inspection.
	if _, err = tx.Exec(ctx, `UPDATE public.owner_schedule_requests r SET status='failed',error_code='schedule_task_stopped',updated_at=now() WHERE r.fluctlight_id=$1 AND r.status IN ('queued','running') AND EXISTS(SELECT 1 FROM public.platform_workflow_intents w WHERE w.intent_id='owner-schedule:'||r.id AND w.status IN ('failed','cancelled','dead_letter'))`, fid); err != nil {
		return nil, err
	}
	var pending string
	err = tx.QueryRow(ctx, `SELECT id FROM public.owner_schedule_requests WHERE fluctlight_id=$1 AND local_date=$2 AND status IN ('queued','running')`, fid, day).Scan(&pending)
	if err == nil {
		return nil, &SurfaceError{Status: 409, Code: "schedule_request_pending", Message: "这一天已有安排请求正在处理，请先查看原请求", Details: map[string]any{"request_id": pending}}
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	snapshot, err := ownerScheduleSnapshot(ctx, tx, owner, fid)
	if err != nil {
		return nil, err
	}
	if snapshot["status"] != "active" {
		return nil, scheduleFailure("fluctlight_not_active", "角色当前不能安排新的日程")
	}
	captured := map[string]any{"scope": snapshot}
	kind := "generate"
	result := map[string]any{}
	status := "queued"
	if promise := stringValue(body["promise_id"]); promise != "" {
		kind = "promise"
		p, err := a.schedulablePromise(ctx, tx, owner, fid, body, snapshot)
		if err != nil {
			return nil, err
		}
		captured["promise_execution"] = promiseScheduleExecution(p)
		var acceptedID, itemID string
		var actualStart, actualEnd time.Time
		err = tx.QueryRow(ctx, `SELECT s.id,i.id,i.start_at,i.end_at FROM public.life_schedule_items i JOIN public.life_schedules s ON s.id=i.schedule_id WHERE s.fluctlight_id=$1 AND s.status='accepted' AND i.promise_request_id=$2 AND i.start_at>=$3 AND i.end_at<=$4 LIMIT 1`, fid, promise, p["window_start"], p["window_end"]).Scan(&acceptedID, &itemID, &actualStart, &actualEnd)
		if err == nil {
			status = "completed"
			result = map[string]any{"disposition": "already_scheduled", "schedule_id": acceptedID, "schedule_item_id": itemID, "promise_request_id": promise, "start_at": actualStart, "end_at": actualEnd}
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		if status == "queued" {
			var other string
			err = tx.QueryRow(ctx, `SELECT id FROM public.owner_schedule_requests WHERE fluctlight_id=$1 AND payload->>'promise_id'=$2 AND status IN ('queued','running')`, fid, promise).Scan(&other)
			if err == nil {
				return nil, &SurfaceError{Status: 409, Code: "schedule_request_pending", Message: "这条约定已有安排请求，请查看原请求", Details: map[string]any{"request_id": other}}
			}
			if !errors.Is(err, pgx.ErrNoRows) {
				return nil, err
			}
		}

	} else if schedule != nil {
		status = "completed"
		result = map[string]any{"disposition": "preserved", "schedule_id": schedule["id"], "revision": schedule["revision"]}
	}
	id := randomID("schedule_request_")
	if _, err = tx.Exec(ctx, `INSERT INTO public.owner_schedule_requests(id,owner_actor_id,fluctlight_id,kind,local_date,timezone,status,payload,snapshot,result,idempotency_key,request_digest) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, id, owner, fid, kind, day, zone, status, jsonBytes(body), jsonBytes(captured), jsonBytes(result), key, digest); err != nil {
		return nil, err
	}
	if status == "queued" {
		intent := "owner-schedule:" + id
		if _, err = tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload,status) VALUES($1,$1,'lifecycle','schedule.owner',$2,'pending')`, intent, jsonBytes(map[string]any{"fluctlight_id": fid, "session_id": id, "local_date": body["local_date"], "correlation_id": intent})); err != nil {
			return nil, err
		}
	}
	out, err := a.ownerScheduleRequestWith(ctx, tx, owner, fid, id)
	if err == nil {
		out["_status"] = 200
		if status == "queued" {
			out["_status"] = 202
		}
	}
	return out, err
}

func (a *App) ownerScheduleRequestWith(ctx context.Context, q lifeContextQuerier, owner, fid, id string) (map[string]any, error) {
	var raw []byte
	err := q.QueryRow(ctx, `SELECT `+ownerScheduleProjectionSQL+` FROM public.owner_schedule_requests r WHERE r.id=$1 AND r.owner_actor_id=$2 AND r.fluctlight_id=$3`, id, owner, fid).Scan(&raw)
	if err != nil {
		return nil, err
	}
	return decodeObject(raw), nil
}

func (a *App) schedulablePromise(ctx context.Context, q lifeContextQuerier, owner, fid string, body, snapshot map[string]any) (map[string]any, error) {
	promise, err := readPromiseLinkTx(ctx, q, owner, fid, stringValue(body["promise_id"]), false)
	if err != nil {
		return nil, err
	}
	if promise["cancelled_at"] != nil || stringValue(promise["commitment_id"]) == "" || stringValue(promise["intention_id"]) == "" || promise["profile_id"] != snapshot["profile_id"] || intValue(promise["revision"]) != intValue(body["expected_promise_revision"]) {
		return nil, scheduleFailure("promise_not_schedulable", "约定未接受、所属人格已变化或版本已更新，请重新查看")
	}
	start, e1 := time.Parse(time.RFC3339Nano, stringValue(body["start_at"]))
	end, e2 := time.Parse(time.RFC3339Nano, stringValue(body["end_at"]))
	windowStart, _ := time.Parse(time.RFC3339Nano, stringValue(promise["window_start"]))
	windowEnd, _ := time.Parse(time.RFC3339Nano, stringValue(promise["window_end"]))
	zone, err := readLifeContextTimezoneWith(ctx, q, fid)
	if err != nil {
		return nil, err
	}
	day, err := scheduleDay(stringValue(body["local_date"]), zone)
	if err != nil {
		return nil, err
	}
	if e1 != nil || e2 != nil || !end.After(start) || !start.After(a.now().Add(30*time.Second)) || start.Before(windowStart) || end.After(windowEnd) || start.Before(day) || end.After(day.AddDate(0, 0, 1)) {
		return nil, invalid("choose a future interval within the accepted promise window and local day")
	}
	var active bool
	var commitmentRevision, intentionRevision int
	err = q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.goal_commitments c JOIN public.fluctlight_intentions i ON i.commitment_id=c.id JOIN public.fluctlight_goals g ON g.id=c.goal_id WHERE c.id=$1 AND c.fluctlight_id=$2 AND i.id=$3 AND i.fluctlight_id=$2 AND g.fluctlight_id=$2 AND g.status='active' AND c.status NOT IN ('completed','cancelled','paused','blocked') AND i.status IN ('candidate','qualified','due'))`, promise["commitment_id"], fid, promise["intention_id"]).Scan(&active)
	if err != nil {
		return nil, err
	}
	if !active {
		return nil, scheduleFailure("promise_execution_changed", "约定的执行状态已变化，当前不能重新安排")
	}
	if err = q.QueryRow(ctx, `SELECT c.revision,i.revision FROM public.goal_commitments c JOIN public.fluctlight_intentions i ON i.commitment_id=c.id WHERE c.id=$1 AND c.fluctlight_id=$2 AND i.id=$3 AND i.fluctlight_id=$2`, promise["commitment_id"], fid, promise["intention_id"]).Scan(&commitmentRevision, &intentionRevision); err != nil {
		return nil, err
	}
	promise["commitment_revision"], promise["intention_revision"] = commitmentRevision, intentionRevision
	return promise, nil
}

// A future appointment is a relation to an already accepted promise. Only this
// scoped Worker may introduce one; ordinary browser/model schedule writes cannot.
func (a *App) validateSchedulePromiseLink(ctx context.Context, tx pgx.Tx, owner, fid, currentID, id string, start, end time.Time) error {
	var oldStart, oldEnd time.Time
	err := tx.QueryRow(ctx, `SELECT start_at,end_at FROM public.life_schedule_items WHERE schedule_id=$1 AND promise_request_id=$2`, currentID, id).Scan(&oldStart, &oldEnd)
	if err == nil {
		if (!start.Equal(oldStart) || !end.Equal(oldEnd)) && ctx.Value(ownerSchedulePromiseKey{}) != id {
			return scheduleFailure("promise_schedule_interval_changed", "约定已有安排不能在普通重排中移动，请使用原约定流程")
		}
		if start.Equal(oldStart) && end.Equal(oldEnd) {
			return nil
		}
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if ctx.Value(ownerSchedulePromiseKey{}) != id {
		return ErrUnauthorized
	}
	var duplicate bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.life_schedule_items i JOIN public.life_schedules s ON s.id=i.schedule_id JOIN public.owner_promise_links p ON p.request_id=i.promise_request_id JOIN public.goal_commitments c ON c.id=p.commitment_id WHERE s.fluctlight_id=$1 AND s.status='accepted' AND i.promise_request_id=$2 AND s.id<>$3 AND p.cancelled_at IS NULL AND c.status NOT IN ('completed','cancelled','paused','blocked') AND i.start_at>=p.window_start AND i.end_at<=p.window_end)`, fid, id, currentID).Scan(&duplicate); err != nil {
		return err
	}
	if duplicate {
		return scheduleFailure("promise_already_scheduled", "这条约定已有日程，请查看原安排")
	}
	p, err := readPromiseLinkTx(ctx, tx, owner, fid, id, true)
	if err != nil {
		return err
	}
	if p["cancelled_at"] != nil || stringValue(p["commitment_id"]) == "" {
		return ErrConflict
	}
	return nil
}

func (a *App) ProcessOwnerScheduleRequest(ctx context.Context, id string) (result map[string]any, runErr error) {
	conn, err := a.DB.Pool().Acquire(ctx)
	if err != nil {
		return nil, err
	}
	var locked bool
	if err = conn.QueryRow(ctx, `SELECT pg_try_advisory_lock(hashtextextended($1,0))`, "owner-schedule:"+id).Scan(&locked); err != nil {
		conn.Release()
		return nil, err
	}
	if !locked {
		conn.Release()
		return nil, errors.New("owner_schedule_already_running")
	}
	defer func() {
		release, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
		defer cancel()
		if _, e := conn.Exec(release, `SELECT pg_advisory_unlock(hashtextextended($1,0))`, "owner-schedule:"+id); e != nil {
			_ = conn.Conn().Close(release)
		}
		conn.Release()
	}()
	var raw []byte
	if err = a.DB.Pool().QueryRow(ctx, `SELECT to_jsonb(r) FROM public.owner_schedule_requests r WHERE id=$1`, id).Scan(&raw); err != nil {
		return nil, err
	}
	request := decodeObject(raw)
	owner, fid := stringValue(request["owner_actor_id"]), stringValue(request["fluctlight_id"])
	if request["status"] == "completed" || request["status"] == "conflict" {
		return mapValue(request["result"]), nil
	}
	defer func() {
		if runErr != nil {
			cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
			defer cancel()
			code := "schedule_generation_failed"
			state := "failed"
			var surface *SurfaceError
			if errors.As(runErr, &surface) {
				code = surface.Code
				state = "conflict"
			}
			_, _ = a.DB.Pool().Exec(cleanup, `UPDATE public.owner_schedule_requests SET status=$2,error_code=$3,updated_at=now() WHERE id=$1 AND status<>'completed'`, id, state, code)
		}
	}()
	ctx = WithProviderCorrelation(ctx, "owner-schedule:"+id)
	body := mapValue(request["payload"])
	captured := mapValue(request["snapshot"])
	frozen := mapValue(captured["scope"])
	zone := stringValue(request["timezone"])
	date := stringValue(body["local_date"])
	day, err := scheduleDay(date, zone)
	if err != nil {
		return nil, err
	}
	today, _ := scheduleDay(a.now().In(day.Location()).Format("2006-01-02"), zone)
	if day.Before(today) {
		return nil, scheduleFailure("schedule_request_date_elapsed", "目标日期已经过去，未补造历史日程")
	}
	currentSnapshot, err := ownerScheduleSnapshot(ctx, a.DB.Pool(), owner, fid)
	if err != nil {
		return nil, err
	}
	if jsonString(currentSnapshot) != jsonString(frozen) {
		return nil, scheduleFailure("schedule_scope_changed", "角色、人格或世界已变化，请重新核对后安排")
	}
	current, err := readScheduleAtWith(ctx, a.DB.Pool(), fid, zone, day.Add(12*time.Hour))
	if err != nil {
		return nil, err
	}
	complete := func(tx pgx.Tx, out map[string]any) error {
		_, e := tx.Exec(ctx, `UPDATE public.owner_schedule_requests SET status='completed',result=$2,error_code=NULL,updated_at=now() WHERE id=$1`, id, jsonBytes(out))
		return e
	}
	if request["kind"] == "generate" && current != nil {
		out := map[string]any{"status": "completed", "disposition": "preserved", "schedule_id": current["id"], "revision": current["revision"]}
		err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error { return complete(tx, out) })
		return out, err
	}
	if intValue(current["revision"]) != intValue(body["expected_revision"]) {
		return nil, scheduleFailure("schedule_revision_conflict", "规划期间日程已变化，原日程保持")
	}
	if _, err = a.DB.Pool().Exec(ctx, `UPDATE public.owner_schedule_requests SET status='running',error_code=NULL,updated_at=now() WHERE id=$1`, id); err != nil {
		return nil, err
	}
	proposal := mapValue(request["proposal"])
	if len(proposal) == 0 {
		if current == nil {
			resource, err := a.DB.GetFluctlight(ctx, fid, owner)
			if err != nil {
				return nil, err
			}
			proposal, err = a.initialSchedulePayload(ctx, fid, date, zone, resource.Identity, resource.LifeProfile)
			if err != nil {
				return nil, err
			}
		}
		if request["kind"] == "promise" {
			p, err := a.schedulablePromise(ctx, a.DB.Pool(), owner, fid, body, frozen)
			if err != nil {
				return nil, err
			}
			base := current
			if base == nil {
				base = cloneMap(proposal)
				base["revision"] = 0
			}
			base = cloneMap(base)
			base["completed_before"] = a.now().UTC().Format(time.RFC3339Nano)
			goals, intentions, err := a.agencyProfile(ctx, fid)
			if err != nil {
				return nil, err
			}
			_, life, err := a.readLifeContextSnapshotAt(ctx, fid, a.now())
			if err != nil {
				return nil, err
			}
			start, _ := time.Parse(time.RFC3339Nano, stringValue(body["start_at"]))
			end, _ := time.Parse(time.RFC3339Nano, stringValue(body["end_at"]))
			if a.Schedule == nil || a.Schedule.Planner() == nil {
				return nil, errors.New("schedule_planner_unavailable")
			}
			proposal, err = a.Schedule.Planner().Plan(ctx, SchedulePlanInput{FluctlightID: fid, OwnerRequest: map[string]any{"purpose": "accepted_promise_appointment", "local_date": date, "start_at": body["start_at"], "end_at": body["end_at"], "activity": p["title"]}, Intent: "Arrange this already accepted promise in the exact requested interval. This is a plan, not execution or fulfillment.", PlannedAction: map[string]any{"title": p["title"], "duration_minutes": int(end.Sub(start) / time.Minute)}, Schedule: base, CurrentLife: life, Agency: map[string]any{"goals": filterActiveProfileRows(goals, stringValue(frozen["profile_id"])), "intentions": filterActiveProfileRows(intentions, stringValue(frozen["profile_id"]))}, Timezone: zone})
			if err != nil {
				return nil, err
			}
			if err = preserveScheduledActionPlans(proposal, base); err != nil {
				return nil, err
			}
			matches := 0
			for _, raw := range arrayValue(proposal["items"]) {
				item := mapValue(raw)
				if item["planned_action_slot"] == true {
					matches++
					from, e1 := time.Parse(time.RFC3339Nano, stringValue(item["start_at"]))
					to, e2 := time.Parse(time.RFC3339Nano, stringValue(item["end_at"]))
					if e1 != nil || e2 != nil || !from.Equal(start) || !to.Equal(end) {
						return nil, errors.New("promise_schedule_interval_mismatch")
					}
					item["promise_request_id"] = body["promise_id"]
					item["activity"] = p["title"]
					delete(item, "intention_id")
					delete(item, "action_plan")
				}
				delete(item, "planned_action_slot")
			}
			if matches != 1 {
				return nil, errors.New("promise_schedule_slot_missing")
			}
			proposal["completed_before"] = base["completed_before"]
		}
		proposal["local_date"], proposal["timezone"] = date, zone
		if _, err = a.DB.Pool().Exec(ctx, `UPDATE public.owner_schedule_requests SET proposal=$2,updated_at=now() WHERE id=$1`, id, jsonBytes(proposal)); err != nil {
			return nil, err
		}
	}
	proposal["expected_revision"] = body["expected_revision"]
	proposal["expected_life_context_revision"] = body["expected_life_context_revision"]
	proposal["idempotency_key"] = "owner-schedule-accept:" + id
	proposal["evidence_refs"] = []any{"owner-schedule:" + id}
	proposal["reason"] = body["reason"]
	if request["kind"] == "promise" {
		ctx = context.WithValue(ctx, ownerSchedulePromiseKey{}, stringValue(body["promise_id"]))
	}
	err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if err := lockSharedWorldMembersTx(ctx, tx, fid); err != nil {
			return err
		}
		if err := requireOwnerMutableTx(ctx, tx, owner, fid); err != nil {
			return err
		}
		var requestStatus string
		if err := tx.QueryRow(ctx, `SELECT status FROM public.owner_schedule_requests WHERE id=$1 FOR UPDATE`, id).Scan(&requestStatus); err != nil {
			return err
		}
		if requestStatus != "running" {
			return scheduleFailure("schedule_request_superseded", "原请求已停止，未应用迟到的规划")
		}

		latest, err := ownerScheduleSnapshot(ctx, tx, owner, fid)
		if err != nil {
			return err
		}
		if jsonString(latest) != jsonString(frozen) {
			return scheduleFailure("schedule_scope_changed", "规划期间角色、人格或世界已变化，未应用旧结果")
		}
		if request["kind"] == "promise" {
			p, e := a.schedulablePromise(ctx, tx, owner, fid, body, latest)
			if e != nil {
				return e
			}
			if jsonString(promiseScheduleExecution(p)) != jsonString(captured["promise_execution"]) {
				return scheduleFailure("promise_execution_changed", "约定执行对象已更新，未应用旧规划")
			}
		}
		var accepted map[string]any
		if current != nil && date == today.Format("2006-01-02") {
			accepted, err = a.replanScheduleWithTx(ctx, tx, owner, fid, proposal)
		} else {
			accepted, err = a.acceptScheduleWithTx(ctx, tx, owner, fid, proposal)
		}
		if err != nil {
			return err
		}
		result = map[string]any{"status": "completed", "schedule_id": accepted["id"], "revision": accepted["revision"], "local_date": date, "timezone": zone, "disposition": "generated"}
		if request["kind"] == "promise" {
			var itemID string
			if err = tx.QueryRow(ctx, `SELECT id FROM public.life_schedule_items WHERE schedule_id=$1 AND promise_request_id=$2`, accepted["id"], body["promise_id"]).Scan(&itemID); err != nil {
				return fmt.Errorf("promise schedule link not persisted: %w", err)
			}
			result["disposition"], result["promise_request_id"], result["schedule_item_id"] = "scheduled", body["promise_id"], itemID
		}
		return complete(tx, result)
	})
	return result, err
}

// Version replacement must retain accepted future appointments. An in-progress
// appointment cannot be split into two relations by an unrelated replan.
func (a *App) preserveSchedulePromises(ctx context.Context, tx pgx.Tx, currentID string, items []any, at time.Time) error {
	if currentID == "" {
		return nil
	}
	rows, err := tx.Query(ctx, `SELECT i.promise_request_id,i.start_at,i.end_at FROM public.life_schedule_items i JOIN public.owner_promise_links p ON p.request_id=i.promise_request_id JOIN public.goal_commitments c ON c.id=p.commitment_id WHERE i.schedule_id=$1 AND i.end_at>$2 AND p.cancelled_at IS NULL AND c.status NOT IN ('completed','cancelled','paused','blocked') AND i.start_at>=p.window_start AND i.end_at<=p.window_end`, currentID, at)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var start, end time.Time
		if err = rows.Scan(&id, &start, &end); err != nil {
			return err
		}
		if start.Before(at) {
			return scheduleFailure("schedule_active_promise_replan_blocked", "当前约定时段进行中，请保留已有安排")
		}
		found := false
		for _, raw := range items {
			item := mapValue(raw)
			if item["promise_request_id"] == id {
				found = true
			}
		}
		if !found {
			return scheduleFailure("schedule_active_promise_missing", "新日程缺少已有约定，原安排保持")
		}
	}
	return rows.Err()
}

func promiseScheduleExecution(p map[string]any) map[string]any {
	return map[string]any{"request_id": p["request_id"], "revision": p["revision"], "commitment_id": p["commitment_id"], "commitment_revision": p["commitment_revision"], "intention_id": p["intention_id"], "intention_revision": p["intention_revision"]}
}
