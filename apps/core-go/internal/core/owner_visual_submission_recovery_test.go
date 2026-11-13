package core

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
)

type visualSubmissionProbe struct {
	WorkflowRuntime
	mu                               sync.Mutex
	status, run                      string
	unavailable, lostAcknowledgement bool
	changeRunAfterFirst              bool
	statusCalls                      int
	requests                         []string
	effects                          map[string]bool
}

func (p *visualSubmissionProbe) Status(_ context.Context, wf, run string) (WorkflowExecution, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.statusCalls++
	if p.changeRunAfterFirst && p.statusCalls > 1 {
		return WorkflowExecution{WorkflowID: wf, RunID: "newer-run", Status: p.status, ControlState: "paused", ControlActions: []string{"resume"}}, nil
	}
	if p.unavailable {
		return WorkflowExecution{}, errors.New("controlled runtime unavailable")
	}
	return WorkflowExecution{WorkflowID: wf, RunID: p.run, Status: p.status, ControlState: "paused", ControlActions: []string{"resume", "cancel"}}, nil
}
func (p *visualSubmissionProbe) Signal(_ context.Context, wf, run, action, id string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.requests = append(p.requests, wf+":"+run+":"+action+":"+id)
	if p.effects == nil {
		p.effects = map[string]bool{}
	}
	p.effects[id] = true
	if p.lostAcknowledgement {
		return errors.New("controlled resume acknowledgement lost")
	}
	return nil
}

// Local ready rows and runtime acknowledgements are controlled fixtures. This
// proves formal Core state transitions, without claiming generated pixels or
// a live Temporal service or model validation.
func visualSubmissionFixture(t *testing.T, stage string) (visualIdentityToolFixture, *visualSubmissionProbe, string, map[string]any) {
	t.Helper()
	f := newVisualIdentityToolFixture(t)
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES($1,'controlled-hash','controlled-revision')`, f.ownerID); err != nil {
		t.Fatal(err)
	}
	generated := f.generateCandidate(t, "reviewed-local-candidate")
	mid := stringValue(mapValue(generated.Result.Output)["media_intent_id"])
	f.makeCurrentCandidateReady(t, mid)
	if stage == "character_sheet" {
		reviewed, err := f.app.ExecuteTool(f.ctx, f.request(visualIdentityCommitReviewCapabilityName, "reviewed-local-canonical", visualIdentityAcceptedReview()))
		if err != nil || reviewed.Result.Status != "accepted" {
			t.Fatal(reviewed, err)
		}
		mid = stringValue(mapValue(reviewed.Result.Output)["character_sheet_media_intent_id"])
		f.makeCurrentCandidateReady(t, mid)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.media_intents SET status='submission_unknown',renderer_submission_started=true,provider_job_id=NULL,revision=3 WHERE id=$1`, mid); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.platform_workflow_intents SET status='failed' WHERE intent_type='media.generation' AND payload->>'intent_id'=$1`, mid); err != nil {
		t.Fatal(err)
	}
	var workflow string
	var visualRevision int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT s.workflow_id,v.current_revision FROM public.fluctlight_visual_identity_sessions s JOIN public.fluctlight_visual_identities v ON v.id=s.visual_identity_id WHERE s.id=$1`, f.sessionID).Scan(&workflow, &visualRevision); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.platform_workflow_intents SET status='started' WHERE workflow_id=$1 AND intent_type='visual_identity.initialize'`, workflow); err != nil {
		t.Fatal(err)
	}
	var workflowRevision int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT intent_revision FROM public.platform_workflow_intents WHERE workflow_id=$1 AND intent_type='visual_identity.initialize'`, workflow).Scan(&workflowRevision); err != nil {
		t.Fatal(err)
	}
	probe := &visualSubmissionProbe{status: "running", run: "original-run"}
	f.app.Workflows = probe
	body := map[string]any{"stage": stage, "expected_attempt": 1, "expected_visual_revision": visualRevision, "expected_media_revision": 3, "workflow_id": workflow, "source_run_id": "original-run", "expected_workflow_revision": workflowRevision, "submission_reviewed": true, "reason": "已人工核对原生成任务及同源就绪图片", "idempotency_key": "reviewed-original-result"}
	return f, probe, mid, body
}

