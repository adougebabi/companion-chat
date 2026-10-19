package core

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const diaryItemSQL = `SELECT d.id,d.local_date::text,d.timezone,d.status,d.revision,d.latest_attempt,d.last_error_code,d.created_at,d.updated_at,g.body,g.plan,g.source_snapshot,g.persona_snapshot,g.model_provenance,g.id,COALESCE(n.favorite,false),COALESCE(n.note,''),COALESCE(n.revision,0) FROM public.fluctlight_diary_days d LEFT JOIN public.fluctlight_diary_attempts g ON g.id=d.active_attempt_id LEFT JOIN public.fluctlight_diary_annotations n ON n.diary_id=d.id AND n.owner_actor_id=$2 `

func scanDiaryItem(row pgx.Row, fid string) (map[string]any, error) {
	var id, date, zone, status string
	var revision, attempt, annotationRevision int
	var lastError, body, activeAttempt *string
	var created, updated time.Time
	var plan, sources, persona, models []byte
	var favorite bool
	var note string
	if err := row.Scan(&id, &date, &zone, &status, &revision, &attempt, &lastError, &created, &updated, &body, &plan, &sources, &persona, &models, &activeAttempt, &favorite, &note, &annotationRevision); err != nil {
		return nil, err
	}
	parsedPlan := decodeObject(plan)
	refs := arrayValue(parsedPlan["source_refs"])
	if refs == nil {
		refs = []any{}
	}
	return map[string]any{"id": id, "fluctlight_id": fid, "date": date, "timezone": zone, "status": status, "revision": revision, "latest_attempt": attempt, "last_error_code": lastError, "body": body, "source": "agent", "refs": refs, "created_at": created.UTC().Format(time.RFC3339Nano), "updated_at": updated.UTC().Format(time.RFC3339Nano), "active_attempt_id": activeAttempt, "source_snapshot": decodeObject(sources), "persona_provenance": mapValue(decodeObject(persona)["diary_persona_provenance"]), "model_provenance": decodeObject(models), "annotation": map[string]any{"favorite": favorite, "note": note, "revision": annotationRevision}}, nil
}

