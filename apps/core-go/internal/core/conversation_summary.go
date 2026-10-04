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
	conversationSummaryPromptVersion    = "conversation-summary.prompt.v2"
	conversationSummaryPolicyVersion    = "conversation-summary.policy.v1"
	conversationSummaryRetainedMessages = 24
	conversationSummaryMaxMessages      = 40
	conversationSummaryTargetTurns      = 20
	conversationSummaryTargetTokens     = 6000
	conversationSummaryMaxRunes         = 12000
	conversationSummaryDefaultBudget    = 24000
	conversationSummaryMaxResults       = 20
)

const conversationSummaryInstruction = "只总结所提供的原始对话消息，保留已明确表达的决定、用户纠正、未完成约定、真实行动结果、必要指代及其时间。区分想买、已安排购买、购买失败、已获得和已穿上；历史发长或衣着必须写成过去事实，不能写成无时间含义的当前设定。计划、模型回复的过去式和亲密表达不能冒充已执行结果或人格变化。不得加入旧摘要、隐藏推理、工具内部参数、数据库标识或未被原文支持的新事实；摘要不写入物品、日程或当前身体状态。"

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

func invalidateConversationSummariesBySourceRefsTx(ctx context.Context, tx pgx.Tx, fluctlightID string, sourceRefs []string) error {
	if len(sourceRefs) == 0 {
		return nil
	}
	_, err := tx.Exec(ctx, `UPDATE public.conversation_summaries SET status='invalidated'
WHERE owner_fluctlight_id=$1 AND status IN ('active','consolidated') AND source_message_refs ?| $2::text[]`, fluctlightID, sourceRefs)
	return err
}

func (a *App) enqueueConversationSummaryIntentTx(ctx context.Context, tx pgx.Tx, fluctlightID, conversationID, sourceMessageID string) error {
	if conversationID == "" {
		if err := tx.QueryRow(ctx, `SELECT conversation_id FROM public.conversation_messages WHERE id=$1`, sourceMessageID).Scan(&conversationID); err != nil {
			return err
		}
	}
	return a.enqueueRuntimeSummaryTx(ctx, tx, fluctlightID, conversationID, sourceMessageID)
}