func TestVisualSubmissionReviewedReadyRestoresOriginalWorkflow(t *testing.T) {
	for _, stage := range []string{"candidate", "character_sheet"} {
		t.Run(stage, func(t *testing.T) {
			f, probe, mid, body := visualSubmissionFixture(t, stage)
			path := "/internal/fluctlights/" + f.fluctlightID + "/visual-sessions/" + f.sessionID + "/resume-reviewed"
			before, err := f.app.OwnerVisualIdentity(f.ctx, f.ownerID, f.fluctlightID)
			if err != nil {
				t.Fatal(err)
			}
			recovery := mapValue(mapValue(before["session"])["submission_recovery"])
			if recovery["status"] != "available" || len(arrayValue(recovery["stages"])) != 1 || mapValue(arrayValue(recovery["stages"])[0])["ready"] != true {
				t.Fatal(recovery)
			}
			result, err := f.app.DispatchOwnerSurface(f.ctx, "POST", path, nil, f.ownerID, body)
			if err != nil || result["status"] != "resume_requested" || result["workflow_id"] != body["workflow_id"] || result["source_run_id"] != "original-run" {
				t.Fatal(result, err)
			}
			replay, err := f.app.DispatchOwnerSurface(f.ctx, "POST", path, nil, f.ownerID, body)
			if err != nil || replay["command_id"] != result["command_id"] || len(probe.requests) != 1 || len(probe.effects) != 1 {
				t.Fatal(replay, probe.requests, err)
			}
			var status, workflow string
			var fence bool
			if err := f.repository.Pool().QueryRow(f.ctx, `SELECT status,workflow_id,renderer_submission_started FROM public.media_intents WHERE id=$1`, mid).Scan(&status, &workflow, &fence); err != nil || status != "completed" || !fence {
				t.Fatal(status, workflow, fence, err)
			}
			intent, err := f.app.readMediaIntent(f.ctx, mid)
			if err != nil {
				t.Fatal(err)
			}
			if err := f.app.RecordMediaActivityFailure(f.ctx, mid, intent.WorkflowID, "late original callback"); err != nil {
				t.Fatal(err)
			}
			state, err := f.app.loadVisualIdentityAgentState(f.ctx, f.sessionID)
			if err != nil {
				t.Fatal(err)
			}
			if err := classifyVisualIdentityAgentState(&state); err != nil {
				t.Fatal(err)
			}
			if stage == "candidate" && state.ActionRequired != visualIdentityCommitReviewCapabilityName {
				t.Fatal("candidate skipped normal review", state)
			}
			if stage == "character_sheet" && (state.CharacterReviewStatus != "awaiting_review" || state.ActionRequired != "") {
				t.Fatal("submission acknowledgement became quality approval", state)
			}
			var parentWorkflows, newMedia int
			if err := f.repository.Pool().QueryRow(f.ctx, `SELECT (SELECT count(*) FROM public.platform_workflow_intents WHERE intent_type='visual_identity.initialize' AND payload->>'session_id'=$1),(SELECT count(*) FROM public.media_intents WHERE id=$2)`, f.sessionID, mid).Scan(&parentWorkflows, &newMedia); err != nil || parentWorkflows != 1 || newMedia != 1 {
				t.Fatal(parentWorkflows, newMedia, err)
			}
		})
	}
}

