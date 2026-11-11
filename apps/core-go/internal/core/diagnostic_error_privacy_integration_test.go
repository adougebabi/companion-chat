package core

import (
	"errors"
	"net/url"
	"strings"
	"testing"
)

func TestAgentDiagnosticErrorDoesNotExposeBearerCredential(t *testing.T) {
	ctx, repo, app, owner, fid, _ := setupDirectPublicationToolTest(t, "agent-secret")
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES($1,'fixture','fixture')`, owner); err != nil {
		t.Fatal(err)
	}
	definition, _ := FormalAgentDefinitionByID(FormalAgentConversationCognition)
	input := ADKStructuredTaskInput{AgentID: definition.ID, Prompt: PromptAssemblyResult{Messages: []map[string]any{{"role": "user", "content": "fixture"}}}, Capability: &ADKCapabilityRequest{AuthorizationActorID: owner, FluctlightID: fid, OperationID: "agent-secret-run"}}
	record, _, err := app.admitFormalRun(ctx, definition, input)
	if err != nil || record == nil {
		t.Fatal(record, err)
	}
	const credential = "synthetic-bearer-value-74291"
	trace := &ADKCapabilityTrace{}
	trace.Append(CapabilityInvocation{CallID: "failed-tool", CapabilityName: "fixture.tool"}, CapabilityResult{CallID: "failed-tool", CapabilityName: "fixture.tool", Status: "failed", Output: map[string]any{"detail": "Authorization: Bearer " + credential}})
	result := ADKStructuredTaskResult{Trace: trace}
	if err := app.finishFormalRun(ctx, record, result, errors.New("Authorization: Bearer "+credential)); err != nil {
		t.Fatal(err)
	}
	var stored string
	var storedResult []byte
	if err := repo.Pool().QueryRow(ctx, `SELECT error_detail,result FROM public.agent_runs WHERE run_id=$1`, record.RunID).Scan(&stored, &storedResult); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stored, credential) || strings.Contains(string(storedResult), credential) {
		t.Fatal("new Agent run persisted the bearer credential in an error or failed Tool result")
	}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.agent_runs SET error_detail=$2 WHERE run_id=$1`, record.RunID, "Authorization: Bearer "+credential); err != nil {
		t.Fatal(err)
	}
	if _, _, err := app.admitFormalRun(ctx, definition, input); err == nil || strings.Contains(err.Error(), credential) {
		t.Fatalf("legacy run replay exposed the credential: %v", err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.agent_runs(fluctlight_id,agent_id,run_id,input_digest,status,error_detail,result,finished_at) VALUES($1,'conversation_cognition','legacy-secret','digest','failed',$2,$3,now())`, fid, "Authorization: Bearer "+credential, jsonBytes(result)); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.diagnostic_events(id,event_type,severity,fluctlight_id,correlation_id,payload) VALUES('legacy-lifecycle-secret','lifecycle.fixture','error',$1,'legacy-lifecycle',$2)`, fid,
		jsonBytes(map[string]any{"safe_cause": "Authorization: Bearer " + credential, "first_safe_cause": "Authorization: Bearer " + credential, "latest_safe_cause": "Authorization: Bearer " + credential})); err != nil {
		t.Fatal(err)
	}
	old, err := app.AgentRunsFiltered(ctx, owner, 10, "")
	if err != nil {
		t.Fatal(err)
	}
	page, err := app.OwnerDiagnosticRecords(ctx, owner, "agent-runs", url.Values{"limit": {"10"}})
	if err != nil {
		t.Fatal(err)
	}
	lifecycle, err := app.OwnerDiagnosticRecords(ctx, owner, "events", url.Values{"correlationId": {"legacy-lifecycle"}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(jsonString(old), credential) || strings.Contains(jsonString(page), credential) || strings.Contains(jsonString(lifecycle), credential) {
		t.Fatal("historical Agent error exposed the bearer credential")
	}
}
