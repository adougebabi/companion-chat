package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"
)

func TestTransactionalMemoryAndAffectRollbackWithCallerOwnedPostgresTransaction(t *testing.T) {
	databaseURL := os.Getenv("GO_CORE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("GO_CORE_TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	repository, err := NewPostgresRepository(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer repository.Close()

	var head string
	if err := repository.Pool().QueryRow(ctx, `SELECT version_num FROM public.alembic_version`).Scan(&head); err != nil || head != "0028_affect_canonical" {
		t.Skipf("test database is not at capability runtime head: head=%q err=%v", head, err)
	}
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	ownerID := "actor_capability_tx_" + suffix
	fluctlightID := "fluctlight_capability_tx_" + suffix
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES($1,'human','active'),($2,'fluctlight','active')`, ownerID, fluctlightID); err != nil {
		t.Fatal(err)
	}
	defer repository.Pool().Exec(context.Background(), `DELETE FROM public.actors WHERE id IN ($1,$2)`, ownerID, fluctlightID)
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) VALUES($1,$2,'llm_defined','active',$3,$4,'{}','{}','{}','{}')`, fluctlightID, ownerID, json.RawMessage(`{"personality_system":{"active_profile_id":"default","profiles":[{"id":"default"}]}}`), json.RawMessage(`{"name":"transaction-test"}`)); err != nil {
		t.Fatal(err)
	}
	defer repository.Pool().Exec(context.Background(), `DELETE FROM public.fluctlights WHERE id=$1`, fluctlightID)
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlight_inner_states(fluctlight_id,revision,pad,mood,momentum,regulation,drives,conflicts,last_updated_at) VALUES($1,0,'{"pleasure":0,"arousal":0,"dominance":0}','{"label":"平静","intensity":0}','{}','{}','[]','[]',now())`, fluctlightID); err != nil {
		t.Fatal(err)
	}
	defer repository.Pool().Exec(context.Background(), `DELETE FROM public.fluctlight_inner_states WHERE fluctlight_id=$1`, fluctlightID)
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlight_affect_profiles(fluctlight_id) VALUES($1)`, fluctlightID); err != nil {
		t.Fatal(err)
	}
	defer repository.Pool().Exec(context.Background(), `DELETE FROM public.fluctlight_affect_profiles WHERE fluctlight_id=$1`, fluctlightID)

	app := &App{DB: repository}
	memory := memoryEventCapability{service: app}
	affect := affectEventCapability{service: app}
	registry := mustCapabilityRegistry(memory, affect)
	resolver := NewStaticContextResolver(map[ContextSlot]ContextLoader{
		SlotCorePersona: func(context.Context, ContextRequest) (any, error) {
			return map[string]any{"personality_system": map[string]any{"active_profile_id": "default", "profiles": []any{map[string]any{"id": "default"}}}}, nil
		},
		SlotMemoryScope: func(context.Context, ContextRequest) (any, error) {
			return map[string]any{"owner_actor_id": ownerID, "memories": []any{}}, nil
		},
		SlotCurrentState: func(context.Context, ContextRequest) (any, error) {
			return map[string]any{"revision": 0}, nil
		},
	})
	runtime, err := NewCapabilityRuntime(registry, resolver)
	if err != nil {
		t.Fatal(err)
	}

	memoryInvocation := CapabilityInvocation{CallID: "memory-" + suffix, CapabilityName: "memory_event", Arguments: json.RawMessage(`{"content":"transactional memory","type":"episodic","confidence":0.9,"importance":0.7}`), SourceFactID: "fact-" + suffix, ProviderRequestID: "provider-" + suffix, Metadata: InvocationMetadata{FluctlightID: fluctlightID}}
	tx, err := repository.Pool().Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	memoryResult, err := runtime.ExecuteTransactional(ctx, tx, memoryInvocation)
	if err != nil || memoryResult.Status != "completed" {
		_ = tx.Rollback(ctx)
		t.Fatalf("memory result=%#v err=%v", memoryResult, err)
	}
	memoryID := stringValue(mapValue(memoryResult.Output)["id"])
	var insideCount int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM public.memories WHERE id=$1`, memoryID).Scan(&insideCount); err != nil || insideCount != 1 {
		_ = tx.Rollback(ctx)
		t.Fatalf("memory not visible inside caller transaction: count=%d err=%v", insideCount, err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	var outsideCount int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.memories WHERE id=$1`, memoryID).Scan(&outsideCount); err != nil || outsideCount != 0 {
		t.Fatalf("memory escaped caller rollback: count=%d err=%v", outsideCount, err)
	}

	affectInvocation := CapabilityInvocation{CallID: "affect-" + suffix, CapabilityName: "affect_event", Arguments: json.RawMessage(`{"event":{"type":"sad","confidence":1}}`), SourceFactID: "fact-" + suffix, ProviderRequestID: "provider-" + suffix, Metadata: InvocationMetadata{FluctlightID: fluctlightID}}
	tx, err = repository.Pool().Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	affectResult, err := runtime.ExecuteTransactional(ctx, tx, affectInvocation)
	if err != nil || affectResult.Status != "completed" {
		_ = tx.Rollback(ctx)
		t.Fatalf("affect result=%#v err=%v", affectResult, err)
	}
	var insideRevision int
	if err := tx.QueryRow(ctx, `SELECT revision FROM public.fluctlight_inner_states WHERE fluctlight_id=$1`, fluctlightID).Scan(&insideRevision); err != nil || insideRevision != 1 {
		_ = tx.Rollback(ctx)
		t.Fatalf("affect revision not visible inside caller transaction: revision=%d err=%v", insideRevision, err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	var outsideRevision, eventCount int
	if err := repository.Pool().QueryRow(ctx, `SELECT revision FROM public.fluctlight_inner_states WHERE fluctlight_id=$1`, fluctlightID).Scan(&outsideRevision); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.fluctlight_inner_state_events WHERE id=$1`, "affect_event_"+stableDigest(fluctlightID+":capability:"+affectInvocation.CallID)).Scan(&eventCount); err != nil {
		t.Fatal(err)
	}
	if outsideRevision != 0 || eventCount != 0 {
		t.Fatalf("affect escaped caller rollback: revision=%d event_count=%d", outsideRevision, eventCount)
	}
}
