package core

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type ownerWorkflowIntent struct {
	ID, Workflow, Queue, Kind, Status, LastError string
	Payload                                      []byte
	Revision                                     int64
}

func (a *App) ownerWorkflowIntent(ctx context.Context, owner, wf string) (ownerWorkflowIntent, error) {
	var i ownerWorkflowIntent
	err := a.DB.Pool().QueryRow(ctx, `SELECT intent_id,workflow_id,task_queue,intent_type,status,payload,intent_revision,COALESCE(last_error,'') FROM public.platform_workflow_intents WHERE workflow_id=$1 OR 'go:'||workflow_id=$1 ORDER BY CASE WHEN workflow_id=$1 THEN 0 ELSE 1 END LIMIT 1`, wf).Scan(&i.ID, &i.Workflow, &i.Queue, &i.Kind, &i.Status, &i.Payload, &i.Revision, &i.LastError)
	if errors.Is(err, pgx.ErrNoRows) {
		return i, ErrNotFound
	}
	if err != nil {
		return i, err
	}
	input := decodeObject(i.Payload)
	if fid := stringValue(input["fluctlight_id"]); fid != "" {
		if _, err := a.DB.GetFluctlight(ctx, fid, owner); err != nil {
			return i, err
		}
	}
	if actor := stringValue(input["owner_actor_id"]); actor != "" && actor != owner {
		return i, ErrUnauthorized
	}
	return i, nil
}

func workflowControlActions(kind string, execution WorkflowExecution) []string {
	status := execution.Status
	result := []string{}
	switch status {
	case "running", "paused":
		result = append(result, "cancel")
		if execution.ControlState == "paused" && containsString(execution.ControlActions, "resume") {
			result = append(result, "resume")
		} else if execution.ControlState == "running" && containsString(execution.ControlActions, "pause") {
			result = append(result, "pause")
		}
	}
	// Domain attempts own their checkpoints, CAS, staged results and fences.
	// Generic restart/reset is deliberately limited to the runtime control probe.
	if kind == "platform.control" {
		switch status {
		case "completed", "failed", "canceled", "cancelled", "terminated", "timed_out":
			result = append(result, "restart", "reset")
		case "running", "paused":
			result = append(result, "reset")
		}
	}
	return result
}
func workflowRecovery(kind string, input map[string]any) map[string]any {
	fid := stringValue(input["fluctlight_id"])
	root := "/c/" + url.PathEscape(fid)
	link := func(href, label string) map[string]any {
		return map[string]any{"href": href, "label": label, "mode": "domain_recovery", "reason": "领域恢复保留原结果、attempt、版本和副作用围栏；工作流命令不代表业务已成功"}
	}
	switch kind {
	case "schedule.current_day":
		return link(root+"/life/schedule?recoverInitial=1", "恢复原日程任务")
	case "studio.generate":
		if id := stringValue(input["studio_job_id"]); id != "" {
			return link("/studio/preview?res="+url.QueryEscape(stringValue(input["studio_resource_id"]))+"&job="+url.QueryEscape(id), "查看创作任务及原结果恢复")
		}
	case "diary.generate":
		return link(root+"/life/diary", "查看日记及失败恢复")
	case "media.delete":
		if id := stringValue(input["asset_id"]); id != "" {
			return link("/studio/media/"+url.PathEscape(id), "查看素材删除及原任务恢复")
		}
	case "media.generation":
		if id := stringValue(input["intent_id"]); id != "" {
			return link("/diag/media-prompts?q="+url.QueryEscape(id), "查看媒体原任务和恢复")
		}
	case "persona.owner_switch":
		return link(root+"/profile", "查看人格切换请求与实际结果")
	case "persona.compile":
		return link(root+"/profile", "查看人格资料整理及恢复")
	case "goal.plan", "goal.evaluate", "intention.trigger":
		return link(root+"/life/goals", "查看目标、约定和原请求恢复")
	case "visual_identity.initialize":
		return link(root+"/media/identity/edit", "查看视觉会话与实际候选")
	case "reflection.run":
		return link(root+"/growth/reflection", "查看反思请求与结算")
	case "cognition.processing":
		if id := stringValue(input["request_id"]); id != "" {
			return link("/diag/request/"+url.PathEscape(id), "查看原请求与执行结果")
		}
	}
	return map[string]any{"mode": "original_source", "href": "/diag/task/" + url.PathEscape(stringValue(input["task_intent_id"])), "label": "查看原任务及业务来源", "reason": "从原业务请求核对结果；不会通过通用重启绕过已提交的领域终态"}
}

