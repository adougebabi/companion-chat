package core

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/ai/decision"
	"github.com/jackc/pgx/v5"
)

const worldEventGenerationInstruction = `你为一个独立共享世界提议本小时的随机事件，也可以判断无事发生。依据真实世界背景、地点、NPC、实际在场成员与近期事件选择；不从职业、日程预测或世界成员关系推断实际在场。保持世界日常连续性，避免每小时强行制造事故。只返回 JSON：decision 为 no_event 或 propose，reason 为中文；propose 时提供 name（简短中文事件名）和 event，可选 target_actor_ref 只能引用 supplied present_characters 的 ref。未指定人物时是世界/地点事件，不能有 character scope 效果。人物伤情需要真实在场、合适的触发地点、minor 等级且可恢复；不能凭想象补一个被影响的人物。
event 是完整 world-event.v1 对象：schema_version、description、category(environment/visitor/shop/encounter/temporary_state)、intensity(low/medium)、duration_minutes(1..1440)、trigger、condition、effects。trigger={mode:"manual",interval_minutes:60,cooldown_minutes:60,probability:1}，表示只执行这次提议，不建立新的自动生成时钟。condition={day:"daily",start:"00:00",end:"00:00",place_ids:[],require_presence:false} 可按事实收紧；时间使用世界时区。effects 为1..8项，每项 kind 只能是 weather/atmosphere/traffic/place_availability/npc_visit/temporary_state/temporary_injury，scope 为 world/place/character；place scope 必须用真实 place_id，其余 scope 不填 place_id；npc_visit 使用真实 npc_id。value 为具体事实文本，关闭地点时为 closed/open。temporary_injury 使用现有 body_effect:{body_part:"hand",description:"...",severity:"minor",impact:"pain"}，scope=character、condition.require_presence=true，最多一项。只使用提供的地点和 NPC 标识，不伪造运行时 ID。提议还须经过独立 Kev 审核，生成内容不证明发生、入库、到达或认识任何人。`

func worldEventGenerationSchema() map[string]any {
	return objectSchema(map[string]any{"decision": enumStringSchema("no_event", "propose"), "name": map[string]any{"type": "string", "maxLength": 256}, "reason": stringSchema(), "target_actor_ref": stringSchema(), "event": openObjectSchema()}, []string{"decision", "reason"}, false)
}

// Repair only registers durable world clocks. It never runs a model in the
// Worker's serial repair loop, and one world gets one clock regardless of members.
func (a *App) EnsureWorldEventGenerationIntents(ctx context.Context, limit int) (int, error) {
	if limit < 1 || limit > 200 {
		limit = 100
	}
	tag, err := a.DB.Pool().Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload,status) SELECT 'world_event_generation:'||w.id,'world_event_generation:'||w.id,'world-events','world.events.generate',jsonb_build_object('world_id',w.id,'owner_actor_id',w.owner_actor_id,'correlation_id','world-events:'||w.id),'pending' FROM public.life_worlds w LEFT JOIN public.platform_workflow_intents i ON i.intent_id='world_event_generation:'||w.id WHERE i.intent_id IS NULL ORDER BY w.id LIMIT $1 ON CONFLICT(intent_id) DO NOTHING`, limit)
	if err != nil {
		return 0, err
	}
	repaired, err := a.DB.Pool().Exec(ctx, `UPDATE public.platform_workflow_intents i SET status='pending',next_attempt_at=now()+interval '1 minute',completed_at=NULL,last_error=NULL WHERE i.intent_id IN(SELECT failed.intent_id FROM public.platform_workflow_intents failed JOIN public.life_worlds w ON w.id=failed.payload->>'world_id' WHERE failed.intent_type='world.events.generate' AND failed.status='failed' ORDER BY failed.completed_at NULLS FIRST,failed.intent_id LIMIT $1)`, limit)
	return int(tag.RowsAffected() + repaired.RowsAffected()), err
}

func (a *App) worldEventGenerationContext(ctx context.Context, world string) (map[string]any, error) {
	var result map[string]any
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SET TRANSACTION ISOLATION LEVEL REPEATABLE READ READ ONLY`); err != nil {
			return err
		}
		var err error
		result, err = a.worldEventGenerationContextWith(ctx, tx, world)
		return err
	})
	return result, err
}

