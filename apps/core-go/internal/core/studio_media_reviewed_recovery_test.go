package core

import (
	"context"
	"fmt"
	"go.temporal.io/sdk/testsuite"
	"strings"
	"testing"
)

// Ready assets are controlled persisted crash-recovery fixtures. No renderer,
// model, object-store fetch or real generated pixels are claimed by this test.
func studioReviewedReadyFixture(t *testing.T, suffix string) (context.Context, *PostgresRepository, *App, string, map[string]any, string, string, map[string]any) {
	t.Helper()
	ctx, repo, app, owner, _, _ := setupDirectPublicationToolTest(t, "studio-review-"+suffix)
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES($1,'controlled-hash','controlled-revision')`, owner); err != nil {
		t.Fatal(err)
	}
	resource, err := app.DispatchOwnerSurface(ctx, "POST", "/internal/studio/resources", nil, owner, map[string]any{"kind": "place", "name": "隔离场景", "body": map[string]any{"description": "fictional empty room"}, "reason": "fixture", "idempotency_key": "resource"})
	if err != nil {
		t.Fatal(err)
	}
	job, err := app.DispatchStudioJobSurface(ctx, "POST", "/internal/studio/jobs", nil, owner, map[string]any{"resource_id": resource["id"], "resource_revision": resource["revision"], "kind": "image_preview", "name": "隔离场景", "body": map[string]any{"description": "fictional empty room"}, "instruction": "empty fictional room", "reason": "fixture", "idempotency_key": "job"})
	if err != nil {
		t.Fatal(err)
	}
	id := stringValue(job["id"])
	mid := "studio_media_" + stableDigest(id)
	workflow := "original-studio-media"
	output := map[string]any{"media_intent_id": mid, "workflow_id": workflow}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.studio_jobs SET status='running',result=$2 WHERE id=$1`, id, jsonBytes(output)); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.studio_job_attempts SET status='running',output=$2 WHERE job_id=$1`, id, jsonBytes(output)); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.media_intents(id,media_scope,studio_owner_actor_id,studio_job_id,kind,mime_type,prompt,provider_request_id,workflow_id,status,renderer_submission_started,revision) VALUES($1,'studio',$2,$3,'image','image/png','{}','original-request',$4,'submission_unknown',true,3)`, mid, owner, id, workflow); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.media_assets(id,media_scope,studio_owner_actor_id,studio_job_id,version,kind,mime_type,byte_size,sha256,bucket,object_key,provider_request_id,workflow_id,status) VALUES($1,'studio',$2,$3,'v1','image','image/png',1,'controlled-digest','private-fixture','unused','original-request',$4,'ready')`, "asset_"+mid, owner, id, workflow); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload,status) VALUES('original-studio-media-intent',$1,'media','media.generation',$2,'failed')`, workflow, jsonBytes(map[string]any{"intent_id": mid})); err != nil {
		t.Fatal(err)
	}
	body := map[string]any{"expected_revision": job["revision"], "expected_attempt": 1, "expected_media_revision": 3, "submission_reviewed": true, "reason": "已人工核对原任务及现有图片归属", "idempotency_key": "resume-reviewed"}
	return ctx, repo, app, owner, job, mid, workflow, body
}