// PostgreSQL intent, command receipts and Runtime are independent panels. A
// Runtime outage never hides a persisted task or changes its business state.
func (a *App) OwnerWorkflowInspection(ctx context.Context, owner, wf string) (map[string]any, error) {
	if err := a.requireOwner(ctx, owner); err != nil {
		return nil, err
	}
	i, err := a.ownerWorkflowIntent(ctx, owner, wf)
	if err != nil {
		return nil, err
	}
	input := decodeObject(i.Payload)
	input["task_intent_id"] = i.ID
	if i.Kind == "studio.generate" {
		var resource string
		if err := a.DB.Pool().QueryRow(ctx, `SELECT resource_id FROM public.studio_jobs WHERE id=$1 AND owner_actor_id=$2`, input["studio_job_id"], owner).Scan(&resource); err != nil {
			return nil, err
		}
		input["studio_resource_id"] = resource
	}
	result := map[string]any{"intent": map[string]any{"intent_id": i.ID, "workflow_id": i.Workflow, "intent_type": i.Kind, "status": i.Status, "last_error": boundedLifecycleCause(i.LastError), "revision": i.Revision, "task_queue": i.Queue, "payload": redactDiagnostic(input)}, "recovery": workflowRecovery(i.Kind, input), "runtime": nil, "runtime_state": "unavailable", "allowed_actions": []string{}, "control_semantics": "暂停在已有检查点生效，不中断正在运行的Activity；取消请求也不撤销已提交副作用；Runtime终态和业务结果独立核对"}
	commands, err := readWorldObjects(ctx, a.DB.Pool(), `SELECT jsonb_build_object('id',id,'source_run_id',source_run_id,'intent_revision',intent_revision,'action',action,'reason',reason,'status',status,'result',result,'created_at',created_at,'updated_at',updated_at) FROM public.workflow_control_commands WHERE owner_actor_id=$1 AND workflow_id=$2 ORDER BY created_at DESC,id DESC LIMIT 51`, owner, i.Workflow)
	if err != nil {
		return nil, err
	}
	result["commands_has_more"] = len(commands) > 50
	if len(commands) > 50 {
		commands = commands[:50]
	}
	result["commands"] = commands
	if a.Workflows == nil {
		return result, nil
	}
	runtimeCtx, cancel := context.WithTimeout(ctx, workflowListTimeout)
	defer cancel()
	execution, err := a.Workflows.Status(runtimeCtx, i.Workflow, "")
	if err != nil {
		result["runtime_error"] = map[string]any{"code": "workflow_runtime_unavailable", "message": "执行器状态读取失败；数据库任务与原命令保留，请重新查询"}
		return result, nil
	}
	result["runtime_state"] = "available"
	result["runtime"] = workflowExecutionMap(execution)
	result["allowed_actions"] = workflowControlActions(i.Kind, execution)
	return result, nil
}

type workflowHistoryCursor struct {
	Owner, Workflow, Run string
	Token                []byte
}

func (a *App) OwnerWorkflowHistoryPage(ctx context.Context, owner, wf string, q url.Values) (map[string]any, error) {
	if err := a.requireOwner(ctx, owner); err != nil {
		return nil, err
	}
	i, err := a.ownerWorkflowIntent(ctx, owner, wf)
	if err != nil {
		return nil, err
	}
	pager, ok := a.Workflows.(WorkflowHistoryPager)
	if !ok {
		return nil, ErrWorkflowRuntime
	}
	run := strings.TrimSpace(q.Get("runId"))
	if run == "" || len(run) > 256 {
		return nil, invalid("读取历史必须明确本次观察的Run ID")
	}
	var cursor workflowHistoryCursor
	if raw := q.Get("cursor"); raw != "" {
		if len(raw) > 65536 {
			return nil, ErrDiagnosticsFilterInvalid
		}
		b, e := base64.RawURLEncoding.DecodeString(raw)
		if e != nil || json.Unmarshal(b, &cursor) != nil || cursor.Owner != owner || cursor.Workflow != i.Workflow || cursor.Run != run {
			return nil, ErrDiagnosticsFilterInvalid
		}
	}
	runtimeCtx, cancel := context.WithTimeout(ctx, workflowListTimeout)
	defer cancel()
	page, err := pager.HistoryPage(runtimeCtx, i.Workflow, run, cursor.Token, 50)
	if err != nil {
		return nil, ErrWorkflowRuntime
	}
	next := ""
	if len(page.NextToken) > 0 {
		next = base64.RawURLEncoding.EncodeToString(jsonBytes(workflowHistoryCursor{owner, i.Workflow, run, page.NextToken}))
	}
	return map[string]any{"workflow_id": page.WorkflowID, "run_id": page.RunID, "items": page.Events, "next_cursor": next, "complete": next == ""}, nil
}

