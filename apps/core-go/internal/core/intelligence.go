package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
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
	SchemaVersion          string                     `json:"schema_version"`
	FluctlightID           string                     `json:"fluctlight_id"`
	OwnerActorID           string                     `json:"owner_actor_id"`
	ConversationID         string                     `json:"conversation_id"`
	SourceFactID           string                     `json:"source_fact_id"`
	CurrentUserText        string                     `json:"current_user_text"`
	SelfActor              map[string]any             `json:"self_actor"`
	CurrentSpeaker         map[string]any             `json:"current_speaker,omitempty"`
	Actors                 []map[string]any           `json:"actors,omitempty"`
	RecentMessages         []map[string]any           `json:"recent_messages"`
	ContextRevision        int                        `json:"context_revision"`
	CorePersonaRevision    int                        `json:"core_persona_revision"`
	DevelopingSelfRevision int                        `json:"developing_self_revision"`
	CurrentStateRevision   int                        `json:"current_state_revision"`
	LifeContextRevision    string                     `json:"life_context_revision"`
	CorePersona            map[string]any             `json:"core_persona"`
	PersonalitySystem      map[string]any             `json:"personality_system,omitempty"`
	PersonalityRuntime     map[string]any             `json:"personality_runtime,omitempty"`
	EffectivePersona       map[string]any             `json:"effective_persona,omitempty"`
	EvolutionOverlays      []map[string]any           `json:"evolution_overlays,omitempty"`
	DevelopingSelf         []map[string]any           `json:"developing_self"`
	CurrentState           map[string]any             `json:"current_state"`
	Identity               map[string]any             `json:"identity"`
	Personality            map[string]any             `json:"personality"`
	BehavioralPolicy       map[string]any             `json:"behavioral_policy"`
	InnerState             map[string]any             `json:"inner_state"`
	AffectProfile          map[string]any             `json:"affect_profile,omitempty"`
	Schedule               map[string]any             `json:"schedule,omitempty"`
	LifeContext            map[string]any             `json:"life_context"`
	Presence               map[string]any             `json:"presence,omitempty"`
	Memories               []map[string]any           `json:"memories"`
	MemoryRetrievalTrace   MemoryRetrievalTrace       `json:"memory_retrieval_trace,omitempty"`
	ActiveMemories         []map[string]any           `json:"active_memories,omitempty"`
	ActiveMemoryTrace      ActiveMemoryRetrievalTrace `json:"active_memory_retrieval_trace,omitempty"`
	Relationships          []map[string]any           `json:"relationships"`
	Hypotheses             []map[string]any           `json:"hypotheses"`
	Capabilities           []map[string]any           `json:"capabilities"`
	DriveSlots             []map[string]any           `json:"drive_slots"`
	PreferenceSlots        []map[string]any           `json:"preference_slots"`
	TriggerPreferences     []map[string]any           `json:"trigger_preferences"`
	VisualIdentity         map[string]any             `json:"visual_identity"`
	Goals                  []map[string]any           `json:"goals,omitempty"`
	Intentions             []map[string]any           `json:"intentions,omitempty"`
	RecentOutcomes         []map[string]any           `json:"recent_outcomes,omitempty"`
	ReferenceIndex         ContextReferenceIndex      `json:"context_reference_index"`
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
	if projection.SchemaVersion == "fluctlight.context.v3" {
		if err := projection.ReferenceIndex.Validate(); err != nil || projection.ReferenceIndex.FluctlightID != projection.FluctlightID || projection.ReferenceIndex.OwnerActorID != projection.OwnerActorID || projection.ReferenceIndex.SpeakerActorID != strings.TrimSpace(stringValue(projection.CurrentSpeaker["actor_id"])) || projection.ReferenceIndex.ConversationID != projection.ConversationID {
			return ContextProjection{}, false
		}
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
	mode := MemoryConversationGlobalOnly
	operation := MemoryForNativeCognition
	if strings.TrimSpace(conversationID) != "" {
		mode = MemoryConversationExact
		operation = MemoryForConversation
	}
	return a.BuildContextProjectionFor(ctx, ContextProjectionRequest{
		AuthorizationActorID: actorID, SpeakerActorID: actorID, FluctlightID: fluctlightID,
		ConversationID: conversationID, SourceFactID: sourceFactID, CurrentUserText: userText,
		MemoryOperation: operation, MemoryConversationMode: mode,
	})
}

