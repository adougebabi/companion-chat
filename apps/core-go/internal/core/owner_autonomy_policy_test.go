package core

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestRoleAutonomyAuthoritativeSettingsCASMuteAndActualToolAdmission(t *testing.T) {
	ctx, repo, app, owner, fid, conversation := setupDirectPublicationToolTest(t, "role-policy")
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES($1,'test-hash','test-revision')`, owner); err != nil {
		t.Fatal(err)
	}
	update := func(body map[string]any) (map[string]any, error) {
		var result map[string]any
		err := newPostgresSurface(repo.Pool()).Transact(ctx, func(txCtx context.Context, _ surfaceTx) error {
			var err error
			result, err = app.UpdateOwnerAutonomyPolicy(txCtx, owner, fid, body)
			return err
		})
		return result, err
	}
	body := map[string]any{"expected_revision": 0, "policy_source": "override", "reason": "test role override", "mode": "active", "outreach_paused": true, "notifications_muted": true, "budget_remaining": 4, "concurrency_limit": 2, "cooldown_seconds": 30, "quiet_hours": map[string]any{}, "confirm_action_types": []any{}, "allowed_actions": []any{"proactive_message", "moment", "capability"}}
	saved, err := update(body)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := update(body); err == nil {
		t.Fatal("stale policy accepted")
	} else {
		var conflict *SurfaceError
		if !errors.As(err, &conflict) || conflict.Status != 409 {
			t.Fatal(err)
		}
	}
	read, err := app.OwnerAutonomyPolicy(ctx, owner, fid)
	if err != nil || read["notifications_muted"] != true || read["runtime_effect"] != "enforced" || intValue(read["revision"]) != intValue(saved["revision"]) {
		t.Fatalf("policy readback %#v %v", read, err)
	}
	policy, err := app.EvaluateAutonomyPolicy(ctx, fid, "proactive_message", time.Now().UTC())
	if err != nil || policy.Allowed || policy.Reason != "outreach_paused" {
		t.Fatalf("outreach not consumed %#v %v", policy, err)
	}
	request := ToolExecutionRequest{AuthorizationPolicy: "autonomy", AgentID: FormalAgentWakeUp, RunID: "paused-outreach", CapabilityName: "conversation.reply", OperationID: "blocked-outreach", AuthorizationActorID: owner, FluctlightID: fid, ConversationID: conversation, Arguments: json.RawMessage(`{"text":"must not be published"}`)}
	receipt, err := app.ExecuteTool(ctx, request)
	if err != nil || receipt.Result.ErrorCode != "policy_outreach_paused" {
		t.Fatalf("actual publication bypass %#v %v", receipt, err)
	}
	// Global changes never overwrite this role's explicit configuration.
	if err := withTransaction(ctx, repo.Pool(), func(tx pgx.Tx) error {
		_, err := applyGlobalAutonomyConfigurationTx(ctx, tx, map[string]any{"mode": "paused", "budget_remaining": 99})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	read, err = app.OwnerAutonomyPolicy(ctx, owner, fid)
	if err != nil || read["mode"] != "active" || intValue(read["budget_remaining"]) != 4 {
		t.Fatalf("global overwrote role %#v %v", read, err)
	}
	// An explicit Owner operation is a separate authorization path.
	request.AuthorizationPolicy = ""
	request.OperationID = "explicit-owner-after-pause"
	receipt, err = app.ExecuteTool(ctx, request)
	if err != nil || receipt.Result.Status != "completed" {
		t.Fatalf("Owner interaction blocked %#v %v", receipt, err)
	}
}
