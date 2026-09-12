package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	conversationSummarySchemaVersion    = "conversation-summary.v1"
	conversationSummaryPromptVersion    = "conversation-summary.prompt.v1"
	conversationSummaryPolicyVersion    = "conversation-summary.policy.v1"
	conversationSummaryRetainedMessages = 24
	conversationSummaryMaxMessages      = 40
	conversationSummaryTargetTurns      = 20
	conversationSummaryTargetTokens     = 6000
	conversationSummaryMaxRunes         = 12000
	conversationSummaryDefaultBudget    = 24000
	conversationSummaryMaxResults       = 20
)

const conversationSummaryInstruction = "只总结所提供的原始对话消息，保留已明确表达的事实、约定、未解决事项与语气变化；不得加入旧摘要、隐藏推理、工具内部参数、数据库标识或未被原文支持的新事实。"

type ConversationSummarySourceMessage struct {
	ID             string    `json:"id"`
	Sequence       int       `json:"sequence"`
	AuthorActorID  string    `json:"author_actor_id"`
	Kind           string    `json:"kind"`
	Text           string    `json:"text"`
	AttachmentRefs []any     `json:"attachment_refs"`
	CreatedAt      time.Time `json:"created_at"`
}

type conversationSummaryProviderResponse struct {
	SchemaVersion string `json:"schema_version"`
	Summary       string `json:"summary"`
}

type conversationSummaryWork struct {
	IntentID          string
	FluctlightID      string
	ConversationID    string
	SourceMessageID   string
	SourceSequence    int
	FromSequence      int
	ToSequence        int
	SourceDigest      string
	SourceMessageRefs []string
	Messages          []ConversationSummarySourceMessage
}

func estimateConversationSummaryTokens(message ConversationSummarySourceMessage) int {
	runes := len([]rune(message.Text))
	bytesCount := len([]byte(message.Text))
	runeEstimate := (runes + 1) / 2
	byteEstimate := (bytesCount + 3) / 4
	if byteEstimate > runeEstimate {
		runeEstimate = byteEstimate
	}
	return runeEstimate + 8
}

func selectConversationSummaryChunk(messages []ConversationSummarySourceMessage) ([]ConversationSummarySourceMessage, bool) {
	if len(messages) > conversationSummaryMaxMessages {
		messages = messages[:conversationSummaryMaxMessages]
	}
	tokens := 0
	completedTurns := 0
	thresholdReached := false
	lastAssistant := -1
	for index, message := range messages {
		tokens += estimateConversationSummaryTokens(message)
		if message.Kind == "assistant" {
			lastAssistant = index
			completedTurns++
		}
		if tokens >= conversationSummaryTargetTokens || completedTurns >= conversationSummaryTargetTurns || index+1 >= conversationSummaryMaxMessages {
			thresholdReached = true
		}
		if thresholdReached && lastAssistant == index {
			return append([]ConversationSummarySourceMessage(nil), messages[:index+1]...), true
		}
	}
	if thresholdReached && lastAssistant >= 0 {
		return append([]ConversationSummarySourceMessage(nil), messages[:lastAssistant+1]...), true
	}
	return nil, false
}

func conversationSummarySourceRefs(messages []ConversationSummarySourceMessage) []string {
	refs := make([]string, 0, len(messages))
	for _, message := range messages {
		refs = append(refs, "message:"+message.ID)
	}
	return refs
}

func conversationSummarySourceRefValues(value any) []string {
	result := make([]string, 0)
	for _, item := range arrayValue(value) {
		if ref := stringValue(item); ref != "" {
			result = append(result, ref)
		}
	}
	return result
}

func conversationSummarySourceDigest(messages []ConversationSummarySourceMessage) string {
	return stableDigest(jsonString(messages))
}

