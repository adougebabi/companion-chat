package workflow

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/core"
	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/migrations"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/testsuite"
)

func TestPersonaPreparationWorkflowRetriesAndReturnsTerminal(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.OnActivity(ProcessPersonaPreparationActivity, mock.Anything, mock.Anything).Return(nil, errors.New("compiler failed")).Once()
	env.OnActivity(ProcessPersonaPreparationActivity, mock.Anything, mock.Anything).Return(map[string]any{"status": "completed"}, nil).Once()
	env.ExecuteWorkflow(PersonaPreparationWorkflow, Input{IntentID: "intent", FluctlightID: "role"})
	if err := env.GetWorkflowError(); err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := env.GetWorkflowResult(&result); err != nil || result["status"] != "completed" {
		t.Fatal(result, err)
	}
	env.AssertExpectations(t)
	if fn, err := workflowFunction("persona.compile"); err != nil || fn == nil {
		t.Fatal(err)
	}
}
func TestPostgresCreationStageOrdering(t *testing.T) {
	databaseURL := os.Getenv("GO_CORE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("GO_CORE_TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.Database = "postgres"
	admin, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	db := fmt.Sprintf("v62_stage_%d", time.Now().UnixNano())
	ident := pgx.Identifier{db}.Sanitize()
	if _, err = admin.Exec(ctx, "CREATE DATABASE "+ident); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = admin.Exec(context.Background(), "DROP DATABASE "+ident) }()
	config.ConnConfig.Database = db
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err = migrations.New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	repoURL, err := url.Parse(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	repoURL.Path = "/" + db
	repo, err := core.NewPostgresRepository(ctx, repoURL.String())
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec("INSERT INTO public.actors(id,actor_type,status) VALUES('order-owner','human','active')")
	exec("INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES('order-owner','hash','revision')")
	app := &core.App{DB: repo}
	if _, err = app.AdmitFluctlight(ctx, "order-owner", "order-role", "摇光", "blank_slate", "", nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	check := func(kind string, want bool) {
		t.Helper()
		var actual bool
		err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.platform_workflow_intents candidate WHERE candidate.intent_type=$1 `+creationStageReadySQL+`)`, kind).Scan(&actual)
		if err != nil || actual != want {
			t.Fatalf("stage %s ready=%v want=%v err=%v", kind, actual, want, err)
		}
	}
	check("persona.compile", false)
	check("wardrobe.initialize", false)
	check("visual_identity.initialize", false)
	check("schedule.current_day", false)
	exec("INSERT INTO public.life_worlds(id,fluctlight_id,name) VALUES('order-world','order-role','生活世界')")
	exec("INSERT INTO public.fluctlight_world_bindings(fluctlight_id,world_id,revision) VALUES('order-role','order-world',1)")
	check("persona.compile", true)
	eligible := func(kind string, want bool) {
		t.Helper()
		rows, err := pool.Query(ctx, dispatcherCandidateQuery(), 100)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		actual := false
		for rows.Next() {
			var id, workflowID, queue, intentType string
			var payload []byte
			var attempt int
			if err := rows.Scan(&id, &workflowID, &queue, &intentType, &payload, &attempt); err != nil {
				t.Fatal(err)
			}
			if intentType == kind {
				actual = true
			}
		}
		if err := rows.Err(); err != nil || actual != want {
			t.Fatalf("full dispatcher %s ready=%v want=%v err=%v", kind, actual, want, err)
		}
	}
	eligible("persona.compile", true)
	eligible("wardrobe.initialize", false)
	exec("UPDATE public.platform_workflow_intents SET status='completed' WHERE intent_type='persona.compile'")
	eligible("wardrobe.initialize", true)
	eligible("visual_identity.initialize", false)
	// General persona dispatch barrier additionally holds wardrobe until compile completes.
	check("visual_identity.initialize", false)
	exec("UPDATE public.fluctlight_wardrobe_preparations SET status='failed' WHERE fluctlight_id='order-role'")
	check("visual_identity.initialize", false)
	exec("UPDATE public.fluctlight_wardrobe_preparations SET status='completed' WHERE fluctlight_id='order-role'")
	check("visual_identity.initialize", true)
	eligible("visual_identity.initialize", true)
	check("schedule.current_day", false)
	eligible("schedule.current_day", false)
	exec("UPDATE public.fluctlight_visual_identities SET status='active',canonical_asset_id='ref',character_sheet_asset_id='sheet' WHERE fluctlight_id='order-role'")
	exec("UPDATE public.fluctlight_visual_identity_sessions SET status='character_sheet_pending' WHERE fluctlight_id='order-role'")
	check("schedule.current_day", false)
	exec("UPDATE public.fluctlight_visual_identity_sessions SET status='completed' WHERE fluctlight_id='order-role'")
	check("schedule.current_day", true)
	eligible("schedule.current_day", true)
	check("wake_up.current", false)
	exec("UPDATE public.platform_workflow_intents SET status='completed' WHERE intent_id='schedule_intent:order-role'")
	check("wake_up.current", false)
	// A stable scheduler keeps running across midnights. Accepted domain state,
	// rather than Workflow completion, releases the initial WakeUp dependency.
	exec("UPDATE public.platform_workflow_intents SET status='started' WHERE intent_id='schedule_intent:order-role'")
	exec(`INSERT INTO public.life_schedules(id,fluctlight_id,local_date,timezone,status,generated_from,evidence_refs,revision,reschedule_policy,idempotency_key,request_digest,result) VALUES('order-initial-schedule','order-role','2026-10-24','Asia/Shanghai','accepted','test','["fact"]',1,'{}','order-initial-schedule','aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','{"id":"order-initial-schedule","status":"accepted","revision":1,"expected_context_revision":"life_ctx_before","resulting_context_revision":"life_ctx_after","replayed":false}')`)
	check("wake_up.current", true)
	exec("UPDATE public.platform_workflow_intents SET next_attempt_at=now() WHERE intent_type='wake_up.current'")
	eligible("wake_up.current", true)
	// Completed bootstrap remains usable when a later unrelated portrait job fails.
	exec("INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload) VALUES('later','later','lifecycle','schedule.current_day','{\"fluctlight_id\":\"order-role\"}')")
	var later bool
	if err = pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.platform_workflow_intents candidate WHERE intent_id='later' `+creationStageReadySQL+`)`).Scan(&later); err != nil || !later {
		t.Fatal(later, err)
	}
}