func TestStudioReviewedReadyRecoveryPreservesOriginalAttempt(t *testing.T) {
	ctx, repo, app, owner, job, mid, oldWorkflow, body := studioReviewedReadyFixture(t, "success")
	path := "/internal/studio/jobs/" + stringValue(job["id"]) + "/resume-reviewed"
	if _, err := app.ProcessMediaIntent(ctx, mid); err == nil {
		t.Fatal("uncertain submission resumed without Owner confirmation")
	}
	result, err := app.DispatchStudioJobSurface(ctx, "POST", path, nil, owner, body)
	if err != nil {
		t.Fatal(err)
	}
	if result["status"] != "running" || intValue(result["latest_attempt"]) != 1 {
		t.Fatal(result)
	}
	replay, err := app.DispatchStudioJobSurface(ctx, "POST", path, nil, owner, body)
	if err != nil || replay["id"] != result["id"] || intValue(replay["revision"]) != intValue(result["revision"]) || jsonString(replay["result"]) != jsonString(result["result"]) {
		t.Fatal(replay, err)
	}
	var mediaStatus, workflow, providerRequest string
	var fence bool
	if err := repo.Pool().QueryRow(ctx, `SELECT status,workflow_id,provider_request_id,renderer_submission_started FROM public.media_intents WHERE id=$1`, mid).Scan(&mediaStatus, &workflow, &providerRequest, &fence); err != nil {
		t.Fatal(err)
	}
	if mediaStatus != "reviewed_ready" || workflow == oldWorkflow || providerRequest != "original-request" || !fence {
		t.Fatal(mediaStatus, workflow, providerRequest, fence)
	}
	if err := app.ReconcileStudioMediaTerminal(ctx, "original-studio-media-intent", "late original failure"); err != nil {
		t.Fatal(err)
	}
	mediaResult, err := app.ProcessMediaIntent(ctx, mid)
	if err != nil || mediaResult["status"] != "completed" {
		t.Fatal(mediaResult, err)
	}
	actual, err := app.StudioJob(ctx, owner, stringValue(job["id"]))
	if err != nil || actual["status"] != "completed" || intValue(actual["latest_attempt"]) != 1 || mapValue(actual["result"])["asset_id"] != "asset_"+mid {
		t.Fatal(actual, err)
	}
	var attempts, recoveries int
	if err := repo.Pool().QueryRow(ctx, `SELECT (SELECT count(*) FROM public.studio_job_attempts WHERE job_id=$1),(SELECT count(*) FROM public.platform_workflow_intents WHERE intent_type='media.generation' AND payload->>'intent_id'=$2)`, job["id"], mid).Scan(&attempts, &recoveries); err != nil {
		t.Fatal(err)
	}
	if attempts != 1 || recoveries != 2 {
		t.Fatal(attempts, recoveries)
	}
	if err := app.AuthorizeAsset(ctx, owner, "asset_"+mid); err != nil {
		t.Fatal(err)
	}
	if err := app.AuthorizeAsset(ctx, owner+"-foreign", "asset_"+mid); err == nil {
		t.Fatal("foreign owner read recovered asset")
	}
	if err := repo.Pool().QueryRow(ctx, `SELECT status FROM public.media_intents WHERE id=$1`, mid).Scan(&mediaStatus); err != nil || mediaStatus != "completed" {
		t.Fatal(mediaStatus, err)
	}
}

func TestStudioReviewedReadyRecoveryRejectsUnverifiedOrStaleState(t *testing.T) {
	cases := []struct {
		name, statement string
		changeBody      func(map[string]any)
	}{
		{name: "unreviewed", changeBody: func(body map[string]any) { body["submission_reviewed"] = false }},
		{name: "stale-job", changeBody: func(body map[string]any) { body["expected_revision"] = 99 }},
		{name: "stale-media", changeBody: func(body map[string]any) { body["expected_media_revision"] = 99 }},
		{name: "old-attempt", changeBody: func(body map[string]any) { body["expected_attempt"] = 2 }},
		{name: "missing-ready", statement: `UPDATE public.media_assets SET status='failed'`},
		{name: "wrong-request", statement: `UPDATE public.media_assets SET provider_request_id='foreign-request'`},
		{name: "wrong-workflow", statement: `UPDATE public.media_assets SET workflow_id='foreign-workflow'`},
		{name: "quality-reject", statement: `UPDATE public.media_intents SET quality_verdict='reject'`},
		{name: "quality-retry", statement: `UPDATE public.media_intents SET quality_verdict='retry'`},
		{name: "quality-legacy", statement: `UPDATE public.media_intents SET quality_verdict='retry_accepted'`},
		{name: "active", statement: `UPDATE public.platform_workflow_intents SET status='started' WHERE intent_id='original-studio-media-intent'`},
		{name: "paused", statement: `UPDATE public.platform_workflow_intents SET status='paused' WHERE intent_id='original-studio-media-intent'`},
		{name: "cancel-requested", statement: `UPDATE public.platform_workflow_intents SET status='cancel_requested' WHERE intent_id='original-studio-media-intent'`},
		{name: "discarded", statement: `UPDATE public.studio_jobs SET status='discarded'`},
		{name: "unbound-attempt", statement: `UPDATE public.studio_job_attempts SET output='{}'`},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			ctx, repo, app, owner, job, mid, workflow, body := studioReviewedReadyFixture(t, test.name)
			if test.statement != "" {
				if _, err := repo.Pool().Exec(ctx, test.statement); err != nil {
					t.Fatal(err)
				}
			}
			if test.changeBody != nil {
				test.changeBody(body)
			}
			if _, err := app.DispatchStudioJobSurface(ctx, "POST", "/internal/studio/jobs/"+stringValue(job["id"])+"/resume-reviewed", nil, owner, body); err == nil {
				t.Fatal("unsafe recovery admitted")
			}
			var status, currentWorkflow string
			var workflows int
			if err := repo.Pool().QueryRow(ctx, `SELECT status,workflow_id,(SELECT count(*) FROM public.platform_workflow_intents WHERE intent_type='media.generation' AND payload->>'intent_id'=$1) FROM public.media_intents WHERE id=$1`, mid).Scan(&status, &currentWorkflow, &workflows); err != nil {
				t.Fatal(err)
			}
			if status != "submission_unknown" || currentWorkflow != workflow || workflows != 1 {
				t.Fatal("rejected command changed original media", status, currentWorkflow, workflows)
			}
		})
	}
}

