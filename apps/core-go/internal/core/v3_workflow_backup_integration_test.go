package core

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"
)

type v3WorkflowProbe struct {
	WorkflowRuntime
	failSignal, unavailable bool
	signals                 []string
}

func (p *v3WorkflowProbe) Status(_ context.Context, wf, run string) (WorkflowExecution, error) {
	if p.unavailable {
		return WorkflowExecution{}, errors.New("controlled executor outage")
	}
	return WorkflowExecution{WorkflowID: wf, RunID: "actual-run", Status: "running", ControlState: "running", ControlActions: []string{"pause"}}, nil
}
func (p *v3WorkflowProbe) Signal(_ context.Context, wf, run, action, id string) error {
	p.signals = append(p.signals, wf+":"+run+":"+action+":"+id)
	if p.failSignal {
		return errors.New("controlled lost acknowledgement")
	}
	return nil
}
func (p *v3WorkflowProbe) HistoryPage(_ context.Context, wf, run string, token []byte, limit int) (WorkflowHistoryPage, error) {
	if limit != 50 {
		return WorkflowHistoryPage{}, errors.New("unexpected history bound")
	}
	page := WorkflowHistoryPage{WorkflowID: wf, RunID: run, Events: []WorkflowHistoryEvent{{ID: 2, Type: "WorkflowTaskCompleted", Resettable: true}}}
	if len(token) == 0 {
		page.NextToken = []byte("next-real-token")
	} else if string(token) != "next-real-token" {
		return WorkflowHistoryPage{}, errors.New("history token was lost")
	}
	return page, nil
}

func TestV3WorkflowPostgresUnknownCommandReplayRunFenceAndHistory(t *testing.T) {
	f := v3OwnerFixture(t, "v3-workflow")
	p := &v3WorkflowProbe{failSignal: true}
	f.app.Workflows = p
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload,status) VALUES('control-intent','control-workflow','lifecycle','platform.control',$1,'started')`, jsonBytes(map[string]any{"owner_actor_id": f.ownerID})); err != nil {
		t.Fatal(err)
	}
	body := map[string]any{"source_run_id": "actual-run", "expected_revision": 1, "reason": "Owner pauses this exact run", "idempotency_key": "original-pause"}
	stale := cloneMap(body)
	stale["source_run_id"] = "old-run"
	stale["idempotency_key"] = "stale-pause"
	_, err := f.app.OwnerWorkflowCommand(f.ctx, f.ownerID, "control-workflow", "pause", stale)
	requireV3SurfaceError(t, err, "revision_conflict")
	if len(p.signals) != 0 {
		t.Fatal("stale run reached executor")
	}
	_, err = f.app.OwnerWorkflowCommand(f.ctx, f.ownerID, "go:control-workflow", "pause", body)
	requireV3SurfaceError(t, err, "workflow_command_unconfirmed")
	var status string
	var commands int
	if err = f.repository.Pool().QueryRow(f.ctx, `SELECT status FROM public.workflow_control_commands WHERE idempotency_key='original-pause'`).Scan(&status); err != nil || status != "unknown" {
		t.Fatal("unknown receipt not durable", status, err)
	}
	// Sending a signal did not rewrite the domain intent to paused/completed.
	if err = f.repository.Pool().QueryRow(f.ctx, `SELECT status FROM public.platform_workflow_intents WHERE intent_id='control-intent'`).Scan(&status); err != nil || status != "started" {
		t.Fatal("signal acceptance rewrote domain truth", status, err)
	}
	replacement := cloneMap(body)
	replacement["idempotency_key"] = "replacement-pause"
	_, err = f.app.OwnerWorkflowCommand(f.ctx, f.ownerID, "control-workflow", "pause", replacement)
	requireV3SurfaceError(t, err, "workflow_command_unconfirmed")
	if len(p.signals) != 1 {
		t.Fatal("unknown original allowed duplicate command")
	}
	changed := cloneMap(body)
	changed["reason"] = "changed proposal"
	_, err = f.app.OwnerWorkflowCommand(f.ctx, f.ownerID, "control-workflow", "pause", changed)
	requireV3SurfaceError(t, err, "idempotency_conflict")
	p.failSignal = false
	sent, err := f.app.OwnerWorkflowCommand(f.ctx, f.ownerID, "control-workflow", "pause", body)
	if err != nil || sent["status"] != "sent" || len(p.signals) != 2 || p.signals[0] != p.signals[1] {
		t.Fatal("retry changed original identity", sent, p.signals, err)
	}
	_, err = f.app.OwnerWorkflowCommand(f.ctx, f.ownerID, "control-workflow", "pause", body)
	if err != nil || len(p.signals) != 2 {
		t.Fatal("sent replay called executor again", err)
	}
	if err = f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.workflow_control_commands`).Scan(&commands); err != nil || commands != 1 {
		t.Fatal("duplicate durable commands", commands, err)
	}
	first, err := f.app.OwnerWorkflowHistoryPage(f.ctx, f.ownerID, "control-workflow", url.Values{"runId": {"actual-run"}})
	if err != nil || first["complete"] != false || first["next_cursor"] == "" {
		t.Fatal("history page lost continuation", first, err)
	}
	cursor := stringValue(first["next_cursor"])
	second, err := f.app.OwnerWorkflowHistoryPage(f.ctx, f.ownerID, "control-workflow", url.Values{"runId": {"actual-run"}, "cursor": {cursor}})
	if err != nil || second["complete"] != true {
		t.Fatal("history did not complete", second, err)
	}
	_, err = f.app.OwnerWorkflowHistoryPage(f.ctx, f.ownerID, "control-workflow", url.Values{"runId": {"other-run"}, "cursor": {cursor}})
	if err == nil {
		t.Fatal("cursor escaped concrete run")
	}
	p.unavailable = true
	inspection, err := f.app.OwnerWorkflowInspection(f.ctx, f.ownerID, "control-workflow")
	if err != nil || inspection["runtime_state"] != "unavailable" || mapValue(inspection["intent"])["intent_id"] != "control-intent" {
		t.Fatal("executor outage hid durable intent", inspection, err)
	}
	if _, err = f.app.OwnerWorkflowInspection(f.ctx, f.foreignOwnerID, "control-workflow"); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("foreign workflow read", err)
	}
	f.requireNoModelRuns(t)
}

