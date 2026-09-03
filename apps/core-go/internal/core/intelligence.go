package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/jackc/pgx/v5"
)

const (
	ClaimConfirmedFact       = "confirmed_fact"
	ClaimObservedFact        = "observed_fact"
	ClaimSupportedHypothesis = "supported_hypothesis"
	ClaimUncertainHypothesis = "uncertain_hypothesis"
	ClaimUnsupportedSelf     = "unsupported_self_claim"
)

var validClaimKinds = map[string]struct{}{
	ClaimConfirmedFact: {}, ClaimObservedFact: {}, ClaimSupportedHypothesis: {},
	ClaimUncertainHypothesis: {}, ClaimUnsupportedSelf: {},
}

// ContextProjection is the only context assembled for cognition, rendering,
// Reflection, and native capability slots. It deliberately carries provenance
// alongside semantic values so model output cannot become an unowned fact.
type ContextProjection struct {
	SchemaVersion          string           `json:"schema_version"`
	FluctlightID           string           `json:"fluctlight_id"`
	ConversationID         string           `json:"conversation_id"`
	SourceFactID           string           `json:"source_fact_id"`
	CurrentUserText        string           `json:"current_user_text"`
	RecentMessages         []map[string]any `json:"recent_messages"`
	ContextRevision        int              `json:"context_revision"`
	CorePersonaRevision    int              `json:"core_persona_revision"`
	DevelopingSelfRevision int              `json:"developing_self_revision"`
	CurrentStateRevision   int              `json:"current_state_revision"`
	CorePersona            map[string]any   `json:"core_persona"`
	DevelopingSelf         []map[string]any `json:"developing_self"`
	CurrentState           map[string]any   `json:"current_state"`
	Identity               map[string]any   `json:"identity"`
	Personality            map[string]any   `json:"personality"`
	BehavioralPolicy       map[string]any   `json:"behavioral_policy"`
	InnerState             map[string]any   `json:"inner_state"`
	LifeContext            map[string]any   `json:"life_context"`
	Presence               map[string]any   `json:"presence,omitempty"`
	Memories               []map[string]any `json:"memories"`
	Relationships          []map[string]any `json:"relationships"`
	Hypotheses             []map[string]any `json:"hypotheses"`
	Capabilities           []map[string]any `json:"capabilities"`
	DriveSlots             []map[string]any `json:"drive_slots"`
	PreferenceSlots        []map[string]any `json:"preference_slots"`
	TriggerPreferences     []map[string]any `json:"trigger_preferences"`
	VisualIdentity         map[string]any   `json:"visual_identity"`
}

func contextProjectionFromValue(value any) (ContextProjection, bool) {
	data, err := json.Marshal(value)
	if err != nil {
		return ContextProjection{}, false
	}
	var projection ContextProjection
	if err := json.Unmarshal(data, &projection); err != nil || projection.FluctlightID == "" {
		return ContextProjection{}, false
	}
	return projection, true
}

type ResponsePlan struct {
	SchemaVersion    string           `json:"schema_version"`
	SourceFactID     string           `json:"source_fact_id"`
	ContextRevision  int              `json:"context_revision"`
	AnswerMode       string           `json:"answer_mode"`
	VisibleText      string           `json:"visible_text,omitempty"`
	ApprovedClaims   []map[string]any `json:"approved_claims"`
	UncertainClaims  []map[string]any `json:"uncertain_claims"`
	OmittedClaims    []map[string]any `json:"omitted_claims"`
	Outline          []any            `json:"response_outline"`
	Tone             string           `json:"tone,omitempty"`
	ToolCalls        []ToolCallV1     `json:"tool_calls"`
	NativeCandidates []map[string]any `json:"native_candidates"`
	SelfEvaluation   map[string]any   `json:"self_evaluation"`
	CoreAlignment    map[string]any   `json:"core_alignment,omitempty"`
	StateExpression  map[string]any   `json:"state_expression,omitempty"`
}

type SelfEvaluation struct {
	Mode        string   `json:"mode"`
	ReasonCodes []string `json:"reason_codes"`
	Confidence  float64  `json:"confidence"`
}

