package core

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

const directReplyRetryKey = "direct_reply_retry_media_intent"
const directReplyRetryProtocol = `主人明确请求补发原回合的文字回复。以生效人格回应原消息。图片任务已在原回合受理，reply_recovery 给出它现在的真实状态和是否已经送达。只用原生 conversation_reply 发布文字。不要再创建图片任务，也不要把受理、生成中或失败说成送达。decision 填 action_type="reply"、简短的 response_intent、influences=[]。`

type directReplyRetryContextKey struct{}

type directReplyRetryReceipt struct {
	intentID  string
	status    string
	delivered bool
}

func directReplyRetryFromContext(ctx context.Context) directReplyRetryReceipt {
	receipt, _ := ctx.Value(directReplyRetryContextKey{}).(directReplyRetryReceipt)
	return receipt
}

func directReplyRetryMediaID(payload map[string]any) string {
	return stringValue(mapValue(payload["agent_partial"])[directReplyRetryKey])
}

func directReplyRetryFailure(code, workflowStatus string) bool {
	switch workflowStatus {
	case "completed", "failed", "dead_letter", "cancelled":
	default:
		return false
	}
	switch code {
	case "agent_partial_committed", "agent_final_contract_invalid", "direct_reply_arguments_unresolved":
		return true
	}
	return false
}

// Reuse only an accepted standalone image from the original Owner's native
// turn. Every other committed mutation keeps the existing retry fence.
func directReplyRetryImage(ctx context.Context, q lifeContextQuerier, registry *CapabilityRegistry, source, owner, fluctlightID, conversationID string) (directReplyRetryReceipt, error) {
	rows, err := q.Query(ctx, `
		SELECT t.capability_name,t.authorization_actor_id,COALESCE(t.invocation->'metadata'->>'source',''),
		       COALESCE(t.invocation->'metadata'->>'subject_actor_id',''),COALESCE(t.result->>'status',''),
		       COALESCE(m.id,''),COALESCE(m.status,''),
		       EXISTS(SELECT 1 FROM public.conversation_messages cm
		              JOIN public.media_assets a ON a.id='asset_'||m.id AND a.owner_fluctlight_id=t.fluctlight_id AND a.status='ready' AND a.kind='image'
		              WHERE cm.conversation_id=$3 AND cm.author_actor_id=t.fluctlight_id AND cm.kind='media_reference' AND cm.attachment_refs ? a.id)
		FROM public.tool_executions t
		LEFT JOIN public.media_intents m ON m.id=t.result->'output'->>'media_intent_id'
		     AND m.owner_fluctlight_id=t.fluctlight_id AND m.conversation_id=$3
		     AND m.kind='image' AND m.message_id IS NULL AND m.moment_id IS NULL
		WHERE t.fluctlight_id=$1 AND t.invocation->>'source_fact_id'=$2
		  AND EXISTS(SELECT 1 FROM public.fluctlight_direct_conversations d WHERE d.conversation_id=$3 AND d.fluctlight_actor_id=$1 AND d.owner_actor_id=$4)
		  AND t.result->>'status' IN ('accepted','completed')`, fluctlightID, source, conversationID, owner)
	if err != nil {
		return directReplyRetryReceipt{}, err
	}
	defer rows.Close()
	var receipt directReplyRetryReceipt
	for rows.Next() {
		var name, authorized, origin, subject, status string
		var found directReplyRetryReceipt
		if err := rows.Scan(&name, &authorized, &origin, &subject, &status, &found.intentID, &found.status, &found.delivered); err != nil {
			return directReplyRetryReceipt{}, err
		}
		if definition, ok := registry.Definition(name); ok && definition.SideEffectClass == "read_only" {
			continue
		}
		if receipt.intentID != "" || name != "media.image.generate" || status != "accepted" || origin != "model_tool" ||
			authorized != owner || subject != owner || found.intentID == "" {
			return directReplyRetryReceipt{}, nil
		}
		receipt = found
	}
	return receipt, rows.Err()
}

// RetryDirectReply schedules a new attempt of the original turn's reply. It
// preserves the user message, media job, operation identities and receipts.
func (a *App) RetryDirectReply(ctx context.Context, actorID, conversationID, turnID string) error {
	return withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var source, fluctlightID, status, code, workflowStatus string
		var raw []byte
		var hasReply, newerUser bool
		err := tx.QueryRow(ctx, `
			SELECT i.id,i.fluctlight_id,i.status,COALESCE(i.error_code,''),COALESCE(w.status,''),i.payload,
			       EXISTS(SELECT 1 FROM public.conversation_messages r WHERE r.conversation_id=$2 AND r.turn_id=$3 AND r.kind='assistant'),
			       EXISTS(SELECT 1 FROM public.conversation_messages n WHERE n.conversation_id=$2 AND n.kind='user' AND n.sequence>m.sequence)
			FROM public.conversation_messages m
			JOIN public.cognition_inbox i ON i.id=m.source_fact_id AND i.event_type='conversation.turn'
			JOIN public.platform_workflow_intents w ON w.intent_id='cognition_intent:'||i.id
			JOIN public.fluctlight_direct_conversations d ON d.conversation_id=m.conversation_id
			     AND d.owner_actor_id=$1 AND d.fluctlight_actor_id=i.fluctlight_id
			JOIN public.conversation_participants p ON p.conversation_id=m.conversation_id AND p.actor_id=$1 AND p.status='active'
			WHERE m.conversation_id=$2 AND m.turn_id=$3 AND m.kind='user' AND m.author_actor_id=$1
			FOR UPDATE OF i,w`, actorID, conversationID, turnID).Scan(&source, &fluctlightID, &status, &code, &workflowStatus, &raw, &hasReply, &newerUser)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		payload := decodeObject(raw)
		if hasReply || directReplyRetryMediaID(payload) != "" && (status == "pending" || status == "claimed") {
			return nil
		}
		if status != "failed" || newerUser || !directReplyRetryFailure(code, workflowStatus) || len(mapValue(payload["agent_result"])) != 0 {
			return ErrConflict
		}
		config, _, err := a.conversationReplySettings(ctx)
		if err != nil {
			return err
		}
		if config.Mode != "direct" {
			return ErrConflict
		}
		receipt, err := directReplyRetryImage(ctx, tx, a.capabilityRegistry(), source, actorID, fluctlightID, conversationID)
		if err != nil {
			return err
		}
		if receipt.intentID == "" {
			return ErrConflict
		}
		retry := intValue(payload["retry_count"]) + 1
		budget, err := a.newConversationTurnBudget(ctx, retry)
		if err != nil {
			return err
		}
		if a.Redis != nil {
			if err := a.Redis.Del(ctx, providerCognitionCancelPrefix+source).Err(); err != nil {
				return err
			}
		}
		partial := mapValue(payload["agent_partial"])
		if partial == nil {
			partial = map[string]any{}
		}
		partial[directReplyRetryKey] = receipt.intentID
		payload["agent_partial"], payload["retry_count"], payload["reply_budget"] = partial, retry, budget
		if _, err := tx.Exec(ctx, `UPDATE public.cognition_inbox SET status='pending',error_code=NULL,processed_at=NULL,claimed_by=NULL,claimed_at=NULL,attempt_count=0,payload=$2::jsonb WHERE id=$1`, source, jsonBytes(payload)); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE public.platform_workflow_intents SET status='pending',workflow_id=$2,attempt_count=0,next_attempt_at=now(),started_at=NULL,completed_at=NULL,last_error=NULL WHERE intent_id=$1`, "cognition_intent:"+source, fmt.Sprintf("cognition:%s:retry:%d", source, retry))
		return err
	})
}