func TestStudioReviewedReadyRecoveryLegacyAndExecutionLock(t *testing.T) {
	ctx, repo, app, owner, job, mid, _, body := studioReviewedReadyFixture(t, "legacy")
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.media_intents SET status='failed' WHERE id=$1`, mid); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.studio_jobs SET status='failed' WHERE id=$1`, job["id"]); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.studio_job_attempts SET status='failed',finished_at=now() WHERE job_id=$1`, job["id"]); err != nil {
		t.Fatal(err)
	}
	path := "/internal/studio/jobs/" + stringValue(job["id"]) + "/resume-reviewed"
	if _, err := app.DispatchStudioJobSurface(ctx, "POST", path, nil, owner+"-foreign", body); err == nil {
		t.Fatal("foreign owner recovered job")
	}
	release, acquired, err := app.tryDailyReviewExecutionLock(ctx, "media-intent", mid)
	if err != nil || !acquired {
		t.Fatal(acquired, err)
	}
	_, lockedErr := app.DispatchStudioJobSurface(ctx, "POST", path, nil, owner, body)
	release()
	if lockedErr == nil {
		t.Fatal("active media execution admitted")
	}
	if _, err := app.DispatchStudioJobSurface(ctx, "POST", path, nil, owner, body); err != nil {
		t.Fatal("legacy original attempt was not recoverable", err)
	}
	if _, err := app.ProcessMediaIntent(ctx, mid); err != nil {
		t.Fatal(err)
	}
	actual, err := app.StudioJob(ctx, owner, stringValue(job["id"]))
	if err != nil || actual["status"] != "completed" || intValue(actual["latest_attempt"]) != 1 {
		t.Fatal(actual, err)
	}
}

func TestStudioReviewedReadyRecoveryConcurrentCommands(t *testing.T) {
	ctx, repo, app, owner, job, mid, _, body := studioReviewedReadyFixture(t, "concurrent")
	results := make(chan error, 2)
	for _, key := range []string{"first-command", "second-command"} {
		input := cloneMap(body)
		input["idempotency_key"] = key
		go func() {
			_, err := app.DispatchStudioJobSurface(ctx, "POST", "/internal/studio/jobs/"+stringValue(job["id"])+"/resume-reviewed", nil, owner, input)
			results <- err
		}()
	}
	successes := 0
	for i := 0; i < 2; i++ {
		if <-results == nil {
			successes++
		}
	}
	var workflows int
	if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.platform_workflow_intents WHERE intent_type='media.generation' AND payload->>'intent_id'=$1`, mid).Scan(&workflows); err != nil {
		t.Fatal(err)
	}
	if successes != 1 || workflows != 2 {
		t.Fatal("concurrent recovery duplicated admission", successes, workflows)
	}
}

