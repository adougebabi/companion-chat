package core

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"sort"
	"strings"
	"time"
)

const (
	memoryRetrievalPlanVersion = "fluctlight.memory-retrieval.v1"
	memoryCandidateLimit       = 200
	maxMemoryQueryCues         = 32
	maxMemoryQueryCueRunes     = 1000
	maxMemoryQueryTokens       = 1024
	maxMemoryRetrievalBudget   = 16000
)

type MemoryRetrievalOperation string

const (
	MemoryForConversation      MemoryRetrievalOperation = "conversation"
	MemoryForWakeUp            MemoryRetrievalOperation = "wake_up"
	MemoryForDailyReview       MemoryRetrievalOperation = "daily_review"
	MemoryForNativeCognition   MemoryRetrievalOperation = "native_cognition"
	MemoryForReflection        MemoryRetrievalOperation = "reflection"
	MemoryForCapabilityPlanner MemoryRetrievalOperation = "capability_planner"
)

type MemoryConversationScopeMode string

const (
	MemoryConversationExact      MemoryConversationScopeMode = "exact"
	MemoryConversationAllowedSet MemoryConversationScopeMode = "allowed_set"
	MemoryConversationGlobalOnly MemoryConversationScopeMode = "global_only"
)

type MemoryQueryCue struct {
	Kind           string `json:"kind"`
	Text           string `json:"text"`
	Ref            string `json:"ref,omitempty"`
	AllowEmbedding bool   `json:"allow_embedding,omitempty"`
}

type MemoryQueryPlan struct {
	SchemaVersion          string                      `json:"schema_version"`
	Operation              MemoryRetrievalOperation    `json:"operation"`
	Mode                   string                      `json:"mode"`
	ViewerActorIDs         []string                    `json:"viewer_actor_ids"`
	ConversationMode       MemoryConversationScopeMode `json:"conversation_mode"`
	ConversationID         string                      `json:"conversation_id,omitempty"`
	AllowedConversationIDs []string                    `json:"allowed_conversation_ids,omitempty"`
	ActiveProfileID        string                      `json:"active_profile_id,omitempty"`
	AllowedTypes           []string                    `json:"allowed_types"`
	Cues                   []MemoryQueryCue            `json:"cues"`
	Query                  string                      `json:"query,omitempty"`
	EmbeddingQuery         string                      `json:"embedding_query,omitempty"`
	ResultLimit            int                         `json:"result_limit"`
	Budget                 int                         `json:"budget"`
	CandidateLimit         int                         `json:"candidate_limit"`
}

type MemoryRetrievalTrace struct {
	PlanVersion              string            `json:"plan_version"`
	PlanID                   string            `json:"plan_id"`
	Operation                string            `json:"operation"`
	Mode                     string            `json:"mode"`
	QueryDigest              string            `json:"query_digest,omitempty"`
	CueKinds                 []string          `json:"cue_kinds"`
	ConversationMode         string            `json:"conversation_mode"`
	AuthorizedViewerCount    int               `json:"authorized_viewer_count"`
	AuthorizedCandidateCount int               `json:"authorized_candidate_count"`
	VectorCandidateCount     int               `json:"vector_candidate_count"`
	EmbeddingDisposition     string            `json:"embedding_disposition"`
	FallbackReason           string            `json:"fallback_reason,omitempty"`
	ResultLimit              int               `json:"result_limit"`
	ResultCount              int               `json:"result_count"`
	Budget                   int               `json:"budget"`
	BudgetUsed               int               `json:"budget_used"`
	TruncatedReason          string            `json:"truncated_reason,omitempty"`
	Ranking                  []MemoryRankTrace `json:"ranking"`
}

type MemoryRankTrace struct {
	MemoryID    string             `json:"memory_id"`
	Components  map[string]float64 `json:"components"`
	Score       float64            `json:"score"`
	Disposition string             `json:"disposition"`
	Reason      string             `json:"reason"`
}

type MemoryRetrievalResult struct {
	Items []map[string]any
	Trace MemoryRetrievalTrace
}

type ContextProjectionRequest struct {
	AuthorizationActorID   string
	SpeakerActorID         string
	FluctlightID           string
	ConversationID         string
	SourceFactID           string
	CurrentUserText        string
	MemoryOperation        MemoryRetrievalOperation
	MemoryConversationMode MemoryConversationScopeMode
	AllowedConversationIDs []string
	MemoryCues             []MemoryQueryCue
}