type Claim struct {
	ID            string   `json:"id,omitempty"`
	Kind          string   `json:"kind"`
	Content       string   `json:"content"`
	EvidenceRefs  []string `json:"evidence_refs"`
	Confidence    float64  `json:"confidence"`
	RepetitionKey string   `json:"repetition_key"`
	SourceFactID  string   `json:"source_fact_id"`
	ExpiresAt     string   `json:"expires_at,omitempty"`
}

// BuildContextProjection composes one bounded, provenance-carrying view for a
// turn. It is intentionally a read model; mutations go through domain owners.
func (a *App) BuildContextProjection(ctx context.Context, actorID, fluctlightID, conversationID, sourceFactID, userText string) (ContextProjection, error) {
	fluctlight, err := a.DB.GetFluctlight(ctx, fluctlightID, actorID)
	if err != nil {
		return ContextProjection{}, err
	}
	inner, err := a.readInnerState(ctx, fluctlightID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return ContextProjection{}, err
	}
	if inner == nil {
		inner = map[string]any{}
	}
	schedule, err := a.readSchedule(ctx, fluctlightID)
	if err != nil {
		return ContextProjection{}, err
	}
	lifeContext, err := a.resolveContext(ctx, fluctlightID, schedule)
	if err != nil {
		return ContextProjection{}, err
	}
	memories, err := a.RetrieveMemoryContext(ctx, actorID, fluctlightID, conversationID, userText, 12, 2400)
	if err != nil {
		return ContextProjection{}, err
	}
	relationships, err := a.readRelationships(ctx, fluctlightID)
	if err != nil {
		return ContextProjection{}, err
	}
	hypotheses, err := a.readActiveHypotheses(ctx, fluctlightID)
	if err != nil {
		return ContextProjection{}, err
	}
	driveSlots, err := a.readDriveSlots(ctx, fluctlightID)
	if err != nil {
		return ContextProjection{}, err
	}
	preferenceSlots, err := a.readPreferenceSlots(ctx, fluctlightID)
	if err != nil {
		return ContextProjection{}, err
	}
	triggerPreferences, err := a.readTriggerPreferences(ctx, fluctlightID)
	if err != nil {
		return ContextProjection{}, err
	}
	visualIdentity, err := a.readVisualIdentityDetail(ctx, fluctlightID)
	if err != nil {
		return ContextProjection{}, err
	}
	recentMessages := make([]map[string]any, 0)
	if conversationID != "" {
		history, historyErr := a.DB.History(ctx, conversationID, actorID, nil, 12)
		if historyErr != nil {
			return ContextProjection{}, historyErr
		}
		recentMessages = make([]map[string]any, 0, len(history.Messages))
		for _, message := range history.Messages {
			recentMessages = append(recentMessages, map[string]any{"id": message.ID, "sequence": message.Sequence, "author_actor_id": message.AuthorActorID, "kind": message.Kind, "text": message.Text, "attachment_refs": message.AttachmentRefs, "created_at": message.CreatedAt.Format(time.RFC3339Nano), "source": "message:" + message.ID})
		}
	}
	developingSelfClaims, err := a.listDevelopingSelfClaims(ctx, fluctlightID)
	if err != nil {
		return ContextProjection{}, err
	}
	developingSelf := make([]map[string]any, 0, len(developingSelfClaims))
	developingSelfRevision := 0
	for _, claim := range developingSelfClaims {
		if claim.Revision > developingSelfRevision {
			developingSelfRevision = claim.Revision
		}
		developingSelf = append(developingSelf, map[string]any{
			"id": claim.ID, "category": claim.Category, "claim": claim.Claim, "value": claim.Value,
			"confidence": claim.Confidence, "evidence_refs": claim.EvidenceRefs, "provenance": claim.Provenance,
			"status": claim.Status, "expires_at": claim.ExpiresAt, "revision": claim.Revision,
		})
	}
	projection := ContextProjection{
		SchemaVersion: "fluctlight.context.v2",
		FluctlightID:  fluctlightID, ConversationID: conversationID, SourceFactID: sourceFactID,
		CurrentUserText: userText, RecentMessages: recentMessages, ContextRevision: fluctlight.CurrentRevision,
		CorePersonaRevision: fluctlight.CurrentRevision, DevelopingSelfRevision: developingSelfRevision, CurrentStateRevision: intValue(inner["revision"]),
		CorePersona:    map[string]any{"authority": "hard_constraint", "data": fluctlight.CorePersona},
		DevelopingSelf: developingSelf,
		CurrentState:   map[string]any{"authority": "transient_state", "data": map[string]any{"inner_state": inner, "life_context": lifeContext}},
		Identity:       fluctlight.Identity, Personality: fluctlight.Personality,
		BehavioralPolicy: fluctlight.BehavioralPolicy, InnerState: inner,
		LifeContext: lifeContext, Memories: memories, Relationships: relationships,
		Hypotheses:   hypotheses,
		Capabilities: capabilityManifestMaps(a.capabilityRegistry().Manifests()),
		DriveSlots:   driveSlots, PreferenceSlots: preferenceSlots, TriggerPreferences: triggerPreferences, VisualIdentity: visualIdentity,
	}
	if presence, ok := lifeContext["presence"].(map[string]any); ok {
		projection.Presence = presence
	}
	return projection, nil
}