func TestStudioReviewedReadyRecoveryPreservesLateQualityFailure(t *testing.T) {
	for _, verdict := range []string{"reject", "retry", "retry_accepted"} {
		t.Run(verdict, func(t *testing.T) {
			ctx, repo, app, owner, job, mid, _, body := studioReviewedReadyFixture(t, "late-quality-"+verdict)
			if _, err := app.DispatchStudioJobSurface(ctx, "POST", "/internal/studio/jobs/"+stringValue(job["id"])+"/resume-reviewed", nil, owner, body); err != nil {
				t.Fatal(err)
			}
			if _, err := repo.Pool().Exec(ctx, `UPDATE public.media_intents SET quality_verdict=$2 WHERE id=$1`, mid, verdict); err != nil {
				t.Fatal(err)
			}
			if result, err := app.ProcessMediaIntent(ctx, mid); err == nil {
				t.Fatal("late quality failure completed", result)
			}
			actual, err := app.StudioJob(ctx, owner, stringValue(job["id"]))
			if err != nil || actual["status"] == "completed" {
				t.Fatal(actual, err)
			}
			var preserved, assetStatus string
			if err := repo.Pool().QueryRow(ctx, `SELECT i.quality_verdict,a.status FROM public.media_intents i JOIN public.media_assets a ON a.id='asset_'||i.id WHERE i.id=$1`, mid).Scan(&preserved, &assetStatus); err != nil {
				t.Fatal(err)
			}
			if preserved != verdict || assetStatus != "ready" {
				t.Fatal(preserved, assetStatus)
			}
		})
	}
}

func TestStudioReviewedReadyRecoveryDiscardCannotBeRepublished(t *testing.T) {
	for _, duringSettlement := range []bool{false, true} {
		t.Run(fmt.Sprint(duringSettlement), func(t *testing.T) {
			ctx, repo, app, owner, job, mid, _, body := studioReviewedReadyFixture(t, fmt.Sprint("discard-", duringSettlement))
			path := "/internal/studio/jobs/" + stringValue(job["id"])
			if duringSettlement {
				resumed, err := app.DispatchStudioJobSurface(ctx, "POST", path+"/resume-reviewed", nil, owner, body)
				if err != nil {
					t.Fatal(err)
				}
				job = resumed
			}
			done := make(chan error, 1)
			go func() {
				if duringSettlement {
					_, err := app.ProcessMediaIntent(ctx, mid)
					done <- err
					return
				}
				_, err := app.DispatchStudioJobSurface(ctx, "POST", path+"/resume-reviewed", nil, owner, body)
				done <- err
			}()
			_, discardErr := app.DispatchStudioJobSurface(ctx, "POST", path+"/discard", nil, owner, map[string]any{"expected_revision": job["revision"], "reason": "明确丢弃此原任务", "idempotency_key": "discard-original"})
			<-done
			if discardErr != nil {
				fresh, err := app.StudioJob(ctx, owner, stringValue(job["id"]))
				if err != nil {
					t.Fatal(err)
				}
				if _, err := app.DispatchStudioJobSurface(ctx, "POST", path+"/discard", nil, owner, map[string]any{"expected_revision": fresh["revision"], "reason": "已重新核对状态，明确丢弃", "idempotency_key": "discard-fresh"}); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := app.ProcessMediaIntent(ctx, mid); err != nil {
				t.Fatal(err)
			}
			actual, err := app.StudioJob(ctx, owner, stringValue(job["id"]))
			if err != nil || actual["status"] != "discarded" || len(mapValue(actual["result"])) != 0 {
				t.Fatal("discarded task was republished", actual, err)
			}
			var saved int
			if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.studio_resource_materials WHERE job_id=$1`, job["id"]).Scan(&saved); err != nil || saved != 0 {
				t.Fatal("recovery created unsolicited material", saved, err)
			}
		})
	}
}

func TestStudioReviewedReadyRecoveryRejectsStaleActivity(t *testing.T) {
	ctx, _, app, owner, job, mid, _, body := studioReviewedReadyFixture(t, "stale-activity")
	if _, err := app.DispatchStudioJobSurface(ctx, "POST", "/internal/studio/jobs/"+stringValue(job["id"])+"/resume-reviewed", nil, owner, body); err != nil {
		t.Fatal(err)
	}
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestActivityEnvironment()
	oldActivity := func(activityCtx context.Context) error {
		_, err := app.ProcessMediaIntent(activityCtx, mid)
		return err
	}
	env.RegisterActivity(oldActivity)
	_, err := env.ExecuteActivity(oldActivity)
	if err == nil || !strings.Contains(err.Error(), "media_execution_workflow_stale") {
		t.Fatal("stale activity accepted", err)
	}
	actual, err := app.StudioJob(ctx, owner, stringValue(job["id"]))
	if err != nil || actual["status"] != "running" {
		t.Fatal(actual, err)
	}
}
