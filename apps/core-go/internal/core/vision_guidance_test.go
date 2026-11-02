package core

import (
	"strings"
	"testing"
)

func TestVisionDisabledExplainsConfigurationAndPreservesUnverifiedCandidate(t *testing.T) {
	f := newVisualIdentityToolFixture(t)
	generated := f.generateCandidate(t, "vision-guidance-candidate")
	asset := f.makeCurrentCandidateReady(t, stringValue(mapValue(generated.Result.Output)["media_intent_id"]))
	output, err := f.app.RunVisualIdentityAgent(f.ctx, VisualIdentityAgentInput{SessionID: f.sessionID})
	if err != nil {
		t.Fatal(err)
	}
	if output.ErrorCode != "vision_disabled" || output.Status != "awaiting_review" || output.CandidateAssetID != asset || output.CanonicalAssetID != "" {
		t.Fatalf("disabled optional review changed candidate or authority: %#v", output)
	}
	for _, required := range []string{"视觉理解未启用", "候选图片已保存", "尚未经视觉检验", "设置", "独立视觉模型", "人工评审", "不会自动重试"} {
		if !strings.Contains(output.Summary, required) {
			t.Errorf("missing recovery explanation %q: %s", required, output.Summary)
		}
	}
	// A pre-upgrade checkpoint gets the same explanation on read without a
	// migration, replaying generation, or rewriting its original history.
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.fluctlight_visual_identity_timeline SET summary='候选图片已保存，视觉检验未完成，等待显式评审' WHERE session_id=$1 AND stage=$2 AND status='skipped'`, f.sessionID, visualIdentityStageVisionReady); err != nil {
		t.Fatal(err)
	}
	detail, err := f.app.readVisualIdentityDetail(f.ctx, f.fluctlightID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, row := range detail["timeline"].([]map[string]any) {
		if mapValue(row["metadata"])["reason"] == "vision_disabled" {
			found = true
			if row["summary"] != output.Summary || mapValue(row["metadata"])["inspection_status"] != "unverified" {
				t.Fatalf("read projection lost reason or explanation: %#v", row)
			}
		}
	}
	if !found {
		t.Fatal("skipped vision checkpoint missing from detail")
	}
	var persistedSummary string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT summary FROM public.fluctlight_visual_identity_timeline WHERE session_id=$1 AND stage=$2 AND status='skipped'`, f.sessionID, visualIdentityStageVisionReady).Scan(&persistedSummary); err != nil {
		t.Fatal(err)
	}
	if persistedSummary != "候选图片已保存，视觉检验未完成，等待显式评审" {
		t.Fatal("read projection rewrote durable history")
	}
}
