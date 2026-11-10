package migrations

import "testing"

func TestVisualModelSnapshotUpgradeKeepsLegacyAndMarksNewSessions(t *testing.T) {
	ctx, pool := isolatedMigrationPool(t)
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO public.fluctlight_visual_identity_sessions(id,visual_identity_id,fluctlight_id,trigger_type,workflow_id,status,current_attempt,character_sheet_media_intent_id)
		VALUES('legacy-visual-session','legacy-visual-id','legacy-character','initialization','legacy-visual-workflow','character_sheet_pending',2,'legacy-media');
		ALTER TABLE public.fluctlight_visual_identity_sessions DROP COLUMN model_provenance;
		UPDATE public.alembic_version SET version_num='0109_media_model_snapshot'`); err != nil {
		t.Fatal(err)
	}
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.fluctlight_visual_identity_sessions(id,visual_identity_id,fluctlight_id,trigger_type,workflow_id,status) VALUES('new-visual-session','new-visual-id','new-character','initialization','new-visual-workflow','queued')`); err != nil {
		t.Fatal(err)
	}
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	var valid bool
	err := pool.QueryRow(ctx, `SELECT (SELECT version_num=$1 FROM public.alembic_version) AND EXISTS(SELECT 1 FROM public.fluctlight_visual_identity_sessions WHERE id='legacy-visual-session' AND status='character_sheet_pending' AND current_attempt=2 AND character_sheet_media_intent_id='legacy-media' AND model_provenance='{}'::jsonb) AND EXISTS(SELECT 1 FROM public.fluctlight_visual_identity_sessions WHERE id='new-visual-session' AND model_provenance->>'_visual_model_snapshot_contract'='v1')`, Head).Scan(&valid)
	if err != nil || !valid {
		t.Fatalf("visual model migration changed old or new session semantics: valid=%t err=%v", valid, err)
	}
}
