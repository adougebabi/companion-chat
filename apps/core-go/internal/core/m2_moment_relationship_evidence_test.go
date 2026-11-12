package core

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

// This exercises the real scoped reference resolution and Moment preparation;
// only the downstream Writer Provider is controlled.
func TestM2MomentRelationshipEvidencePreservesBusinessValues(t *testing.T) {
	ctx, repo, app, owner, self, conversation := setupDirectPublicationToolTest(t, "m2-moment-relationship")
	config := defaultConversationReplySettings()
	config.Mode = "fact_only"
	config.RetryLimit = 0
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.runtime_settings(key,value_json) VALUES($1,$2) ON CONFLICT(key) DO UPDATE SET value_json=EXCLUDED.value_json`, conversationReplySettingKey, jsonString(config)); err != nil {
		t.Fatal(err)
	}
	const relationshipID = "private-m2-relationship-record"
	business := map[string]any{
		"id": "business-relationship-id", "source_ref": "business-source-ref", "revision": 7, "valid": false,
		"schema": map[string]any{"properties": map[string]any{"item_ids": map[string]any{"type": "array"}}},
	}
	provenance := map[string]any{"actor_id": owner, "source": "private-core-provenance", "evidence_refs": []any{"private-core-evidence"}}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.relationships(id,owner_fluctlight_id,target_actor_id,role,metrics,trend,summary,emotional_association,provenance,revision) VALUES($1,$2,$3,$4,'{}','stable','关系摘要',$5,$6,1)`, relationshipID, self, owner, jsonBytes(map[string]any{"label": "朋友"}), jsonBytes(business), jsonBytes(provenance)); err != nil {
		t.Fatal(err)
	}
	projection, err := app.BuildContextProjectionFor(ctx, ContextProjectionRequest{AuthorizationActorID: owner, SpeakerActorID: owner, FluctlightID: self, ConversationID: conversation, MemoryOperation: MemoryForConversation, MemoryConversationMode: MemoryConversationExact})
	if err != nil {
		t.Fatal(err)
	}
	ref := ""
	for key, entry := range projection.ReferenceIndex.ByRef {
		if entry.Kind == ContextReferenceRelationship && entry.EntityID == relationshipID {
			ref = key
			break
		}
	}
	if ref == "" {
		t.Fatal("current relationship was not offered in the scoped Core index")
	}
	var request map[string]any
	app.Provider.HTTP = &http.Client{Transport: newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		request = payload
		return fakeProviderResult{Text: "今天想记录一点温暖的见闻。"}
	})}
	ctx = withFrozenProviderAssignment(ctx, conversationReplyModelRole, providerAssignment{Role: conversationReplyModelRole, BaseURL: "http://controlled.invalid", ModelID: "fixture", Timeout: 10 * time.Second})
	ctx, err = app.withMomentWriterRun(ctx, ToolExecutionRequest{AuthorizationActorID: owner, FluctlightID: self, ConversationID: conversation})
	if err != nil {
		t.Fatal(err)
	}
	service := ctx.Value(momentWriterRunKey{}).(*factOnlyMomentService)
	invocation := CapabilityInvocation{CallID: "m2-relationship-evidence", CapabilityName: "moment.publish", Arguments: jsonBytes(map[string]any{"evidence_refs": []any{ref}}),
		Metadata:        InvocationMetadata{Source: "model_tool", AuthorizationActorID: owner, SubjectActorID: owner, FluctlightID: self, ConversationID: conversation, OperationID: "m2-relationship-evidence"},
		ContextSnapshot: map[string]any{"context_reference_index": projection.ReferenceIndex}}
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
	if len(facts) != 1 || stringValue(mapValue(facts[0])["kind"]) != string(ContextReferenceRelationship) {
		t.Fatalf("selected relationship evidence did not reach the Writer: %#v", facts)
	}
	value := mapValue(mapValue(facts[0])["value"])
	if got := mapValue(value["emotional_association"]); jsonString(got) != jsonString(business) {
		t.Fatalf("relationship business data changed: got=%s want=%s", jsonString(got), jsonString(business))
	}
	if target := mapValue(value["target_actor"]); stringValue(target["actor"]) != "actor_user" {
		t.Fatalf("relationship target was not projected as the current user: %#v", target)
	}
	wire := jsonString(facts)
	for _, forbidden := range []string{relationshipID, owner, self, "private-core-provenance", "private-core-evidence", "target_actor_id", "profile_id"} {
		if strings.Contains(wire, forbidden) {
			t.Errorf("relationship evidence exposed Core envelope %q", forbidden)
		}
	}
	var stored []byte
	if err := repo.Pool().QueryRow(ctx, `SELECT emotional_association FROM public.relationships WHERE id=$1`, relationshipID).Scan(&stored); err != nil || jsonString(decodeObject(stored)) != jsonString(business) {
		t.Fatalf("relationship source changed: %s err=%v", stored, err)
	}
}

func TestM2MomentRelationshipEvidenceKeepsUnknownTargetUnknown(t *testing.T) {
	row := map[string]any{
		"id": "private-relationship", "target_actor_id": "unknown-private-actor", "target_actor_type": "human",
		"role": map[string]any{"label": "朋友"}, "emotional_association": map[string]any{"source_ref": "business-ref", "valid": false},
	}
	before := jsonString(row)
	entry := ContextReference{Kind: ContextReferenceRelationship, Snapshot: jsonBytes(row)}
	fact := momentReplyEvidenceFact(entry, ContextProjection{})
	facts := momentReplyToolFacts([]map[string]any{fact})
	if len(facts) != 1 {
		t.Fatalf("relationship evidence missing: %#v", facts)
	}
	value := mapValue(facts[0]["value"])
	if target := mapValue(value["target_actor"]); target["actor"] != "unknown" || target["type"] != "human" {
		t.Fatalf("unknown target was guessed or lost its known type: %#v", target)
	}
	if association := mapValue(value["emotional_association"]); association["source_ref"] != "business-ref" || association["valid"] != false {
		t.Fatalf("business association was changed: %#v", association)
	}
	if strings.Contains(jsonString(facts), "unknown-private-actor") || jsonString(row) != before {
		t.Fatal("unknown Core actor leaked or source was mutated")
	}
}