func (a *App) worldEventGenerationContextWith(ctx context.Context, q lifeContextQuerier, world string) (map[string]any, error) {
	var raw []byte
	if err := q.QueryRow(ctx, `SELECT jsonb_build_object('name',name,'background',background,'rules',rules,'timezone',timezone,'revision',revision,'owner_actor_id',owner_actor_id) FROM public.life_worlds WHERE id=$1`, world).Scan(&raw); err != nil {
		return nil, err
	}
	data := decodeObject(raw)
	places, err := readWorldObjects(ctx, q, `SELECT jsonb_build_object('id',id,'name',name,'description',description,'parent_place_id',parent_place_id) FROM public.world_places WHERE world_id=$1 AND status='active' ORDER BY id`, world)
	if err != nil {
		return nil, err
	}
	npcs, err := readWorldObjects(ctx, q, `SELECT jsonb_build_object('id',id,'name',name,'role',role) FROM public.world_npcs WHERE world_id=$1 AND status='active' AND enabled ORDER BY id`, world)
	if err != nil {
		return nil, err
	}
	members, err := readWorldObjects(ctx, q, `SELECT jsonb_build_object('id',f.id,'name',f.identity->>'name') FROM public.fluctlight_world_bindings b JOIN public.fluctlights f ON f.id=b.fluctlight_id WHERE b.world_id=$1 AND f.status='active' ORDER BY f.id`, world)
	if err != nil {
		return nil, err
	}
	present := []any{}
	for _, raw := range members {
		m := mapValue(raw)
		fid := stringValue(m["id"])
		_, life, err := resolveLifeContextSnapshotWith(ctx, q, fid, a.now().UTC())
		if err != nil {
			return nil, err
		}
		presence, err := npcCharacterPresenceWith(ctx, q, fid, life, a.now().UTC())
		if err != nil {
			return nil, err
		}
		if presence == nil {
			continue
		}
		present = append(present, map[string]any{"ref": fmt.Sprintf("a%d", len(present)+1), "name": m["name"], "place_id": life["place_id"], "activity": life["activity"], "fluctlight_id": fid})
	}
	recent, err := readWorldObjects(ctx, q, `SELECT jsonb_build_object('description',definition->>'description','status',status,'started_at',started_at,'end_at',end_at) FROM public.world_event_runs WHERE world_id=$1 AND status='applied' ORDER BY started_at DESC,id LIMIT 12`, world)
	if err != nil {
		return nil, err
	}
	data["places"], data["npcs"], data["present_characters"], data["recent_events"] = places, npcs, present, recent
	data["current_time"] = a.now().UTC().Format(instantLayout)
	if zone, err := time.LoadLocation(stringValue(data["timezone"])); err == nil {
		data["current_time"] = formatLocalInstant(a.now(), zone)
	}
	return data, nil
}

func worldEventGenerationModelView(data map[string]any) map[string]any {
	view := copyPromptFields(data, "name", "background", "rules", "timezone", "current_time", "places", "npcs", "recent_events")
	actors := []any{}
	for _, raw := range arrayValue(data["present_characters"]) {
		actors = append(actors, copyPromptFields(mapValue(raw), "ref", "name", "place_id", "activity"))
	}
	view["present_characters"] = actors
	return view
}

