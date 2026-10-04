package core

import (
	"fmt"
	"github.com/jackc/pgx/v5"
	"testing"
	"time"
)

func actorFactSourceMessage(t *testing.T, f independentToolE2EFixture, key, author, text string) string {
	t.Helper()
	id := "actor-source-" + key + f.suffix
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.conversation_messages(id,conversation_id,sequence,author_actor_id,kind,text,attachment_refs,idempotency_key) SELECT $1::text,$2::text,COALESCE(max(sequence),0)+1,$3::text,'user',$4::text,'[]',$1::text FROM public.conversation_messages WHERE conversation_id=$2::text`, id, f.conversationID, author, text); err != nil {
		t.Fatal(err)
	}
	return id
}
func recordActorFact(t *testing.T, f independentToolE2EFixture, key, operation, attribute string, value any, source string) string {
	t.Helper()
	args := map[string]any{"operation": operation, "attribute": attribute, "value": value, "reason": "明确人物事实及纠正"}
	if source != "" {
		args["source_message_id"] = source
	}
	receipt, err := f.app.ExecuteTool(f.ctx, f.request(actorFactCapabilityName, key, args))
	if err != nil || receipt.Result.Status != "completed" {
		t.Fatalf("record %s: %#v %v", key, receipt, err)
	}
	return stringValue(mapValue(receipt.Result.Output)["fact_id"])
}
func TestActorCorrectionPersistsAndRetiresOnlySourcedDerivations(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	old := recordActorFact(t, f, "old-location", "assert", "location_scope", "same_building", "")
	// The unchanged Raw message is historical evidence, not a second authority.
	rawMessage := actorFactSourceMessage(t, f, "old-interaction", f.ownerID, "我想见你")
	correctedMessage := actorFactSourceMessage(t, f, "correction", f.ownerID, "我人在国外，并不在同一栋楼")
	memory, err := f.app.ExecuteTool(f.ctx, f.request("memory_event", "old-derived-memory", map[string]any{"content": "用户就在同楼", "type": "semantic", "confidence": 0.9, "importance": 0.9, "actor_fact_ids": []any{old}}))
	if err != nil {
		t.Fatal(err)
	}
	memoryID := stringValue(mapValue(memory.Result.Output)["memory_id"])
	replacement := recordActorFact(t, f, "location-correction", "correct", "location_scope", "abroad", correctedMessage)
	query, err := f.app.ExecuteTool(f.ctx, f.request(actorInspectCapabilityName, "current-actor", map[string]any{}))
	if err != nil {
		t.Fatal(err)
	}
	facts := arrayValue(mapValue(query.Result.Output)["facts"])
	if len(facts) != 1 || mapValue(facts[0])["value"] != "abroad" {
		t.Fatalf("current facts %#v", facts)
	}
	var memoryStatus, text string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT provenance_status FROM public.memories WHERE id=$1`, memoryID).Scan(&memoryStatus); err != nil || memoryStatus != "invalid" {
		t.Fatalf("dependent memory %s %v", memoryStatus, err)
	}
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT text FROM public.conversation_messages WHERE id=$1`, rawMessage).Scan(&text); err != nil || text != "我想见你" {
		t.Fatalf("raw interaction deleted: %s %v", text, err)
	}
	// A late writer cannot resurrect an old assertion after correction.
	err = withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		return attachActorFactArtifactTx(f.ctx, tx, f.fluctlightID, old, "memory", memoryID, 0, f.app.now())
	})
	if err == nil {
		t.Fatal("late stale source linked")
	}
	restarted := &App{DB: f.repository, Provider: f.app.Provider}
	query, err = restarted.ExecuteTool(f.ctx, f.request(actorInspectCapabilityName, "after-restart", map[string]any{"history": true}))
	if err != nil {
		t.Fatal(err)
	}
	var sawCorrection, sawActive bool
	for _, raw := range arrayValue(mapValue(query.Result.Output)["facts"]) {
		fact := mapValue(raw)
		if fact["fact_id"] == old && fact["status"] == "corrected" {
			sawCorrection = true
		}
		if fact["fact_id"] == replacement && fact["status"] == "active" {
			sawActive = true
		}
	}
	if !sawCorrection || !sawActive {
		t.Fatal(query.Result.Output)
	}
}
func TestActorFactUnknownTimezoneAndLaterLocationChangeRemainDistinct(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	abroad := actorFactSourceMessage(t, f, "abroad", f.ownerID, "我目前在国外，具体时区暂时不告诉你")
	recordActorFact(t, f, "abroad-fact", "assert", "location_scope", "abroad", abroad)
	at := time.Now().UTC()
	f.app.Clock = func() time.Time { return at }
	back := actorFactSourceMessage(t, f, "return", f.ownerID, "我今天已经回国了")
	at = at.Add(time.Minute)
	recordActorFact(t, f, "return-fact", "change", "location_scope", "domestic", back)
	query, err := f.app.ExecuteTool(f.ctx, f.request(actorInspectCapabilityName, "location-history", map[string]any{"history": true}))
	if err != nil {
		t.Fatal(err)
	}
	var oldHistorical bool
	for _, raw := range arrayValue(mapValue(query.Result.Output)["facts"]) {
		fact := mapValue(raw)
		if fact["value"] == "abroad" && fact["status"] == "active" && fact["valid_until"] != nil {
			oldHistorical = true
		}
		if fact["attribute"] == "timezone" {
			t.Fatal("abroad inferred timezone")
		}
	}
	if !oldHistorical {
		t.Fatal(query.Result.Output)
	}
}
func TestActorFactForeignSourceCannotConfirmOrCorrectTheSubject(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	source := actorFactSourceMessage(t, f, "self-utterance", f.fluctlightID, "用户也许在国外")
	receipt, err := f.app.ExecuteTool(f.ctx, f.request(actorFactCapabilityName, "self-inference", map[string]any{"operation": "assert", "attribute": "location_scope", "value": "abroad", "source_message_id": source, "reason": "推测"}))
	if err != nil || mapValue(receipt.Result.Output)["status"] != "uncertain" {
		t.Fatalf("inference %#v %v", receipt, err)
	}
	receipt, err = f.app.ExecuteTool(f.ctx, f.request(actorFactCapabilityName, "self-correction", map[string]any{"operation": "correct", "attribute": "location_scope", "value": "abroad", "source_message_id": source, "reason": "错误归属"}))
	if err == nil || receipt.Result.ErrorCode != "actor_inference_cannot_correct" {
		t.Fatalf("self statement upgraded %#v %v", receipt, err)
	}
}

func TestActorCorrectionCannotReturnThroughResidentActiveSelfOrLateSnapshot(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	old := recordActorFact(t, f, "lineage-old", "assert", "location_scope", "same_building", "")
	frozen, err := readActorFactsWith(f.ctx, f.repository.Pool(), f.fluctlightID, "", f.app.now(), false, 16)
	if err != nil {
		t.Fatal(err)
	}
	request := f.request("active_memory_event", "lineage-active", map[string]any{"operation": "create", "kind": "temporary_context", "content": "用户在同楼，可当面送东西", "confidence": 0.95, "importance": 0.95, "original_time_expression": "", "time_precision": "unknown", "actor_fact_ids": []any{old}})
	request.ConversationID = ""
	active, err := f.app.ExecuteTool(f.ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	confirm := f.request("active_memory_event", "lineage-confirm", map[string]any{"operation": "confirm", "target_ref": mapValue(active.Result.Output)["target_ref"]})
	confirm.ConversationID = ""
	if _, err := f.app.ExecuteTool(f.ctx, confirm); err != nil {
		t.Fatal(err)
	}
	claimID := "lineage-self-" + f.suffix
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.fluctlight_developing_self_claims(id,fluctlight_id,category,claim,value,confidence,evidence_refs,status) VALUES($1,$2,'habit','习惯在同楼送东西','{}',0.9,'[]','active')`, claimID, f.fluctlightID); err != nil {
		t.Fatal(err)
	}
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		return attachActorFactArtifactTx(f.ctx, tx, f.fluctlightID, old, "developing_self", claimID, 1, f.app.now())
	}); err != nil {
		t.Fatal(err)
	}
	correction := actorFactSourceMessage(t, f, "lineage-correction", f.ownerID, "我在国外，不能当面送东西")
	recordActorFact(t, f, "lineage-correct", "correct", "location_scope", "abroad", correction)
	restarted := &App{DB: f.repository, Provider: f.app.Provider}
	recalled, err := restarted.retrieveActiveMemories(f.ctx, ActiveMemoryQuery{AuthorizationActorID: f.ownerID, OwnerFluctlightID: f.fluctlightID, ConversationID: f.conversationID, Cue: "同楼", At: f.app.now(), Limit: 30})
	if err != nil || len(recalled.Items) != 0 {
		t.Fatalf("Active returned retired assertion %#v %v", recalled, err)
	}
	resident, err := restarted.readResidentMemorySnapshot(f.ctx, f.ownerID, f.ownerID, f.fluctlightID, f.app.now())
	if err != nil || len(resident.Active) != 0 {
		t.Fatalf("Resident returned retired assertion %#v %v", resident, err)
	}
	self, err := restarted.listDevelopingSelfClaims(f.ctx, f.fluctlightID)
	if err != nil || len(self) != 0 {
		t.Fatalf("Self returned retired assertion %#v %v", self, err)
	}
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		return validateActorFactSnapshotTx(f.ctx, tx, f.fluctlightID, frozen, f.app.now())
	}); err == nil {
		t.Fatal("late model using old Actor snapshot could commit")
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.conversation_messages SET text=text WHERE id=$1`, correction); err != nil {
		t.Fatal(err)
	}
	var status string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT status FROM public.actor_facts WHERE source_message_id=$1`, correction).Scan(&status); err != nil || status != "active" {
		t.Fatalf("no-op edit invalidated source: %s %v", status, err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.conversation_messages SET text='修订后的自述' WHERE id=$1`, correction); err != nil {
		t.Fatal(err)
	}
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT status FROM public.actor_facts WHERE source_message_id=$1`, correction).Scan(&status); err != nil || status != "quarantined" {
		t.Fatalf("edited source remained current: %s %v", status, err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `DELETE FROM public.conversation_messages WHERE id=$1`, correction); err != nil {
		t.Fatal(err)
	}
}

func TestActorBackgroundPrioritizesExplicitLocationAndTimezoneWithoutInventingSpeaker(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	recordActorFact(t, f, "priority-location", "assert", "location_scope", "abroad", "")
	recordActorFact(t, f, "priority-zone", "assert", "timezone", "America/Los_Angeles", "")
	for i := 0; i < 20; i++ {
		recordActorFact(t, f, fmt.Sprint("preference-", i), "assert", fmt.Sprint("preference_", i), "optional", "")
	}
	projection, err := f.app.BuildContextProjectionFor(f.ctx, ContextProjectionRequest{AuthorizationActorID: f.ownerID, TargetActorID: f.ownerID, TriggerSource: "native_event", FluctlightID: f.fluctlightID, ConversationID: f.conversationID, MemoryOperation: MemoryForNativeCognition, MemoryConversationMode: MemoryConversationGlobalOnly})
	if err != nil {
		t.Fatal(err)
	}
	view := projectionTimeView(projection)
	if len(projection.CurrentSpeaker) != 0 || view["actor_user_timezone"] != "America/Los_Angeles" {
		t.Fatalf("trigger invented speaker or lost explicit timezone %#v %#v", projection.CurrentSpeaker, view)
	}
	found := false
	for _, fact := range compactActorBackground(projection) {
		if fact["attribute"] == "location_scope" && fact["value"] == "abroad" {
			found = true
		}
	}
	if !found {
		t.Fatal("necessary background displaced by optional preferences")
	}
}

func TestActorSourceEditWithdrawsCognitionEvidenceWithoutErasingRawAdmission(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	source := actorFactSourceMessage(t, f, "withdrawn-admission", f.ownerID, "我住在国外")
	factID := "withdrawn-fact-" + f.suffix
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.cognition_inbox(id,fluctlight_id,sequence,event_type,payload,causation_id,correlation_id,idempotency_key,occurred_at,status) VALUES($1,$2,1000,'conversation.turn',$3,$1,$1,$1,now(),'processed')`, factID, f.fluctlightID, jsonBytes(map[string]any{"text": "我住在国外", "conversation_id": f.conversationID, "actor_id": f.ownerID})); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.conversation_messages SET source_fact_id=$2 WHERE id=$1`, source, factID); err != nil {
		t.Fatal(err)
	}
	recordActorFact(t, f, "withdrawn-location", "assert", "location_scope", "abroad", source)
	var fingerprint string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT public.cognition_source_fingerprint(payload) FROM public.cognition_inbox WHERE id=$1`, factID).Scan(&fingerprint); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.conversation_messages SET text='我撤回这个说法' WHERE id=$1`, source); err != nil {
		t.Fatal(err)
	}
	var rawText string
	var invalidated bool
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT payload->>'text',(payload->>'source_message_invalidated')::boolean FROM public.cognition_inbox WHERE id=$1`, factID).Scan(&rawText, &invalidated); err != nil || rawText != "我住在国外" || !invalidated {
		t.Fatalf("source audit lost or not withdrawn %s %v %v", rawText, invalidated, err)
	}
	var live bool
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT public.memory_source_is_live('fact',$1,NULL,$2)`, factID, fingerprint).Scan(&live); err != nil || live {
		t.Fatalf("withdrawn original source remained current %v %v", live, err)
	}
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT public.memory_source_is_live('fact',id,NULL,public.cognition_source_fingerprint(payload)) FROM public.cognition_inbox WHERE id=$1`, factID).Scan(&live); err != nil || live {
		t.Fatalf("new fingerprint resurrected withdrawal %v %v", live, err)
	}
}

func TestActorCorrectionRejectsUnrelatedAttributeAtomically(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	location := recordActorFact(t, f, "cross-location", "assert", "location_scope", "same_building", "")
	zone := recordActorFact(t, f, "cross-zone", "assert", "timezone", "Asia/Shanghai", "")
	receipt, err := f.app.ExecuteTool(f.ctx, f.request(actorFactCapabilityName, "cross-correct", map[string]any{"operation": "correct", "attribute": "location_scope", "value": "abroad", "reason": "纠正", "corrected_fact_ids": []any{zone}}))
	if err == nil || receipt.Result.ErrorCode != "actor_correction_target_invalid" {
		t.Fatalf("unrelated correction %#v %v", receipt, err)
	}
	var count int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.actor_facts WHERE id=ANY($1) AND status='active' AND revision=1`, []string{location, zone}).Scan(&count); err != nil || count != 2 {
		t.Fatalf("atomic correction count=%d err=%v", count, err)
	}
}