func (a *App) BuildContextProjectionFor(ctx context.Context, request ContextProjectionRequest) (ContextProjection, error) {
	retry, _ := ctx.Value(contextProjectionRetryKey{}).(int)
	actorID := strings.TrimSpace(request.AuthorizationActorID)
	speakerActorID := strings.TrimSpace(request.SpeakerActorID)
	if speakerActorID == "" {
		speakerActorID = actorID
	}
	fluctlightID := strings.TrimSpace(request.FluctlightID)
	conversationID := strings.TrimSpace(request.ConversationID)
	sourceFactID := strings.TrimSpace(request.SourceFactID)
	userText := request.CurrentUserText
	projectionAt := time.Now().UTC()
	fluctlight, schedule, lifeContext, err := a.readFoundationLifeSnapshotAt(ctx, fluctlightID, actorID, projectionAt)
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
	affectPolicy, affectProfile, err := a.readAffectProfile(ctx, fluctlightID)
	if err != nil {
		return ContextProjection{}, err
	}
	inner = projectAffectStateAt(inner, affectPolicy, projectionAt)
	annotateLifeContextClock(lifeContext, stringValue(fluctlight.Identity["timezone"]))
	personalitySystem := mapValue(fluctlight.CorePersona["personality_system"])
	personalityRuntime, err := a.readPersonalityRuntime(ctx, fluctlightID, stringValue(personalitySystem["active_profile_id"]))
	if err != nil {
		return ContextProjection{}, err
	}
	effectivePersona, evolutionOverlays, err := a.readEffectivePersonaProjection(ctx, fluctlight, personalitySystem, personalityRuntime)
	if err != nil {
		return ContextProjection{}, err
	}
	relationships, err := a.readRelationships(ctx, fluctlightID, speakerActorID)
	if err != nil {
		return ContextProjection{}, err
	}
	if conversationID != "" {
		filtered := make([]map[string]any, 0, 1)
		for _, relationship := range relationships {
			if stringValue(relationship["target_actor_id"]) == speakerActorID {
				filtered = append(filtered, relationship)
			}
		}
		relationships = filtered
	}
	hypotheses, err := a.readActiveHypotheses(ctx, fluctlightID)
	if err != nil {
		return ContextProjection{}, err
	}
	driveSlots, err := a.readDriveSlots(ctx, fluctlightID)
	if err != nil {
		return ContextProjection{}, err
	}
	inner["drives"] = mergeEffectiveDriveState(inner["drives"], driveSlots)
	preferenceSlots, err := a.readPreferenceSlots(ctx, fluctlightID)
	if err != nil {
		return ContextProjection{}, err
	}
	triggerPreferences, err := a.readTriggerPreferences(ctx, fluctlightID)
	if err != nil {
		return ContextProjection{}, err
	}
	goals, intentions, err := a.agencyProfile(ctx, fluctlightID)
	if err != nil {
		return ContextProjection{}, err
	}
	if conversationID != "" {
		goals, intentions = filterAgencyForTarget(goals, intentions, speakerActorID)
	}
	recentOutcomes, err := a.readRecentActionOutcomes(ctx, fluctlightID, 12)
	if err != nil {
		return ContextProjection{}, err
	}
	recentMessages := make([]map[string]any, 0)
	if conversationID != "" {
		// Fetch a bounded candidate window; WorkingMemory applies the actual token
		// budget and whole-turn selection. A fixed 12-row fetch must not remain the
		// effective Recent Context boundary.
		history, historyErr := a.DB.History(ctx, conversationID, speakerActorID, nil, 200)
		if historyErr != nil {
			return ContextProjection{}, historyErr
		}
		recentMessages = make([]map[string]any, 0, len(history.Messages))
		for _, message := range history.Messages {
			recentMessages = append(recentMessages, map[string]any{"id": message.ID, "sequence": message.Sequence, "author_actor_id": message.AuthorActorID, "kind": message.Kind, "text": message.Text, "attachment_refs": message.AttachmentRefs, "created_at": message.CreatedAt.Format(time.RFC3339Nano), "source": "message:" + message.ID})
		}
	}
	activeResult, err := a.retrieveActiveMemories(ctx, ActiveMemoryQuery{AuthorizationActorID: actorID, OwnerFluctlightID: fluctlightID, ConversationID: conversationID, Cue: userText, At: projectionAt, Limit: activeMemoryResultLimit})
	if err != nil {
		return ContextProjection{}, err
	}
	memoryCues := buildProjectionMemoryCues(request.MemoryOperation, request.MemoryCues, userText, lifeContext, inner, recentMessages, activeResult.Items, goals, intentions, recentOutcomes, hypotheses)
	viewers := []string{speakerActorID}
	memoryPlan, err := buildMemoryQueryPlan(request.MemoryOperation, viewers, request.MemoryConversationMode, conversationID, request.AllowedConversationIDs, stringValue(mapValue(personalityRuntime)["active_profile_id"]), memoryCues, 12, 2400)
	if err != nil {
		return ContextProjection{}, err
	}
	memoryResult, err := a.retrieveMemoryWithPlan(ctx, actorID, fluctlightID, memoryPlan)
	if err != nil {
		return ContextProjection{}, err
	}
	memories := memoryResult.Items
	visualIdentity, err := a.readVisualIdentityDetail(ctx, fluctlightID)
	if err != nil {
		return ContextProjection{}, err
	}
	fluctlightDisplayName := firstString(fluctlight.Identity["name"], firstString(fluctlight.Identity["display_name"], "摇光"))
	relationshipActorIDs := make([]string, 0, len(relationships))
	for _, relationship := range relationships {
		if target := strings.TrimSpace(stringValue(relationship["target_actor_id"])); target != "" {
			relationshipActorIDs = append(relationshipActorIDs, target)
		}
	}
	actors, selfActor, currentSpeaker := a.buildActorProjection(ctx, fluctlightID, speakerActorID, fluctlightDisplayName, recentMessages, relationshipActorIDs)
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
		FluctlightID:  fluctlightID, OwnerActorID: actorID, ConversationID: conversationID, SourceFactID: sourceFactID,
		CurrentUserText: userText, SelfActor: selfActor, CurrentSpeaker: currentSpeaker, Actors: actors, RecentMessages: recentMessages, ContextRevision: fluctlight.CurrentRevision,
		CorePersonaRevision: fluctlight.CurrentRevision, DevelopingSelfRevision: developingSelfRevision, CurrentStateRevision: intValue(inner["revision"]),
		LifeContextRevision: stringValue(lifeContext["context_revision"]),
		CorePersona:         map[string]any{"authority": "hard_constraint", "data": fluctlight.CorePersona}, PersonalitySystem: personalitySystem, PersonalityRuntime: personalityRuntime,
		EffectivePersona: effectivePersona, EvolutionOverlays: evolutionOverlays,
		DevelopingSelf: developingSelf,
		CurrentState:   map[string]any{"authority": "transient_state", "data": map[string]any{"inner_state": inner, "affect_profile": affectProfile, "life_context": lifeContext}},
		Schedule:       schedule,
		Identity:       fluctlight.Identity, Personality: fluctlight.Personality,
		BehavioralPolicy: fluctlight.BehavioralPolicy, InnerState: inner, AffectProfile: affectProfile,
		LifeContext: lifeContext, Memories: memories, MemoryRetrievalTrace: memoryResult.Trace, ActiveMemories: activeResult.Items, ActiveMemoryTrace: activeResult.Trace, Relationships: relationships,
		Hypotheses:   hypotheses,
		Capabilities: capabilityDefinitionMaps(a.capabilityRegistry().Definitions()),
		DriveSlots:   driveSlots, PreferenceSlots: preferenceSlots, TriggerPreferences: triggerPreferences, VisualIdentity: visualIdentity,
		Goals: goals, Intentions: intentions, RecentOutcomes: recentOutcomes,
	}
	if presence, ok := lifeContext["presence"].(map[string]any); ok {
		projection.Presence = presence
	}
	if err := a.validateCognitionAuthorityRevisions(ctx, fluctlightID, projection.ContextRevision, projection.CurrentStateRevision, projection.LifeContextRevision, projectionAt); err != nil {
		if (errors.Is(err, ErrFoundationRevisionStale) || errors.Is(err, ErrCurrentStateRevisionStale) || errors.Is(err, ErrLifeContextStale)) && retry < 2 {
			return a.BuildContextProjectionFor(context.WithValue(ctx, contextProjectionRetryKey{}, retry+1), request)
		}
		if errors.Is(err, ErrFoundationRevisionStale) || errors.Is(err, ErrCurrentStateRevisionStale) || errors.Is(err, ErrLifeContextStale) {
			return ContextProjection{}, ErrContextProjectionUnstable
		}
		return ContextProjection{}, err
	}
	if err := buildContextReferenceIndex(&projection); err != nil {
		return ContextProjection{}, err
	}
	referenceCapacity := maxContextReferences - len(projection.ReferenceIndex.ByRef)
	if referenceCapacity < 0 {
		referenceCapacity = 0
	}
	if len(projection.ActiveMemories) > referenceCapacity {
		projection.ActiveMemories = projection.ActiveMemories[:referenceCapacity]
		projection.ActiveMemoryTrace.SelectedCount = len(projection.ActiveMemories)
		projection.ActiveMemoryTrace.TruncatedReason = "context_reference_limit"
	}
	if err := addActiveMemoryReferences(&projection.ReferenceIndex, projection.ActiveMemories); err != nil {
		return ContextProjection{}, err
	}
	return projection, nil
}

