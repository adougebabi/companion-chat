package core

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

// Two distinct Core relationships deliberately share the same business value.
// Only the identical selected reference may replace its default context row.
func TestM2MomentRelationshipEvidenceDeduplicatesOnlyExactReference(t *testing.T) {
	for _, scenario := range []string{"selected_twice", "invalid_reference", "stale_reference"} {
		t.Run(scenario, func(t *testing.T) {
			ctx, repo, app, owner, self, _ := setupDirectPublicationToolTest(t, "m2-moment-dedup-"+scenario)
			other := "private-other-actor-" + scenario
			if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES($1,'human','active')`, other); err != nil {
				t.Fatal(err)
			}
			business := map[string]any{"note": "SHARED_RELATIONSHIP_BUSINESS", "source_ref": "business-ref", "valid": false}
			firstID, secondID := "private-first-"+scenario, "private-second-"+scenario
			for _, relation := range []struct{ id, target string }{{firstID, owner}, {secondID, other}} {
				if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.relationships(id,owner_fluctlight_id,target_actor_id,role,metrics,trend,emotional_association,revision) VALUES($1,$2,$3,$4,'{}','stable',$5,1)`, relation.id, self, relation.target, jsonBytes(map[string]any{"label": "朋友"}), jsonBytes(business)); err != nil {
					t.Fatal(err)
				}
			}
			projection, err := app.BuildContextProjectionFor(ctx, ContextProjectionRequest{AuthorizationActorID: owner, SpeakerActorID: owner, FluctlightID: self, MemoryOperation: MemoryForConversation, MemoryConversationMode: MemoryConversationGlobalOnly})
			if err != nil {
				t.Fatal(err)
			}
			ref := ""
			for key, entry := range projection.ReferenceIndex.ByRef {
				if entry.Kind == ContextReferenceRelationship && entry.EntityID == firstID {
					ref = key
					break
				}
			}
			if ref == "" {
				t.Fatal("selected relationship was not in the scoped Core index")
			}
			refs := []any{ref, ref}
			if scenario == "invalid_reference" {
				refs = []any{"relationship:ctx_00000000000000000000000000000000"}
			}
			if scenario == "stale_reference" {
				if _, err := repo.Pool().Exec(ctx, `UPDATE public.relationships SET revision=revision+1 WHERE id=$1`, firstID); err != nil {
					t.Fatal(err)
				}
				refs = []any{ref}
			}
			var request map[string]any
			app.Provider.HTTP = &http.Client{Transport: newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
				request = payload
				return fakeProviderResult{Text: "今天想记录一点温暖的见闻。"}
			})}
			ctx = withFrozenProviderAssignment(ctx, conversationReplyModelRole, providerAssignment{Role: conversationReplyModelRole, BaseURL: "http://controlled.invalid", ModelID: "fixture", Timeout: 10 * time.Second})
			ctx, err = app.withMomentWriterRun(ctx, ToolExecutionRequest{AuthorizationActorID: owner, FluctlightID: self})
			if err != nil {
				t.Fatal(err)
			}
			service := ctx.Value(momentWriterRunKey{}).(*factOnlyMomentService)
			invocation := CapabilityInvocation{CallID: "m2-dedup-" + scenario, CapabilityName: "moment.publish", Arguments: jsonBytes(map[string]any{"evidence_refs": refs}),
				Metadata:        InvocationMetadata{Source: "model_tool", AuthorizationActorID: owner, SubjectActorID: owner, FluctlightID: self, OperationID: "m2-dedup-" + scenario},
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
			wire := jsonString(runtime)
			if count := strings.Count(wire, "SHARED_RELATIONSHIP_BUSINESS"); count != 2 {
				t.Fatalf("two distinct relationships should appear once each, got %d: %s", count, wire)
			}
			if strings.Count(wire, "business-ref") != 2 || strings.Count(wire, `"valid":false`) != 2 {
				t.Fatalf("relationship business values were not retained once per source: %s", wire)
			}
			facts := arrayValue(runtime["tool_facts"])
			relations := arrayValue(runtime["relationships"])
			if scenario == "selected_twice" && (len(facts) != 1 || len(relations) != 1) {
				t.Fatalf("selected relationship did not replace only its own default row: facts=%#v relationships=%#v", facts, relations)
			}
			if scenario == "selected_twice" && stringValue(mapValue(mapValue(mapValue(facts[0])["value"])["target_actor"])["actor"]) != "actor_user" {
				t.Fatalf("selected relationship lost its target: %#v", facts)
			}
			if scenario != "selected_twice" && (len(facts) != 0 || len(relations) != 2) {
				t.Fatalf("invalid or stale reference removed or created a relationship: facts=%#v relationships=%#v", facts, relations)
			}
			for _, forbidden := range []string{firstID, secondID, other, ref, "_core_evidence_ref"} {
				if strings.Contains(wire, forbidden) {
					t.Errorf("internal relationship identity escaped into Writer request: %q", forbidden)
				}
			}
			var count int
			if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.relationships WHERE id=ANY($1::text[])`, []string{firstID, secondID}).Scan(&count); err != nil || count != 2 {
				t.Fatalf("source relationships changed: count=%d err=%v", count, err)
			}
		})
	}
}