func enqueueConversationSummaryChunkTx(ctx context.Context, tx pgx.Tx, fluctlightID, conversationID, sourceMessageID string, sourceSequence int, chunk []ConversationSummarySourceMessage) error {
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
	work := conversationSummaryWork{IntentID: strings.TrimSpace(intentID), FluctlightID: strings.TrimSpace(fluctlightID), ConversationID: strings.TrimSpace(conversationID), SourceMessageID: strings.TrimSpace(sourceMessageID), SourceSequence: sourceSequence, FromSequence: fromSequence, ToSequence: toSequence, SourceDigest: strings.TrimSpace(sourceDigest), SourceMessageRefs: append([]string(nil), sourceMessageRefs...)}
	return a.processRuntimeSummary(ctx, work)
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

func invalidateSummaryIDTx(ctx context.Context, tx pgx.Tx, owner, id string) error {
	command, err := tx.Exec(ctx, `UPDATE public.conversation_summaries SET status='invalidated',revision=revision+1 WHERE id=$1 AND owner_fluctlight_id=$2 AND status IN ('active','consolidated')`, id, owner)
	if err != nil {
		return err
	}
	if command.RowsAffected() != 1 {
		return ErrConflict
	}
	return nil
}
func restoreSummaryIDTx(ctx context.Context, tx pgx.Tx, owner, id string, before map[string]any) error {
	messages, err := readConversationSummaryMessages(ctx, tx, stringValue(before["conversation_id"]), intValue(before["from_sequence"]), intValue(before["to_sequence"]), conversationSummaryMaxMessages)
	if err != nil {
		return err
	}
	if conversationSummarySourceDigest(messages) != stringValue(before["source_digest"]) {
		return errors.New("repair_summary_source_changed")
	}
	_, err = tx.Exec(ctx, `UPDATE public.conversation_summaries SET status=$3,revision=revision+1 WHERE id=$1 AND owner_fluctlight_id=$2`, id, owner, before["status"])
	return err
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
	rows, err := a.DB.Pool().Query(ctx, `SELECT id,from_sequence,to_sequence,source_message_refs,source_digest,summary,revision,completed_at,
		COALESCE(ending_state,''),open_threads,core_events,COALESCE(timezone,''),COALESCE(local_date::text,'')
		FROM public.conversation_summaries WHERE owner_fluctlight_id=$1 AND conversation_id=$2 AND status='active' ORDER BY to_sequence DESC,revision DESC LIMIT $3`, query.FluctlightID, query.ConversationID, conversationSummaryMaxResults)
	if err != nil {
		return ConversationSummaryRetrievalResult{}, err
	}
	defer rows.Close()
	type candidate struct {
		id, digest, summary, endingState, timezone, localDate string
		from, to, revision                                    int
		refs                                                  []string
		completed                                             time.Time
		openThreads, coreEvents                               []byte
	}
	candidates := make([]candidate, 0)
	for rows.Next() {
		var item candidate
		var refs []byte
		if err := rows.Scan(&item.id, &item.from, &item.to, &refs, &item.digest, &item.summary, &item.revision, &item.completed, &item.endingState, &item.openThreads, &item.coreEvents, &item.timezone, &item.localDate); err != nil {
			return ConversationSummaryRetrievalResult{}, err
		}
		item.refs = conversationSummarySourceRefValues(decodeArray(refs))
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
		windowLength := candidate.to - candidate.from + 1
		validSource := false
		var sourceMessages []ConversationSummarySourceMessage
		if windowLength > 0 && windowLength <= conversationSummaryMaxMessages {
			var sourceErr error
			sourceMessages, sourceErr = readConversationSummaryMessages(ctx, a.DB.Pool(), query.ConversationID, candidate.from, candidate.to, windowLength)
			if sourceErr != nil {
				return ConversationSummaryRetrievalResult{}, sourceErr
			}
			validSource = len(sourceMessages) == windowLength && sameStringSlice(conversationSummarySourceRefs(sourceMessages), candidate.refs) && conversationSummarySourceDigest(sourceMessages) == candidate.digest
		}
		if !validSource {
			disposition, reason = "dropped", "source_invalid"
		} else if len(items) >= query.Limit {
			disposition, reason = "dropped", "result_limit"
		} else if trace.BudgetUsed+cost > query.MaxRunes {
			disposition, reason = "dropped", "summary_budget"
		} else {
			trace.BudgetUsed += cost
			item := map[string]any{
				"ref":     "summary:ctx_" + stableDigest(candidate.id+"\x1f"+fmt.Sprint(candidate.revision)+"\x1f"+candidate.digest),
				"summary": candidate.summary, "from_sequence": candidate.from, "to_sequence": candidate.to,
				"source_digest": candidate.digest, "source_message_refs": candidate.refs, "revision": candidate.revision,
				"source_kind": "episode_memory", "completed_at": candidate.completed.UTC().Format(time.RFC3339Nano),
				"started_at": sourceMessages[0].CreatedAt.UTC().Format(time.RFC3339Nano),
				"ended_at":   sourceMessages[len(sourceMessages)-1].CreatedAt.UTC().Format(time.RFC3339Nano),
			}
			for key, value := range map[string]string{"ending_state": candidate.endingState, "timezone": candidate.timezone, "local_date": candidate.localDate} {
				if value != "" {
					item[key] = value
				}
			}
			if values := decodeArray(candidate.openThreads); len(values) > 0 {
				item["open_threads"] = values
			}
			if values := decodeArray(candidate.coreEvents); len(values) > 0 {
				item["core_events"] = values
			}
			items = append(items, item)
		}
		trace.Selections = append(trace.Selections, ConversationSummarySelectionTrace{SummaryID: candidate.id, Disposition: disposition, Reason: reason, Runes: cost})
	}
	trace.SelectedCount = len(items)
	sort.Slice(items, func(i, j int) bool { return intValue(items[i]["from_sequence"]) < intValue(items[j]["from_sequence"]) })
	return ConversationSummaryRetrievalResult{Items: items, Trace: trace}, nil
}

// Explicit scoped repair is a projection lifecycle operation; Raw messages
// and the original summary remain available for audit.
