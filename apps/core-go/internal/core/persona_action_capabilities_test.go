package core

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func TestPersonaToolsAreAvailableToAgentsAndCommitThroughDomainService(t *testing.T) {
	registry := mustCapabilityRegistry(builtinCapabilities(nil)...)
	for _, name := range []string{personaTakeoverCapabilityName, personaSwitchCapabilityName} {
		definition, ok := registry.Definition(name)
		if !ok || definition.InternalOnly || definition.Type != CapabilityTypeAction || definition.SuccessBoundary != "persona_action_committed" {
			t.Fatalf("invalid persona Tool contract %s: %#v", name, definition)
		}
		if class, err := classifyCapabilityExecution(personaActionCapability{}, definition); err != nil || class != CapabilityExecutionTransactionalMutation {
			t.Fatalf("persona execution class %s: %s %v", name, class, err)
		}
		for _, surface := range []CapabilitySurface{CapabilitySurfaceConversation, CapabilitySurfaceWakeUp, CapabilitySurfaceAutonomy, CapabilitySurfaceNativeCognition} {
			found := false
			for _, candidate := range registry.Catalog(surface) {
				if candidate.Name == name {
					found = true
				}
			}
			if !found {
				t.Fatalf("persona Tool %s unavailable to %s Agent", name, surface)
			}
		}
	}
}

func TestPersonaPolicyInvocationUsesStablePolicyIdentity(t *testing.T) {
	base := preparedPersonaAction{SchemaVersion: personaActionPlanVersion, CapabilityName: personaSwitchCapabilityName, OperationID: "operation-1", FluctlightID: "fl-1", ConversationID: "conv-1", EvidenceID: "evidence-1", OccurredAt: time.Now(), Decision: "switch", Arguments: map[string]any{"decision": "switch", "target_profile_id": "profile-b"}}
	first := personaActionRequestDigest(base)
	base.OccurredAt = base.OccurredAt.Add(time.Hour)
	if second := personaActionRequestDigest(base); first == "" || first != second {
		t.Fatalf("request digest changed with generated audit time: %q/%q", first, second)
	}
}

