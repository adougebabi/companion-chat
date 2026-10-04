package core

// Legacy fixture helpers preserve historical source/rebuild regression coverage.
// They are excluded from production: runtime has one cumulative summary producer.
import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
)

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
		var invalidated bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.conversation_summaries WHERE owner_fluctlight_id=$1 AND conversation_id=$2 AND from_sequence=$3 AND to_sequence=$4 AND source_digest=$5 AND status='invalidated')`, work.FluctlightID, work.ConversationID, work.FromSequence, work.ToSequence, work.SourceDigest).Scan(&invalidated); err != nil {
			return err
		}
		if invalidated {
			return errors.New("conversation_summary_source_invalidated")
		}
		var consolidated bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.conversation_summaries WHERE owner_fluctlight_id=$1 AND conversation_id=$2 AND from_sequence=$3 AND to_sequence=$4 AND schema_version=$5 AND status='consolidated')`, work.FluctlightID, work.ConversationID, work.FromSequence, work.ToSequence, conversationSegmentSchemaVersion).Scan(&consolidated); err != nil {
			return err
		}
		if consolidated {
			result = map[string]any{"status": "superseded", "reason": "new_segment_consolidated", "from_sequence": work.FromSequence, "to_sequence": work.ToSequence}
			return nil
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
		var priorSchema string
		priorStatus := "active"
		err = tx.QueryRow(ctx, `SELECT id,revision,schema_version FROM public.conversation_summaries WHERE owner_fluctlight_id=$1 AND conversation_id=$2 AND from_sequence=$3 AND to_sequence=$4 AND status='active' FOR UPDATE`, work.FluctlightID, work.ConversationID, work.FromSequence, work.ToSequence).Scan(&priorID, &priorRevision, &priorSchema)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if priorID != "" && priorSchema == conversationSegmentSchemaVersion {
			result = map[string]any{"status": "superseded", "reason": "new_segment_active", "from_sequence": work.FromSequence, "to_sequence": work.ToSequence}
			return nil
		}
		if errors.Is(err, pgx.ErrNoRows) {
			priorStatus = "invalidated"
			err = tx.QueryRow(ctx, `SELECT id,revision FROM public.conversation_summaries WHERE owner_fluctlight_id=$1 AND conversation_id=$2 AND from_sequence=$3 AND to_sequence=$4 AND status='invalidated' AND source_digest<>$5 ORDER BY revision DESC LIMIT 1 FOR UPDATE`, work.FluctlightID, work.ConversationID, work.FromSequence, work.ToSequence, work.SourceDigest).Scan(&priorID, &priorRevision)
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
			if errors.Is(err, pgx.ErrNoRows) {
				var coveredThrough int
				if err := tx.QueryRow(ctx, `SELECT COALESCE(MAX(to_sequence),0) FROM public.conversation_summaries WHERE owner_fluctlight_id=$1 AND conversation_id=$2 AND status IN ('active','invalidated','consolidated')`, work.FluctlightID, work.ConversationID).Scan(&coveredThrough); err != nil {
					return err
				}
				if coveredThrough+1 != work.FromSequence {
					return errors.New("conversation_summary_projection_stale")
				}
			}
		}
		revision := 1
		if priorID != "" {
			revision = priorRevision + 1
			if priorStatus == "active" {
				updated, err := tx.Exec(ctx, `UPDATE public.conversation_summaries SET status='superseded' WHERE id=$1 AND status='active' AND revision=$2`, priorID, priorRevision)
				if err != nil || updated.RowsAffected() != 1 {
					if err != nil {
						return err
					}
					return errors.New("conversation_summary_projection_stale")
				}
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
