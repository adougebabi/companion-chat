package core

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

func setupDiarySnapshotFixture(t *testing.T, suffix string) (context.Context, *PostgresRepository, *App, string, string, string, string) {
	t.Helper()
	ctx, repo, app, owner, fid, conversation := setupDirectPublicationToolTest(t, suffix)
	app.Clock = func() time.Time { return time.Date(2026, 10, 16, 2, 0, 0, 0, time.UTC) }
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.conversation_participants SET joined_at='2019-01-01T00:00:00Z' WHERE conversation_id=$1`, conversation); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.conversation_messages(id,conversation_id,sequence,author_actor_id,kind,text,attachment_refs,idempotency_key,created_at) SELECT $3::text,$1::text,COALESCE(MAX(sequence),0)+1,$2::text,'user','日记来源样本','[]',$4::text,'2020-01-01T01:00:00Z' FROM public.conversation_messages WHERE conversation_id=$1`, conversation, owner, "diary-source-"+suffix, "diary-key-"+suffix); err != nil {
		t.Fatal(err)
	}
	var accepted map[string]any
	if err := newPostgresSurface(repo.Pool()).Transact(ctx, func(txCtx context.Context, _ surfaceTx) error {
		var err error
		accepted, err = app.RequestOwnerDiary(txCtx, owner, fid, map[string]any{"local_date": "2020-01-01", "timezone": "Asia/Shanghai", "expected_revision": 0, "reason": "isolated snapshot fixture", "idempotency_key": "diary-request-" + suffix})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	diary := stringValue(accepted["diary_id"])
	var attempt string
	if err := repo.Pool().QueryRow(ctx, `SELECT id FROM public.fluctlight_diary_attempts WHERE diary_id=$1`, diary).Scan(&attempt); err != nil {
		t.Fatal(err)
	}
	zone, _ := time.LoadLocation("Asia/Shanghai")
	start, _ := time.ParseInLocation("2006-01-02", "2020-01-01", zone)
	sources, err := readDiarySourcesWith(ctx, repo.Pool(), fid, start, start.AddDate(0, 0, 1), app.now())
	if err != nil {
		t.Fatal(err)
	}
	plan := map[string]any{"write": true, "source_refs": []any{"message:diary-source-" + suffix}, "reason": "fixture"}
	persona := map[string]any{"identity": map[string]any{"name": "测试角色"}}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.fluctlight_diary_attempts SET status='running',source_snapshot=$2,source_digest=$3,plan=$4,persona_snapshot=$5 WHERE id=$1`, attempt, jsonBytes(sources), stableDigest(jsonString(sources)), jsonBytes(plan), jsonBytes(persona)); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.fluctlight_diary_days SET status='running' WHERE id=$1`, diary); err != nil {
		t.Fatal(err)
	}
	return ctx, repo, app, owner, fid, diary, attempt
}

func TestDiaryPostgresRestoresFrozenExpressionBinding(t *testing.T) {
	ctx, repo, app, owner, fid, diary, attempt := setupDiarySnapshotFixture(t, "diary-frozen")
	provenance := map[string]any{diaryModelSnapshotContractKey: diaryModelSnapshotVersion}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.fluctlight_diary_attempts SET model_provenance=$2 WHERE id=$1`, attempt, jsonBytes(provenance)); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.provider_endpoints(id,kind,base_url,secret_purpose,capability_status,checked_at) VALUES('old-diary-endpoint','openai_compatible','http://old-diary.invalid','old-diary-purpose','ready',now())`); err != nil {
		t.Fatal(err)
	}
	temperature, topP := 0.0, 0.73
	old := providerAssignment{Role: conversationReplyModelRole, EndpointID: "old-diary-endpoint", SecretPurpose: "old-diary-purpose", BaseURL: "http://old-diary.invalid", ModelID: "diary-model-a", Timeout: 30 * time.Second, PromptBudgetPolicyVersion: promptBudgetPolicyVersionV1, Temperature: &temperature, TopP: &topP}
	_, _, err := app.diaryStageModel(ctx, attempt, "expression", provenance, FormalAgentDiaryExpression, "diary_expression_text", conversationReplyModelRole, func() (diaryModelSnapshot, providerAssignment, error) {
		return newDiaryModelSnapshot(FormalAgentDiaryExpression, "diary_expression_text", conversationReplyModelRole, old, 9, 2), old, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.provider_endpoints(id,kind,base_url,secret_purpose,capability_status,checked_at) VALUES('new-diary-endpoint','openai_compatible','http://new-diary.invalid','new-diary-purpose','ready',now())`); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,required_capabilities,token_budget,timeout_seconds,retry_policy) VALUES('conversation_reply','new-diary-endpoint','diary-model-b','structured_output',4096,90,'{}') ON CONFLICT(role) DO UPDATE SET provider_endpoint_id=excluded.provider_endpoint_id,model_id=excluded.model_id,timeout_seconds=excluded.timeout_seconds`); err != nil {
		t.Fatal(err)
	}
	var wire map[string]any
	var destination string
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		wire = payload
		return fakeProviderResult{Text: "今天记下了这段真实交流。"}
	})
	app.Provider.HTTP = &http.Client{Transport: providerErrorTestTransport(func(request *http.Request) (*http.Response, error) {
		destination = request.URL.Host
		return router.RoundTrip(request)
	})}
	result, err := app.ProcessDiaryGeneration(ctx, attempt)
	if err != nil || result["status"] != "ready" || wire["model"] != "diary-model-a" || destination != "old-diary.invalid" {
		t.Fatalf("frozen diary result=%#v model=%v destination=%q err=%v", result, wire["model"], destination, err)
	}
	if wire["temperature"] != 0.0 || wire["top_p"] != topP || wire["max_tokens"] != nil || wire["max_completion_tokens"] != nil {
		t.Fatalf("frozen sampling or no-cap contract changed: %#v", wire)
	}
	item, err := app.OwnerDiary(ctx, owner, fid, diary)
	if err != nil {
		t.Fatal(err)
	}
	public := jsonString(item["model_provenance"])
	if strings.Contains(public, "old-diary-purpose") || strings.Contains(public, "old-diary.invalid") || strings.Contains(public, diaryModelSnapshotsKey) {
		t.Fatalf("private binding exposed: %s", public)
	}
}