type contextProjectionRetryKey struct{}

func filterAgencyForTarget(goals, intentions []map[string]any, targetActorID string) ([]map[string]any, []map[string]any) {
	filteredGoals := make([]map[string]any, 0, len(goals))
	allowedGoalIDs := map[string]struct{}{}
	for _, goal := range goals {
		if stringValue(goal["scope"]) != "relationship" || stringValue(goal["target_actor_id"]) == strings.TrimSpace(targetActorID) {
			filteredGoals = append(filteredGoals, goal)
			if id := stringValue(goal["id"]); id != "" {
				allowedGoalIDs[id] = struct{}{}
			}
		}
	}
	filteredIntentions := make([]map[string]any, 0, len(intentions))
	for _, intention := range intentions {
		goalID := stringValue(intention["goal_id"])
		if goalID == "" {
			filteredIntentions = append(filteredIntentions, intention)
			continue
		}
		if _, ok := allowedGoalIDs[goalID]; ok {
			filteredIntentions = append(filteredIntentions, intention)
		}
	}
	return filteredGoals, filteredIntentions
}

// filterActiveProfileRows keeps shared rows and rows owned by the currently
// dominant profile. Profile-specific agency must never leak another
// personality's goals into cognition; shared rows remain a deliberate
// fallback for legacy and cross-profile facts.
func filterActiveProfileRows(values []map[string]any, activeProfileID string) []map[string]any {
	activeProfileID = strings.TrimSpace(activeProfileID)
	result := make([]map[string]any, 0, len(values))
	for _, value := range values {
		profileID := strings.TrimSpace(stringValue(value["profile_id"]))
		if profileID == "" || (activeProfileID != "" && profileID == activeProfileID) {
			result = append(result, value)
		}
	}
	return result
}