func (a *App) OwnerDiaryList(ctx context.Context, owner, fid string, q url.Values) (map[string]any, error) {
	if _, err := a.DB.GetFluctlight(ctx, fid, owner); err != nil {
		return nil, err
	}
	limit, err := reflectionReadPage(q)
	if err != nil {
		return nil, err
	}
	favorite := q.Get("favorite") == "true"
	if value := q.Get("favorite"); value != "" && value != "true" && value != "false" {
		return nil, invalid("favorite must be boolean")
	}
	scope := stableDigest(jsonString([]any{"diary", owner, fid, favorite}))
	cursor, err := decodeOwnerReadCursor(q.Get("cursor"), scope)
	if err != nil {
		return nil, invalid("invalid diary cursor")
	}
	date := "9999-12-31"
	if !cursor.Time.IsZero() {
		date = cursor.Time.Format("2006-01-02")
	}
	rows, err := a.DB.Pool().Query(ctx, diaryItemSQL+`WHERE d.fluctlight_id=$1 AND (NOT $3 OR n.favorite=true) AND ($4='' OR (d.local_date,d.id)<($5::date,$4)) ORDER BY d.local_date DESC,d.id DESC LIMIT $6`, fid, owner, favorite, cursor.ID, date, limit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []any{}
	next := ""
	last := ownerReadCursor{Scope: scope}
	for rows.Next() {
		if len(items) == limit {
			next = encodeOwnerReadCursor(last)
			break
		}
		item, err := scanDiaryItem(rows, fid)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
		day, _ := time.Parse("2006-01-02", stringValue(item["date"]))
		last = ownerReadCursor{Scope: scope, ID: stringValue(item["id"]), Time: day}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	zone, err := readLifeContextTimezoneWith(ctx, a.DB.Pool(), fid)
	if err != nil {
		return nil, err
	}
	return map[string]any{"items": items, "next_cursor": next, "timezone": zone}, nil
}

func (a *App) OwnerDiary(ctx context.Context, owner, fid, id string) (map[string]any, error) {
	if _, err := a.DB.GetFluctlight(ctx, fid, owner); err != nil {
		return nil, err
	}
	item, err := scanDiaryItem(a.DB.Pool().QueryRow(ctx, diaryItemSQL+`WHERE d.fluctlight_id=$1 AND d.id=$3`, fid, owner, id), fid)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, &SurfaceError{Status: 404, Code: "diary_not_found", Message: "日记不存在或不属于当前角色"}
	}
	if err != nil {
		return nil, err
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT g.id,g.attempt,g.status,g.error_code,g.created_at,g.finished_at,g.plan,i.intent_id,i.workflow_id,i.status FROM public.fluctlight_diary_attempts g LEFT JOIN public.platform_workflow_intents i ON i.payload->>'diary_attempt_id'=g.id AND i.intent_type='diary.generate' WHERE g.diary_id=$1 ORDER BY g.attempt DESC LIMIT 21`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	attempts := []any{}
	more := false
	for rows.Next() {
		var aid, status string
		var ordinal int
		var code, task, wf, taskStatus *string
		var at time.Time
		var finished *time.Time
		var plan []byte
		if err := rows.Scan(&aid, &ordinal, &status, &code, &at, &finished, &plan, &task, &wf, &taskStatus); err != nil {
			return nil, err
		}
		if len(attempts) == 20 {
			more = true
			break
		}
		attempts = append(attempts, map[string]any{"id": aid, "attempt": ordinal, "status": status, "error_code": code, "created_at": at.UTC().Format(time.RFC3339Nano), "finished_at": finished, "plan": decodeObject(plan), "task_id": task, "workflow_id": wf, "workflow_status": taskStatus})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	item["attempts"], item["has_more_attempts"] = attempts, more
	return item, nil
}

func (a *App) OwnerDiaryReplies(ctx context.Context, owner, fid, id string, q url.Values) (map[string]any, error) {
	if _, err := a.OwnerDiary(ctx, owner, fid, id); err != nil {
		return nil, err
	}
	limit, err := reflectionReadPage(q)
	if err != nil {
		return nil, err
	}
	scope := stableDigest(jsonString([]any{"diary-replies", owner, fid, id}))
	cursor, err := decodeOwnerReadCursor(q.Get("cursor"), scope)
	if err != nil {
		return nil, invalid("invalid reply cursor")
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT id,created_at,payload FROM public.owner_surface_records WHERE owner_actor_id=$1 AND fluctlight_id=$2 AND kind='owner_request' AND payload->>'kind'='diary_reply' AND payload->'payload'->>'diary_id'=$3 AND ($4='' OR (created_at,id)<($5,$4)) ORDER BY created_at DESC,id DESC LIMIT $6`, owner, fid, id, cursor.ID, cursor.Time, limit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []any{}
	next := ""
	last := ownerReadCursor{Scope: scope}
	for rows.Next() {
		var rid string
		var at time.Time
		var raw []byte
		if err := rows.Scan(&rid, &at, &raw); err != nil {
			return nil, err
		}
		if len(items) == limit {
			next = encodeOwnerReadCursor(last)
			break
		}
		items = append(items, decodeObject(raw))
		last = ownerReadCursor{Scope: scope, ID: rid, Time: at}
	}
	return map[string]any{"items": items, "next_cursor": next}, rows.Err()
}

func (a *App) RequestOwnerDiary(ctx context.Context, owner, fid string, body map[string]any) (map[string]any, error) {
	if _, err := a.DB.GetFluctlight(ctx, fid, owner); err != nil {
		return nil, err
	}
	tx, ok := SurfaceDBTx(ctx)
	if !ok {
		return nil, errors.New("diary_transaction_required")
	}
	date := stringValue(body["local_date"])
	if err := requireOwnerMutableTx(ctx, tx, owner, fid); err != nil {
		return nil, err
	}
	zone, err := readLifeContextTimezoneWith(ctx, tx, fid)
	if err != nil {
		return nil, err
	}
	if z := stringValue(body["timezone"]); z != "" {
		zone = z
	}
	location, err := time.LoadLocation(zone)
	if err != nil {
		return nil, invalid("invalid diary timezone")
	}
	start, err := time.ParseInLocation("2006-01-02", date, location)
	if err != nil || start.Format("2006-01-02") != date {
		return nil, invalid("invalid diary date")
	}
	if start.AddDate(0, 0, 1).After(a.now().UTC()) {
		return nil, &SurfaceError{Status: 422, Code: "diary_day_not_closed", Message: "请选择已经结束的本地日期，避免把未发生的一天写成经历"}
	}
	id := "diary_" + stableDigest(fid+":"+date+":"+zone)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "diary-day:"+id); err != nil {
		return nil, err
	}
	var revision, attempt int
	var status string
	err = tx.QueryRow(ctx, `SELECT revision,latest_attempt,status FROM public.fluctlight_diary_days WHERE id=$1 FOR UPDATE`, id).Scan(&revision, &attempt, &status)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if err == nil && (status == "failed" || status == "inactive") {
		expected, ok := parseRevision(body["expected_revision"])
		if !ok || expected != revision {
			return nil, &SurfaceError{Status: 409, Code: "diary_revision_conflict", Message: "日记状态已变化，请重新读取后确认重试"}
		}
		if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_diary_days SET status='pending',latest_attempt=latest_attempt+1,revision=revision+1,last_error_code=NULL,updated_at=now() WHERE id=$1`, id); err != nil {
			return nil, err
		}
		if err := copyDiaryRetryStagesTx(ctx, tx, fid, id, attempt, attempt+1); err != nil {
			return nil, err
		}
	}
	id, err = enqueueDiaryAttemptTx(ctx, tx, fid, date, zone, a.now().UTC())
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `UPDATE public.platform_workflow_intents SET next_attempt_at=now() WHERE intent_type='diary.generate' AND status IN ('pending','retry') AND payload->>'diary_attempt_id' IN (SELECT id FROM public.fluctlight_diary_attempts WHERE diary_id=$1 AND status='pending')`, id); err != nil {
		return nil, err
	}
	var dayStatus string
	var next int
	if err := tx.QueryRow(ctx, `SELECT status,latest_attempt FROM public.fluctlight_diary_days WHERE id=$1`, id).Scan(&dayStatus, &next); err != nil {
		return nil, err
	}
	audit, err := (postgresSurfaceTx{tx}).Audit(ctx, owner, fid, "diary_generate", stringValue(body["reason"]), id)
	if err != nil {
		return nil, err
	}
	return map[string]any{"diary_id": id, "fluctlight_id": fid, "status": dayStatus, "attempt": next, "status_url": "/api/fluctlights/" + url.PathEscape(fid) + "/diary/" + url.PathEscape(id), "audit_id": audit}, nil
}

func (a *App) AnnotateOwnerDiary(ctx context.Context, owner, fid, id string, body map[string]any) (map[string]any, error) {
	if _, err := a.DB.GetFluctlight(ctx, fid, owner); err != nil {
		return nil, err
	}
	tx, ok := SurfaceDBTx(ctx)
	if !ok {
		return nil, errors.New("diary_transaction_required")
	}
	expected, valid := parseRevision(body["expected_revision"])
	favorite, flag := body["favorite"].(bool)
	note, noteOK := body["note"].(string)
	if !valid || !flag || !noteOK || len([]rune(note)) > 5000 {
		return nil, invalid("invalid diary annotation")
	}
	var scope string
	if err := tx.QueryRow(ctx, `SELECT id FROM public.fluctlight_diary_days WHERE id=$1 AND fluctlight_id=$2 FOR UPDATE`, id, fid).Scan(&scope); errors.Is(err, pgx.ErrNoRows) {
		return nil, &SurfaceError{Status: 404, Code: "diary_not_found", Message: "日记不存在"}
	} else if err != nil {
		return nil, err
	}
	var revision int
	err := tx.QueryRow(ctx, `SELECT revision FROM public.fluctlight_diary_annotations WHERE diary_id=$1 AND owner_actor_id=$2 FOR UPDATE`, id, owner).Scan(&revision)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if revision != expected {
		return nil, &SurfaceError{Status: 409, Code: "diary_annotation_conflict", Message: "注记已在其他页面修改，草稿应保留，请核对最新版本"}
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.fluctlight_diary_annotations(diary_id,owner_actor_id,favorite,note,revision) VALUES($1,$2,$3,$4,1) ON CONFLICT(diary_id,owner_actor_id) DO UPDATE SET favorite=$3,note=$4,revision=fluctlight_diary_annotations.revision+1,updated_at=now()`, id, owner, favorite, note)
	if err != nil {
		return nil, err
	}
	audit, err := (postgresSurfaceTx{tx}).Audit(ctx, owner, fid, "diary_annotation", "Owner收藏或注记，不改写角色主观正文", id)
	if err != nil {
		return nil, err
	}
	return map[string]any{"diary_id": id, "favorite": favorite, "note": note, "revision": revision + 1, "audit_id": audit}, nil
}

const ownerDiaryReplyFactType = "diary.owner_reply.submitted"

func (a *App) enqueueOwnerDiaryReply(ctx context.Context, tx pgx.Tx, owner, fid, id string, body map[string]any) (string, error) {
	var attempt, text string
	var revision int
	var status string
	if err := tx.QueryRow(ctx, `SELECT g.id,g.body,d.revision,f.status FROM public.fluctlight_diary_days d JOIN public.fluctlight_diary_attempts g ON g.id=d.active_attempt_id JOIN public.fluctlights f ON f.id=d.fluctlight_id WHERE d.id=$1 AND d.fluctlight_id=$2 AND f.created_by_actor_id=$3 FOR SHARE OF f,d,g`, body["diary_id"], fid, owner).Scan(&attempt, &text, &revision, &status); errors.Is(err, pgx.ErrNoRows) {
		return "", &SurfaceError{Status: 404, Code: "diary_not_found", Message: "尚无可回应的角色日记"}
	} else if err != nil {
		return "", err
	}
	expected, ok := parseRevision(body["expected_revision"])
	if !ok || expected != revision {
		return "", &SurfaceError{Status: 409, Code: "diary_revision_conflict", Message: "日记已变化，请核对最新正文后回应"}
	}
	if status == "retired" {
		return "", ErrUnauthorized
	}
	feedback := strings.TrimSpace(stringValue(body["text"]))
	if feedback == "" || len([]rune(feedback)) > 2000 {
		return "", invalid("invalid diary reply")
	}
	_, conversation, err := a.directTarget(ctx, fid)
	if err != nil {
		return "", err
	}
	return a.enqueueNativeFactTx(ctx, tx, fid, conversation, id, ownerDiaryReplyFactType, "diary-reply:"+id, map[string]any{"request_id": id, "owner_actor_id": owner, "kind": "diary_reply", "diary_id": body["diary_id"], "diary_attempt_id": attempt, "diary_body": text, "feedback": feedback, "evidence_semantics": "Owner response to a subjective diary, not proof of physical presence or an instruction to rewrite its body"})
}
