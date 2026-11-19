package core

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/capability"
)

func TestActorFactPrecisionThroughFrozenAuthorityAndWriter(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	values := []string{`9007199254740993`, `123456789012345678901234567890`, `0.12345678901234567890123`, `7`, `false`, `null`, `{"id":"business-id","items":[9007199254740993,false,null],"schema":{"properties":{"item_ids":{"default":9007199254740993}}}}`}
	rows := []map[string]any{}
	for i, raw := range values {
		attribute := []string{"preferences", "interests", "background", "shared_information", "rules", "wants_you_to_know", "habits"}[i]
		id := recordActorFact(t, f, attribute, "assert", attribute, "fixture-placeholder", "")
		if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.actor_facts SET value_json=$2 WHERE id=$1`, id, raw); err != nil {
			t.Fatal(err)
		}
		facts, err := readActorFactsWith(f.ctx, f.repository.Pool(), f.fluctlightID, f.ownerID, f.app.now(), false, 32, attribute)
		if err != nil || len(facts) != 1 {
			t.Fatalf("read %d: %#v %v", i, facts, err)
		}
		if got := jsonString(facts[0]["value"]); got != raw {
			t.Fatalf("storage read changed %s into %s", raw, got)
		}
		rows = append(rows, facts[0])
	}
	projection := ContextProjection{FluctlightID: f.fluctlightID, OwnerActorID: f.ownerID, ConversationID: f.conversationID, CurrentSpeaker: map[string]any{"actor_id": f.ownerID, "ref": "actor_user"}, Actors: []map[string]any{{"actor_id": f.ownerID, "ref": "actor_user", "type": "human"}}, ActorFacts: rows}
	if err := buildContextReferenceIndex(&projection); err != nil {
		t.Fatal(err)
	}
	originalIndex := jsonString(projection.ReferenceIndex)
	invocation := CapabilityInvocation{SchemaVersion: CapabilityInvocationSchemaVersion, CallID: "precision-call", CapabilityName: actorInspectCapabilityName, Arguments: json.RawMessage(`{}`), ContextSnapshot: ContextSnapshotFromProjection(projection)}
	payload := map[string]any{"context_projection": projection, "context_reference_index": projection.ReferenceIndex, "capability_invocations": []CapabilityInvocation{invocation}}
	inbox := "precision-inbox" + f.suffix
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.cognition_frozen_actions(id,decision_id,inbox_id,fluctlight_id,action_type,payload,state_revision,provider_request_id,status) VALUES($1,'precision-decision',$2,$3,'reply',$4,0,'precision-provider','completed')`, "precision-frozen"+f.suffix, inbox, f.fluctlightID, jsonBytes(payload)); err != nil {
		t.Fatal(err)
	}
	frozen, found, err := f.app.LoadFrozenTurn(f.ctx, inbox)
	if err != nil || !found {
		t.Fatal(found, err)
	}
	restored, ok := contextProjectionFromValue(frozen.Payload["context_projection"])
	if !ok || jsonString(restored.ReferenceIndex) != originalIndex {
		t.Fatal("frozen reference identity or validation changed")
	}
	for i, row := range restored.ActorFacts {
		if jsonString(row["value"]) != values[i] {
			t.Fatalf("projection changed value %d: %s", i, jsonString(row["value"]))
		}
		entry := restored.ReferenceIndex.ByRef[stringValue(row["ref"])]
		fact := factOnlyReplyEvidenceFact(entry, restored)
		if got := jsonString(mapValue(fact["value"])["value"]); got != values[i] {
			t.Fatalf("Writer evidence changed value %d: %s", i, got)
		}
		if got := jsonString(mapValue(momentReplyEvidenceFact(entry, restored)["value"])["value"]); got != values[i] {
			t.Fatal("Moment changed Actor value", got)
		}
	}
	coreCalls, err := capabilityInvocationsFromValue(frozen.Payload["capability_invocations"])
	if err != nil {
		t.Fatal(err)
	}
	sharedCalls, err := capability.CapabilityInvocationsFromValue(frozen.Payload["capability_invocations"])
	if err != nil {
		t.Fatal(err)
	}
	for _, snapshot := range []any{coreCalls[0].ContextSnapshot, sharedCalls[0].ContextSnapshot, capability.BoundedSnapshotValue(invocation.ContextSnapshot), NewSnapshotContextResolver(invocation.ContextSnapshot).Snapshot} {
		index, err := contextReferenceIndexFromValue(mapValue(snapshot)["context_reference_index"])
		if err != nil || jsonString(index) != originalIndex {
			t.Fatal("capability copy changed frozen reference", err)
		}
	}
	prepared, err := capability.WithCapabilityPreparedData(invocation, "actor_value", rows[len(rows)-1]["value"])
	if err != nil {
		t.Fatal(err)
	}
	value, found, err := capability.CapabilityPreparedData(prepared, "actor_value")
	if err != nil || !found || jsonString(value) != values[len(values)-1] {
		t.Fatal("prepared replay changed Actor value", value, err)
	}
	input := replyContextFixture()
	input.Projection.ActorFacts = restored.ActorFacts
	input.Projection.Actors = restored.Actors
	input.Projection.OwnerActorID = f.ownerID
	input.Projection.CurrentSpeaker = restored.CurrentSpeaker
	input.Projection.AsOf = formatInstant(f.app.now())
	prompt, err := buildFactOnlyReplyContext(input, providerAssignment{})
	if err != nil {
		t.Fatal(err)
	}
	wire := jsonString(prompt.Messages)
	for _, literal := range []string{"9007199254740993", "123456789012345678901234567890", "0.12345678901234567890123", "business-id", "item_ids"} {
		if !strings.Contains(wire, literal) {
			t.Fatal("Writer request lost business value", literal)
		}
	}
	// A previously rounded snapshot retains its original value/ref; no guessed
	// restoration of old data or reference identity migration is performed.
	legacy := ContextProjection{FluctlightID: f.fluctlightID, OwnerActorID: f.ownerID, ActorFacts: []map[string]any{{"fact_id": "legacy", "revision": 1, "value": float64(9007199254740992)}}}
	if err := buildContextReferenceIndex(&legacy); err != nil {
		t.Fatal(err)
	}
	if old, ok := contextProjectionFromValue(decodeObject(jsonBytes(legacy))); !ok || jsonString(old.ReferenceIndex) != jsonString(legacy.ReferenceIndex) {
		t.Fatal("legacy reference compatibility changed")
	}
}