func TestActorCorrectionFiltersEvolutionOverlayAfterReload(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	old := recordActorFact(t, f, "overlay-location", "assert", "location_scope", "same_building", "")
	baseline := PersonaEvolutionState{FluctlightID: f.fluctlightID, ProfileID: "default", ProfileRef: "personality:ctx_" + stableDigest("overlay-test"), BehaviorPolicy: map[string]any{}, Personality: map[string]any{"traits": map[string]any{"openness": 0.5}}}
	decision, err := CompileEvolutionOverlay(baseline, EvolutionOverlayRequest{Candidate: ReflectionOverlayCandidateV2{FieldPath: "traits.openness", Direction: "increase", Strength: 1, Confidence: 0.9, EvidenceRefs: []string{"fact:a", "fact:b"}}, EvidenceWindows: []string{"window-a", "window-b"}, OccurredAt: f.app.now()}, EvolutionOverlayPolicy{})
	if err != nil || decision.Overlay == nil {
		t.Fatalf("decision %#v %v", decision, err)
	}
	after, err := ApplyEvolutionOverlay(baseline, decision)
	if err != nil {
		t.Fatal(err)
	}
	err = withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		if _, err := persistEvolutionOverlayTx(f.ctx, tx, baseline, after, *decision.Overlay); err != nil {
			return err
		}
		return attachActorFactArtifactTx(f.ctx, tx, f.fluctlightID, old, "evolution_overlay", decision.Overlay.ID, decision.Overlay.Revision, f.app.now())
	})
	if err != nil {
		t.Fatal(err)
	}
	recordActorFact(t, f, "overlay-correction", "correct", "location_scope", "abroad", "")
	loaded, err := loadPersonaEvolutionState(f.ctx, f.repository.Pool(), baseline)
	if err != nil || len(loaded.Overlays) != 1 || loaded.Overlays[0].Status != OverlaySuperseded || portraitOverlayRevision(loaded) != 0 {
		t.Fatalf("stale overlay %#v %v", loaded, err)
	}
	var status string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT status FROM public.fluctlight_evolution_overlays WHERE id=$1`, decision.Overlay.ID).Scan(&status); err != nil || status != "active" {
		t.Fatalf("audit lost %s %v", status, err)
	}
}
