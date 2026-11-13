package core

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestMomentSelectedAppearanceKeepsBusinessValuesAndRemovesProvenEventIDs(t *testing.T) {
	for _, selected := range []bool{true, false} {
		name := "selected"
		if !selected {
			name = "default"
		}
		t.Run(name, func(t *testing.T) { exerciseMomentAppearanceEvidence(t, selected) })
	}
}

func exerciseMomentAppearanceEvidence(t *testing.T, selected bool) {
	t.Helper()
	ctx, repo, app, owner, self, conversation := setupDirectPublicationToolTest(t, "appearance-evidence")
	business := map[string]any{
		"id": "owner-injury-label", "source_ref": "owner-source", "revision": 7,
		"valid": false, "count": 0, "unknown": nil,
		"schema": map[string]any{"properties": map[string]any{"item_ids": map[string]any{"type": "array"}}},
	}
	fields := map[string]any{
		"hair_length": map[string]any{"status": "unknown"},
		"hair_style":  map[string]any{"status": "cleared"},
		"injuries":    map[string]any{"status": "known", "value": []any{business, "Owner 提供的旧伤描述"}},
	}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.fluctlight_appearance_states SET state_json=$2 WHERE fluctlight_id=$1`, self, jsonBytes(fields)); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	life := currentLifeForTest(t, ctx, app, self, now)
	created, err := app.CreateLifeEvent(ctx, owner, self, map[string]any{
		"kind": "body_injury", "start_at": now.Add(-time.Minute).Format(time.RFC3339Nano), "end_at": now.Add(time.Minute).Format(time.RFC3339Nano),
		"activity": "脚踝扭伤", "evidence_refs": []any{"owner:isolated-test"}, "idempotency_key": "appearance-evidence-injury",
		"expected_life_context_revision": life["context_revision"],
		"body_effect":                    map[string]any{"body_part": "ankle", "description": "脚踝扭伤", "severity": "moderate", "impact": "mobility_limited"},
	})
	if err != nil {
		t.Fatal(err)
	}
	injuryID := stringValue(created["id"])
	life = currentLifeForTest(t, ctx, app, self, time.Now().UTC())
	recovered, err := app.CreateLifeEvent(ctx, owner, self, map[string]any{
		"kind": "body_recovery", "start_at": now.Add(-time.Second).Format(time.RFC3339Nano), "end_at": now.Add(time.Minute).Format(time.RFC3339Nano),
		"activity": "恢复脚踝活动", "evidence_refs": []any{"owner:isolated-recovery"}, "idempotency_key": "appearance-evidence-recovery",
		"expected_life_context_revision": life["context_revision"], "body_effect": map[string]any{"injury_event_id": injuryID},
	})
	if err != nil || injuryID == "" || stringValue(recovered["id"]) == "" {
		t.Fatalf("body event fixture failed: %v", err)
	}
	var original []byte
	if err := repo.Pool().QueryRow(ctx, `SELECT state_json FROM public.fluctlight_appearance_states WHERE fluctlight_id=$1`, self).Scan(&original); err != nil {
		t.Fatal(err)
	}
	projection, err := app.BuildContextProjectionFor(ctx, ContextProjectionRequest{AuthorizationActorID: owner, SpeakerActorID: owner, FluctlightID: self, ConversationID: conversation, MemoryOperation: MemoryForConversation, MemoryConversationMode: MemoryConversationExact})
	if err != nil {
		t.Fatal(err)
	}
	ref := stringValue(projection.EffectiveAppearance["ref"])
	if ref == "" || projection.ReferenceIndex.ByRef[ref].Kind != ContextReferenceAppearance {
		t.Fatal("appearance was not offered in the real Core reference index")
	}
	var request map[string]any
	app.Provider.HTTP = &http.Client{Transport: newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		request = payload
		return fakeProviderResult{Text: "今天想记下身体慢慢恢复的变化。"}
	})}
	ctx = withFrozenProviderAssignment(ctx, conversationReplyModelRole, providerAssignment{Role: conversationReplyModelRole, BaseURL: "http://controlled.invalid", ModelID: "fixture", Timeout: 10 * time.Second})
	ctx, err = app.withMomentWriterRun(ctx, ToolExecutionRequest{AuthorizationActorID: owner, FluctlightID: self, ConversationID: conversation})
	if err != nil {
		t.Fatal(err)
	}
	service := ctx.Value(momentWriterRunKey{}).(*factOnlyMomentService)
	refs := []any{}
	if selected {
		refs = append(refs, ref)
	}
	invocation := CapabilityInvocation{CallID: "appearance-evidence", CapabilityName: "moment.publish", Arguments: jsonBytes(map[string]any{"evidence_refs": refs}), Metadata: InvocationMetadata{Source: "model_tool", AuthorizationActorID: owner, SubjectActorID: owner, FluctlightID: self, ConversationID: conversation, OperationID: "appearance-evidence"}, ContextSnapshot: map[string]any{"context_reference_index": projection.ReferenceIndex}}
	if _, err := service.prepare(ctx, invocation); err != nil {
		t.Fatal(err)
	}
	var runtime map[string]any
	for _, raw := range arrayValue(request["messages"]) {
		content := stringValue(mapValue(raw)["content"])
		if strings.HasPrefix(content, "[RUNTIME CONTEXT]\n") {
			text := strings.TrimSuffix(strings.TrimPrefix(content, "[RUNTIME CONTEXT]\n"), "\n[/RUNTIME CONTEXT]")
			if err := json.Unmarshal([]byte(text), &runtime); err != nil {
				t.Fatal(err)
			}
		}
	}
	facts := arrayValue(runtime["tool_facts"])
	state := mapValue(mapValue(runtime["current_state"])["data"])
	value := mapValue(state["appearance"])
	if selected {
		if len(facts) != 1 || mapValue(facts[0])["kind"] != string(ContextReferenceAppearance) {
			t.Fatalf("selected appearance did not reach the physical Writer request: %#v", facts)
		}
		if _, exists := state["appearance"]; exists {
			t.Fatal("same selected appearance remained duplicated in default runtime")
		}
		value = mapValue(mapValue(facts[0])["value"])
	}
	body := mapValue(value["body_fields"])
	injuries := arrayValue(mapValue(body["injuries"])["value"])
	if len(injuries) != 3 || jsonString(injuries[0]) != jsonString(business) || injuries[1] != "Owner 提供的旧伤描述" {
		t.Fatalf("appearance business value changed: %s", jsonString(injuries))
	}
	if mapValue(body["hair_length"])["status"] != "unknown" || mapValue(body["hair_style"])["status"] != "cleared" || mapValue(injuries[2])["status"] != "recovered" {
		t.Fatalf("appearance states were lost: %s", jsonString(body))
	}
	for _, forbidden := range []string{injuryID, stringValue(recovered["id"]), self, "body_revision", "wardrobe_revision"} {
		if strings.Contains(jsonString(value), forbidden) {
			t.Errorf("selected appearance exposed Core metadata %q", forbidden)
		}
	}
	var after []byte
	if err := repo.Pool().QueryRow(ctx, `SELECT state_json FROM public.fluctlight_appearance_states WHERE fluctlight_id=$1`, self).Scan(&after); err != nil || jsonString(decodeObject(after)) != jsonString(decodeObject(original)) {
		t.Fatalf("appearance source mutated: %v", err)
	}
}

func TestMomentInjuryEvidenceRequiresMatchingProvenance(t *testing.T) {
	effect := map[string]any{"operation": "injury", "body_part": "ankle", "description": "扭伤", "severity": "minor", "impact": "pain"}
	events := map[string]map[string]any{"confirmed-injury": {"kind": "body_injury", "effect": effect}}
	for _, injury := range []map[string]any{
		{"id": "owner-label", "revision": 0, "valid": false},
		{"id": "confirmed-injury", "status": "active", "description": "Owner 的不同描述"},
		{"id": "confirmed-injury", "status": "unknown", "body_part": "ankle", "description": "扭伤", "severity": "minor", "impact": "pain"},
	} {
		before := jsonString(injury)
		if got := momentInjuryEvidenceValue(injury, events); jsonString(got) != before || jsonString(injury) != before {
			t.Fatalf("unproven business injury changed: %s", jsonString(got))
		}
	}
}

func TestMomentAppearanceEvidenceDoesNotUseOtherCharactersBodyEvents(t *testing.T) {
	ctx, repo, app, _, self, _ := setupDirectPublicationToolTest(t, "appearance-foreign-event")
	effect := map[string]any{"operation": "injury", "body_part": "ankle", "description": "扭伤", "severity": "minor", "impact": "pain"}
	seedTurnConversation(t, ctx, repo, "appearance-other-owner", "appearance-other-character", "appearance-other-conversation")
	seedControlledCurrentCapture(t, ctx, repo, "appearance-other-character")
	now := time.Now().UTC()
	life := currentLifeForTest(t, ctx, app, "appearance-other-character", now)
	other, err := app.CreateLifeEvent(ctx, "appearance-other-owner", "appearance-other-character", map[string]any{
		"kind": "body_injury", "start_at": now.Add(-time.Minute).Format(time.RFC3339Nano), "end_at": now.Add(time.Minute).Format(time.RFC3339Nano),
		"activity": "扭伤", "evidence_refs": []any{"owner:isolated-other"}, "idempotency_key": "appearance-other-event",
		"expected_life_context_revision": life["context_revision"], "body_effect": effect,
	})
	if err != nil {
		t.Fatal(err)
	}
	injury := copyPromptFields(effect, "body_part", "description", "severity", "impact")
	injury["id"], injury["status"] = other["id"], "active"
	row := map[string]any{"body_fields": map[string]any{"injuries": map[string]any{"status": "known", "value": []any{injury}}}}
	fact, err := newFactOnlyReplyService(app).appearanceEvidenceFact(ctx, ContextReference{Kind: ContextReferenceAppearance, EntityID: self, Snapshot: jsonBytes(row)}, ContextProjection{FluctlightID: self})
	if err != nil {
		t.Fatal(err)
	}
	items := arrayValue(mapValue(mapValue(mapValue(fact["value"])["body_fields"])["injuries"])["value"])
	if len(items) != 1 || jsonString(items[0]) != jsonString(injury) {
		t.Fatalf("other character's event changed this character's business data: %s", jsonString(items))
	}
}