func worldEventAdmissionState(data, proposal map[string]any) map[string]any {
	world := copyPromptFields(data, "name", "background", "rules", "timezone", "current_time", "recent_events")
	d := mapValue(proposal["event"])
	places, npcs := map[string]bool{}, map[string]bool{}
	for _, id := range arrayValue(mapValue(d["condition"])["place_ids"]) {
		places[stringValue(id)] = true
	}
	for _, raw := range arrayValue(d["effects"]) {
		effect := mapValue(raw)
		if id := stringValue(effect["place_id"]); id != "" {
			places[id] = true
		}
		if id := stringValue(effect["npc_id"]); id != "" {
			npcs[id] = true
		}
	}
	actors := []any{}
	for _, raw := range arrayValue(data["present_characters"]) {
		actor := mapValue(raw)
		if actor["ref"] == proposal["target_actor_ref"] || places[stringValue(actor["place_id"])] {
			actors = append(actors, copyPromptFields(actor, "ref", "name", "place_id", "activity"))
			places[stringValue(actor["place_id"])] = true
		}
	}
	selectedPlaces, selectedNPCs := []any{}, []any{}
	for _, raw := range arrayValue(data["places"]) {
		if places[stringValue(mapValue(raw)["id"])] {
			selectedPlaces = append(selectedPlaces, copyPromptData(raw))
		}
	}
	for _, raw := range arrayValue(data["npcs"]) {
		if npcs[stringValue(mapValue(raw)["id"])] {
			selectedNPCs = append(selectedNPCs, copyPromptData(raw))
		}
	}
	world["places"], world["npcs"], world["present_characters"] = selectedPlaces, selectedNPCs, actors
	return map[string]any{"world": world, "candidate": copyPromptData(proposal)}
}