// buildProjectionMemoryCues only constructs local FTS/lexical cues. These
// private state values never set AllowEmbedding and therefore cannot enter the
// external embedding request boundary.
func buildProjectionMemoryCues(operation MemoryRetrievalOperation, base []MemoryQueryCue, currentText string, life, state map[string]any, recent, active, goals, intentions, outcomes, hypotheses []map[string]any) []MemoryQueryCue {
	result := append([]MemoryQueryCue(nil), base...)
	for index := range result {
		result[index].AllowEmbedding = false
	}
	addLocal := func(kind string, value any) {
		text := strings.TrimSpace(stringValue(value))
		if text != "" {
			result = append(result, MemoryQueryCue{Kind: kind, Text: text})
		}
	}
	if operation == MemoryForConversation {
		addLocal("current_message", currentText)
	}
	for _, field := range []string{"scene", "activity", "location"} {
		addLocal("life_"+field, life[field])
	}
	for _, field := range []string{"mood", "pad", "drives", "conflicts"} {
		if value := state[field]; value != nil {
			addLocal("current_state_"+field, jsonString(value))
		}
	}
	start := 0
	if len(recent) > 6 {
		start = len(recent) - 6
	}
	for _, message := range recent[start:] {
		addLocal("recent_"+firstString(message["kind"], "message"), message["text"])
	}
	for _, item := range active {
		addLocal("active_memory", item["content"])
	}
	for _, goal := range goals {
		if status := stringValue(goal["status"]); status == "active" || status == "candidate" || status == "paused" {
			addLocal("goal", goal["description"])
		}
	}
	for _, intention := range intentions {
		if status := stringValue(intention["status"]); status == "pending" || status == "qualified" || status == "active" || status == "paused" {
			addLocal("intention", intention["action"])
		}
	}
	for _, outcome := range outcomes {
		safe := compactRecentActionOutcomes([]map[string]any{outcome})
		if len(safe) > 0 {
			addLocal("recent_outcome", jsonString(safe[0]))
		}
	}
	for _, hypothesis := range hypotheses {
		if status := stringValue(hypothesis["status"]); status == "active" || status == "uncertain" {
			addLocal("unresolved", hypothesis["content"])
		}
	}
	return result
}

