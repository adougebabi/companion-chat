package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	capabilitycontract "github.com/fluctlight/local-ai-companion/apps/core-go/internal/capability"
)

// Exercise the actual pre-model builder/validator used by cognition, without a
// database, Provider or synthetic successful fallback.
func TestContextReferenceCapacityIsAdvisory(t *testing.T) {
	for _, test := range []struct {
		name                string
		count, contentBytes int
	}{
		{"reference_count", 180, 100},
		{"aggregate_bytes", 64, 4096},
		{"single_snapshot_bytes", 1, 40 * 1024},
	} {
		t.Run(test.name, func(t *testing.T) {
			projection := ContextProjection{FluctlightID: "capacity-self", OwnerActorID: "capacity-owner", ConversationID: "capacity-conversation"}
			for i := 0; i < test.count; i++ {
				projection.Memories = append(projection.Memories, map[string]any{"id": fmt.Sprintf("memory-%d", i), "revision": 1, "content": strings.Repeat("x", test.contentBytes)})
			}
			if err := buildContextReferenceIndex(&projection); err != nil {
				t.Fatalf("valid cognition blocked by an advisory capacity threshold: %v", err)
			}
			if len(projection.ReferenceIndex.ByRef) != test.count {
				t.Fatal("references silently discarded")
			}
			for _, row := range projection.Memories {
				entry := projection.ReferenceIndex.ByRef[stringValue(row["ref"])]
				if stringValue(decodeObject(entry.Snapshot)["content"]) != strings.Repeat("x", test.contentBytes) {
					t.Fatal("snapshot truncated")
				}
			}
			encoded := jsonBytes(projection.ReferenceIndex)
			var replay ContextReferenceIndex
			if err := json.Unmarshal(encoded, &replay); err != nil {
				t.Fatal(err)
			}
			if err := replay.Validate(); err != nil {
				t.Fatalf("valid large index cannot survive tool/replay validation: %v", err)
			}
		})
	}
}

func TestLargeReferenceIndexKeepsIntegrityAndActiveMemory(t *testing.T) {
	projection := ContextProjection{FluctlightID: "self", OwnerActorID: "owner", ConversationID: "conversation"}
	for i := 0; i < 180; i++ {
		projection.Memories = append(projection.Memories, map[string]any{"id": fmt.Sprint(i), "revision": 1, "content": "fact"})
	}
	if err := buildContextReferenceIndex(&projection); err != nil {
		t.Fatal(err)
	}
	rows := []map[string]any{{"id": "active", "owner_fluctlight_id": "self", "conversation_id": "conversation", "status": "active", "revision": 1, "content": "actual current commitment"}}
	if err := addActiveMemoryReferences(&projection.ReferenceIndex, rows); err != nil {
		t.Fatal(err)
	}
	if len(projection.ReferenceIndex.ByRef) != 181 || stringValue(rows[0]["ref"]) == "" {
		t.Fatal("active refs discarded at former count cap")
	}
	foreign := cloneMap(rows[0])
	foreign["owner_fluctlight_id"] = "foreign"
	if err := addActiveMemoryReferences(&projection.ReferenceIndex, []map[string]any{foreign}); err == nil {
		t.Fatal("capacity warning bypassed ownership")
	}
	ref := stringValue(projection.Memories[0]["ref"])
	entry := projection.ReferenceIndex.ByRef[ref]
	entry.Snapshot = json.RawMessage(`{"content":"tampered"}`)
	projection.ReferenceIndex.ByRef[ref] = entry
	if err := projection.ReferenceIndex.Validate(); err == nil || err.Error() != "context_reference_token_invalid" {
		t.Fatal("large index skipped hash validation", err)
	}
}