func selectActiveProfileRelationships(values []map[string]any, activeProfileID string) []map[string]any {
	activeProfileID = strings.TrimSpace(activeProfileID)
	selected := make(map[string]map[string]any)
	order := make([]string, 0, len(values))
	for _, value := range values {
		target := strings.TrimSpace(stringValue(value["target_actor_id"]))
		if target == "" {
			continue
		}
		profileID := strings.TrimSpace(stringValue(value["profile_id"]))
		if profileID != "" && activeProfileID != "" && profileID != activeProfileID {
			continue
		}
		current, exists := selected[target]
		if !exists {
			selected[target] = value
			order = append(order, target)
			continue
		}
		currentProfile := strings.TrimSpace(stringValue(current["profile_id"]))
		if profileID == activeProfileID && currentProfile != activeProfileID {
			selected[target] = value
		}
	}
	result := make([]map[string]any, 0, len(order))
	for _, target := range order {
		result = append(result, selected[target])
	}
	return result
}

func (a *App) buildActorProjection(ctx context.Context, selfActorID, speakerActorID, selfDisplayName string, messages []map[string]any, extraActorIDs []string) ([]map[string]any, map[string]any, map[string]any) {
	self := map[string]any{"ref": "actor_self", "actor_id": selfActorID, "type": "fluctlight", "display_name": firstString(selfDisplayName, "摇光")}
	actors := []map[string]any{self}
	refs := map[string]map[string]any{selfActorID: self}
	add := func(actorID, ref, fallbackType string) {
		actorID = strings.TrimSpace(actorID)
		if actorID == "" {
			return
		}
		if _, exists := refs[actorID]; exists {
			return
		}
		actorType := fallbackType
		_ = a.DB.Pool().QueryRow(ctx, `SELECT actor_type FROM public.actors WHERE id=$1`, actorID).Scan(&actorType)
		displayName := ""
		if firstString(actorType, "unknown") == "fluctlight" {
			_ = a.DB.Pool().QueryRow(ctx, `SELECT COALESCE(identity->>'name','') FROM public.fluctlights WHERE id=$1`, actorID).Scan(&displayName)
		}
		actor := map[string]any{"ref": ref, "actor_id": actorID, "type": firstString(actorType, "unknown"), "display_name": firstString(displayName, ref)}
		refs[actorID] = actor
		actors = append(actors, actor)
	}
	if strings.TrimSpace(speakerActorID) != "" {
		var speakerType string
		_ = a.DB.Pool().QueryRow(ctx, `SELECT actor_type FROM public.actors WHERE id=$1`, speakerActorID).Scan(&speakerType)
		if speakerType == "human" || speakerType == "" {
			add(speakerActorID, "actor_user", "human")
		} else {
			add(speakerActorID, "actor_b", speakerType)
		}
	}
	next := 2
	for _, message := range messages {
		actorID := stringValue(message["author_actor_id"])
		if actorID == "" {
			continue
		}
		if _, exists := refs[actorID]; exists {
			continue
		}
		add(actorID, fmt.Sprintf("actor_%c", rune('a'+next)), "unknown")
		next++
	}
	for _, actorID := range extraActorIDs {
		if strings.TrimSpace(actorID) == "" {
			continue
		}
		if _, exists := refs[strings.TrimSpace(actorID)]; exists {
			continue
		}
		add(actorID, fmt.Sprintf("actor_%c", rune('a'+next)), "unknown")
		next++
	}
	var speaker map[string]any
	if value, ok := refs[strings.TrimSpace(speakerActorID)]; ok {
		speaker = value
	}
	return actors, self, speaker
}