func buildMemoryQueryPlan(operation MemoryRetrievalOperation, viewers []string, mode MemoryConversationScopeMode, conversationID string, allowedConversationIDs []string, activeProfileID string, cues []MemoryQueryCue, limit, budget int) (MemoryQueryPlan, error) {
	switch operation {
	case MemoryForConversation, MemoryForWakeUp, MemoryForDailyReview, MemoryForNativeCognition, MemoryForReflection, MemoryForCapabilityPlanner:
	default:
		return MemoryQueryPlan{}, errors.New("memory_retrieval_operation_invalid")
	}
	viewerSet := make(map[string]struct{}, len(viewers))
	for _, viewer := range viewers {
		if viewer = strings.TrimSpace(viewer); viewer != "" {
			viewerSet[viewer] = struct{}{}
		}
	}
	if len(viewerSet) == 0 {
		return MemoryQueryPlan{}, errors.New("memory_retrieval_viewer_required")
	}
	viewerIDs := make([]string, 0, len(viewerSet))
	for viewer := range viewerSet {
		viewerIDs = append(viewerIDs, viewer)
	}
	sort.Strings(viewerIDs)
	allowedSet := make(map[string]struct{}, len(allowedConversationIDs))
	for _, allowed := range allowedConversationIDs {
		if allowed = strings.TrimSpace(allowed); allowed != "" {
			allowedSet[allowed] = struct{}{}
		}
	}
	allowedIDs := make([]string, 0, len(allowedSet))
	for allowed := range allowedSet {
		allowedIDs = append(allowedIDs, allowed)
	}
	sort.Strings(allowedIDs)
	conversationID = strings.TrimSpace(conversationID)
	switch mode {
	case MemoryConversationExact:
		if conversationID == "" {
			return MemoryQueryPlan{}, errors.New("memory_conversation_exact_required")
		}
	case MemoryConversationAllowedSet:
		if len(allowedIDs) == 0 {
			return MemoryQueryPlan{}, errors.New("memory_conversation_set_required")
		}
	case MemoryConversationGlobalOnly:
		conversationID = ""
		allowedIDs = nil
	default:
		return MemoryQueryPlan{}, errors.New("memory_conversation_scope_invalid")
	}
	if limit < 1 {
		limit = 1
	}
	if limit > 50 {
		limit = 50
	}
	if budget < 128 {
		budget = 128
	}
	if budget > maxMemoryRetrievalBudget {
		budget = maxMemoryRetrievalBudget
	}
	if len(cues) > maxMemoryQueryCues {
		cues = cues[:maxMemoryQueryCues]
	}
	cleanCues := make([]MemoryQueryCue, 0, len(cues))
	queryParts := make([]string, 0, len(cues))
	embeddingParts := make([]string, 0, 1)
	cueSeen := make(map[string]struct{}, len(cues))
	for _, cue := range cues {
		cue.Kind = strings.TrimSpace(cue.Kind)
		cue.Text = strings.TrimSpace(cue.Text)
		cue.Ref = strings.TrimSpace(cue.Ref)
		if cue.Kind == "" || cue.Text == "" {
			continue
		}
		runes := []rune(cue.Text)
		if len(runes) > maxMemoryQueryCueRunes {
			cue.Text = string(runes[:maxMemoryQueryCueRunes])
		}
		identity := cue.Kind + "\x1f" + cue.Text + "\x1f" + cue.Ref
		if _, duplicate := cueSeen[identity]; duplicate {
			continue
		}
		cueSeen[identity] = struct{}{}
		candidateQuery := strings.Join(append(append([]string(nil), queryParts...), cue.Text), "\n")
		if EstimatePromptTokens(candidateQuery) > maxMemoryQueryTokens {
			continue
		}
		cleanCues = append(cleanCues, cue)
		queryParts = append(queryParts, cue.Text)
		if cue.AllowEmbedding {
			embeddingParts = append(embeddingParts, cue.Text)
		}
	}
	query := strings.Join(queryParts, "\n")
	retrievalMode := "lexical_salience"
	if query == "" {
		retrievalMode = "salience_recent"
	} else if len(embeddingParts) > 0 {
		retrievalMode = "semantic_hybrid"
	}
	return MemoryQueryPlan{
		SchemaVersion: memoryRetrievalPlanVersion, Operation: operation, Mode: retrievalMode,
		ViewerActorIDs: viewerIDs, ConversationMode: mode, ConversationID: conversationID,
		AllowedConversationIDs: allowedIDs, ActiveProfileID: strings.TrimSpace(activeProfileID),
		AllowedTypes: []string{"episodic", "semantic", "relationship", "autobiographical"},
		Cues:         cleanCues, Query: query, EmbeddingQuery: strings.Join(embeddingParts, "\n"),
		ResultLimit: limit, Budget: budget, CandidateLimit: memoryCandidateLimit,
	}, nil
}

