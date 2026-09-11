package core

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestBuildMemoryQueryPlanSeparatesLocalCuesFromEmbeddingEgress(t *testing.T) {
	plan, err := buildMemoryQueryPlan(MemoryForWakeUp, []string{"owner"}, MemoryConversationGlobalOnly, "", nil, "default", nil, 12, 2400)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Mode != "salience_recent" || plan.Query != "" || plan.EmbeddingQuery != "" {
		t.Fatalf("empty plan=%#v", plan)
	}
	local := buildProjectionMemoryCues(MemoryForWakeUp, []MemoryQueryCue{{Kind: "fact", Text: "private fact", AllowEmbedding: true}}, "", map[string]any{"scene": "书房"}, []map[string]any{{"status": "active", "description": "完成报告"}}, nil, nil, nil)
	plan, err = buildMemoryQueryPlan(MemoryForWakeUp, []string{"owner"}, MemoryConversationGlobalOnly, "", nil, "default", local, 12, 2400)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Mode != "lexical_salience" || !strings.Contains(plan.Query, "书房") || !strings.Contains(plan.Query, "完成报告") || plan.EmbeddingQuery != "" {
		t.Fatalf("private local cues crossed embedding boundary: %#v", plan)
	}
	synthetic, err := buildMemoryQueryPlan(MemoryForConversation, []string{"owner"}, MemoryConversationExact, "conv", nil, "default", []MemoryQueryCue{{Kind: "synthetic", Text: "non-sensitive-test", AllowEmbedding: true}}, 12, 2400)
	if err != nil || synthetic.EmbeddingQuery != "non-sensitive-test" {
		t.Fatalf("explicit synthetic embedding cue=%#v err=%v", synthetic, err)
	}
	if _, err := buildMemoryQueryPlan(MemoryForConversation, []string{"owner"}, MemoryConversationExact, "", nil, "default", nil, 12, 2400); err == nil {
		t.Fatal("exact conversation plan accepted an empty conversation")
	}
}