// annotateLifeContextClock adds the semantic wall-clock facts that model
// decisions need. The internal RFC3339 instant remains a Core-only snapshot
// field and is removed by provider-context compaction.
func annotateLifeContextClock(lifeContext map[string]any, timezone string) {
	if lifeContext == nil {
		return
	}
	timezone = canonicalTimezone(timezone)
	if timezone == "" {
		timezone = "Asia/Shanghai"
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return
	}
	instant := time.Now().UTC()
	if raw := stringValue(lifeContext["instant"]); raw != "" {
		if parsed, parseErr := time.Parse(time.RFC3339Nano, raw); parseErr == nil {
			instant = parsed
		}
	}
	lifeContext["current_time"] = instant.In(location).Format("2006-01-02 15:04:05 MST")
	lifeContext["timezone"] = timezone
}

func capabilityDefinitionMaps(definitions []CapabilityDefinition) []map[string]any {
	result := make([]map[string]any, 0, len(definitions))
	for _, definition := range definitions {
		result = append(result, map[string]any{
			"name": definition.Name, "version": definition.Version,
			"description": definition.Description, "side_effect_class": definition.SideEffectClass,
			"concurrency_class": definition.ConcurrencyClass, "target_kinds": definition.TargetKinds,
			"input_schema": definition.InputSchema, "output_schema": definition.OutputSchema,
			"supports_cancel": definition.SupportsCancel, "supports_retry": definition.SupportsRetry,
			"requires_preflight": definition.RequiresPreflight,
		})
	}
	return result
}