func TestContextReferenceCapacityWarningIsInspectableWithoutDatabase(t *testing.T) {
	secret := strings.Repeat("PRIVATE_SNAPSHOT_BODY_DO_NOT_LOG", 6000)
	projection := ContextProjection{FluctlightID: "self", OwnerActorID: "owner", SourceFactID: "inbox-test", AsOf: "2026-10-14T11:50:30.000+08:00", Memories: []map[string]any{{"id": "large-memory", "revision": 1, "content": secret}}}
	if err := buildContextReferenceIndex(&projection); err != nil {
		t.Fatal(err)
	}
	report := projection.ReferenceIndex.capacityReport()
	if report.ReferenceCount != 1 || report.EncodedBytes <= contextReferenceIndexWarningBytes || report.LargestSnapshotBytes <= contextReferenceSnapshotWarningBytes || report.ByKind[ContextReferenceMemory].Count != 1 {
		t.Fatal("capacity measurement incomplete", report)
	}
	if strings.Contains(jsonString(report), "PRIVATE_SNAPSHOT_BODY") || len(report.LargestSnapshots) != 1 {
		t.Fatal("diagnostic metadata leaks content or loses largest entry")
	}
	var logs bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	defer slog.SetDefault(old)
	(&App{}).recordContextReferenceCapacity(context.Background(), projection, "context_projection")
	text := logs.String()
	for _, required := range []string{"context.reference.capacity_warning", "continue_complete_context", "as_of", "advisory_thresholds", "reference_count", "encoded_bytes", "by_kind", "largest_snapshots", "inbox-test", "continuing"} {
		if !strings.Contains(text, required) {
			t.Errorf("DB-down operational log missing %s: %s", required, text)
		}
	}
	if strings.Contains(text, "PRIVATE_SNAPSHOT_BODY") {
		t.Fatal("snapshot content logged")
	}
}

func TestProjectionSnapshotPreservesLargeReferenceIndex(t *testing.T) {
	projection := ContextProjection{FluctlightID: "self", OwnerActorID: "owner", Memories: []map[string]any{{"id": "large-memory", "revision": 1, "content": strings.Repeat("valid", 40000)}}}
	if err := buildContextReferenceIndex(&projection); err != nil {
		t.Fatal(err)
	}
	snapshot := ContextSnapshotFromProjection(projection)
	index, err := contextReferenceIndexFromValue(snapshot["context_reference_index"])
	if err != nil || len(index.ByRef) != 1 {
		t.Fatal("frozen Tool lost large index", err)
	}
	if jsonString(index) != jsonString(projection.ReferenceIndex) {
		t.Fatal("frozen references changed")
	}
}

func TestLargeInternalContextSnapshotSurvivesBothResolvers(t *testing.T) {
	content := strings.Repeat("internal-current-state", 12000)
	coreSnapshot := (CapabilityContext{Identity: ContextIdentity{FluctlightID: "self"}, State: &CurrentStateContext{Data: map[string]any{"content": content}}}).Snapshot()
	if stringValue(mapValue(coreSnapshot["current_state"])["content"]) != content {
		t.Fatal("Core silently erased large required slot")
	}
	canonicalSnapshot := (capabilitycontract.CapabilityContext{Identity: capabilitycontract.ContextIdentity{FluctlightID: "self"}, State: &capabilitycontract.CurrentStateContext{Data: map[string]any{"content": content}}}).Snapshot()
	if stringValue(mapValue(canonicalSnapshot["current_state"])["content"]) != content {
		t.Fatal("canonical runtime silently erased large required slot")
	}
	for _, snapshot := range []map[string]any{coreSnapshot, canonicalSnapshot, map[string]any{"identity": map[string]any{"fluctlight_id": "self"}, "current_state": map[string]any{"content": content[:80000]}, "core_persona": map[string]any{"background": content[:80000]}}} {
		coreResolver := NewSnapshotContextResolver(snapshot)
		resolved, err := coreResolver.Resolve(context.Background(), ContextRequest{FluctlightID: "self"}, []ContextSlot{SlotCurrentState})
		if err != nil {
			t.Fatal("Core failed to rehydrate valid large snapshot", err)
		}
		if jsonString(resolved.State.Data) != jsonString(snapshot["current_state"]) {
			t.Fatal("Core changed snapshot data")
		}
		if _, err := coreResolver.Resolve(context.Background(), ContextRequest{FluctlightID: "foreign"}, []ContextSlot{SlotCurrentState}); err == nil {
			t.Fatal("Core lost scope check")
		}
		canonicalResolver := capabilitycontract.NewSnapshotContextResolver(snapshot)
		other, err := canonicalResolver.Resolve(context.Background(), capabilitycontract.ContextRequest{FluctlightID: "self"}, []capabilitycontract.ContextSlot{capabilitycontract.SlotCurrentState})
		if err != nil {
			t.Fatal("canonical runtime failed rehydration", err)
		}
		if jsonString(other.State.Data) != jsonString(snapshot["current_state"]) {
			t.Fatal("canonical runtime changed data")
		}
		if _, err := canonicalResolver.Resolve(context.Background(), capabilitycontract.ContextRequest{FluctlightID: "foreign"}, []capabilitycontract.ContextSlot{capabilitycontract.SlotCurrentState}); err == nil {
			t.Fatal("canonical runtime lost scope check")
		}
	}
}