func TestPostgresDirectPersonaToolsCommitRejectReplayConflictAndAudit(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	ownerID, fluctlightID := "actor_persona_tool_"+suffix, "fluctlight_persona_tool_"+suffix
	corePersona := map[string]any{"personality_system": map[string]any{
		"mode": "multiple", "active_profile_id": "spark",
		"profiles":       []any{map[string]any{"id": "spark", "name": "Spark"}, map[string]any{"id": "twilight", "name": "Twilight"}},
		"switching":      map[string]any{"cooldown_seconds": 0, "rules": []any{map[string]any{"id": "safety"}}},
		"takeover_rules": []any{map[string]any{"id": "takeover-night", "kind": "turn_takeover", "version": "turn-takeover.v1", "source_profile_id": "spark", "target_profile_id": "twilight", "condition": "night"}},
	}}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES($1,'human','active'),($2,'fluctlight','active')`, ownerID, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) VALUES($1,$2,'blank_slate','active',$3,'{}','{}','{}','{}','{}')`, fluctlightID, ownerID, jsonBytes(corePersona)); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlight_personality_runtime(fluctlight_id,active_profile_id,revision) VALUES($1,'spark',0)`, fluctlightID); err != nil {
		t.Fatal(err)
	}
	app := &App{DB: repository}
	service := newPersonaActionService(app)
	app.Capabilities = mustCapabilityRegistry(
		personaActionCapability{name: personaTakeoverCapabilityName, service: service},
		personaActionCapability{name: personaSwitchCapabilityName, service: service},
	)
	app.ContextResolver = NewAppContextResolver(app)

	switchRequest := ToolExecutionRequest{
		CapabilityName: personaSwitchCapabilityName, OperationID: "switch-" + suffix,
		AuthorizationActorID: ownerID, FluctlightID: fluctlightID, EvidenceID: "owner-command-" + suffix,
		Arguments: jsonBytes(map[string]any{"decision": "switch", "source_profile_id": "spark", "target_profile_id": "twilight", "trigger_id": "switch:safety", "reason": "safety"}),
	}
	first, err := app.ExecuteTool(ctx, switchRequest)
	if err != nil || first.Result.Status != "completed" || stringValue(mapValue(first.Result.Output)["active_profile_id"]) != "twilight" {
		t.Fatalf("switch receipt=%#v err=%v", first, err)
	}
	var active string
	var revision int
	if err := repository.Pool().QueryRow(ctx, `SELECT active_profile_id,revision FROM public.fluctlight_personality_runtime WHERE fluctlight_id=$1`, fluctlightID).Scan(&active, &revision); err != nil || active != "twilight" || revision != 1 {
		t.Fatalf("runtime active=%q revision=%d err=%v", active, revision, err)
	}
	retry := switchRequest
	retry.NativeToolCallID, retry.ProviderRequestID = "native-retry-"+suffix, "provider-retry-"+suffix
	replayed, err := app.ExecuteTool(ctx, retry)
	if err != nil || replayed.Result.Status != "completed" || !boolValueForTest(mapValue(replayed.Result.Output)["replayed"]) {
		t.Fatalf("switch replay receipt=%#v err=%v", replayed, err)
	}
	conflict := switchRequest
	conflict.Arguments = jsonBytes(map[string]any{"decision": "switch", "source_profile_id": "spark", "target_profile_id": "twilight", "trigger_id": "switch:safety", "reason": "different-payload"})
	if receipt, conflictErr := app.ExecuteTool(ctx, conflict); conflictErr == nil || receipt.Result.Status != "failed" {
		t.Fatalf("switch conflict receipt=%#v err=%v", receipt, conflictErr)
	}
	rejected := switchRequest
	rejected.OperationID = "switch-rejected-" + suffix
	rejected.Arguments = jsonBytes(map[string]any{"decision": "switch", "source_profile_id": "twilight", "target_profile_id": "missing", "trigger_id": "switch:safety"})
	if receipt, rejectErr := app.ExecuteTool(ctx, rejected); rejectErr != nil || receipt.Result.Status != "rejected" || receipt.Result.ErrorCode != "personality_target_profile_not_found" {
		t.Fatalf("switch rejection receipt=%#v err=%v", receipt, rejectErr)
	}

	// Takeover commits a turn-scoped decision audit and deliberately leaves the
	// persistent dominant profile untouched.
	if _, err := repository.Pool().Exec(ctx, `UPDATE public.fluctlight_personality_runtime SET active_profile_id='spark',previous_profile_id=NULL,revision=2,cooldown_until=NULL WHERE fluctlight_id=$1`, fluctlightID); err != nil {
		t.Fatal(err)
	}
	takeover := ToolExecutionRequest{
		CapabilityName: personaTakeoverCapabilityName, OperationID: "takeover-" + suffix,
		AuthorizationActorID: ownerID, FluctlightID: fluctlightID, EvidenceID: "turn-evidence-" + suffix,
		Arguments: jsonBytes(map[string]any{"decision": takeoverDecisionTakeoverB, "rule_id": "takeover-night", "source_profile_id": "spark", "target_profile_id": "twilight"}),
	}
	takeoverReceipt, err := app.ExecuteTool(ctx, takeover)
	if err != nil || takeoverReceipt.Result.Status != "completed" || stringValue(mapValue(takeoverReceipt.Result.Output)["disposition"]) != "applied" {
		t.Fatalf("takeover receipt=%#v err=%v", takeoverReceipt, err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT active_profile_id FROM public.fluctlight_personality_runtime WHERE fluctlight_id=$1`, fluctlightID).Scan(&active); err != nil || active != "spark" {
		t.Fatalf("takeover changed persistent profile active=%q err=%v", active, err)
	}
	badTakeover := takeover
	badTakeover.OperationID = "takeover-rejected-" + suffix
	badTakeover.Arguments = json.RawMessage(`{"decision":"takeover_b","rule_id":"missing","source_profile_id":"spark","target_profile_id":"twilight"}`)
	if receipt, rejectErr := app.ExecuteTool(ctx, badTakeover); rejectErr != nil || receipt.Result.Status != "rejected" || receipt.Result.ErrorCode != "persona_takeover_rule_not_found" {
		t.Fatalf("takeover rejection receipt=%#v err=%v", receipt, rejectErr)
	}
	var audits int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.platform_outbox_events WHERE fluctlight_id=$1 AND aggregate_type='persona_action'`, fluctlightID).Scan(&audits); err != nil || audits != 4 {
		t.Fatalf("persona audit count=%d err=%v", audits, err)
	}
}