func TestVisualSubmissionReviewedReadyKeepsUnknownResumeIdentity(t *testing.T) {
	f, probe, mid, body := visualSubmissionFixture(t, "candidate")
	probe.lostAcknowledgement = true
	path := "/internal/fluctlights/" + f.fluctlightID + "/visual-sessions/" + f.sessionID + "/resume-reviewed"
	if _, err := f.app.DispatchOwnerSurface(f.ctx, "POST", path, nil, f.ownerID, body); err == nil || !strings.Contains(err.Error(), "原图片结果已核对") {
		t.Fatal(err)
	}
	var status string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT status FROM public.media_intents WHERE id=$1`, mid).Scan(&status); err != nil || status != "completed" {
		t.Fatal(status, err)
	}
	probe.lostAcknowledgement = false
	result, err := f.app.DispatchOwnerSurface(f.ctx, "POST", path, nil, f.ownerID, body)
	if err != nil || result["status"] != "resume_requested" || len(probe.requests) != 2 || probe.requests[0] != probe.requests[1] || len(probe.effects) != 1 {
		t.Fatal(result, probe.requests, err)
	}
}

func TestVisualSubmissionReviewedReadyRejectsMissingEvidenceAndStaleSource(t *testing.T) {
	cases := []struct {
		name, statement string
		edit            func(map[string]any)
		runtime         string
	}{
		{name: "unreviewed", edit: func(body map[string]any) { body["submission_reviewed"] = false }},
		{name: "old-attempt", edit: func(body map[string]any) { body["expected_attempt"] = 2 }},
		{name: "old-visual-version", edit: func(body map[string]any) { body["expected_visual_revision"] = 99 }},
		{name: "old-media-version", edit: func(body map[string]any) { body["expected_media_revision"] = 99 }},
		{name: "old-workflow-version", edit: func(body map[string]any) { body["expected_workflow_revision"] = 99 }},
		{name: "old-run", edit: func(body map[string]any) { body["source_run_id"] = "old-run" }},
		{name: "runtime-unavailable", runtime: "unavailable"},
		{name: "closed-original-run", runtime: "completed"},
		{name: "missing-ready", statement: `UPDATE public.media_assets SET status='failed'`},
		{name: "wrong-workflow", statement: `UPDATE public.media_assets SET workflow_id='unrelated-workflow'`},
		{name: "wrong-request", statement: `UPDATE public.media_assets SET provider_request_id='unrelated-request'`},
		{name: "quality-retry", statement: `UPDATE public.media_intents SET quality_verdict='retry'`},
		{name: "quality-legacy", statement: `UPDATE public.media_intents SET quality_verdict='retry_accepted'`},
		{name: "quality-rejected", statement: `UPDATE public.media_intents SET quality_verdict='reject'`},
		{name: "active-media", statement: `UPDATE public.platform_workflow_intents SET status='started' WHERE intent_type='media.generation'`},
		{name: "paused-media", statement: `UPDATE public.platform_workflow_intents SET status='paused' WHERE intent_type='media.generation'`},
		{name: "cancelled-session", statement: `UPDATE public.fluctlight_visual_identity_sessions SET status='cancelled'`},
		{name: "inactive-session", statement: `UPDATE public.fluctlight_visual_identities SET active_session_id=NULL`},
		{name: "unknown-legacy-prompt", statement: `UPDATE public.media_intents SET prompt='not-json'`},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			f, probe, mid, body := visualSubmissionFixture(t, "candidate")
			if test.edit != nil {
				test.edit(body)
			}
			if test.statement != "" {
				if _, err := f.repository.Pool().Exec(f.ctx, test.statement); err != nil {
					t.Fatal(err)
				}
			}
			if test.runtime == "unavailable" {
				probe.unavailable = true
			} else if test.runtime != "" {
				probe.status = test.runtime
			}
			if _, err := f.app.DispatchOwnerSurface(f.ctx, "POST", "/internal/fluctlights/"+f.fluctlightID+"/visual-sessions/"+f.sessionID+"/resume-reviewed", nil, f.ownerID, body); err == nil {
				t.Fatal("unsafe recovery admitted")
			}
			var status string
			if err := f.repository.Pool().QueryRow(f.ctx, `SELECT status FROM public.media_intents WHERE id=$1`, mid).Scan(&status); err != nil || status != "submission_unknown" || len(probe.requests) != 0 {
				t.Fatal(status, probe.requests, err)
			}
			if test.name == "unknown-legacy-prompt" {
				if _, err := f.app.OwnerVisualIdentity(f.ctx, f.ownerID, f.fluctlightID); err != nil {
					t.Fatal("legacy prompt broke Owner read", err)
				}
			}
		})
	}
}

func TestVisualSubmissionReviewedReadyFencesOwnerAndMediaExecution(t *testing.T) {
	f, probe, mid, body := visualSubmissionFixture(t, "candidate")
	path := "/internal/fluctlights/" + f.fluctlightID + "/visual-sessions/" + f.sessionID + "/resume-reviewed"
	if _, err := f.app.DispatchOwnerSurface(f.ctx, "POST", path, nil, f.ownerID+"-foreign", body); err == nil {
		t.Fatal("foreign Owner recovered image")
	}
	release, acquired, err := f.app.tryDailyReviewExecutionLock(f.ctx, "media-intent", mid)
	if err != nil || !acquired {
		t.Fatal(acquired, err)
	}
	_, err = f.app.DispatchOwnerSurface(f.ctx, "POST", path, nil, f.ownerID, body)
	release()
	if err == nil || len(probe.requests) != 0 {
		t.Fatal("active execution bypassed", probe.requests, err)
	}
}

func TestVisualSubmissionReviewedReadyDoesNotResumeSupersededAttempt(t *testing.T) {
	f, probe, _, body := visualSubmissionFixture(t, "candidate")
	probe.lostAcknowledgement = true
	path := "/internal/fluctlights/" + f.fluctlightID + "/visual-sessions/" + f.sessionID + "/resume-reviewed"
	if _, err := f.app.DispatchOwnerSurface(f.ctx, "POST", path, nil, f.ownerID, body); err == nil {
		t.Fatal("lost RPC was reported confirmed")
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.fluctlight_visual_identities SET active_session_id=NULL WHERE fluctlight_id=$1`, f.fluctlightID); err != nil {
		t.Fatal(err)
	}
	probe.lostAcknowledgement = false
	if _, err := f.app.DispatchOwnerSurface(f.ctx, "POST", path, nil, f.ownerID, body); err == nil || len(probe.requests) != 1 {
		t.Fatal("superseded source reached another resume", probe.requests, err)
	}
}