func TestDiaryPostgresLegacyRunningWithoutSnapshotFailsExplicitly(t *testing.T) {
	ctx, repo, app, _, _, diary, attempt := setupDiarySnapshotFixture(t, "diary-legacy-snapshot")
	result, err := app.ProcessDiaryGeneration(ctx, attempt)
	if result != nil || !errors.Is(err, errDiaryModelSnapshotMissing) {
		t.Fatalf("legacy running attempt silently rebound: %#v %v", result, err)
	}
	var status, code string
	if err := repo.Pool().QueryRow(ctx, `SELECT status,COALESCE(last_error_code,'') FROM public.fluctlight_diary_days WHERE id=$1`, diary).Scan(&status, &code); err != nil || status != "failed" || code != "diary_model_snapshot_missing" {
		t.Fatalf("legacy failure status=%q code=%q err=%v", status, code, err)
	}
}

func TestDiaryPostgresRestoresFrozenMaterialBinding(t *testing.T) {
	ctx, repo, app, _, _, _, attempt := setupDiarySnapshotFixture(t, "diary-material-frozen")
	provenance := map[string]any{diaryModelSnapshotContractKey: diaryModelSnapshotVersion}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.fluctlight_diary_attempts SET plan=NULL,model_provenance=$2 WHERE id=$1`, attempt, jsonBytes(provenance)); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.provider_endpoints(id,kind,base_url,secret_purpose,capability_status,checked_at) VALUES('old-material-endpoint','openai_compatible','http://old-material.invalid','old-material-purpose','ready',now())`); err != nil {
		t.Fatal(err)
	}
	old := providerAssignment{Role: "generic_llm", EndpointID: "old-material-endpoint", SecretPurpose: "old-material-purpose", BaseURL: "http://old-material.invalid", ModelID: "material-model-a", Timeout: 30 * time.Second, PromptBudgetPolicyVersion: promptBudgetPolicyVersionV1}
	_, _, err := app.diaryStageModel(ctx, attempt, "material", provenance, FormalAgentDiaryMaterial, "diary_material_v1", "cognitive_assessment", func() (diaryModelSnapshot, providerAssignment, error) {
		return newDiaryModelSnapshot(FormalAgentDiaryMaterial, "diary_material_v1", "cognitive_assessment", old, 0, 0), old, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.provider_endpoints(id,kind,base_url,secret_purpose,capability_status,checked_at) VALUES('new-material-endpoint','openai_compatible','http://new-material.invalid','new-material-purpose','ready',now())`); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,required_capabilities,token_budget,timeout_seconds,retry_policy) VALUES('generic_llm','new-material-endpoint','material-model-b','structured_output',4096,90,'{}') ON CONFLICT(role) DO UPDATE SET provider_endpoint_id=excluded.provider_endpoint_id,model_id=excluded.model_id,timeout_seconds=excluded.timeout_seconds`); err != nil {
		t.Fatal(err)
	}
	var wire map[string]any
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		wire = payload
		return fakeProviderResult{Structured: map[string]any{"write": false, "source_refs": []any{}, "reason": "本次不写"}}
	})
	app.Provider.HTTP = &http.Client{Transport: router}
	result, err := app.ProcessDiaryGeneration(ctx, attempt)
	if err != nil || result["status"] != "no_content" || wire["model"] != "material-model-a" {
		t.Fatalf("material snapshot result=%#v model=%v err=%v", result, wire["model"], err)
	}
}
