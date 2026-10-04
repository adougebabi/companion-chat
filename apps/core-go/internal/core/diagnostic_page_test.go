package core

import (
	"fmt"
	"testing"
	"time"
)

func TestDiagnosticModelPagesKeepCreationOrderPrecisionAndSnapshotMembership(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	owner, fluctlight := "diag-page-owner", "diag-page-fluctlight"
	seedLifeContextFluctlight(t, ctx, repository, owner, fluctlight)
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES($1,'hash','revision')`, owner); err != nil {
		t.Fatal(err)
	}
	app := &App{DB: repository}
	base := time.Now().UTC().Add(-time.Hour).Truncate(time.Millisecond)
	for index := 0; index < 7; index++ {
		status := "completed"
		if index == 0 {
			status = "failed"
		}
		if index == 2 {
			status = "queued"
		}
		at := base.Add(time.Duration(index) * time.Microsecond)
		if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.diagnostic_model_runs(id,role,model_id,prompt,status,correlation_id,created_at) VALUES($1,'reply','m','{}',$2,'pages',$3)`, fmt.Sprintf("row-%d", index), status, at); err != nil {
			t.Fatal(err)
		}
	}
	first, err := app.ModelRunsPage(ctx, owner, 2, "pages", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Items) != 2 || first.Items[0]["id"] != "row-6" || first.Items[1]["id"] != "row-5" || first.NextCursor == "" {
		t.Fatal(first)
	}
	// Updating a tuple changes xmin, but the immutable insertion xid must keep
	// it visible in this pagination snapshot.
	if _, err := repository.Pool().Exec(ctx, `UPDATE public.diagnostic_model_runs SET status='failed',error_code='new_failure' WHERE id='row-4'`); err != nil {
		t.Fatal(err)
	}
	// A late commit with an old created_at cannot enter the earlier snapshot.
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.diagnostic_model_runs(id,role,model_id,prompt,status,correlation_id,created_at) VALUES('late-row','reply','m','{}','completed','pages',$1)`, base.Add(3*time.Microsecond)); err != nil {
		t.Fatal(err)
	}
	ids := []string{stringValue(first.Items[0]["id"]), stringValue(first.Items[1]["id"])}
	cursor := first.NextCursor
	for cursor != "" {
		page, err := app.ModelRunsPage(ctx, owner, 2, "pages", cursor)
		if err != nil {
			t.Fatal(err)
		}
		for _, row := range page.Items {
			ids = append(ids, stringValue(row["id"]))
		}
		cursor = page.NextCursor
	}
	if fmt.Sprint(ids) != "[row-6 row-5 row-4 row-3 row-2 row-1 row-0]" {
		t.Fatalf("missing/duplicate/backdated insertion: %v", ids)
	}
	if _, err := app.ModelRunsPage(ctx, owner, 2, "other-filter", first.NextCursor); err == nil {
		t.Fatal("cursor changed filter")
	}
	refreshed, err := app.ModelRunsPage(ctx, owner, 20, "pages", "")
	if err != nil || len(refreshed.Items) != 8 {
		t.Fatalf("refresh %#v %v", refreshed, err)
	}
}
func TestDiagnosticAgentUnionSortsBeforePaginationWithoutErrorPriority(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	owner, fluctlight := "agent-page-owner", "agent-page-fluctlight"
	seedLifeContextFluctlight(t, ctx, repository, owner, fluctlight)
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES($1,'hash','revision')`, owner); err != nil {
		t.Fatal(err)
	}
	base := time.Now().UTC().Add(-time.Hour)
	for index := 0; index < 4; index++ {
		if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.agent_runs(fluctlight_id,agent_id,run_id,input_digest,status,started_at,correlation_id) VALUES($1,'conversation_cognition',$2,'d','completed',$3,'union-pages')`, fluctlight, fmt.Sprintf("run-%d", index), base.Add(time.Duration(index)*time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.diagnostic_events(id,event_type,severity,fluctlight_id,correlation_id,payload,created_at) VALUES('old-failed','agent.run.termination','error',$1,'union-pages','{"status":"failed","agent_id":"reflection","reason":"old_failure"}',$2),('new-failed','agent.run.termination','error',$1,'union-pages','{"status":"failed","agent_id":"reflection","reason":"new_failure"}',$3)`, fluctlight, base.Add(-time.Minute), base.Add(2*time.Minute+30*time.Second)); err != nil {
		t.Fatal(err)
	}
	app := &App{DB: repository}
	cursor := ""
	ids := []string{}
	for {
		page, err := app.AgentRunsPage(ctx, owner, 2, "union-pages", cursor)
		if err != nil {
			t.Fatal(err)
		}
		for _, row := range page.Items {
			ids = append(ids, stringValue(row["run_id"]))
		}
		cursor = page.NextCursor
		if cursor == "" {
			break
		}
	}
	if fmt.Sprint(ids) != "[run-3 event:new-failed run-2 run-1 run-0 event:old-failed]" {
		t.Fatal(ids)
	}
}

func TestDiagnosticPagesSameInstantNullTailAndMalformedCursor(t *testing.T) {
	ctx, repo := isolatedCoreTestRepository(t)
	owner, fl := "null-page-owner", "null-page-fl"
	seedLifeContextFluctlight(t, ctx, repo, owner, fl)
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES($1,'hash','r')`, owner); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `ALTER TABLE public.diagnostic_model_runs ALTER COLUMN created_at DROP NOT NULL`); err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC().Add(-time.Hour)
	for _, id := range []string{"same-a", "same-b", "same-c"} {
		if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.diagnostic_model_runs(id,role,model_id,prompt,status,correlation_id,created_at) VALUES($1,'reply','m','{}','completed','null-pages',$2)`, id, at); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.diagnostic_model_runs(id,role,model_id,prompt,status,correlation_id,created_at) VALUES('null-a','reply','m','{}','failed','null-pages',NULL),('null-b','reply','m','{}','queued','null-pages',NULL)`); err != nil {
		t.Fatal(err)
	}
	app := &App{DB: repo}
	ids := []string{}
	cursor := ""
	for {
		page, err := app.ModelRunsPage(ctx, owner, 1, "null-pages", cursor)
		if err != nil {
			t.Fatal(err)
		}
		for _, row := range page.Items {
			ids = append(ids, stringValue(row["id"]))
		}
		cursor = page.NextCursor
		if cursor == "" {
			break
		}
	}
	if fmt.Sprint(ids) != "[same-c same-b same-a null-b null-a]" {
		t.Fatal(ids)
	}
	for _, raw := range []string{"%not-base64", "e30", "bm90LWpzb24"} {
		if _, err := app.ModelRunsPage(ctx, owner, 1, "null-pages", raw); err == nil {
			t.Fatal("malformed cursor accepted", raw)
		}
	}
}