// One persisted UTC-hour slot prevents repeated generation after retries or a
// Worker restart. Proposals here are staging/audit, never executable events.
func (a *App) GenerateWorldEventCycle(ctx context.Context, world string) (result map[string]any, runErr error) {
	at := a.now().UTC()
	slot := at.Truncate(time.Hour)
	next := slot.Add(time.Hour)
	result = map[string]any{"world_id": world, "next_at": next.Format(time.RFC3339Nano)}
	var exists bool
	if err := a.DB.Pool().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.life_worlds WHERE id=$1)`, world).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		result["status"] = "world_deleted"
		return result, nil
	}
	conn, err := a.DB.Pool().Acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Release()
	var locked bool
	if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock(hashtextextended($1,0))`, "world-event-generation:"+world).Scan(&locked); err != nil {
		return nil, err
	}
	if !locked {
		result["status"] = "already_running"
		return result, nil
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
		defer cancel()
		if _, err := conn.Exec(cleanup, `SELECT pg_advisory_unlock(hashtextextended($1,0))`, "world-event-generation:"+world); err != nil {
			_ = conn.Conn().Close(cleanup)
		}
	}()
	id := "world_generation_" + stableDigest(world+":"+slot.Format(time.RFC3339))
	defer func() {
		if runErr != nil {
			cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
			defer cancel()
			_, err := conn.Exec(cleanup, `UPDATE public.world_event_generations SET status='failed',error_code='world_event_generation_failed',updated_at=now() WHERE id=$1 AND status IN('generating','review_pending','awaiting_kev')`, id)
			if err != nil {
				runErr = errors.Join(runErr, err)
			}
		}
	}()
	if _, err := conn.Exec(ctx, `UPDATE public.world_event_generations SET status='expired',error_code='hour_slot_expired',updated_at=now() WHERE world_id=$1 AND slot_at<$2 AND status IN('review_pending','awaiting_kev','generating')`, world, slot); err != nil {
		return nil, err
	}
	var stored []byte
	err = conn.QueryRow(ctx, `SELECT to_jsonb(g) FROM public.world_event_generations g WHERE id=$1`, id).Scan(&stored)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	state := decodeObject(stored)
	if status := stringValue(state["status"]); status != "" && status != "review_pending" && status != "awaiting_kev" {
		if status == "generating" {
			status = "interrupted"
			_, err = conn.Exec(ctx, `UPDATE public.world_event_generations SET status='interrupted',error_code='generation_interrupted',updated_at=now() WHERE id=$1`, id)
			if err != nil {
				return nil, err
			}
		}
		result["status"] = status
		result["generation_id"] = id
		result["run_id"] = state["run_id"]
		return result, nil
	}
	correlation := "world-event-generation:" + id
	clock := providerPromptDiagnostics(ctx)
	ctx = WithPromptDiagnostics(WithProviderCorrelation(ctx, correlation), map[string]any{"world_id": world, "world_event_generation_id": id, "world_clock_correlation_id": clock["world_clock_correlation_id"], "world_clock_intent_id": clock["world_clock_intent_id"]})
	proposal := mapValue(state["proposal"])
	data := mapValue(state["context"])
	if len(proposal) == 0 {
		data, err = a.worldEventGenerationContext(ctx, world)
		if err != nil {
			return nil, err
		}
		if value := stringValue(clock["world_clock_correlation_id"]); value != "" {
			data["world_clock_correlation_id"] = value
		}
		ctx = WithPromptDiagnostics(ctx, map[string]any{"world_id": world, "world_name": data["name"], "owner_actor_id": data["owner_actor_id"], "world_event_generation_id": id, "world_clock_correlation_id": clock["world_clock_correlation_id"], "world_clock_intent_id": clock["world_clock_intent_id"]})
		if _, err = conn.Exec(ctx, `INSERT INTO public.world_event_generations(id,world_id,slot_at,status,context,model_correlation_id) VALUES($1,$2,$3,'generating',$4,$5)`, id, world, slot, jsonBytes(data), correlation); err != nil {
			return nil, err
		}
		messages := (&PromptComposer{}).ComposeTaskMessages("cognitive_assessment", []map[string]any{{"role": "system", "content": worldEventGenerationInstruction}, {"role": "user", "content": promptTaskData(worldEventGenerationModelView(data))}})
		run, modelErr := a.runFormalStructuredTask(ctx, FormalAgentWorldEventGeneration, messages, nil, "world_event_generation_v1", worldEventGenerationSchema(), false, nil)
		if modelErr != nil {
			cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
			_, _ = conn.Exec(cleanup, `UPDATE public.world_event_generations SET status='failed',error_code='world_event_model_failed',updated_at=now() WHERE id=$1`, id)
			cancel()
			return nil, modelErr
		}
		proposal = run.Completion.Structured
		if proposal["decision"] == "no_event" {
			_, err = conn.Exec(ctx, `UPDATE public.world_event_generations SET status='no_event',proposal=$2,updated_at=now() WHERE id=$1`, id, jsonBytes(proposal))
			result["status"] = "no_event"
			return result, err
		}
		if err := validateWorldEventDefinition(mapValue(proposal["event"])); err != nil {
			_, _ = conn.Exec(ctx, `UPDATE public.world_event_generations SET status='failed',proposal=$2,error_code='world_event_proposal_invalid',updated_at=now() WHERE id=$1`, id, jsonBytes(proposal))
			return nil, err
		}
		if stringValue(proposal["name"]) == "" {
			return nil, errors.New("world_event_name_required")
		}
		if _, err = conn.Exec(ctx, `UPDATE public.world_event_generations SET status='review_pending',proposal=$2,updated_at=now() WHERE id=$1`, id, jsonBytes(proposal)); err != nil {
			return nil, err
		}
	}
	scope := decision.Scope{ActorSelf: "world:" + world, ActorUser: stringValue(data["owner_actor_id"]), Agent: "world_event_generation", RunID: id, CorrelationID: correlation, StateVersion: stableDigest(jsonString(data))}
	r, outcome, err := a.decideKev(ctx, "world.event.admit", scope, worldEventAdmissionState(data, proposal), id, "Decide whether this candidate may enter the world's executable event store. Require consistency with the actual world, locations, present participants and recent events. Do not approve invented identity/presence, incoherent or unsupported harm, or treating plans as completed facts. No means reject; unclear means do not admit.")
	if err != nil {
		return nil, err
	}
	if outcome != decision.Yes {
		status := "awaiting_kev"
		application := "not_applied"
		if outcome == decision.No {
			status = "rejected"
			application = "rejected"
			_, err = conn.Exec(ctx, `UPDATE public.world_event_generations SET status='rejected',kev_decision_id=$2,kev_record=$3,updated_at=now() WHERE id=$1`, id, nullableString(r.Record.ID), jsonBytes(r.Record))
			if err != nil {
				return nil, err
			}
		}
		if outcome != decision.No {
			audit := r.Record
			audit.Scope = scope
			audit.Point = "world.event.admit"
			audit.ApplicationStatus = "not_applied"
			if _, err := conn.Exec(ctx, `UPDATE public.world_event_generations SET status='awaiting_kev',kev_decision_id=$2,kev_record=$3,error_code=$4,updated_at=now() WHERE id=$1`, id, nullableString(r.Record.ID), jsonBytes(audit), nullableString(r.Record.Reason)); err != nil {
				return nil, err
			}
		}
		a.recordDiagnosticEvent(ctx, "world.event.admission", "info", "", id, correlation, map[string]any{"status": status, "decision_point": "world.event.admit", "reason": r.Record.Reason, "world_id": world})
		if err := r.Finish(ctx, application, "mandatory_approval_required", ""); err != nil {
			return nil, err
		}
		result["status"] = status
		return result, nil
	}
	var admittedRun map[string]any
	err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if err := lockWorldRuntimeTx(ctx, tx, world); err != nil {
			return err
		}
		if !a.now().UTC().Before(next) {
			return errors.New("world_event_candidate_expired")
		}
		var rawConfig string
		var version int64
		if err := tx.QueryRow(ctx, `SELECT value_json,revision FROM public.runtime_settings WHERE key='kev' FOR SHARE`).Scan(&rawConfig, &version); err != nil {
			return err
		}
		config, err := decodeKevConfig([]byte(rawConfig))
		if err != nil {
			return err
		}
		if version != r.Record.ConfigVersion || !config.Allows("world.event.admit") {
			return errors.New("world_event_kev_config_changed")
		}
		definition := mapValue(proposal["event"])
		current, err := a.worldEventGenerationContextWith(ctx, tx, world)
		if err != nil {
			return err
		}
		fingerprint := func(value map[string]any) string {
			return stableDigest(jsonString(copyPromptFields(value, "name", "background", "rules", "timezone", "revision", "places", "npcs", "present_characters", "recent_events")))
		}
		if fingerprint(current) != fingerprint(data) {
			return errors.New("world_event_context_changed")
		}
		if mapValue(definition["trigger"])["mode"] != "manual" {
			return errors.New("world_event_generated_trigger_invalid")
		}
		if err := validateWorldEventReferencesTx(ctx, tx, world, definition); err != nil {
			return err
		}
		target := ""
		ref := stringValue(proposal["target_actor_ref"])
		for _, raw := range arrayValue(data["present_characters"]) {
			actor := mapValue(raw)
			if actor["ref"] == ref {
				target = stringValue(actor["fluctlight_id"])
			}
		}
		if ref != "" && target == "" {
			return errors.New("world_event_target_unknown")
		}
		for _, raw := range arrayValue(definition["effects"]) {
			if mapValue(raw)["scope"] == "character" && target == "" {
				return errors.New("world_event_target_required")
			}
		}
		if target != "" {
			var member bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.fluctlight_world_bindings b JOIN public.fluctlights f ON f.id=b.fluctlight_id WHERE b.world_id=$1 AND b.fluctlight_id=$2 AND f.status='active')`, world, target).Scan(&member); err != nil {
				return err
			}
			if !member {
				return ErrLifeContextStale
			}
			_, life, err := resolveLifeContextSnapshotWith(ctx, tx, target, a.now().UTC())
			if err != nil {
				return err
			}
			presence, err := npcCharacterPresenceWith(ctx, tx, target, life, a.now().UTC())
			if err != nil {
				return err
			}
			if presence == nil {
				return errors.New("world_event_target_not_present")
			}
		}
		template := "generated_template_" + stableDigest(id)
		if _, err := tx.Exec(ctx, `INSERT INTO public.world_event_templates(id,world_id,generation_id,name,definition,enabled,next_check_at) VALUES($1,$2,$3,$4,$5,true,$6)`, template, world, id, stringValue(proposal["name"]), jsonBytes(definition), at); err != nil {
			return err
		}
		admittedRun, err = a.queueWorldEventTx(ctx, tx, stringValue(data["owner_actor_id"]), target, map[string]any{"id": template, "world_id": world, "revision": 1, "definition": definition}, "manual", "world-event-generation:"+id, a.now().UTC())
		if err != nil {
			return err
		}
		audit := r.Record
		audit.ApplicationStatus = "applied"
		audit.ActionID = stringValue(admittedRun["id"])
		_, err = tx.Exec(ctx, `UPDATE public.world_event_generations SET status='approved',kev_decision_id=$2,kev_record=$3,template_id=$4,run_id=$5,updated_at=now() WHERE id=$1 AND status='review_pending'`, id, r.Record.ID, jsonBytes(audit), template, admittedRun["id"])
		return err
	})
	if err != nil {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		_, markErr := conn.Exec(cleanup, `UPDATE public.world_event_generations SET status='failed',error_code='world_event_admission_failed',updated_at=now() WHERE id=$1 AND status='review_pending'`, id)
		cancel()
		if markErr != nil {
			err = errors.Join(err, markErr)
		}
		if finishErr := r.Finish(ctx, "not_applied", "candidate_invalid_or_stale", ""); finishErr != nil {
			return nil, errors.Join(err, finishErr)
		}
		return nil, err
	}
	if err := r.Finish(ctx, "applied", "", stringValue(admittedRun["id"])); err != nil {
		return nil, err
	}
	result["status"], result["generation_id"], result["run_id"] = "approved", id, admittedRun["id"]
	return result, nil
}

func (a *App) executeAmbientWorldEvent(ctx context.Context, run map[string]any, at time.Time) error {
	return withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		world := stringValue(run["world_id"])
		if err := lockWorldRuntimeTx(ctx, tx, world); err != nil {
			return err
		}
		var raw []byte
		if err := tx.QueryRow(ctx, `SELECT to_jsonb(r) FROM public.world_event_runs r WHERE id=$1 AND world_id=$2 FOR UPDATE`, run["id"], world).Scan(&raw); err != nil {
			return err
		}
		r := decodeObject(raw)
		if r["status"] != "queued" {
			return nil
		}
		filtered := func(reason string) error {
			_, err := tx.Exec(ctx, `UPDATE public.world_event_runs SET status='filtered',result=jsonb_build_object('reason',$2::text),started_at=$3,updated_at=now() WHERE id=$1`, r["id"], reason, at)
			return err
		}
		var templateRaw []byte
		if err := tx.QueryRow(ctx, `SELECT to_jsonb(t) FROM public.world_event_templates t WHERE id=$1 AND world_id=$2 FOR UPDATE`, r["template_id"], world).Scan(&templateRaw); err != nil {
			return err
		}
		t := decodeObject(templateRaw)
		if t["enabled"] != true || intValue(t["revision"]) != intValue(r["template_revision"]) {
			return filtered("template_changed_or_disabled")
		}
		expires, _ := time.Parse(time.RFC3339Nano, stringValue(r["expires_at"]))
		if !expires.After(at) {
			return filtered("execution_window_expired")
		}
		d := mapValue(r["definition"])
		if err := validateWorldEventDefinition(d); err != nil {
			return err
		}
		if err := validateWorldEventReferencesTx(ctx, tx, world, d); err != nil {
			return err
		}
		for _, raw := range arrayValue(d["effects"]) {
			if mapValue(raw)["scope"] == "character" {
				return filtered("character_target_required")
			}
		}
		condition := copyPromptData(mapValue(d["condition"])).(map[string]any)
		condition["place_ids"] = []any{}
		zone, err := readWorldTimezoneWith(ctx, tx, world)
		if err != nil {
			return err
		}
		loc, err := time.LoadLocation(zone)
		if err != nil {
			return err
		}
		if reason := worldEventConditionReason(condition, nil, at.In(loc)); reason != "" {
			return filtered(reason)
		}
		if mapValue(d["condition"])["require_presence"] == true {
			present := false
			members, err := readWorldObjects(ctx, tx, `SELECT jsonb_build_object('id',f.id) FROM public.fluctlight_world_bindings b JOIN public.fluctlights f ON f.id=b.fluctlight_id WHERE b.world_id=$1 AND f.status='active'`, world)
			if err != nil {
				return err
			}
			for _, raw := range members {
				fid := stringValue(mapValue(raw)["id"])
				_, life, err := resolveLifeContextSnapshotWith(ctx, tx, fid, at)
				if err != nil {
					return err
				}
				source, err := npcCharacterPresenceWith(ctx, tx, fid, life, at)
				if err != nil {
					return err
				}
				if source != nil && worldEventConditionReason(mapValue(d["condition"]), life, at.In(loc)) == "" {
					present = true
					break
				}
			}
			if !present {
				return filtered("confirmed_presence_required")
			}
		}
		end := at.Add(time.Duration(intValue(d["duration_minutes"])) * time.Minute)
		for _, raw := range arrayValue(d["effects"]) {
			effect := mapValue(raw)
			if effect["kind"] == "npc_visit" {
				if err := a.executeWorldNPCVisitTx(ctx, tx, "", r, effect, at, end); err != nil {
					return err
				}
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE public.world_event_runs SET status='applied',started_at=$2,end_at=$3,attempt_count=attempt_count+1,revision=revision+1,result=jsonb_build_object('source','world_execution','world_id',world_id),updated_at=now() WHERE id=$1`, r["id"], at, end); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE public.world_event_templates SET last_triggered_at=$2,updated_at=now() WHERE id=$1`, t["id"], at)
		if err != nil {
			return err
		}
		return a.notifyWorldEventMembersTx(ctx, tx, r, d, at)
	})
}

func (a *App) notifyWorldEventMembersTx(ctx context.Context, tx pgx.Tx, run, definition map[string]any, at time.Time) error {
	var name string
	if err := tx.QueryRow(ctx, `SELECT name FROM public.life_worlds WHERE id=$1`, run["world_id"]).Scan(&name); err != nil {
		return err
	}
	members, err := readWorldObjects(ctx, tx, `SELECT jsonb_build_object('id',f.id) FROM public.fluctlight_world_bindings b JOIN public.fluctlights f ON f.id=b.fluctlight_id WHERE b.world_id=$1 AND f.status='active' ORDER BY f.id`, run["world_id"])
	if err != nil {
		return err
	}
	for _, raw := range members {
		fid := stringValue(mapValue(raw)["id"])
		_, life, err := resolveLifeContextSnapshotWith(ctx, tx, fid, at)
		if err != nil {
			return err
		}
		source, err := npcCharacterPresenceWith(ctx, tx, fid, life, at)
		if err != nil {
			return err
		}
		if source == nil {
			continue
		}
		effects := []any{}
		for _, raw := range arrayValue(definition["effects"]) {
			effect := mapValue(raw)
			if effect["scope"] == "world" || effect["scope"] == "place" && effect["place_id"] == life["place_id"] {
				effects = append(effects, copyPromptData(effect))
			}
		}
		if len(effects) == 0 {
			continue
		}
		if _, err := a.enqueueNativeFactTx(ctx, tx, fid, "", stringValue(run["id"]), "life.world.event", "world-observation:"+stringValue(run["id"])+":"+fid, map[string]any{"world_id": run["world_id"], "world_name": name, "description": definition["description"], "effects": effects, "occurred_at": at.Format(time.RFC3339Nano), "source": "executed_world_event"}); err != nil {
			return err
		}
	}
	return nil
}