func capabilityManifestMaps(manifests []CapabilityManifest) []map[string]any {
	result := make([]map[string]any, 0, len(manifests))
	for _, manifest := range manifests {
		result = append(result, map[string]any{
			"name": manifest.Name, "version": manifest.Version,
			"description": manifest.Description, "side_effect_class": manifest.SideEffectClass,
			"concurrency_class": manifest.ConcurrencyClass, "target_kinds": manifest.TargetKinds,
			"input_schema": manifest.Parameters, "output_schema": manifest.OutputSchema,
			"supports_cancel": manifest.SupportsCancel, "supports_retry": manifest.SupportsRetry,
			"requires_preflight": manifest.RequiresPreflight,
		})
	}
	return result
}

// RetrieveMemoryContext performs authorization before ranking. The ranking is
// deliberately bounded and deterministic; vector/FTS providers can be added
// behind this authority without changing the prompt contract.
func (a *App) RetrieveMemoryContext(ctx context.Context, actorID, fluctlightID, conversationID, query string, limit, tokenBudget int) ([]map[string]any, error) {
	if limit < 1 {
		limit = 1
	}
	if limit > 50 {
		limit = 50
	}
	if tokenBudget < 128 {
		tokenBudget = 128
	}
	fluctlight, err := a.DB.GetFluctlight(ctx, fluctlightID, actorID)
	if err != nil {
		return nil, err
	}
	var ownerActorID string
	if err := a.DB.Pool().QueryRow(ctx, `SELECT created_by_actor_id FROM public.fluctlights WHERE id=$1`, fluctlightID).Scan(&ownerActorID); err != nil {
		return nil, err
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT id,type,content,actor_refs,conversation_id,event_refs,evidence_refs,confidence,importance,emotional_significance,visibility,status,revision,created_at,COALESCE(ts_rank_cd(search_document,plainto_tsquery('simple',$2)),0) FROM public.memories WHERE owner_fluctlight_id=$1 AND status='active' ORDER BY created_at DESC,id DESC LIMIT 200`, fluctlightID, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	queryTokens := tokenize(query)
	type scoredMemory struct {
		value   map[string]any
		score   float64
		created time.Time
	}
	scored := make([]scoredMemory, 0)
	for rows.Next() {
		var id, typ, content, visibility, status string
		var actorRefs, eventRefs, evidenceRefs []byte
		var conversationRef *string
		var confidence, importance, emotional float64
		var revision int
		var created time.Time
		var searchRank float64
		if err := rows.Scan(&id, &typ, &content, &actorRefs, &conversationRef, &eventRefs, &evidenceRefs, &confidence, &importance, &emotional, &visibility, &status, &revision, &created, &searchRank); err != nil {
			return nil, err
		}
		actors := decodeArray(actorRefs)
		if !memoryVisibleToActor(visibility, fluctlight.ID, ownerActorID, actorID, actors) {
			continue
		}
		if conversationID != "" && conversationRef != nil && *conversationRef != "" && *conversationRef != conversationID {
			// Conversation-scoped memories remain useful only to the same
			// conversation; private/global memories have a NULL scope.
			continue
		}
		score := importance + emotional*0.5 + confidence*0.25 + searchRank
		lowerContent := strings.ToLower(content)
		for _, token := range queryTokens {
			if strings.Contains(lowerContent, token) {
				score += 1
			}
		}
		value := map[string]any{
			"id": id, "type": typ, "content": content, "confidence": confidence,
			"importance": importance, "emotional_significance": emotional,
			"visibility": visibility, "status": status, "revision": revision,
			"conversation_id": conversationRef, "event_refs": decodeArray(eventRefs),
			"evidence_refs": decodeArray(evidenceRefs), "source": "memory:" + id,
			"created_at": created.Format(time.RFC3339Nano),
		}
		scored = append(scored, scoredMemory{value: value, score: score, created: created})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// Vector ranking is optional: a missing/degraded embedding role must not
	// make ordinary cognition fail. When available, it contributes a bounded
	// cosine score after the authorization query has already selected rows.
	if strings.TrimSpace(query) != "" {
		if _, assignmentErr := a.Provider.assignment(ctx, "embedding"); assignmentErr == nil {
			if _, queryVector, embedErr := a.Provider.Embed(ctx, query); embedErr == nil {
				vectorRows, vectorErr := a.DB.Pool().Query(ctx, `SELECT e.memory_id,e.embedding FROM public.memory_embeddings e JOIN public.memories m ON m.id=e.memory_id WHERE m.owner_fluctlight_id=$1 AND m.status='active' AND e.status='ready' AND e.memory_revision=m.revision ORDER BY e.created_at DESC LIMIT 200`, fluctlightID)
				if vectorErr == nil {
					vectors := make(map[string][]float64)
					for vectorRows.Next() {
						var memoryID string
						var raw []byte
						if scanErr := vectorRows.Scan(&memoryID, &raw); scanErr != nil {
							continue
						}
						var vector []float64
						if json.Unmarshal(raw, &vector) == nil {
							vectors[memoryID] = vector
						}
					}
					vectorRows.Close()
					for index := range scored {
						memoryID := stringValue(scored[index].value["id"])
						if vector := vectors[memoryID]; len(vector) == len(queryVector) && len(vector) > 0 {
							scored[index].score += cosineSimilarity(queryVector, vector)
						}
					}
				}
			}
		}
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score
		}
		return scored[i].created.After(scored[j].created)
	})
	result := make([]map[string]any, 0, limit)
	used := 0
	for _, item := range scored {
		if len(result) >= limit {
			break
		}
		cost := len([]rune(stringValue(item.value["content"]))) + 32
		if used+cost > tokenBudget && len(result) > 0 {
			break
		}
		used += cost
		result = append(result, item.value)
	}
	return result, nil
}

func cosineSimilarity(left, right []float64) float64 {
	if len(left) == 0 || len(left) != len(right) {
		return 0
	}
	var dot, leftNorm, rightNorm float64
	for index := range left {
		dot += left[index] * right[index]
		leftNorm += left[index] * left[index]
		rightNorm += right[index] * right[index]
	}
	if leftNorm == 0 || rightNorm == 0 {
		return 0
	}
	return dot / (math.Sqrt(leftNorm) * math.Sqrt(rightNorm))
}

func memoryVisibleToActor(visibility, fluctlightID, ownerActorID, actorID string, actorRefs []any) bool {
	switch visibility {
	case "private", "owner", "":
		return ownerActorID == actorID
	case "participants":
		if ownerActorID == actorID {
			return true
		}
		for _, value := range actorRefs {
			if stringValue(value) == actorID || stringValue(value) == fluctlightID {
				return true
			}
		}
		return false
	default:
		return false
	}
}

func tokenize(value string) []string {
	words := strings.FieldsFunc(strings.ToLower(value), func(r rune) bool { return unicode.IsSpace(r) || unicode.IsPunct(r) })
	result := make([]string, 0, len(words))
	for _, word := range words {
		if len([]rune(word)) >= 2 {
			result = append(result, word)
		}
	}
	return result
}

func (a *App) readActiveHypotheses(ctx context.Context, fluctlightID string) ([]map[string]any, error) {
	rows, err := a.DB.Pool().Query(ctx, `SELECT id,source_fact_id,claim_type,content,evidence_refs,confidence,repetition_key,status,expires_at,created_at FROM public.cognition_claims WHERE fluctlight_id=$1 AND status IN ('active','uncertain') AND (expires_at IS NULL OR expires_at > now()) ORDER BY confidence DESC,created_at DESC LIMIT 50`, fluctlightID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]map[string]any, 0)
	for rows.Next() {
		var id, source, kind, content string
		var refs []byte
		var confidence float64
		var repetition, status string
		var expires, created *time.Time
		if err := rows.Scan(&id, &source, &kind, &content, &refs, &confidence, &repetition, &status, &expires, &created); err != nil {
			return nil, err
		}
		var expiresValue, createdValue any
		if expires != nil {
			expiresValue = expires.Format(time.RFC3339Nano)
		}
		if created != nil {
			createdValue = created.Format(time.RFC3339Nano)
		}
		result = append(result, map[string]any{"id": id, "source_fact_id": source, "claim_type": kind, "content": content, "evidence_refs": decodeArray(refs), "confidence": confidence, "repetition_key": repetition, "status": status, "expires_at": expiresValue, "created_at": createdValue})
	}
	return result, rows.Err()
}

func normalizeResponsePlan(decision map[string]any, sourceFactID string, context ContextProjection) (map[string]any, error) {
	if decision == nil {
		return nil, errors.New("response_plan_missing")
	}
	base := decision
	if nested := mapValue(decision["response_plan"]); len(nested) > 0 {
		base = nested
	}
	plan := map[string]any{
		"schema_version":   "fluctlight.response-plan.v1",
		"source_fact_id":   sourceFactID,
		"context_revision": context.ContextRevision,
		"answer_mode":      firstString(base["answer_mode"], "direct"),
		"approved_claims":  []any{}, "uncertain_claims": []any{}, "omitted_claims": []any{},
		"response_outline": arrayValue(base["response_outline"]),
		"tone":             firstString(base["tone"], "natural"),
		"self_evaluation":  mapValue(base["self_evaluation"]),
		"core_alignment":   mapValue(base["core_alignment"]),
		"state_expression": mapValue(base["state_expression"]),
	}
	if len(mapValue(plan["core_alignment"])) == 0 {
		plan["core_alignment"] = mapValue(decision["core_alignment"])
	}
	if len(mapValue(plan["state_expression"])) == 0 {
		plan["state_expression"] = mapValue(decision["state_expression"])
	}
	if action := firstString(base["action_type"], firstString(decision["action_type"], "")); action != "" {
		plan["action_type"] = action
	}
	if intent := firstString(base["response_intent"], firstString(decision["response_intent"], "")); intent != "" {
		plan["response_intent"] = intent
	}
	if calls := toolCallsFromValue(base["tool_calls"]); len(calls) > 0 {
		plan["tool_calls"] = calls
	}
	compositeActionType := firstString(plan["action_type"], firstString(decision["action_type"], "reply"))
	if composite, compositeErr := normalizeCompositeAction(decision, toolCallsFromValue(plan["tool_calls"]), sourceFactID, compositeActionType); compositeErr == nil {
		plan["composite_action"] = composite
	}
	if text := firstString(base["visible_text"], firstString(base["draft"], "")); text != "" {
		plan["visible_text"] = text
	}
	claims := arrayValue(base["claims"])
	if len(claims) == 0 {
		claims = append(arrayValue(base["approved_claims"]), arrayValue(base["uncertain_claims"])...)
	}
	approved, uncertain, omitted, err := evaluateClaims(claims, sourceFactID, context)
	if err != nil {
		return nil, err
	}
	plan["approved_claims"], plan["uncertain_claims"], plan["omitted_claims"] = approved, uncertain, omitted
	if self := mapValue(plan["self_evaluation"]); len(self) == 0 {
		mode := "accepted"
		if len(uncertain) > 0 {
			mode = "uncertain"
		} else if len(omitted) > 0 && len(approved) == 0 {
			mode = "omit"
		}
		plan["self_evaluation"] = map[string]any{"mode": mode, "reason_codes": []any{}, "confidence": 1.0}
	} else {
		if _, ok := self["mode"]; !ok {
			self["mode"] = "accepted"
		}
		plan["self_evaluation"] = self
	}
	if stringValue(mapValue(plan["self_evaluation"])["mode"]) == "omit" {
		delete(plan, "visible_text")
	}
	if err := validateResponsePlan(plan); err != nil {
		return nil, err
	}
	return plan, nil
}

func validateResponsePlan(plan map[string]any) error {
	version := stringValue(plan["schema_version"])
	if version != "fluctlight.response-plan.v1" {
		return errors.New("response_plan_schema_invalid")
	}
	if stringValue(plan["source_fact_id"]) == "" {
		return errors.New("response_plan_source_invalid")
	}
	if mode := stringValue(mapValue(plan["self_evaluation"])["mode"]); mode != "accepted" && mode != "uncertain" && mode != "omit" && mode != "deferred" {
		return errors.New("response_plan_self_evaluation_invalid")
	}
	if len([]rune(firstString(plan["visible_text"], ""))) > 32000 {
		return errors.New("response_plan_visible_text_too_large")
	}
	return nil
}

func evaluateClaims(rawClaims []any, sourceFactID string, context ContextProjection) ([]any, []any, []any, error) {
	active := make(map[string]map[string]any, len(context.Hypotheses))
	allowedEvidence := map[string]struct{}{sourceFactID: {}}
	for _, hypothesis := range context.Hypotheses {
		active[stringValue(hypothesis["repetition_key"])] = hypothesis
		allowedEvidence[stringValue(hypothesis["id"])] = struct{}{}
		allowedEvidence[stringValue(hypothesis["source_fact_id"])] = struct{}{}
	}
	for _, memory := range context.Memories {
		memoryID := stringValue(memory["id"])
		allowedEvidence[memoryID] = struct{}{}
		allowedEvidence["memory:"+memoryID] = struct{}{}
	}
	if eventID := stringValue(context.LifeContext["event_id"]); eventID != "" {
		allowedEvidence[eventID] = struct{}{}
	}
	approved, uncertain, omitted := make([]any, 0), make([]any, 0), make([]any, 0)
	for index, raw := range rawClaims {
		claim := mapValue(raw)
		if len(claim) == 0 {
			return nil, nil, nil, fmt.Errorf("claim_%d_invalid", index)
		}
		kind := firstString(claim["kind"], firstString(claim["claim_type"], ""))
		kind = normalizeClaimKind(kind)
		if _, ok := validClaimKinds[kind]; !ok {
			return nil, nil, nil, fmt.Errorf("claim_%d_kind_invalid", index)
		}
		content := strings.TrimSpace(stringValue(claim["content"]))
		if content == "" || len([]rune(content)) > 1000 {
			return nil, nil, nil, fmt.Errorf("claim_%d_content_invalid", index)
		}
		refs := arrayValue(claim["evidence_refs"])
		if len(refs) > 0 && !validateEvidenceRefs(refs, allowedEvidence) {
			return nil, nil, nil, fmt.Errorf("claim_%d_evidence_invalid", index)
		}
		repetitionKey := firstString(claim["repetition_key"], repetitionKeyFor(content))
		confidence, confidenceErr := boundedNumberOrError(claim["confidence"], 0.0)
		if confidenceErr != nil {
			return nil, nil, nil, fmt.Errorf("claim_%d_confidence_invalid", index)
		}
		claim["kind"], claim["content"], claim["evidence_refs"] = kind, content, refs
		claim["confidence"], claim["repetition_key"], claim["source_fact_id"] = confidence, repetitionKey, sourceFactID
		if kind == ClaimUnsupportedSelf && len(refs) == 0 {
			claim["reason_code"] = "unsupported_self_claim"
			omitted = append(omitted, claim)
			continue
		}
		if (kind == ClaimConfirmedFact || kind == ClaimObservedFact) && len(refs) == 0 {
			claim["reason_code"] = "evidence_required"
			omitted = append(omitted, claim)
			continue
		}
		if existing := active[repetitionKey]; existing != nil && sameEvidence(refs, arrayValue(existing["evidence_refs"])) {
			claim["reason_code"] = "repeated_without_new_evidence"
			omitted = append(omitted, claim)
			continue
		}
		if kind == ClaimUncertainHypothesis || confidence < 0.5 {
			uncertain = append(uncertain, claim)
		} else {
			approved = append(approved, claim)
		}
	}
	return approved, uncertain, omitted, nil
}

func normalizeClaimKind(kind string) string {
	switch kind {
	case "semantic", "observation", "recall_confirmation", "memory_recall", "preference_recall", "user_preference", "confirmation":
		// Common providers use these broad labels for an evidence-backed
		// statement. The Runtime keeps the claim grounded as an observed fact;
		// it does not promote it to a stronger semantic state.
		return ClaimObservedFact
	default:
		return kind
	}
}

func boundedNumberOrError(value any, fallback float64) (float64, error) {
	parsed, ok := numberFloat(value)
	if value == nil {
		return fallback, nil
	}
	if !ok || parsed < 0 || parsed > 1 {
		return 0, errors.New("claim_confidence_invalid")
	}
	return parsed, nil
}

func repetitionKeyFor(value string) string {
	return strings.Join(tokenize(value), " ")
}

func sameEvidence(left, right []any) bool {
	if len(left) != len(right) {
		return false
	}
	leftKeys, rightKeys := make(map[string]struct{}), make(map[string]struct{})
	for _, item := range left {
		leftKeys[stringValue(item)] = struct{}{}
	}
	for _, item := range right {
		rightKeys[stringValue(item)] = struct{}{}
	}
	if len(leftKeys) != len(rightKeys) {
		return false
	}
	for key := range leftKeys {
		if _, ok := rightKeys[key]; !ok {
			return false
		}
	}
	return true
}

func persistClaimsTx(ctx context.Context, tx pgx.Tx, fluctlightID, sourceFactID string, plan map[string]any) error {
	for _, raw := range append(arrayValue(plan["approved_claims"]), append(arrayValue(plan["uncertain_claims"]), arrayValue(plan["omitted_claims"])...)...) {
		claim := mapValue(raw)
		kind := firstString(claim["kind"], "")
		status := "active"
		if kind == ClaimUnsupportedSelf || stringValue(claim["reason_code"]) != "" {
			status = "rejected"
		} else if kind == ClaimUncertainHypothesis {
			status = "uncertain"
		}
		id := "claim_" + stableDigest(fluctlightID+":"+firstString(claim["repetition_key"], repetitionKeyFor(stringValue(claim["content"]))))
		var expires any
		if status == "uncertain" || kind == ClaimSupportedHypothesis {
			expires = time.Now().UTC().Add(7 * 24 * time.Hour)
		}
		_, err := tx.Exec(ctx, `INSERT INTO public.cognition_claims(id,fluctlight_id,source_fact_id,claim_type,content,evidence_refs,confidence,repetition_key,status,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) ON CONFLICT(fluctlight_id,repetition_key) DO NOTHING`, id, fluctlightID, sourceFactID, kind, stringValue(claim["content"]), jsonBytes(arrayValue(claim["evidence_refs"])), boundedNumber(claim["confidence"], 0), firstString(claim["repetition_key"], repetitionKeyFor(stringValue(claim["content"]))), status, expires)
		if err != nil {
			return err
		}
	}
	return nil
}