// An external control cannot be part of a PostgreSQL transaction. Persist the
// immutable command before the RPC; serialize by workflow using a session lock.
// Retrying the same identity can safely resend Temporal request IDs. A start
// with an unknown outcome is observed by Memo instead of blindly repeated.
func (a *App) OwnerWorkflowCommand(ctx context.Context, owner, wf, action string, body map[string]any) (map[string]any, error) {
	if err := a.requireOwner(ctx, owner); err != nil {
		return nil, err
	}
	if !containsString([]string{"pause", "resume", "cancel", "reset", "restart"}, action) || !eventKeys(body, "source_run_id", "expected_revision", "idempotency_key", "reason", "history_point") {
		return nil, ErrInvalidArguments
	}
	run, key, reason := strings.TrimSpace(stringValue(body["source_run_id"])), strings.TrimSpace(stringValue(body["idempotency_key"])), strings.TrimSpace(stringValue(body["reason"]))
	revision, ok := positiveInt64(body["expected_revision"])
	if !ok || run == "" || len(run) > 256 || key == "" || len(key) > 256 || reason == "" || len([]rune(reason)) > 1000 {
		return nil, invalid("具体Run、任务版本、原命令身份和理由均为必需")
	}
	point, validPoint := positiveInt64(body["history_point"])
	if action == "reset" && !validPoint {
		return nil, invalid("请选择真实历史中已完成的Workflow Task作为重置点")
	}
	if action != "reset" && body["history_point"] != nil {
		return nil, ErrInvalidArguments
	}
	i, err := a.ownerWorkflowIntent(ctx, owner, wf)
	if err != nil {
		return nil, err
	}
	if a.Workflows == nil {
		return nil, ErrWorkflowRuntime
	}
	conn, err := a.DB.Pool().Acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Release()
	var locked bool
	if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock(hashtextextended($1,0))`, "workflow-control:"+i.Workflow).Scan(&locked); err != nil {
		return nil, err
	}
	if !locked {
		return nil, &SurfaceError{Status: 409, Code: "workflow_command_busy", Message: "同一工作流的原命令仍在处理，请稍后核对"}
	}
	defer func() {
		unlockCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer cancel()
		if _, e := conn.Exec(unlockCtx, `SELECT pg_advisory_unlock(hashtextextended($1,0))`, "workflow-control:"+i.Workflow); e != nil {
			_ = conn.Conn().Close(unlockCtx)
		}
	}()
	digest := stableDigest(i.Workflow + ":" + action + ":" + jsonString(body))
	id, state, savedDigest := "", "", ""
	var saved []byte
	err = conn.QueryRow(ctx, `SELECT id,status,digest,result FROM public.workflow_control_commands WHERE owner_actor_id=$1 AND idempotency_key=$2`, owner, key).Scan(&id, &state, &savedDigest, &saved)
	fresh := errors.Is(err, pgx.ErrNoRows)
	if err != nil && !fresh {
		return nil, err
	}
	if !fresh {
		if savedDigest != digest {
			return nil, &SurfaceError{Status: 409, Code: "idempotency_conflict", Message: "该命令身份已属于其他输入"}
		}
		if state == "sent" {
			return decodeObject(saved), nil
		}
		if state == "rejected" {
			return nil, &SurfaceError{Status: 422, Code: "workflow_command_rejected", Message: "执行器明确未接受原命令，请核对实际状态后再决定操作"}
		}
	}
	rpcCtx, cancel := context.WithTimeout(ctx, workflowListTimeout)
	defer cancel()
	if fresh {
		execution, e := a.Workflows.Status(rpcCtx, i.Workflow, "")
		if e != nil {
			return nil, ErrWorkflowRuntime
		}
		if execution.RunID != run {
			return nil, &SurfaceError{Status: 409, Code: "revision_conflict", Message: "工作流已经切换Run，请核对新执行后再决定操作"}
		}
		if !containsString(workflowControlActions(i.Kind, execution), action) {
			return nil, &SurfaceError{Status: 422, Code: "workflow_domain_recovery_required", Message: "该操作应通过详情中的领域恢复入口执行，不能绕过领域attempt和结果围栏"}
		}
		id = randomID("workflow_command_")
		err = workflowControlTransaction(ctx, conn, func(tx pgx.Tx) error {
			var current int64
			if err := tx.QueryRow(ctx, `SELECT intent_revision FROM public.platform_workflow_intents WHERE intent_id=$1 FOR UPDATE`, i.ID).Scan(&current); err != nil {
				return err
			}
			if current != revision {
				return &SurfaceError{Status: 409, Code: "revision_conflict", Message: "任务版本已变化，请核对实际状态"}
			}
			var pending bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.workflow_control_commands WHERE workflow_id=$1 AND status IN ('prepared','unknown'))`, i.Workflow).Scan(&pending); err != nil {
				return err
			}
			if pending {
				return &SurfaceError{Status: 409, Code: "workflow_command_unconfirmed", Message: "原命令结果尚未确认，请先用原命令身份重试或查询"}
			}
			if _, err := tx.Exec(ctx, `INSERT INTO public.workflow_control_commands(id,owner_actor_id,idempotency_key,digest,workflow_id,intent_id,source_run_id,intent_revision,action,reason,input,status) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,'prepared')`, id, owner, key, digest, i.Workflow, i.ID, run, revision, action, reason, jsonBytes(body)); err != nil {
				return err
			}
			if action == "restart" {
				input := decodeObject(i.Payload)
				input["intent_id"] = "owner_restart_intent:" + id
				input["owner_actor_id"] = owner
				input["source_workflow_id"] = i.Workflow
				input["source_control_command_id"] = id
				if _, err := tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload,status) VALUES($1,$2,$3,$4,$5,'manual_prepared')`, "owner_restart_intent:"+id, "owner-restart:"+id, i.Queue, i.Kind, jsonBytes(input)); err != nil {
					return err
				}
			}
			_, err := tx.Exec(ctx, `INSERT INTO public.platform_workflow_management_audit(id,action,workflow_id,actor_id,authorized,details) VALUES($1,$2,$3,$4,'true',$5)`, randomID("workflow_audit_"), action, i.Workflow, owner, jsonBytes(map[string]any{"command_id": id, "source_run_id": run, "intent_revision": revision, "reason": reason, "phase": "prepared"}))
			return err
		})
		if err != nil {
			return nil, err
		}
	}
	var execution WorkflowExecution
	switch action {
	case "pause", "resume":
		err = a.Workflows.Signal(rpcCtx, i.Workflow, run, action, id)
	case "cancel":
		err = a.Workflows.Cancel(rpcCtx, i.Workflow, run, id)
	case "reset":
		execution, err = a.Workflows.Reset(rpcCtx, i.Workflow, run, point, reason, id)
	case "restart":
		// A deterministic successor workflow rejects duplicate IDs even when it has
		// already closed. Thus prepared-before-RPC and lost-response retries are safe.
		input := decodeObject(i.Payload)
		input["intent_id"] = "owner_restart_intent:" + id
		input["owner_actor_id"] = owner
		input["source_workflow_id"] = i.Workflow
		input["source_control_command_id"] = id
		execution, err = a.Workflows.Restart(rpcCtx, WorkflowStart{WorkflowID: "owner-restart:" + id, TaskQueue: i.Queue, IntentType: i.Kind, Payload: jsonBytes(input), OwnerCommandID: id})
	}
	result := map[string]any{"command_id": id, "workflow_id": i.Workflow, "source_run_id": run, "operation": action, "status": "sent", "effect": "request_sent; observe runtime and domain outcome independently"}
	if execution.RunID != "" {
		result["execution"] = workflowExecutionMap(execution)
		if action == "restart" {
			result["successor_intent_id"] = "owner_restart_intent:" + id
		}
	}
	resultState := "sent"
	if err != nil {
		resultState = "unknown"
		if errors.Is(err, ErrWorkflowCommandRejected) {
			resultState = "rejected"
		}
		result["status"] = resultState
		result["error_code"] = "workflow_command_" + resultState
	}
	saveCtx, saveCancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer saveCancel()
	saveErr := workflowControlTransaction(saveCtx, conn, func(tx pgx.Tx) error {
		if action == "restart" && resultState == "rejected" {
			if _, e := tx.Exec(saveCtx, `UPDATE public.platform_workflow_intents SET status='failed',last_error='owner_start_rejected',completed_at=now() WHERE intent_id=$1 AND status='manual_prepared'`, "owner_restart_intent:"+id); e != nil {
				return e
			}
		}
		if action == "restart" && err == nil {
			if _, e := tx.Exec(saveCtx, `UPDATE public.platform_workflow_intents SET status='started',started_at=COALESCE(started_at,now()),attempt_count=1 WHERE intent_id=$1 AND status='manual_prepared'`, "owner_restart_intent:"+id); e != nil {
				return e
			}
		}
		if _, err := tx.Exec(saveCtx, `UPDATE public.workflow_control_commands SET status=$2,result=$3,updated_at=now() WHERE id=$1`, id, resultState, jsonBytes(result)); err != nil {
			return err
		}
		_, err := tx.Exec(saveCtx, `INSERT INTO public.platform_workflow_management_audit(id,action,workflow_id,actor_id,authorized,details) VALUES($1,$2,$3,$4,'true',$5)`, randomID("workflow_audit_"), action, i.Workflow, owner, jsonBytes(result))
		return err
	})
	if saveErr != nil {
		return nil, saveErr
	}
	if errors.Is(err, ErrWorkflowCommandRejected) {
		return nil, &SurfaceError{Status: 422, Code: "workflow_command_rejected", Message: "执行器明确拒绝原命令，没有宣称生效；请核对实际状态", Details: map[string]any{"command_id": id}}
	}
	if err != nil {
		return nil, &SurfaceError{Status: 503, Code: "workflow_command_unconfirmed", Message: "原命令结果尚未确认；保留原身份与输入后重试，不得创建替代命令", Details: map[string]any{"command_id": id, "source_run_id": run}}
	}
	return result, nil
}

func matchWorkflowControls(method, path string) bool {
	p := surfaceParts(path)
	return len(p) == 5 && p[0] == "workflow-controls" && p[2] == "commands" && p[4] == "retry" && method == "POST" || len(p) == 2 && p[0] == "workflow-controls" && method == "GET" || len(p) == 3 && p[0] == "workflow-controls" && (method == "GET" && p[2] == "history" || method == "POST" && containsString([]string{"pause", "resume", "cancel", "restart", "reset"}, p[2]))
}
func (a *App) dispatchWorkflowControls(ctx context.Context, method, path string, q url.Values, owner string, body map[string]any) (map[string]any, error) {
	p := surfaceParts(path)
	if method == "POST" && len(p) == 5 {
		if err := a.requireOwner(ctx, owner); err != nil {
			return nil, err
		}
		i, err := a.ownerWorkflowIntent(ctx, owner, p[1])
		if err != nil {
			return nil, err
		}
		var action string
		var raw []byte
		if err := a.DB.Pool().QueryRow(ctx, `SELECT action,input FROM public.workflow_control_commands WHERE id=$1 AND owner_actor_id=$2 AND workflow_id=$3`, p[3], owner, i.Workflow).Scan(&action, &raw); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, ErrNotFound
			}
			return nil, err
		}
		return a.OwnerWorkflowCommand(ctx, owner, p[1], action, decodeObject(raw))
	}
	if method == "POST" {
		return a.OwnerWorkflowCommand(ctx, owner, p[1], p[2], body)
	}
	if len(p) == 3 {
		return a.OwnerWorkflowHistoryPage(ctx, owner, p[1], q)
	}
	return a.OwnerWorkflowInspection(ctx, owner, p[1])
}

func workflowControlTransaction(ctx context.Context, db interface {
	Begin(context.Context) (pgx.Tx, error)
}, apply func(pgx.Tx) error) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.WithoutCancel(ctx))
	if err = apply(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
