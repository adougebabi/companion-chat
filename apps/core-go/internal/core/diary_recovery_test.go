package core

import (
	"context"
	"testing"
	"time"
)

// The completed draft is an explicit crash-state fixture, not live LLM proof.
// Recovery runs the production source check, retry and final commit with no
// Provider available, proving that a persisted completed stage is reused.
func TestDiaryTerminalWorkflowRecoveryReusesCompletedStagesWithoutProvider(t *testing.T) {
	ctx, repo, app, owner, fid, conversation := setupDirectPublicationToolTest(t, "diary-recovery")
	app.Provider = nil
	app.Clock = func() time.Time { return time.Date(2026, 10, 16, 2, 0, 0, 0, time.UTC) }
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.conversation_participants SET joined_at='2019-01-01T00:00:00Z' WHERE conversation_id=$1`, conversation); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.conversation_messages(id,conversation_id,sequence,author_actor_id,kind,text,attachment_refs,idempotency_key,created_at) SELECT 'diary-recovery-source',$1::text,COALESCE(MAX(sequence),0)+1,$2::text,'user','实际存在的测试交流素材','[]','diary-recovery-source-key','2020-01-01T01:00:00Z' FROM public.conversation_messages WHERE conversation_id=$1`, conversation, owner); err != nil {
		t.Fatal(err)
	}
	request := func(expected any) (map[string]any, error) {
		var result map[string]any
		err := newPostgresSurface(repo.Pool()).Transact(ctx, func(txCtx context.Context, _ surfaceTx) error {
			var err error
			result, err = app.RequestOwnerDiary(txCtx, owner, fid, map[string]any{"local_date": "2020-01-01", "timezone": "Asia/Shanghai", "expected_revision": expected, "reason": "recover a staged generation", "idempotency_key": "diary-recovery"})
			return err
		})
		return result, err
	}
	accepted, err := request(0)
	if err != nil {
		t.Fatal(err)
	}
	id := stringValue(accepted["diary_id"])
	var oldAttempt, intent string
	if err := repo.Pool().QueryRow(ctx, `SELECT g.id,i.intent_id FROM public.fluctlight_diary_attempts g JOIN public.platform_workflow_intents i ON i.payload->>'diary_attempt_id'=g.id WHERE g.diary_id=$1`, id).Scan(&oldAttempt, &intent); err != nil {
		t.Fatal(err)
	}
	zone, _ := time.LoadLocation("Asia/Shanghai")
	start, _ := time.ParseInLocation("2006-01-02", "2020-01-01", zone)
	snapshot, err := readDiarySourcesWith(ctx, repo.Pool(), fid, start, start.AddDate(0, 0, 1), app.now())
	if err != nil {
		t.Fatal(err)
	}
	body := "这是在最终提交前已经持久化的受控正文。"
	plan := map[string]any{"write": true, "source_refs": []any{"message:diary-recovery-source"}, "reason": "controlled staged result"}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.fluctlight_diary_attempts SET status='running',source_snapshot=$2,source_digest=$3,plan=$4,body=$5,persona_snapshot=$6 WHERE id=$1`, oldAttempt, jsonBytes(snapshot), stableDigest(jsonString(snapshot)), jsonBytes(plan), body, jsonBytes(map[string]any{"diary_persona_provenance": map[string]any{"profile_id": "controlled-profile", "source_revision": 0}})); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.fluctlight_diary_days SET status='running' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	if err := app.ReconcileDiaryWorkflowTerminal(ctx, intent, "failed"); err != nil {
		t.Fatal(err)
	}
	failed, err := app.OwnerDiary(ctx, owner, fid, id)
	if err != nil || failed["status"] != "failed" {
		t.Fatalf("terminal fallback %#v %v", failed, err)
	}
	retried, err := request(failed["revision"])
	if err != nil || intValue(retried["attempt"]) != 2 {
		t.Fatalf("retry %#v %v", retried, err)
	}
	var nextAttempt string
	if err := repo.Pool().QueryRow(ctx, `SELECT id FROM public.fluctlight_diary_attempts WHERE diary_id=$1 AND attempt=2`, id).Scan(&nextAttempt); err != nil {
		t.Fatal(err)
	}
	result, err := app.ProcessDiaryGeneration(ctx, nextAttempt)
	if err != nil || result["status"] != "ready" {
		t.Fatalf("staged recovery %#v %v", result, err)
	}
	var savedBody, reused string
	var events int
	if err := repo.Pool().QueryRow(ctx, `SELECT g.body,g.model_provenance->>'reused_from_attempt_id' FROM public.fluctlight_diary_days d JOIN public.fluctlight_diary_attempts g ON g.id=d.active_attempt_id WHERE d.id=$1`, id).Scan(&savedBody, &reused); err != nil || savedBody != body || reused != oldAttempt {
		t.Fatalf("completed draft changed %q %q %v", savedBody, reused, err)
	}
	if _, err := app.ProcessDiaryGeneration(ctx, nextAttempt); err != nil {
		t.Fatal(err)
	}
	if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.platform_outbox_events WHERE kind='diary.generated' AND aggregate_id=$1`, id).Scan(&events); err != nil || events != 1 {
		t.Fatalf("duplicate diary publication %d %v", events, err)
	}
}