// RetrieveMemoryContext performs authorization before ranking. The ranking is
// deliberately bounded and deterministic; vector/FTS providers can be added
// behind this authority without changing the prompt contract.
func (a *App) RetrieveMemoryContext(ctx context.Context, actorID, fluctlightID, conversationID, query string, limit, tokenBudget int) ([]map[string]any, error) {
	mode := MemoryConversationGlobalOnly
	if strings.TrimSpace(conversationID) != "" {
		mode = MemoryConversationExact
	}
	cues := []MemoryQueryCue{}
	if strings.TrimSpace(query) != "" {
		cues = append(cues, MemoryQueryCue{Kind: "query", Text: query})
	}
	plan, err := buildMemoryQueryPlan(MemoryForConversation, []string{actorID}, mode, conversationID, nil, "", cues, limit, tokenBudget)
	if err != nil {
		return nil, err
	}
	result, err := a.retrieveMemoryWithPlan(ctx, actorID, fluctlightID, plan)
	return result.Items, err
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

func memoryVisibleToActor(visibility, _ string, ownerActorID, actorID string, actorRefs []any) bool {
	switch visibility {
	case "private", "owner", "":
		return ownerActorID == actorID
	case "participants":
		if ownerActorID == actorID {
			return true
		}
		for _, value := range actorRefs {
			if stringValue(value) == actorID {
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
	if personalityDecision := mapValue(decision["personality_decision"]); len(personalityDecision) > 0 {
		plan["personality_decision"] = personalityDecision
		if profileID := stringValue(personalityDecision["target_profile_id"]); profileID != "" {
			plan["profile_id"] = profileID
		}
	}
	if outputDecision := mapValue(decision["output_preference_decision"]); len(outputDecision) > 0 {
		if normalized, err := normalizeOutputPreferenceDecision(outputDecision, stringValue(mapValue(context.PersonalityRuntime)["active_profile_id"])); err != nil {
			return nil, err
		} else {
			plan["output_preference_decision"] = normalized
		}
	}
	if stringValue(plan["profile_id"]) == "" {
		if active := stringValue(mapValue(context.PersonalityRuntime)["active_profile_id"]); active != "" {
			plan["profile_id"] = active
		}
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
	// Capability invocations live only in the root canonical sidecar. The
	// response plan is a visible projection and is never a replay source.
	calls, callsErr := capabilityInvocationsFromValue(decision["capability_invocations"])
	if callsErr != nil {
		return nil, callsErr
	}
	compositeActionType := firstString(plan["action_type"], firstString(decision["action_type"], "reply"))
	if composite, compositeErr := normalizeCompositeAction(decision, calls, sourceFactID, compositeActionType); compositeErr == nil {
		plan["composite_action"] = composite
	}
	if text := firstString(base["visible_text"], firstString(base["draft"], "")); text != "" {
		plan["visible_text"] = text
	}
	claims := arrayValue(base["claims"])
	if len(claims) == 0 {
		claims = append(arrayValue(base["approved_claims"]), arrayValue(base["uncertain_claims"])...)
	}
	claims = normalizeCognitionClaims(claims, sourceFactID)
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
		if strings.TrimSpace(stringValue(self["mode"])) == "" {
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

// normalizeCognitionClaims keeps the persisted claim contract canonical while
// accepting the two provider-side conventions that appeared before the
// response schema was closed: `claim` as an alias for `content`, and semantic
// context references such as `current_message.content` or
// `life_context.activity`. These aliases are bound to the current source fact;
// Core does not trust arbitrary provider-supplied IDs as evidence.
func normalizeCognitionClaims(raw []any, sourceFactID string) []any {
	result := make([]any, 0, len(raw))
	for _, item := range raw {
		claim := mapValue(item)
		if len(claim) == 0 {
			result = append(result, item)
			continue
		}
		normalized := cloneMap(claim)
		if stringValue(normalized["content"]) == "" {
			if legacy := strings.TrimSpace(stringValue(normalized["claim"])); legacy != "" {
				normalized["content"] = legacy
			}
		}
		if stringValue(normalized["kind"]) == "" && stringValue(normalized["claim_type"]) == "" && stringValue(normalized["claim"]) != "" {
			normalized["kind"] = ClaimObservedFact
		}
		refs := arrayValue(normalized["evidence_refs"])
		if len(refs) > 0 {
			canonical := make([]any, 0, len(refs))
			for _, rawRef := range refs {
				ref := strings.TrimSpace(stringValue(rawRef))
				if isSemanticCognitionEvidenceRef(ref) {
					// The current life context is part of the frozen turn
					// projection, so the turn fact is the durable evidence anchor.
					ref = sourceFactID
				}
				canonical = append(canonical, ref)
			}
			normalized["evidence_refs"] = canonical
		}
		result = append(result, normalized)
	}
	return result
}

func isSemanticCognitionEvidenceRef(value string) bool {
	if value == "current_message" || value == "current_message.content" || value == "current_life_context" || value == "life_context" {
		return true
	}
	return strings.HasPrefix(value, "life_context.") || strings.HasPrefix(value, "current_state.")
}

func normalizeOutputPreferenceDecision(value map[string]any, activeProfileID string) (map[string]any, error) {
	result := cloneMap(value)
	channel := strings.TrimSpace(stringValue(result["channel"]))
	if channel == "" {
		channel = "none"
	}
	switch channel {
	case "text", "image", "none":
		// These channels are currently realized by the conversation path.
	case "voice", "moment":
		// The schema can receive these future channels for forward-compatible
		// personas, but this runtime must not silently execute them yet.
		result["matched"] = false
		result["status"] = "unsupported"
		result["reason"] = "output_channel_not_installed"
	default:
		return nil, errors.New("output_preference_channel_invalid")
	}
	result["channel"] = channel
	if _, ok := result["matched"].(bool); !ok {
		return nil, errors.New("output_preference_matched_invalid")
	}
	if confidence, ok := numberFloat(result["confidence"]); !ok || confidence < 0 || confidence > 1 {
		return nil, errors.New("output_preference_confidence_invalid")
	}
	if activeProfileID != "" {
		result["profile_id"] = activeProfileID
	}
	return result, nil
}

// evaluateOutputPreferenceAction is the final Core-owned reconciliation
// between the model's semantic preference decision and the frozen action. It
// never invents a media concept or executes a side effect; it records whether
// the requested channel was actually bound to an installed capability.
func evaluateOutputPreferenceAction(value map[string]any, action string, calls []CapabilityInvocation, registries ...*CapabilityRegistry) map[string]any {
	result := cloneMap(value)
	matched, _ := result["matched"].(bool)
	channel := stringValue(result["channel"])
	if channel == "image" && matched {
		bound := false
		var registry *CapabilityRegistry
		if len(registries) > 0 {
			registry = registries[0]
		}
		for _, invocation := range calls {
			if registry != nil {
				if definition, ok := registry.Definition(invocation.CapabilityName); ok && definition.OutputRole == "media" && definition.IsDeferredOutput() {
					bound = true
					break
				}
			}
		}
		if bound {
			result["status"] = "authorized"
		} else {
			result["status"] = "matched_without_capability_request"
		}
	} else if channel == "text" && matched && action == "reply" {
		result["status"] = "authorized"
	} else if channel == "none" || !matched {
		result["status"] = "no_op"
	}
	return result
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
