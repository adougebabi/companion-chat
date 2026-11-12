package core

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

// The Moment Writer receives this relationship through its ordinary context,
// without an explicitly selected evidence reference.
func TestM2MomentDefaultRelationshipContextPreservesBusinessValues(t *testing.T) {
	ctx, repo, app, owner, self, conversation := setupDirectPublicationToolTest(t, "m2-moment-default-relationship")
	const relationshipID = "private-m2-default-relationship"
	business := map[string]any{"id": "business-context-id", "source_ref": "business-context-ref", "revision": 7, "valid": false,
		"schema": map[string]any{"properties": map[string]any{"item_ids": map[string]any{"type": "array"}}}}
	provenance := map[string]any{"actor_id": owner, "source": "private-context-provenance", "evidence_refs": []any{"private-context-evidence"}}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.relationships(id,owner_fluctlight_id,target_actor_id,role,metrics,trend,summary,emotional_association,provenance,revision) VALUES($1,$2,$3,$4,'{}','stable','关系摘要',$5,$6,1)`, relationshipID, self, owner, jsonBytes(map[string]any{"label": "朋友"}), jsonBytes(business), jsonBytes(provenance)); err != nil {
		t.Fatal(err)
	}
	var request map[string]any
	app.Provider.HTTP = &http.Client{Transport: newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		request = payload
		return fakeProviderResult{Text: "今天想记录一点温暖的见闻。"}
	})}
	ctx = withFrozenProviderAssignment(ctx, conversationReplyModelRole, providerAssignment{Role: conversationReplyModelRole, BaseURL: "http://controlled.invalid", ModelID: "fixture", Timeout: 10 * time.Second})
	ctx, err := app.withMomentWriterRun(ctx, ToolExecutionRequest{AuthorizationActorID: owner, FluctlightID: self, ConversationID: conversation})
	if err != nil {
		t.Fatal(err)
	}
	service := ctx.Value(momentWriterRunKey{}).(*factOnlyMomentService)
	invocation := CapabilityInvocation{CallID: "m2-default-relationship", CapabilityName: "moment.publish", Arguments: jsonBytes(map[string]any{}),
		Metadata: InvocationMetadata{AuthorizationActorID: owner, SubjectActorID: owner, FluctlightID: self, ConversationID: conversation, OperationID: "m2-default-relationship"}}
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
	relationships := arrayValue(runtime["relationships"])
	if len(relationships) != 1 {
		t.Fatalf("default relationship context missing: %#v", relationships)
	}
	value := mapValue(relationships[0])
	if got := mapValue(value["emotional_association"]); jsonString(got) != jsonString(business) {
		t.Fatalf("default relationship business data changed: got=%s want=%s", jsonString(got), jsonString(business))
	}
	if actor := mapValue(value["target_actor"]); stringValue(actor["display_name"]) != "actor_user" {
		t.Fatalf("current relationship target was lost: %#v", actor)
	}
	wire := jsonString(relationships)
	for _, forbidden := range []string{relationshipID, owner, self, "private-context-provenance", "private-context-evidence", "target_actor_id", "profile_id"} {
		if strings.Contains(wire, forbidden) {
			t.Errorf("default relationship view exposed Core envelope %q", forbidden)
		}
	}
	var stored []byte
	if err := repo.Pool().QueryRow(ctx, `SELECT emotional_association FROM public.relationships WHERE id=$1`, relationshipID).Scan(&stored); err != nil || jsonString(decodeObject(stored)) != jsonString(business) {
		t.Fatalf("relationship source changed: %s err=%v", stored, err)
	}
}

func TestM2MomentDefaultRelationshipContextDoesNotGuessUnknownActor(t *testing.T) {
	projection := ContextProjection{
		Actors: []map[string]any{{"actor_id": "known-private-actor", "ref": "actor_user", "type": "human"}},
		Relationships: []map[string]any{
			{"id": "private-first", "target_actor_id": "unknown-private-actor", "role": map[string]any{"label": "朋友"}, "emotional_association": map[string]any{"source_ref": "business-ref", "valid": false}},
			{"id": "private-second", "target_actor_id": "known-private-actor", "role": map[string]any{"label": "同伴"}},
		},
	}
	before := jsonString(projection)
	values := momentReplyRelationshipContext(projection)
	if len(values) != 2 || len(mapValue(values[0]["target_actor"])) != 0 || stringValue(mapValue(values[1]["target_actor"])["display_name"]) != "actor_user" {
		t.Fatalf("unknown actor was guessed or relationship rows changed: %#v", values)
	}
	if association := mapValue(values[0]["emotional_association"]); association["source_ref"] != "business-ref" || association["valid"] != false {
		t.Fatalf("business association was changed: %#v", association)
	}
	wire := jsonString(values)
	for _, forbidden := range []string{"unknown-private-actor", "known-private-actor", "private-first", "private-second"} {
		if strings.Contains(wire, forbidden) {
			t.Errorf("Core identity leaked: %q", forbidden)
		}
	}
	if jsonString(projection) != before {
		t.Fatal("relationship context mutated its source projection")
	}
}
