package core

import "testing"

func TestModelRunReadKeepsIndependentReplyParentProvenance(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	owner, fid := "model-parent-owner", "model-parent-fixture"
	seedLifeContextFluctlight(t, ctx, repository, owner, fid)
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES($1,'test-hash','test-revision')`, owner); err != nil {
		t.Fatal(err)
	}
	// Controlled diagnostic fixture, not evidence of a real model invocation.
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.diagnostic_model_runs(id,role,binding_role,scenario,model_id,prompt,status,correlation_id,metrics) VALUES('reply-parent-test','conversation_reply','conversation_reply','fact_only_reply','test-reply','{}','completed','reply:child','{"run_id":"reply:child","parent_run_id":"turn:original","model_call_id":"physical-reply"}')`); err != nil {
		t.Fatal(err)
	}
	page, err := (&App{DB: repository}).ModelRunsPage(ctx, owner, 10, "reply:child", "")
	if err != nil || len(page.Items) != 1 || page.Items[0]["parent_run_id"] != "turn:original" || page.Items[0]["logical_run_id"] != "reply:child" {
		t.Fatal("Independent parent provenance did not survive the actual read projection", page, err)
	}
}