func (a *App) retrieveMemoryWithPlan(ctx context.Context, authorizationActorID, fluctlightID string, plan MemoryQueryPlan) (MemoryRetrievalResult, error) {
	if plan.SchemaVersion != memoryRetrievalPlanVersion || strings.TrimSpace(authorizationActorID) == "" || strings.TrimSpace(fluctlightID) == "" {
		return MemoryRetrievalResult{}, errors.New("memory_retrieval_plan_invalid")
	}
	fluctlight, err := a.DB.GetFluctlight(ctx, fluctlightID, authorizationActorID)
	if err != nil {
		return MemoryRetrievalResult{}, err
	}
	ownerActorID := authorizationActorID
	ownerVisible := false
	for _, viewer := range plan.ViewerActorIDs {
		if viewer == ownerActorID {
			ownerVisible = true
			break
		}
	}
	trace := MemoryRetrievalTrace{
		PlanVersion: plan.SchemaVersion, PlanID: "memory-plan:" + stableDigest(string(jsonBytes(plan))),
		Operation: string(plan.Operation), Mode: plan.Mode, ConversationMode: string(plan.ConversationMode),
		AuthorizedViewerCount: len(plan.ViewerActorIDs), EmbeddingDisposition: "not_requested",
		ResultLimit: plan.ResultLimit, Budget: plan.Budget, Ranking: []MemoryRankTrace{},
	}
	if plan.Query != "" {
		trace.QueryDigest = stableDigest(plan.Query)
	}
	for _, cue := range plan.Cues {
		trace.CueKinds = append(trace.CueKinds, cue.Kind)
	}
	rows, err := a.DB.Pool().Query(ctx, `
		SELECT id,type,content,actor_refs,conversation_id,event_refs,evidence_refs,personality_perspectives,
		       confidence,importance,emotional_significance,visibility,status,revision,created_at,
		       CASE WHEN query_terms.query IS NULL THEN 0 ELSE COALESCE(ts_rank_cd(search_document,query_terms.query),0) END
		FROM public.memories
		CROSS JOIN LATERAL (
		  SELECT CASE WHEN cardinality(tsvector_to_array(to_tsvector('simple',$2::text)))=0 THEN NULL::tsquery
		              ELSE to_tsquery('simple',array_to_string(tsvector_to_array(to_tsvector('simple',$2::text)),' | ')) END AS query
		) query_terms
		WHERE owner_fluctlight_id=$1
		  AND status='active'
		  AND type=ANY($3::text[])
		  AND (
		    (visibility IN ('private','owner') AND $4)
		    OR (visibility='participants' AND actor_refs ?| $5::text[])
		  )
		  AND (
		    ($6='global_only' AND conversation_id IS NULL)
		    OR ($6='exact' AND (conversation_id IS NULL OR conversation_id=$7))
		    OR ($6='allowed_set' AND (conversation_id IS NULL OR conversation_id=ANY($8::text[])))
		  )
		ORDER BY CASE WHEN query_terms.query IS NOT NULL AND search_document @@ query_terms.query THEN 1 ELSE 0 END DESC,
		         CASE WHEN query_terms.query IS NULL THEN 0 ELSE COALESCE(ts_rank_cd(search_document,query_terms.query),0) END DESC,
		         importance DESC,created_at DESC,id DESC
		LIMIT $9`, fluctlight.ID, plan.Query, plan.AllowedTypes, ownerVisible, plan.ViewerActorIDs, string(plan.ConversationMode), nullableString(plan.ConversationID), plan.AllowedConversationIDs, plan.CandidateLimit)
	if err != nil {
		return MemoryRetrievalResult{}, err
	}
	defer rows.Close()
	queryTokens := tokenize(plan.Query)
	type scoredMemory struct {
		value      map[string]any
		score      float64
		components map[string]float64
		created    time.Time
	}
	scored := make([]scoredMemory, 0)
	for rows.Next() {
		var id, typ, content, visibility, status string
		var actorRefs, eventRefs, evidenceRefs, perspectives []byte
		var conversationRef *string
		var confidence, importance, emotional float64
		var revision int
		var created time.Time
		var searchRank float64
		if err := rows.Scan(&id, &typ, &content, &actorRefs, &conversationRef, &eventRefs, &evidenceRefs, &perspectives, &confidence, &importance, &emotional, &visibility, &status, &revision, &created, &searchRank); err != nil {
			return MemoryRetrievalResult{}, err
		}
		components := map[string]float64{"importance": importance, "emotional_significance": emotional * 0.5, "confidence": confidence * 0.25, "fts_rank": searchRank}
		score := importance + emotional*0.5 + confidence*0.25 + searchRank
		ageHours := math.Max(0, time.Since(created).Hours())
		components["recency"] = 0.25 / (1 + ageHours/(7*24))
		score += components["recency"]
		lowerContent := strings.ToLower(content)
		for _, token := range queryTokens {
			if strings.Contains(lowerContent, token) {
				components["lexical_overlap"]++
			}
		}
		score += components["lexical_overlap"]
		value := map[string]any{
			"id": id, "type": typ, "content": content, "confidence": confidence,
			"importance": importance, "emotional_significance": emotional,
			"visibility": visibility, "status": status, "revision": revision,
			"conversation_id": conversationRef, "event_refs": decodeArray(eventRefs),
			"evidence_refs": decodeArray(evidenceRefs), "source": "memory:" + id,
			"created_at": created.Format(time.RFC3339Nano), "actor_refs": decodeArray(actorRefs),
		}
		if values := decodeArray(perspectives); len(values) > 0 {
			value["personality_perspectives"] = values
		}
		scored = append(scored, scoredMemory{value: value, score: score, components: components, created: created})
	}
	if err := rows.Err(); err != nil {
		return MemoryRetrievalResult{}, err
	}
	trace.AuthorizedCandidateCount = len(scored)
	if plan.Mode == "semantic_hybrid" && a.Provider != nil && strings.TrimSpace(plan.EmbeddingQuery) != "" {
		trace.EmbeddingDisposition = "fallback"
		modelID, queryVector, embedErr := a.Provider.Embed(WithProviderScenario(ctx, "memory_retrieval"), plan.EmbeddingQuery)
		if embedErr != nil {
			trace.FallbackReason = "embedding_unavailable"
		} else {
			candidateIDs := make([]string, 0, len(scored))
			for _, candidate := range scored {
				candidateIDs = append(candidateIDs, stringValue(candidate.value["id"]))
			}
			if len(candidateIDs) == 0 {
				trace.FallbackReason = "no_authorized_candidates"
			} else if vectorRows, vectorErr := a.DB.Pool().Query(ctx, `SELECT e.memory_id,e.embedding FROM public.memory_embeddings e JOIN public.memories m ON m.id=e.memory_id WHERE e.memory_id=ANY($1::text[]) AND e.status='ready' AND e.model_id=$2 AND e.memory_revision=m.revision AND m.status='active' ORDER BY e.created_at DESC`, candidateIDs, modelID); vectorErr != nil {
				trace.FallbackReason = "vector_query_failed"
			} else {
				vectors := make(map[string][]float64)
				invalidVectors := 0
				for vectorRows.Next() {
					var memoryID string
					var raw []byte
					if scanErr := vectorRows.Scan(&memoryID, &raw); scanErr != nil {
						invalidVectors++
						trace.FallbackReason = "vector_scan_failed"
						break
					}
					var vector []float64
					if json.Unmarshal(raw, &vector) == nil && len(vector) == len(queryVector) && len(vector) > 0 {
						vectors[memoryID] = vector
					} else {
						invalidVectors++
					}
				}
				if rowsErr := vectorRows.Err(); rowsErr != nil {
					trace.FallbackReason = "vector_rows_failed"
				}
				vectorRows.Close()
				trace.VectorCandidateCount = len(vectors)
				if len(vectors) > 0 {
					trace.EmbeddingDisposition = "applied"
					for index := range scored {
						if vector := vectors[stringValue(scored[index].value["id"])]; len(vector) > 0 {
							scored[index].components["vector_similarity"] = cosineSimilarity(queryVector, vector)
							scored[index].score += scored[index].components["vector_similarity"]
						}
					}
				} else if trace.FallbackReason == "" {
					trace.FallbackReason = "vector_incompatible_or_missing"
				}
				if invalidVectors > 0 && len(vectors) > 0 {
					trace.FallbackReason = "partial_vector_incompatible"
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
	items := make([]map[string]any, 0, plan.ResultLimit)
	used := 0
	for _, item := range scored {
		disposition, reason := "selected", "ranked"
		if len(items) >= plan.ResultLimit {
			trace.TruncatedReason = "result_limit"
			disposition, reason = "dropped", "result_limit"
			trace.Ranking = append(trace.Ranking, MemoryRankTrace{MemoryID: stringValue(item.value["id"]), Components: item.components, Score: item.score, Disposition: disposition, Reason: reason})
			continue
		}
		cost := len([]rune(string(jsonBytes(map[string]any{
			"type": item.value["type"], "content": item.value["content"], "confidence": item.value["confidence"],
			"importance": item.value["importance"], "emotional_significance": item.value["emotional_significance"],
			"created_at": item.value["created_at"], "personality_perspectives": item.value["personality_perspectives"],
		}))))
		if used+cost > plan.Budget {
			trace.TruncatedReason = "budget"
			disposition, reason = "dropped", "budget"
			trace.Ranking = append(trace.Ranking, MemoryRankTrace{MemoryID: stringValue(item.value["id"]), Components: item.components, Score: item.score, Disposition: disposition, Reason: reason})
			continue
		}
		used += cost
		items = append(items, item.value)
		trace.Ranking = append(trace.Ranking, MemoryRankTrace{MemoryID: stringValue(item.value["id"]), Components: item.components, Score: item.score, Disposition: disposition, Reason: reason})
	}
	trace.ResultCount = len(items)
	trace.BudgetUsed = used
	return MemoryRetrievalResult{Items: items, Trace: trace}, nil
}