func (a *App) enqueueConversationSummaryIntentTx(ctx context.Context, tx pgx.Tx, fluctlightID, conversationID, sourceMessageID string) error {
	if strings.TrimSpace(fluctlightID) == "" || strings.TrimSpace(sourceMessageID) == "" {
		return errors.New("conversation_summary_source_identity_invalid")
	}
	if strings.TrimSpace(conversationID) == "" {
		if err := tx.QueryRow(ctx, `SELECT conversation_id FROM public.conversation_messages WHERE id=$1`, sourceMessageID).Scan(&conversationID); err != nil {
			return err
		}
	}
	var sourceSequence int
	var sourceAuthor, sourceKind string
	if err := tx.QueryRow(ctx, `SELECT sequence,author_actor_id,kind FROM public.conversation_messages WHERE id=$1 AND conversation_id=$2`, sourceMessageID, conversationID).Scan(&sourceSequence, &sourceAuthor, &sourceKind); err != nil {
		return err
	}
	if sourceAuthor != fluctlightID || sourceKind != "assistant" {
		return errors.New("conversation_summary_source_message_invalid")
	}
	eligibleThrough := sourceSequence - conversationSummaryRetainedMessages
	if eligibleThrough < 1 {
		return nil
	}
	var coveredThrough int
	if err := tx.QueryRow(ctx, `SELECT COALESCE(MAX(to_sequence),0) FROM public.conversation_summaries WHERE owner_fluctlight_id=$1 AND conversation_id=$2 AND status='active'`, fluctlightID, conversationID).Scan(&coveredThrough); err != nil {
		return err
	}
	if eligibleThrough <= coveredThrough {
		return nil
	}
	messages, err := readConversationSummaryMessages(ctx, tx, conversationID, coveredThrough+1, eligibleThrough, conversationSummaryMaxMessages)
	if err != nil {
		return err
	}
	chunk, ready := selectConversationSummaryChunk(messages)
	if !ready || len(chunk) == 0 {
		return nil
	}
	fromSequence, toSequence := chunk[0].Sequence, chunk[len(chunk)-1].Sequence
	refs := conversationSummarySourceRefs(chunk)
	digest := conversationSummarySourceDigest(chunk)
	identity := stableDigest(strings.Join([]string{fluctlightID, conversationID, fmt.Sprint(fromSequence), fmt.Sprint(toSequence), digest}, "\x1f"))
	intentID := "conversation_summary_intent:" + identity
	workflowID := "conversation_summary:" + identity
	payload := map[string]any{
		"intent_id": intentID, "fluctlight_id": fluctlightID, "conversation_id": conversationID,
		"source_message_id": sourceMessageID, "source_sequence": sourceSequence,
		"from_sequence": fromSequence, "to_sequence": toSequence,
		"source_digest": digest, "source_message_refs": refs,
	}
	inserted, err := tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload) VALUES($1,$2,'lifecycle','conversation.summary',$3) ON CONFLICT(intent_id) DO NOTHING`, intentID, workflowID, jsonBytes(payload))
	if err != nil {
		return err
	}
	if inserted.RowsAffected() == 1 {
		return nil
	}
	var existing []byte
	if err := tx.QueryRow(ctx, `SELECT payload FROM public.platform_workflow_intents WHERE intent_id=$1 AND intent_type='conversation.summary'`, intentID).Scan(&existing); err != nil {
		return err
	}
	if jsonString(decodeObject(existing)) != jsonString(payload) {
		return errors.New("conversation_summary_intent_identity_conflict")
	}
	return nil
}

type conversationSummaryRows interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func readConversationSummaryMessages(ctx context.Context, queryer conversationSummaryRows, conversationID string, fromSequence, toSequence, limit int) ([]ConversationSummarySourceMessage, error) {
	if strings.TrimSpace(conversationID) == "" || fromSequence < 1 || toSequence < fromSequence || limit < 1 || limit > conversationSummaryMaxMessages {
		return nil, errors.New("conversation_summary_window_invalid")
	}
	rows, err := queryer.Query(ctx, `SELECT id,sequence,author_actor_id,kind,text,attachment_refs,created_at FROM public.conversation_messages WHERE conversation_id=$1 AND sequence >= $2 AND sequence <= $3 ORDER BY sequence LIMIT $4`, conversationID, fromSequence, toSequence, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]ConversationSummarySourceMessage, 0, limit)
	for rows.Next() {
		var message ConversationSummarySourceMessage
		var attachmentRefs []byte
		if err := rows.Scan(&message.ID, &message.Sequence, &message.AuthorActorID, &message.Kind, &message.Text, &attachmentRefs, &message.CreatedAt); err != nil {
			return nil, err
		}
		message.AttachmentRefs = decodeArray(attachmentRefs)
		result = append(result, message)
	}
	return result, rows.Err()
}

func decodeConversationSummaryProviderResponse(value map[string]any) (conversationSummaryProviderResponse, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return conversationSummaryProviderResponse{}, errors.New("conversation_summary_response_invalid")
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	var response conversationSummaryProviderResponse
	if err := decoder.Decode(&response); err != nil {
		return conversationSummaryProviderResponse{}, fmt.Errorf("conversation_summary_response_invalid: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return conversationSummaryProviderResponse{}, errors.New("conversation_summary_response_trailing_payload")
	}
	response.Summary = strings.TrimSpace(response.Summary)
	if response.SchemaVersion != conversationSummarySchemaVersion || response.Summary == "" || len([]rune(response.Summary)) > conversationSummaryMaxRunes {
		return conversationSummaryProviderResponse{}, errors.New("conversation_summary_response_semantic_invalid")
	}
	return response, nil
}

func (a *App) ProcessConversationSummaryIntent(ctx context.Context, intentID, fluctlightID, conversationID, sourceMessageID string, sourceSequence, fromSequence, toSequence int, sourceDigest string, sourceMessageRefs []string) (map[string]any, error) {
	work := conversationSummaryWork{
		IntentID: strings.TrimSpace(intentID), FluctlightID: strings.TrimSpace(fluctlightID), ConversationID: strings.TrimSpace(conversationID),
		SourceMessageID: strings.TrimSpace(sourceMessageID), SourceSequence: sourceSequence,
		FromSequence: fromSequence, ToSequence: toSequence, SourceDigest: strings.TrimSpace(sourceDigest),
		SourceMessageRefs: append([]string(nil), sourceMessageRefs...),
	}
	if err := a.validateConversationSummaryWorkIdentity(ctx, work); err != nil {
		return nil, err
	}
	messages, err := readConversationSummaryMessages(ctx, a.DB.Pool(), work.ConversationID, work.FromSequence, work.ToSequence, conversationSummaryMaxMessages)
	if err != nil {
		return nil, err
	}
	work.Messages = messages
	if err := validateConversationSummarySources(work); err != nil {
		return nil, err
	}
	if a.Provider == nil {
		return nil, errors.New("conversation_summary_provider_unavailable")
	}
	assignment, err := a.Provider.assignment(ctx, "reflection")
	if err != nil {
		return nil, err
	}
	requestDigest := stableDigest(strings.Join([]string{work.SourceDigest, assignment.EndpointID, assignment.ModelID, conversationSummaryPromptVersion, conversationSummarySchemaVersion, conversationSummaryPolicyVersion}, "\x1f"))
	if existing, found, err := a.readReadyConversationSummary(ctx, work, requestDigest); err != nil {
		return nil, err
	} else if found {
		existing["replayed"] = true
		return existing, nil
	}
	providerMessages := conversationSummaryProviderMessages(work.Messages)
	correlationID := "conversation-summary:" + work.IntentID
	providerContext := WithProviderCorrelation(WithProviderScenario(ctx, "conversation_summary"), correlationID)
	structured, err := a.Provider.StructuredWithSchema(providerContext, "reflection", []map[string]any{
		{"role": "system", "content": conversationSummaryInstruction},
		{"role": "user", "content": jsonString(map[string]any{"source_messages": providerMessages})},
	}, "conversation_summary_v1", conversationSummaryProviderSchema(), false)
	if err != nil {
		return nil, err
	}
	response, err := decodeConversationSummaryProviderResponse(structured)
	if err != nil {
		return nil, err
	}
	liveAssignment, err := a.Provider.assignment(ctx, "reflection")
	if err != nil {
		return nil, err
	}
	if liveAssignment.EndpointID != assignment.EndpointID || liveAssignment.ModelID != assignment.ModelID {
		return nil, errors.New("conversation_summary_provider_assignment_stale")
	}
	providerRequestID := "provider:" + stableDigest("reflection:"+correlationID)
	return a.settleConversationSummary(ctx, work, response, assignment, providerRequestID, requestDigest)
}

func conversationSummaryProviderMessages(messages []ConversationSummarySourceMessage) []map[string]any {
	result := make([]map[string]any, 0, len(messages))
	for _, message := range messages {
		result = append(result, map[string]any{
			"role": message.Kind, "text": message.Text, "occurred_at": message.CreatedAt.UTC().Format(time.RFC3339Nano),
			"attachment_count": len(message.AttachmentRefs),
		})
	}
	return result
}

func (a *App) validateConversationSummaryWorkIdentity(ctx context.Context, work conversationSummaryWork) error {
	if a == nil || a.DB == nil || work.IntentID == "" || work.FluctlightID == "" || work.ConversationID == "" || work.SourceMessageID == "" || work.SourceSequence < 1 || work.FromSequence < 1 || work.ToSequence < work.FromSequence || work.ToSequence > work.SourceSequence || work.SourceDigest == "" || len(work.SourceMessageRefs) == 0 || len(work.SourceMessageRefs) > conversationSummaryMaxMessages {
		return errors.New("conversation_summary_work_invalid")
	}
	var payload []byte
	if err := a.DB.Pool().QueryRow(ctx, `SELECT payload FROM public.platform_workflow_intents WHERE intent_id=$1 AND intent_type='conversation.summary'`, work.IntentID).Scan(&payload); err != nil {
		return err
	}
	expected := decodeObject(payload)
	if stringValue(expected["fluctlight_id"]) != work.FluctlightID || stringValue(expected["conversation_id"]) != work.ConversationID || stringValue(expected["source_message_id"]) != work.SourceMessageID || intValue(expected["source_sequence"]) != work.SourceSequence || intValue(expected["from_sequence"]) != work.FromSequence || intValue(expected["to_sequence"]) != work.ToSequence || stringValue(expected["source_digest"]) != work.SourceDigest || !sameStringSlice(conversationSummarySourceRefValues(expected["source_message_refs"]), work.SourceMessageRefs) {
		return errors.New("conversation_summary_intent_conflict")
	}
	var author, kind string
	var sequence int
	if err := a.DB.Pool().QueryRow(ctx, `SELECT author_actor_id,kind,sequence FROM public.conversation_messages WHERE id=$1 AND conversation_id=$2`, work.SourceMessageID, work.ConversationID).Scan(&author, &kind, &sequence); err != nil {
		return err
	}
	if author != work.FluctlightID || kind != "assistant" || sequence != work.SourceSequence {
		return errors.New("conversation_summary_source_message_stale")
	}
	return nil
}

func validateConversationSummarySources(work conversationSummaryWork) error {
	if len(work.Messages) == 0 || work.Messages[0].Sequence != work.FromSequence || work.Messages[len(work.Messages)-1].Sequence != work.ToSequence || work.Messages[len(work.Messages)-1].Kind != "assistant" {
		return errors.New("conversation_summary_source_window_stale")
	}
	for index, message := range work.Messages {
		if message.Sequence != work.FromSequence+index {
			return errors.New("conversation_summary_source_sequence_gap")
		}
	}
	if conversationSummarySourceDigest(work.Messages) != work.SourceDigest || !sameStringSlice(conversationSummarySourceRefs(work.Messages), work.SourceMessageRefs) {
		return errors.New("conversation_summary_source_drift")
	}
	return nil
}

func sameStringSlice(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func (a *App) readReadyConversationSummary(ctx context.Context, work conversationSummaryWork, requestDigest string) (map[string]any, bool, error) {
	var id, summary string
	var revision int
	err := a.DB.Pool().QueryRow(ctx, `SELECT id,summary,revision FROM public.conversation_summaries WHERE owner_fluctlight_id=$1 AND conversation_id=$2 AND from_sequence=$3 AND to_sequence=$4 AND source_digest=$5 AND request_digest=$6 AND status='active'`, work.FluctlightID, work.ConversationID, work.FromSequence, work.ToSequence, work.SourceDigest, requestDigest).Scan(&id, &summary, &revision)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return map[string]any{"summary_id": id, "status": "active", "revision": revision, "from_sequence": work.FromSequence, "to_sequence": work.ToSequence, "summary": summary}, true, nil
}

func (a *App) settleConversationSummary(ctx context.Context, work conversationSummaryWork, response conversationSummaryProviderResponse, assignment providerAssignment, providerRequestID, requestDigest string) (map[string]any, error) {
	var result map[string]any
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var lockedConversationID string
		if err := tx.QueryRow(ctx, `SELECT id FROM public.conversations WHERE id=$1 FOR UPDATE`, work.ConversationID).Scan(&lockedConversationID); err != nil {
			return err
		}
		messages, err := readConversationSummaryMessages(ctx, tx, work.ConversationID, work.FromSequence, work.ToSequence, conversationSummaryMaxMessages)
		if err != nil {
			return err
		}
		liveWork := work
		liveWork.Messages = messages
		if err := validateConversationSummarySources(liveWork); err != nil {
			return err
		}
		if existing, found, err := readReadyConversationSummaryTx(ctx, tx, work, requestDigest); err != nil {
			return err
		} else if found {
			existing["replayed"] = true
			result = existing
			return nil
		}
		var priorID string
		var priorRevision int
		err = tx.QueryRow(ctx, `SELECT id,revision FROM public.conversation_summaries WHERE owner_fluctlight_id=$1 AND conversation_id=$2 AND from_sequence=$3 AND to_sequence=$4 AND status='active' FOR UPDATE`, work.FluctlightID, work.ConversationID, work.FromSequence, work.ToSequence).Scan(&priorID, &priorRevision)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if errors.Is(err, pgx.ErrNoRows) {
			var coveredThrough int
			if err := tx.QueryRow(ctx, `SELECT COALESCE(MAX(to_sequence),0) FROM public.conversation_summaries WHERE owner_fluctlight_id=$1 AND conversation_id=$2 AND status='active'`, work.FluctlightID, work.ConversationID).Scan(&coveredThrough); err != nil {
				return err
			}
			if coveredThrough+1 != work.FromSequence {
				return errors.New("conversation_summary_projection_stale")
			}
		}
		revision := 1
		if priorID != "" {
			revision = priorRevision + 1
			updated, err := tx.Exec(ctx, `UPDATE public.conversation_summaries SET status='superseded' WHERE id=$1 AND status='active' AND revision=$2`, priorID, priorRevision)
			if err != nil || updated.RowsAffected() != 1 {
				if err != nil {
					return err
				}
				return errors.New("conversation_summary_projection_stale")
			}
		}
		summaryID := "conversation_summary_" + stableDigest(work.IntentID+"\x1f"+requestDigest)
		idempotencyKey := "conversation-summary:" + work.IntentID + ":" + requestDigest
		_, err = tx.Exec(ctx, `INSERT INTO public.conversation_summaries(id,owner_fluctlight_id,conversation_id,from_sequence,to_sequence,source_message_refs,source_digest,summary,status,revision,supersedes_summary_id,provider_endpoint_id,model_id,provider_request_id,prompt_version,schema_version,policy_version,request_digest,idempotency_key) VALUES($1,$2,$3,$4,$5,$6,$7,$8,'active',$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)`, summaryID, work.FluctlightID, work.ConversationID, work.FromSequence, work.ToSequence, jsonBytes(work.SourceMessageRefs), work.SourceDigest, response.Summary, revision, nullableString(priorID), assignment.EndpointID, assignment.ModelID, providerRequestID, conversationSummaryPromptVersion, conversationSummarySchemaVersion, conversationSummaryPolicyVersion, requestDigest, idempotencyKey)
		if err != nil {
			return err
		}
		if err := appendOutboxTx(ctx, tx, "conversation.summary.ready", "conversation_summary", summaryID, work.FluctlightID, work.SourceMessageID, "conversation-summary:"+work.ConversationID, "conversation-summary-ready:"+summaryID, map[string]any{
			"summary_id": summaryID, "conversation_id": work.ConversationID, "from_sequence": work.FromSequence, "to_sequence": work.ToSequence,
			"source_digest": work.SourceDigest, "revision": revision, "status": "active",
		}); err != nil {
			return err
		}
		result = map[string]any{"summary_id": summaryID, "status": "active", "revision": revision, "from_sequence": work.FromSequence, "to_sequence": work.ToSequence, "summary": response.Summary, "replayed": false}
		return nil
	})
	return result, err
}

func readReadyConversationSummaryTx(ctx context.Context, tx pgx.Tx, work conversationSummaryWork, requestDigest string) (map[string]any, bool, error) {
	var id, summary string
	var revision int
	err := tx.QueryRow(ctx, `SELECT id,summary,revision FROM public.conversation_summaries WHERE owner_fluctlight_id=$1 AND conversation_id=$2 AND from_sequence=$3 AND to_sequence=$4 AND source_digest=$5 AND request_digest=$6 AND status='active' FOR UPDATE`, work.FluctlightID, work.ConversationID, work.FromSequence, work.ToSequence, work.SourceDigest, requestDigest).Scan(&id, &summary, &revision)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return map[string]any{"summary_id": id, "status": "active", "revision": revision, "from_sequence": work.FromSequence, "to_sequence": work.ToSequence, "summary": summary}, true, nil
}

type ConversationSummaryQuery struct {
	AuthorizationActorID string
	FluctlightID         string
	ConversationID       string
	Limit                int
	MaxRunes             int
}

type ConversationSummarySelectionTrace struct {
	SummaryID   string `json:"summary_id"`
	Disposition string `json:"disposition"`
	Reason      string `json:"reason"`
	Runes       int    `json:"runes"`
}

type ConversationSummaryRetrievalTrace struct {
	CandidateCount int                                 `json:"candidate_count"`
	SelectedCount  int                                 `json:"selected_count"`
	Budget         int                                 `json:"budget"`
	BudgetUsed     int                                 `json:"budget_used"`
	Selections     []ConversationSummarySelectionTrace `json:"selections"`
}

type ConversationSummaryRetrievalResult struct {
	Items []map[string]any                  `json:"items"`
	Trace ConversationSummaryRetrievalTrace `json:"trace"`
}

func (a *App) retrieveConversationSummaries(ctx context.Context, query ConversationSummaryQuery) (ConversationSummaryRetrievalResult, error) {
	if a == nil || a.DB == nil || strings.TrimSpace(query.AuthorizationActorID) == "" || strings.TrimSpace(query.FluctlightID) == "" || strings.TrimSpace(query.ConversationID) == "" {
		return ConversationSummaryRetrievalResult{}, errors.New("conversation_summary_query_invalid")
	}
	if _, err := a.DB.GetFluctlight(ctx, query.FluctlightID, query.AuthorizationActorID); err != nil {
		return ConversationSummaryRetrievalResult{}, err
	}
	if query.Limit < 1 {
		query.Limit = 1
	}
	if query.Limit > conversationSummaryMaxResults {
		query.Limit = conversationSummaryMaxResults
	}
	if query.MaxRunes < 1 {
		query.MaxRunes = conversationSummaryDefaultBudget
	}
	if query.MaxRunes > conversationSummaryDefaultBudget {
		query.MaxRunes = conversationSummaryDefaultBudget
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT id,from_sequence,to_sequence,source_digest,summary,revision,completed_at FROM public.conversation_summaries WHERE owner_fluctlight_id=$1 AND conversation_id=$2 AND status='active' ORDER BY to_sequence DESC,revision DESC LIMIT $3`, query.FluctlightID, query.ConversationID, conversationSummaryMaxResults)
	if err != nil {
		return ConversationSummaryRetrievalResult{}, err
	}
	defer rows.Close()
	type candidate struct {
		id, digest, summary string
		from, to, revision  int
		completed           time.Time
	}
	candidates := make([]candidate, 0)
	for rows.Next() {
		var item candidate
		if err := rows.Scan(&item.id, &item.from, &item.to, &item.digest, &item.summary, &item.revision, &item.completed); err != nil {
			return ConversationSummaryRetrievalResult{}, err
		}
		candidates = append(candidates, item)
	}
	if err := rows.Err(); err != nil {
		return ConversationSummaryRetrievalResult{}, err
	}
	trace := ConversationSummaryRetrievalTrace{CandidateCount: len(candidates), Budget: query.MaxRunes, Selections: make([]ConversationSummarySelectionTrace, 0, len(candidates))}
	items := make([]map[string]any, 0, query.Limit)
	for _, candidate := range candidates {
		cost := len([]rune(candidate.summary))
		disposition, reason := "selected", "recent_source_window"
		if len(items) >= query.Limit {
			disposition, reason = "dropped", "result_limit"
		} else if trace.BudgetUsed+cost > query.MaxRunes {
			disposition, reason = "dropped", "summary_budget"
		} else {
			trace.BudgetUsed += cost
			items = append(items, map[string]any{
				"ref":     "summary:ctx_" + stableDigest(candidate.id+"\x1f"+fmt.Sprint(candidate.revision)+"\x1f"+candidate.digest),
				"summary": candidate.summary, "from_sequence": candidate.from, "to_sequence": candidate.to,
				"source_digest": candidate.digest, "source_kind": "summary_projection", "completed_at": candidate.completed.UTC().Format(time.RFC3339Nano),
			})
		}
		trace.Selections = append(trace.Selections, ConversationSummarySelectionTrace{SummaryID: candidate.id, Disposition: disposition, Reason: reason, Runes: cost})
	}
	trace.SelectedCount = len(items)
	sort.Slice(items, func(i, j int) bool { return intValue(items[i]["from_sequence"]) < intValue(items[j]["from_sequence"]) })
	return ConversationSummaryRetrievalResult{Items: items, Trace: trace}, nil
}
