package core

import (
	"context"
	"errors"
	"testing"
)

func personaPreparationFixture(t *testing.T, router *fakeProviderRouter) (context.Context, *PostgresRepository, *App) {
	t.Helper()
	ctx, repo := isolatedCoreTestRepository(t)
	seedCognitiveProviderRole(t, ctx, repo, "preparation-provider")
	app := newTestApp(t, repo, router)
	if _, err := repo.Pool().Exec(ctx, "INSERT INTO public.actors(id,actor_type,status) VALUES('prep-owner','human','active')"); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, "INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES('prep-owner','hash','rev')"); err != nil {
		t.Fatal(err)
	}
	if _, err := app.AdmitFluctlight(ctx, "prep-owner", "prep-role", "摇光", "blank_slate", "", nil, nil, nil, "Asia/Shanghai"); err != nil {
		t.Fatal(err)
	}
	return ctx, repo, app
}
func latestPreparationID(t *testing.T, ctx context.Context, repo *PostgresRepository) string {
	t.Helper()
	var id string
	if err := repo.Pool().QueryRow(ctx, "SELECT intent_id FROM public.platform_workflow_intents WHERE intent_type='persona.compile' ORDER BY created_at DESC,intent_id DESC LIMIT 1").Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}
func TestPostgresPersonaPreparationAdmissionCompileReplay(t *testing.T) {
	router := newFakeProviderRouter()
	ctx, repo, app := personaPreparationFixture(t, router)
	id := latestPreparationID(t, ctx, repo)
	if len(router.requests) != 0 {
		t.Fatal("admission called a model")
	}
	var portraits int
	if err := repo.Pool().QueryRow(ctx, "SELECT count(*) FROM public.fluctlight_working_personas").Scan(&portraits); err != nil || portraits != 0 {
		t.Fatal(portraits, err)
	}
	if err := app.VerifyWorkingPersonasReady(ctx); err != nil {
		t.Fatal("durable pending preparation blocked startup", err)
	}
	detail, err := app.FluctlightDetail(ctx, "prep-owner", "prep-role")
	if err != nil || mapValue(detail["persona_preparation"])["status"] != "pending" {
		t.Fatal("pending preparation was not readable through detail", detail, err)
	}
	if _, err := app.AdmitFluctlight(ctx, "prep-owner", "prep-role", "摇光", "blank_slate", "", nil, nil, nil, "Asia/Shanghai"); err != nil {
		t.Fatal(err)
	}
	var intents int
	if err := repo.Pool().QueryRow(ctx, "SELECT count(*) FROM public.platform_workflow_intents WHERE intent_type='persona.compile'").Scan(&intents); err != nil || intents != 1 {
		t.Fatal(intents, err)
	}
	for n := 0; n < 2; n++ {
		if _, err := app.ProcessPersonaPreparationIntent(ctx, id); err != nil {
			t.Fatal(err)
		}
	}
	if err := repo.Pool().QueryRow(ctx, "SELECT count(*) FROM public.fluctlight_working_personas WHERE source_revision=0 AND status='completed'").Scan(&portraits); err != nil || portraits != 1 {
		t.Fatal(portraits, err)
	}
	if len(router.requests) != 0 {
		t.Fatal("blank baseline called a model")
	}
	if err := app.VerifyWorkingPersonasReady(ctx); err != nil {
		t.Fatal(err)
	}
}
func TestPostgresPersonaPreparationSaveFailureRecoveryAndCAS(t *testing.T) {
	fail := true
	calls := 0
	router := newFakeProviderRouter().on("persona_compilation_response", func(_ map[string]any) fakeProviderResult {
		calls++
		if fail {
			return fakeProviderResult{Status: 500}
		}
		return fakeProviderResult{Structured: map[string]any{"portrait_text": "摇光温暖直接，喜欢咖啡。"}}
	})
	ctx, repo, app := personaPreparationFixture(t, router)
	if _, err := app.ProcessPersonaPreparationIntent(ctx, latestPreparationID(t, ctx, repo)); err != nil {
		t.Fatal(err)
	}
	proposal, err := app.ProposeFoundation(ctx, "prep-owner", "prep-role", map[string]any{"reason": "修改资料", "expected_revision": 0, "changes": map[string]any{"identity": map[string]any{"name": "新名字"}}})
	if err != nil {
		t.Fatal(err)
	}
	zero := 0
	result, err := app.AdmitFoundationDecisionExpected(ctx, "prep-owner", "prep-role", stringValue(proposal["id"]), "accept", "确认保存", &zero)
	if err != nil || calls != 0 || mapValue(result["persona_preparation"])["status"] != "pending" {
		t.Fatal(result, calls, err)
	}
	saved, err := repo.GetFluctlight(ctx, "prep-role", "prep-owner")
	if err != nil || saved.CurrentRevision != 1 || saved.Identity["name"] != "新名字" {
		t.Fatal(saved, err)
	}
	id := latestPreparationID(t, ctx, repo)
	if _, err = app.ProcessPersonaPreparationIntent(ctx, id); err == nil {
		t.Fatal("failed compiler succeeded")
	}
	var revision int
	if err = repo.Pool().QueryRow(ctx, "SELECT source_revision FROM public.fluctlight_working_personas WHERE fluctlight_id='prep-role'").Scan(&revision); err != nil || revision != 0 {
		t.Fatal(revision, err)
	}
	// Temporal reconciliation owns terminal failure; simulate that boundary.
	if _, err = repo.Pool().Exec(ctx, "UPDATE public.platform_workflow_intents SET status='failed' WHERE intent_id=$1", id); err != nil {
		t.Fatal(err)
	}
	if _, err = app.RecoverPersonaPreparation(ctx, "prep-owner", "prep-role", 0, "recover"); !errors.Is(err, ErrConflict) {
		t.Fatal("stale recovery accepted", err)
	}
	fail = false
	first, err := app.RecoverPersonaPreparation(ctx, "prep-owner", "prep-role", 1, "recover")
	if err != nil || first["id"] == id {
		t.Fatal(first, err)
	}
	again, err := app.RecoverPersonaPreparation(ctx, "prep-owner", "prep-role", 1, "recover")
	if err != nil || again["id"] != first["id"] {
		t.Fatal(again, err)
	}
	if _, err = app.ProcessPersonaPreparationIntent(ctx, stringValue(first["id"])); err != nil {
		t.Fatal(err)
	}
	if err = repo.Pool().QueryRow(ctx, "SELECT source_revision FROM public.fluctlight_working_personas WHERE fluctlight_id='prep-role'").Scan(&revision); err != nil || revision != 1 {
		t.Fatal(revision, err)
	}
	if err = app.VerifyWorkingPersonasReady(ctx); err != nil {
		t.Fatal(err)
	}
}
func TestPostgresPersonaPreparationRetirementDuringCompileNeverPublishes(t *testing.T) {
	var app *App
	var ctx context.Context
	calls := 0
	router := newFakeProviderRouter().on("persona_compilation_response", func(_ map[string]any) fakeProviderResult {
		calls++
		expected := 1
		if _, err := app.RetireFluctlight(ctx, "prep-owner", "prep-role", "停止角色", &expected); err != nil {
			t.Fatal(err)
		}
		return fakeProviderResult{Structured: map[string]any{"portrait_text": "准备返回的画像"}}
	})
	var repo *PostgresRepository
	ctx, repo, app = personaPreparationFixture(t, router)
	old := latestPreparationID(t, ctx, repo)
	proposal, err := app.ProposeFoundation(ctx, "prep-owner", "prep-role", map[string]any{"reason": "修改资料", "expected_revision": 0, "changes": map[string]any{"identity": map[string]any{"name": "新名字"}}})
	if err != nil {
		t.Fatal(err)
	}
	zero := 0
	if _, err = app.AdmitFoundationDecisionExpected(ctx, "prep-owner", "prep-role", stringValue(proposal["id"]), "accept", "确认保存", &zero); err != nil {
		t.Fatal(err)
	}
	if _, err = app.ProcessPersonaPreparationIntent(ctx, old); err != nil || calls != 0 {
		t.Fatal("superseded preparation called model", calls, err)
	}
	_, _ = app.ProcessPersonaPreparationIntent(ctx, latestPreparationID(t, ctx, repo))
	var count int
	if err = repo.Pool().QueryRow(ctx, "SELECT count(*) FROM public.fluctlight_working_personas WHERE fluctlight_id='prep-role'").Scan(&count); err != nil || count != 0 || calls != 1 {
		t.Fatal("retired portrait published", count, calls, err)
	}
	if _, err = app.RecoverPersonaPreparation(ctx, "prep-owner", "prep-role", 1, "retired"); err == nil {
		t.Fatal("retired recovery admitted")
	}
}
