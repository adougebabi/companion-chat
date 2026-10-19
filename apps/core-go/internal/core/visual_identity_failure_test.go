package core

import "testing"

func TestVisualIdentityFailedMediaSettlesParentWithoutRequeueOrProvider(t *testing.T) {
	f := newVisualIdentityToolFixture(t)
	r, err := f.app.ExecuteTool(f.ctx, f.request(visualIdentityGenerateCandidateCapabilityName, "failed-child", map[string]any{"reason": "initial"}))
	if err != nil || r.Result.Status != "accepted" {
		t.Fatal(r, err)
	}
	id := stringValue(mapValue(r.Result.Output)["media_intent_id"])
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.media_intents SET status='failed' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		result, err := f.app.ProcessVisualIdentity(f.ctx, f.sessionID)
		if err != nil || result["status"] != "failed" || result["accepted"] != false {
			t.Fatal(result, err)
		}
	}
	var status string
	var failures int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT status FROM public.fluctlight_visual_identity_sessions WHERE id=$1`, f.sessionID).Scan(&status); err != nil || status != "failed" {
		t.Fatal(status, err)
	}
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.fluctlight_visual_identity_timeline WHERE session_id=$1 AND stage='failed'`, f.sessionID).Scan(&failures); err != nil || failures != 1 {
		t.Fatal(failures, err)
	}
}
