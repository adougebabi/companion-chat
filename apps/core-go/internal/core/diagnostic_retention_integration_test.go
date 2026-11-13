package core

import (
	"bytes"
	"fmt"
	"testing"
)

func TestDiagnosticRetentionDefaultsPrunePrivateRecordsAndPreserveRecovery(t *testing.T) {
	f := v3OwnerFixture(t, "diagnostic-retention")
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.agent_runs(fluctlight_id,agent_id,run_id,input_digest,status,result) VALUES($1,'conversation_cognition','retained-business-run','fixture-digest','completed','{"trace":{"tool_result":"retained-committed-evidence"}}')`, f.fluctlightID); err != nil {
		t.Fatal(err)
	}
	var recoveryBefore []byte
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT to_jsonb(r) FROM public.agent_runs r WHERE run_id='retained-business-run'`).Scan(&recoveryBefore); err != nil {
		t.Fatal(err)
	}
	// Each table has one expired private row and 10,001 fresh rows. All timestamps
	// are distinct, so the row-cap expectation is independent of tie ordering.
	inserts := []string{
		`INSERT INTO public.diagnostic_events(id,event_type,severity,correlation_id,payload,created_at) SELECT 'retention-'||n,'fixture','info','retention-scope','{"prompt":"SYNTHETIC_PRIVATE"}',CASE WHEN n=0 THEN now()-interval '31 days' ELSE now()-n*interval '1 second' END FROM generate_series(0,10001) n`,
		`INSERT INTO public.diagnostic_model_runs(id,role,model_id,prompt,response,status,correlation_id,created_at) SELECT 'retention-'||n,'reply','fixture','{"content":"SYNTHETIC_PRIVATE"}','{"content":"SYNTHETIC_RESPONSE"}','completed','retention-scope',CASE WHEN n=0 THEN now()-interval '31 days' ELSE now()-n*interval '1 second' END FROM generate_series(0,10001) n`,
		`INSERT INTO public.diagnostic_turns(id,fluctlight_id,correlation_id,status,created_at) SELECT 'retention-'||n,'fixture','retention-scope','completed',CASE WHEN n=0 THEN now()-interval '31 days' ELSE now()-n*interval '1 second' END FROM generate_series(0,10001) n`,
		`INSERT INTO public.diagnostic_workflow_links(id,correlation_id,workflow_id,created_at) SELECT 'retention-'||n,'retention-scope','fixture-workflow',CASE WHEN n=0 THEN now()-interval '31 days' ELSE now()-n*interval '1 second' END FROM generate_series(0,10001) n`,
	}
	for _, sql := range inserts {
		if _, err := f.repository.Pool().Exec(f.ctx, sql); err != nil {
			t.Fatal(err)
		}
	}
	removed, err := f.app.PruneDiagnostics(f.ctx, 0, 0)
	if err != nil || removed != 8 {
		t.Fatal("default prune", removed, err)
	}
	for _, table := range []string{"diagnostic_events", "diagnostic_model_runs", "diagnostic_turns", "diagnostic_workflow_links"} {
		var count, expired int
		query := fmt.Sprintf(`SELECT count(*),count(*) FILTER(WHERE id IN ('retention-0','retention-10001')) FROM public.%s`, table)
		if err := f.repository.Pool().QueryRow(f.ctx, query).Scan(&count, &expired); err != nil || count != 10000 || expired != 0 {
			t.Fatal("age/row-cap target", table, count, expired, err)
		}
	}
	again, err := f.app.PruneDiagnostics(f.ctx, 0, 0)
	if err != nil || again != 0 {
		t.Fatal("prune replay", again, err)
	}
	var recoveryAfter []byte
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT to_jsonb(r) FROM public.agent_runs r WHERE run_id='retained-business-run'`).Scan(&recoveryAfter); err != nil || !bytes.Equal(recoveryBefore, recoveryAfter) {
		t.Fatal("diagnostic pruning changed authoritative recovery result", err)
	}
}