func TestMemoryRetrievalAppliesOnlyAuthorizedCurrentVectorTuple(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID := "memory-vector-owner"
	fluctlightID := "memory-vector-fluctlight"
	endpointID := "memory-vector-endpoint"
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.provider_endpoints(id,kind,base_url,secret_purpose,capability_status,checked_at) VALUES($1,'openai_compatible','http://memory-vector.invalid','memory-vector-secret','ready',now())`, endpointID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,required_capabilities,token_budget,timeout_seconds,retry_policy) VALUES('embedding',$1,'memory-vector-model','embedding',4096,5,'{}')`, endpointID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES($1,'human','active'),($2,'fluctlight','active')`, ownerID, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) VALUES($1,$2,'blank_slate','active','{}','{}','{}','{}','{}','{}')`, fluctlightID, ownerID); err != nil {
		t.Fatal(err)
	}
	app := &App{DB: repository}
	semantic := &MemorySemanticInput{Type: "semantic", Content: "authorized vector memory", Confidence: 0.8, Importance: 0.7, EmotionalSignificance: 0.2}
	command := memoryLifecycleTestCommand(MemoryCreate, "vector-retrieval", semantic, nil, nil)
	command.OwnerFluctlightID, command.OwnerActorID, command.ActorID = fluctlightID, ownerID, fluctlightID
	command.RequestDigest = memoryCommandDigest(command)
	created := applyMemoryLifecycleTestCommand(t, ctx, app, command)
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.memory_embeddings(id,memory_id,memory_revision,provider_endpoint_id,model_id,dimensions,embedding,embedding_vector,status,embedded_at) VALUES('memory-vector-ready',$1,0,$2,'memory-vector-model',2,'[1,0]','[1,0]'::vector,'ready',now())`, created.MemoryID, endpointID); err != nil {
		t.Fatal(err)
	}
	app.Provider = &ProviderClient{DB: repository, HTTP: &http.Client{Transport: projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		return embeddingHTTPResponse(request, http.StatusOK, `{"data":[{"embedding":[1,0]}]}`), nil
	})}}
	plan, err := buildMemoryQueryPlan(MemoryForNativeCognition, []string{ownerID}, MemoryConversationGlobalOnly, "", nil, "default", []MemoryQueryCue{{Kind: "synthetic", Text: "authorized vector", AllowEmbedding: true}}, 12, 2400)
	if err != nil {
		t.Fatal(err)
	}
	result, err := app.retrieveMemoryWithPlan(ctx, ownerID, fluctlightID, plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 1 || stringValue(result.Items[0]["id"]) != created.MemoryID || result.Trace.EmbeddingDisposition != "applied" || result.Trace.VectorCandidateCount != 1 || result.Trace.FallbackReason != "" {
		t.Fatalf("vector retrieval items=%#v trace=%#v", result.Items, result.Trace)
	}
}

func TestParticipantsVisibilityDoesNotTreatFluctlightSelfAsEveryViewer(t *testing.T) {
	if memoryVisibleToActor("participants", "fl", "owner", "unrelated", []any{"fl"}) {
		t.Fatal("Fluctlight self reference authorized an unrelated viewer")
	}
	if !memoryVisibleToActor("participants", "fl", "owner", "participant", []any{"participant", "fl"}) {
		t.Fatal("explicit participant was denied")
	}
}

func TestMemoryRetrievalFiltersAudienceAndConversationBeforeCandidateLimit(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID := "memory-retrieval-owner"
	fluctlightID := "memory-retrieval-fluctlight"
	viewerID := "memory-retrieval-viewer"
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES($1,'human','active'),($2,'fluctlight','active'),($3,'fluctlight','active')`, ownerID, fluctlightID, viewerID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) VALUES($1,$2,'blank_slate','active','{}','{}','{}','{}','{}','{}'),($3,$2,'blank_slate','active','{}','{}','{}','{}','{}','{}')`, fluctlightID, ownerID, viewerID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `
		INSERT INTO public.memories(id,owner_fluctlight_id,type,content,actor_refs,conversation_id,event_refs,evidence_refs,personality_perspectives,confidence,importance,emotional_significance,visibility,status,revision,canonical_key,request_digest,created_at)
		SELECT 'private-'||value,$1,'semantic','new private distractor '||value,'[]',NULL,'[]','["fact"]','[]',0.9,1,0.5,'private','active',0,lpad(to_hex(value),32,'0'),lpad(to_hex(value+1000),32,'0'),now()+value*interval '1 second'
		FROM generate_series(1,205) value`, fluctlightID); err != nil {
		t.Fatal(err)
	}
	fixtures := []struct {
		id, content, actors, conversation, visibility string
	}{
		{"authorized-old", "relevant archive memory", `["` + viewerID + `"]`, "conv-allowed", "participants"},
		{"global-participant", "global shared memory", `["` + viewerID + `"]`, "", "participants"},
		{"wrong-conversation", "relevant but wrong conversation", `["` + viewerID + `"]`, "conv-other", "participants"},
		{"self-only", "Fluctlight self is not viewer authorization", `["` + fluctlightID + `"]`, "conv-allowed", "participants"},
		{"owner-private", "owner only memory", `[]`, "", "private"},
		{"oversized", strings.Repeat("长", 5000), `["` + viewerID + `"]`, "conv-allowed", "participants"},
	}
	for index, fixture := range fixtures {
		var conversation any
		if fixture.conversation != "" {
			conversation = fixture.conversation
		}
		canonicalKey := fmt.Sprintf("%032x", index+10000)
		requestDigest := fmt.Sprintf("%032x", index+20000)
		if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.memories(id,owner_fluctlight_id,type,content,actor_refs,conversation_id,event_refs,evidence_refs,personality_perspectives,confidence,importance,emotional_significance,visibility,status,revision,canonical_key,request_digest,created_at) VALUES($1,$2,'semantic',$3,$4::jsonb,$5,'[]','["fact"]','[]',0.9,0.8,0.2,$6,'active',0,$7,$8,$9)`, fixture.id, fluctlightID, fixture.content, fixture.actors, conversation, fixture.visibility, canonicalKey, requestDigest, time.Now().UTC().Add(-time.Duration(index+1)*time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	app := &App{DB: repository}
	plan, err := buildMemoryQueryPlan(MemoryForConversation, []string{viewerID}, MemoryConversationExact, "conv-allowed", nil, "default", []MemoryQueryCue{{Kind: "current_message", Text: "relevant archive"}}, 12, 2400)
	if err != nil {
		t.Fatal(err)
	}
	result, err := app.retrieveMemoryWithPlan(ctx, ownerID, fluctlightID, plan)
	if err != nil {
		t.Fatal(err)
	}
	ids := make(map[string]struct{}, len(result.Items))
	for _, item := range result.Items {
		ids[stringValue(item["id"])] = struct{}{}
	}
	for _, required := range []string{"authorized-old", "global-participant"} {
		if _, exists := ids[required]; !exists {
			t.Fatalf("authorized Memory %q was crowded out: items=%#v trace=%#v", required, ids, result.Trace)
		}
	}
	for _, forbidden := range []string{"wrong-conversation", "self-only", "owner-private", "oversized", "private-205"} {
		if _, exists := ids[forbidden]; exists {
			t.Fatalf("unauthorized/out-of-budget Memory %q entered ranking result: %#v", forbidden, ids)
		}
	}
	if result.Trace.AuthorizedCandidateCount != 3 || result.Trace.ConversationMode != "exact" || result.Trace.QueryDigest == "" {
		t.Fatalf("retrieval trace=%#v", result.Trace)
	}
	globalPlan, err := buildMemoryQueryPlan(MemoryForNativeCognition, []string{viewerID}, MemoryConversationGlobalOnly, "", nil, "default", nil, 12, 2400)
	if err != nil {
		t.Fatal(err)
	}
	global, err := app.retrieveMemoryWithPlan(ctx, ownerID, fluctlightID, globalPlan)
	if err != nil {
		t.Fatal(err)
	}
	if len(global.Items) != 1 || stringValue(global.Items[0]["id"]) != "global-participant" || global.Trace.Mode != "salience_recent" {
		t.Fatalf("global-only retrieval=%#v trace=%#v", global.Items, global.Trace)
	}
}