func TestVisualSubmissionReviewedReadyKeepsLegacyReviewAbsenceExplicit(t *testing.T) {
	f, _, mid, body := visualSubmissionFixture(t, "character_sheet")
	if _, err := f.repository.Pool().Exec(f.ctx, `DELETE FROM public.fluctlight_visual_asset_reviews WHERE media_intent_id=$1`, mid); err != nil {
		t.Fatal(err)
	}
	result, err := f.app.DispatchOwnerSurface(f.ctx, "POST", "/internal/fluctlights/"+f.fluctlightID+"/visual-sessions/"+f.sessionID+"/resume-reviewed", nil, f.ownerID, body)
	if err != nil || result["status"] != "resume_requested" {
		t.Fatal(result, err)
	}
	var reviews int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.fluctlight_visual_asset_reviews WHERE media_intent_id=$1`, mid).Scan(&reviews); err != nil || reviews != 0 {
		t.Fatal("recovery invented legacy quality approval", reviews, err)
	}
}

func TestVisualSubmissionReviewedReadyRunChangeAfterLocalCommitStaysUnconfirmed(t *testing.T) {
	f, probe, mid, body := visualSubmissionFixture(t, "candidate")
	probe.changeRunAfterFirst = true
	path := "/internal/fluctlights/" + f.fluctlightID + "/visual-sessions/" + f.sessionID + "/resume-reviewed"
	for retry := 0; retry < 2; retry++ {
		_, err := f.app.DispatchOwnerSurface(f.ctx, "POST", path, nil, f.ownerID, body)
		var surface *SurfaceError
		if !errors.As(err, &surface) || surface.Code != "visual_submission_resume_unconfirmed" {
			t.Fatal("changed Run was reported resumed", err)
		}
	}
	var status string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT status FROM public.media_intents WHERE id=$1`, mid).Scan(&status); err != nil || status != "completed" || len(probe.requests) != 0 {
		t.Fatal(status, probe.requests, err)
	}
}

func TestVisualSubmissionReviewedReadyConcurrentReplayHasOneResumeEffect(t *testing.T) {
	f, probe, mid, body := visualSubmissionFixture(t, "candidate")
	path := "/internal/fluctlights/" + f.fluctlightID + "/visual-sessions/" + f.sessionID + "/resume-reviewed"
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			_, err := f.app.DispatchOwnerSurface(f.ctx, "POST", path, nil, f.ownerID, cloneMap(body))
			results <- err
		}()
	}
	successes := 0
	for i := 0; i < 2; i++ {
		if <-results == nil {
			successes++
		}
	}
	result, err := f.app.DispatchOwnerSurface(f.ctx, "POST", path, nil, f.ownerID, body)
	if successes < 1 || err != nil || result["status"] != "resume_requested" || len(probe.requests) != 1 || len(probe.effects) != 1 {
		t.Fatal(successes, result, probe.requests, err)
	}
	var commands int
	var status string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT status,(SELECT count(*) FROM public.workflow_control_commands WHERE workflow_id=$2) FROM public.media_intents WHERE id=$1`, mid, body["workflow_id"]).Scan(&status, &commands); err != nil || status != "completed" || commands != 1 {
		t.Fatal(status, commands, err)
	}
}
