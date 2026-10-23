package migrations

import "testing"

func TestDurableTurnTraceUpgradeBackfillsOnceAndPreservesInbox(t *testing.T) {
	ctx, pool := isolatedMigrationPool(t)
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.cognition_inbox(id,fluctlight_id,sequence,event_type,payload,causation_id,correlation_id,idempotency_key,occurred_at,status,error_code) VALUES('old-chat-inbox','trace-role',1,'conversation.turn','{"conversation_id":"trace-conversation","turn_id":"trace-turn"}','trace-turn','turn:trace-turn','trace-key',now(),'failed','provider_gateway_timeout'); UPDATE public.alembic_version SET version_num=$1`, DurableTurnDiagnosticsPreviousHead); err != nil {
		t.Fatal(err)
	}
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	var found bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.diagnostic_turns WHERE id='old-chat-inbox' AND source_event_id='old-chat-inbox' AND correlation_id='turn:trace-turn') AND EXISTS(SELECT 1 FROM public.cognition_inbox WHERE id='old-chat-inbox' AND status='failed' AND error_code='provider_gateway_timeout')`).Scan(&found); err != nil || !found {
		t.Fatal(found, err)
	}
	// Explicit diagnostic cleanup must not be undone on every process restart.
	if _, err := pool.Exec(ctx, `DELETE FROM public.diagnostic_turns WHERE id='old-chat-inbox'`); err != nil {
		t.Fatal(err)
	}
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.diagnostic_turns WHERE id='old-chat-inbox')`).Scan(&found); err != nil || found {
		t.Fatal("diagnostic cleanup was undone", found, err)
	}
}