func TestLargeInternalPreparedPayloadSurvivesCanonicalValidation(t *testing.T) {
	content := strings.Repeat("server-owned-prepared-data", 10000)
	definition := imageGenerateCapability{}.Definition()
	for _, prepare := range []func(CapabilityInvocation, string, any) (CapabilityInvocation, error){withCapabilityPreparedData, capabilitycontract.WithCapabilityPreparedData} {
		invocation := CapabilityInvocation{CallID: "large-prepared", CapabilityName: definition.Name, Arguments: json.RawMessage(`{"intent":"portrait"}`), SourceFactID: "source", ProviderRequestID: "provider"}
		prepared, err := prepare(invocation, "internal_state", map[string]any{"content": content})
		if err != nil {
			t.Fatal("valid internal prepared state rejected", err)
		}
		if err := prepared.Validate(definition); err != nil {
			t.Fatal("canonical invocation still rejects large internal envelope", err)
		}
		for _, read := range []func(CapabilityInvocation, string) (any, bool, error){capabilityPreparedData, capabilitycontract.CapabilityPreparedData} {
			value, found, err := read(prepared, "internal_state")
			if err != nil || !found || stringValue(mapValue(value)["content"]) != content {
				t.Fatal("prepared data changed or vanished", err)
			}
		}
	}
}

func TestContextReferenceDuplicateAtAdvisoryThresholdIsIdempotent(t *testing.T) {
	index := newContextReferenceIndex(ContextProjection{FluctlightID: "capacity-self", OwnerActorID: "capacity-owner"})
	for i := 0; i < 128; i++ {
		if _, err := index.add(ContextReferenceMemory, fmt.Sprintf("memory-%d", i), 1, map[string]any{"content": "same"}); err != nil {
			t.Fatal(err)
		}
	}
	before := jsonString(index)
	ref, err := index.add(ContextReferenceMemory, "memory-0", 1, map[string]any{"content": "same"})
	if err != nil || ref == "" || before != jsonString(index) {
		t.Fatalf("same offered reference should replay, not fail capacity: ref=%s error=%v", ref, err)
	}
}

func TestActionOutcomeRetainsSelectedReferencesAndResultingState(t *testing.T) {
	for _, contentBytes := range []int{100, 6000} {
		t.Run(fmt.Sprintf("snapshot_bytes_%d", contentBytes), func(t *testing.T) {
			projection := ContextProjection{FluctlightID: "self", OwnerActorID: "owner"}
			for i := 0; i < maxDecisionInfluences+1; i++ {
				projection.Memories = append(projection.Memories, map[string]any{"id": fmt.Sprint(i), "revision": 1, "content": strings.Repeat("x", contentBytes)})
			}
			if err := buildContextReferenceIndex(&projection); err != nil {
				t.Fatal(err)
			}
			influences := make([]any, 0, maxDecisionInfluences+1)
			for _, row := range projection.Memories {
				influences = append(influences, map[string]any{"ref": row["ref"], "role": "grounds", "confidence": 1.0, "note": "actual selected evidence"})
			}
			if _, err := validateDecisionInfluences(influences, projection.ReferenceIndex); err == nil || err.Error() != "decision_influences_too_large" {
				t.Fatal("internal capacity policy changed the model influence contract", err)
			}
			decision := map[string]any{"influences": influences[:maxDecisionInfluences]}
			if _, err := freezeDecisionInfluences(decision, projection, true); err != nil {
				t.Fatal(err)
			}
			causality, err := frozenDecisionCausality(decision)
			if err != nil {
				t.Fatal(err)
			}
			references, err := actionOutcomeContextReferences(causality["context_references"])
			if err != nil {
				t.Fatal(err)
			}
			resultingState := newContextReferenceIndex(projection)
			ref, err := resultingState.add(ContextReferenceState, "self", 2, map[string]any{"revision": 2, "mood": map[string]any{"label": "calm"}})
			if err != nil {
				t.Fatal(err)
			}
			references[ref] = resultingState.ByRef[ref]
			causality["context_references"] = references
			outcomes, err := buildActionOutcomes("action", "self", "inbox", "no_op", nil, causality, nil)
			if err != nil {
				t.Fatal("Core resulting state must not push valid selected evidence over an internal capacity gate", err)
			}
			if len(outcomes) != 1 || jsonString(outcomes[0].ContextReferences) != jsonString(references) {
				t.Fatal("settlement lost or changed reference snapshots")
			}
		})
	}
}
