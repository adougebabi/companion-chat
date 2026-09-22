package core

import (
	"encoding/json"
	"testing"
)

func TestDelegatedToolPolicyKeepsOwnerCallsIndependentAndChecksActualAction(t *testing.T) {
	ctx, repo, app, owner, fl, conversation := setupDirectPublicationToolTest(t, "policy")
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.autonomy_policies(fluctlight_id,mode,allowed_actions,budget_remaining,quiet_hours,concurrency_limit,revision) VALUES($1,'active','["capability"]',5,'{}',1,0) ON CONFLICT(fluctlight_id) DO UPDATE SET allowed_actions='["capability"]',mode='active',budget_remaining=5`, fl); err != nil {
		t.Fatal(err)
	}
	request := ToolExecutionRequest{AuthorizationPolicy: "autonomy", CapabilityName: "conversation.reply", OperationID: "delegated-denied", AuthorizationActorID: owner, FluctlightID: fl, ConversationID: conversation, Arguments: json.RawMessage(`{"text":"delegated reply"}`)}
	result, err := app.ExecuteTool(ctx, request)
	if err != nil || result.Result.Status != "rejected" || result.Result.ErrorCode != "policy_action_not_allowed" {
		t.Fatalf("specific publication permission bypassed: %#v %v", result, err)
	}
	var count int
	if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant'`, conversation).Scan(&count); err != nil || count != 0 {
		t.Fatalf("denied publication wrote %d messages: %v", count, err)
	}
	request.AuthorizationPolicy = ""
	request.OperationID = "explicit-owner"
	result, err = app.ExecuteTool(ctx, request)
	if err != nil || result.Result.Status != "completed" {
		t.Fatalf("explicit Owner command was incorrectly gated: %#v %v", result, err)
	}
}

func TestDelegatedToolBudgetReservesOncePerRunAndHonorsPolicyRevocation(t *testing.T) {
	ctx, repo, app, owner, fl, conversation := setupDirectPublicationToolTest(t, "policy-budget")
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.autonomy_policies(fluctlight_id,mode,allowed_actions,budget_remaining,quiet_hours,concurrency_limit,revision) VALUES($1,'active','["proactive_message","moment","capability"]',1,'{}',1,0) ON CONFLICT(fluctlight_id) DO UPDATE SET allowed_actions='["proactive_message","moment","capability"]',mode='active',budget_remaining=1`, fl); err != nil {
		t.Fatal(err)
	}
	request := ToolExecutionRequest{AuthorizationPolicy: "autonomy", AgentID: FormalAgentWakeUp, RunID: "one-authorized-run", CapabilityName: "conversation.reply", OperationID: "first-reply", AuthorizationActorID: owner, FluctlightID: fl, ConversationID: conversation, Arguments: json.RawMessage(`{"text":"first"}`)}
	for _, op := range []string{"first-reply", "second-reply"} {
		request.OperationID = op
		result, err := app.ExecuteTool(ctx, request)
		if err != nil || result.Result.Status != "completed" {
			t.Fatalf("authorized same-run command %s: %#v %v", op, result, err)
		}
	}
	var budget string
	if err := repo.Pool().QueryRow(ctx, `SELECT budget_remaining FROM public.autonomy_policies WHERE fluctlight_id=$1`, fl).Scan(&budget); err != nil || budget != "0" {
		t.Fatalf("budget=%s %v", budget, err)
	}
	request.RunID = "new-run"
	request.OperationID = "exhausted"
	result, err := app.ExecuteTool(ctx, request)
	if err != nil || result.Result.ErrorCode != "policy_budget_exhausted" {
		t.Fatalf("exhausted budget: %#v %v", result, err)
	}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.autonomy_policies SET mode='paused' WHERE fluctlight_id=$1`, fl); err != nil {
		t.Fatal(err)
	}
	request.RunID = "one-authorized-run"
	request.OperationID = "after-revocation"
	result, err = app.ExecuteTool(ctx, request)
	if err != nil || result.Result.ErrorCode != "policy_autonomy_mode_blocked" {
		t.Fatalf("policy revocation ignored: %#v %v", result, err)
	}
	request.OperationID = "first-reply"
	result, err = app.ExecuteTool(ctx, request)
	if err != nil || !result.Replayed || result.Result.Status != "completed" {
		t.Fatalf("committed fact lost after revocation: %#v %v", result, err)
	}
}