func TestV3BackupPostgresAdmissionReceiptRetryTokenFenceAndRestoreComparison(t *testing.T) {
	f := v3OwnerFixture(t, "v3-backup")
	body := map[string]any{"reason": "Owner requests business backup", "idempotency_key": "backup-create"}
	created, err := f.app.OwnerBackupCommand(f.ctx, f.ownerID, "", "create", body)
	if err != nil || created["status"] != "queued" {
		t.Fatal(created, err)
	}
	id := stringValue(created["id"])
	replay, err := f.app.OwnerBackupCommand(f.ctx, f.ownerID, "", "create", body)
	if err != nil || replay["id"] != id || replay["_replay"] != true {
		t.Fatal("backup replay duplicated", replay, err)
	}
	token := strings.Repeat("original-private-", 3)
	claim, err := ClaimBackupOperator(f.ctx, f.repository.Pool(), id, token)
	if err != nil || intValue(claim["attempt"]) != 1 {
		t.Fatal(claim, err)
	}
	if _, err = ClaimBackupOperator(f.ctx, f.repository.Pool(), id, token); err != nil {
		t.Fatal("same operator cannot recover original claim", err)
	}
	if _, err = ClaimBackupOperator(f.ctx, f.repository.Pool(), id, strings.Repeat("other-private", 4)); err == nil {
		t.Fatal("second operator stole running job")
	}
	if err = RecordBackupOperator(f.ctx, f.repository.Pool(), id, token, "complete", "completed", 1, map[string]any{}, map[string]any{"artifact_dir": "/private/fixture"}); err == nil {
		t.Fatal("unchecked result completed backup")
	}
	if err = RecordBackupOperator(f.ctx, f.repository.Pool(), id, token, "controlled_failure", "failed", 1, map[string]any{}, map[string]any{"artifact_dir": "/private/fixture"}); err != nil {
		t.Fatal(err)
	}
	failed, err := f.app.OwnerBackupJob(f.ctx, f.ownerID, id)
	if err != nil {
		t.Fatal(err)
	}
	retry, err := f.app.OwnerBackupCommand(f.ctx, f.ownerID, id, "retry", map[string]any{"expected_revision": failed["revision"], "reason": "Owner retries failed backup", "idempotency_key": "backup-retry"})
	if err != nil || retry["status"] != "queued" {
		t.Fatal(retry, err)
	}
	token2 := strings.Repeat("new-private-", 4)
	if _, err = ClaimBackupOperator(f.ctx, f.repository.Pool(), id, token2); err != nil {
		t.Fatal(err)
	}
	verified := map[string]any{"artifacts_verified": true, "database_counts_verified": true, "media_verified": true, "source_inventory_verified": true, "restorable": true, "private_path": "must-not-leak"}
	private := map[string]any{"artifact_dir": "/private/fixture", "private_secret": "must-not-leak"}
	if err = RecordBackupOperator(f.ctx, f.repository.Pool(), id, token, "complete", "completed", 1, verified, private); err == nil {
		t.Fatal("old operator overwrote new attempt")
	}
	if err = RecordBackupOperator(f.ctx, f.repository.Pool(), id, token2, "complete", "completed", 1, verified, private); err == nil {
		t.Fatal("old attempt wrote with current token")
	}
	if err = RecordBackupOperator(f.ctx, f.repository.Pool(), id, token2, "complete", "completed", 2, verified, private); err != nil {
		t.Fatal(err)
	}
	saved, err := f.app.OwnerBackupJob(f.ctx, f.ownerID, id)
	if err != nil || saved["status"] != "completed" || strings.Contains(jsonString(saved), "must-not-leak") || strings.Contains(jsonString(saved), "/private/fixture") || strings.Contains(jsonString(saved), token2) {
		t.Fatal("public receipt leaked private execution data", saved, err)
	}
	restore, err := f.app.OwnerBackupCommand(f.ctx, f.ownerID, id, "restore", map[string]any{"expected_revision": saved["revision"], "reason": "Owner verifies isolated recovery", "idempotency_key": "restore-create", "confirm": true, "target_name": "fluctlight_restore_test0001"})
	if err != nil {
		t.Fatal(err)
	}
	restoreID := stringValue(restore["id"])
	restoreToken := strings.Repeat("restore-private-", 3)
	claimed, err := ClaimBackupOperator(f.ctx, f.repository.Pool(), restoreID, restoreToken)
	if err != nil || mapValue(claimed["source"])["artifact_dir"] != "/private/fixture" {
		t.Fatal("restore lost private source", claimed, err)
	}
	if err = RecordBackupOperator(f.ctx, f.repository.Pool(), restoreID, restoreToken, "unchecked_restore", "completed", 1, verified, private); err == nil {
		t.Fatal("restore without source comparison falsely completed")
	}
	verified["restore_comparison_verified"], verified["models_started"] = true, false
	if err = RecordBackupOperator(f.ctx, f.repository.Pool(), restoreID, restoreToken, "isolated_verified", "completed", 1, verified, private); err != nil {
		t.Fatal(err)
	}
	parent, err := f.app.OwnerBackupJob(f.ctx, f.ownerID, id)
	if err != nil || mapValue(parent["result"])["restore_comparison_verified"] != true || mapValue(parent["result"])["last_restore_job_id"] != restoreID || intValue(parent["revision"]) <= intValue(saved["revision"]) {
		t.Fatal("real restore receipt did not update source comparison", parent, err)
	}
	if _, err = f.app.OwnerBackupJob(f.ctx, f.foreignOwnerID, id); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("foreign Owner read backup", err)
	}
	f.requireNoModelRuns(t)
}
