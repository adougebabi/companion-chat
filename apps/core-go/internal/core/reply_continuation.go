package core

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

type replyContinuationKey struct{}

func enqueueReplyContinuationTx(ctx context.Context, tx pgx.Tx, ref string) error {
	_, err := tx.Exec(ctx, `INSERT INTO public.conversation_reply_continuations(draft_id,workflow_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, ref, "reply_continuation_v1:"+stableDigest(ref))
	return err
}

func authorizeReplyContinuationTx(ctx context.Context, tx pgx.Tx, source, fluctlight, conversation string) error {
	ref, _ := ctx.Value(replyContinuationKey{}).(string)
	if ref == "" {
		return ErrUnauthorized
	}
	var authorized bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.conversation_reply_drafts d JOIN public.conversation_reply_continuations c ON c.draft_id=d.id WHERE d.id=$1 AND d.source_fact_id=$2 AND d.fluctlight_id=$3 AND d.conversation_id=$4 AND d.expires_at>now() AND d.status IN ('needs_action','waiting_media','ready') AND c.status='started')`, ref, source, fluctlight, conversation).Scan(&authorized)
	if err != nil {
		return err
	}
	if !authorized {
		return ErrUnauthorized
	}
	return nil
}

// This resumes the native loop with a restricted registry and the original
// source. It never accepts a new user message or applies cognition a second time.
func (a *App) ProcessReplyContinuation(ctx context.Context, ref string) (map[string]any, error) {
	var scopeBytes, configBytes, bindingBytes []byte
	var status, source, actor, fluctlight, conversation string
	var version int64
	err := a.DB.Pool().QueryRow(ctx, `SELECT d.execution_scope,d.config_snapshot,d.binding_snapshot,d.config_version,d.status,d.source_fact_id,d.authorization_actor_id,d.fluctlight_id,d.conversation_id FROM public.conversation_reply_drafts d JOIN public.conversation_reply_continuations c ON c.draft_id=d.id WHERE d.id=$1 AND c.status IN ('started','completed')`, ref).Scan(&scopeBytes, &configBytes, &bindingBytes, &version, &status, &source, &actor, &fluctlight, &conversation)
	if err != nil {
		return nil, err
	}
	if status == "published" {
		return map[string]any{"status": "completed", "draft_ref": ref}, a.CompleteReplyContinuation(ctx, ref)
	}
	var stored struct {
		Scope        ToolExecutionRequest `json:"scope"`
		CurrentInput string               `json:"current_input"`
	}
	if err := json.Unmarshal(scopeBytes, &stored); err != nil {
		return nil, err
	}
	var settings ConversationReplySettings
	if err := json.Unmarshal(configBytes, &settings); err != nil {
		return nil, err
	}
	var binding struct {
		Assignment providerAssignment `json:"assignment"`
	}
	if err := json.Unmarshal(bindingBytes, &binding); err != nil {
		return nil, err
	}
	// Credentials stay in their existing encrypted store. The endpoint/model and
	// generation parameters remain the snapshot accepted by the original run.
	if binding.Assignment.EndpointID != "" {
		var purpose string
		if err := a.DB.Pool().QueryRow(ctx, `SELECT secret_purpose FROM public.provider_endpoints WHERE id=$1`, binding.Assignment.EndpointID).Scan(&purpose); err != nil {
			return nil, err
		}
		binding.Assignment.Secret, err = a.Provider.secret(ctx, purpose)
		if err != nil {
			return nil, err
		}
	}
	ctx, release, leaseErr := a.enterLogicalRun(ctx, fluctlight, "reply_continuation")
	if leaseErr != nil {
		return nil, leaseErr
	}
	defer release()
	ctx = WithProviderCancellationKey(ctx, firstString(stringValue(decodeObject(scopeBytes)["cancellation_marker"]), source))
	run := &privateReplyRun{Settings: settings, ConfigVersion: version, Assignment: binding.Assignment, Scope: stored.Scope, CurrentInput: stored.CurrentInput, DraftRef: ref}
	ctx = context.WithValue(ctx, privateReplyRunKey{}, run)
	ctx = context.WithValue(ctx, replyContinuationKey{}, ref)
	if source != "" && stored.Scope.Surface != CapabilitySurfaceWakeUp {
		var sourceStatus string
		if err := a.DB.Pool().QueryRow(ctx, `SELECT status FROM public.cognition_inbox WHERE id=$1`, source).Scan(&sourceStatus); err != nil {
			return nil, err
		}
		if sourceStatus != "waiting_reply" {
			return nil, newCapabilityError("reply_source_closed", false, ErrConflict)
		}
	}
	states, err := a.readReplyActions(ctx, ref)
	if err != nil {
		return nil, err
	}
	if replyActionsPending(states) {
		return nil, newCapabilityError("reply_actions_pending", true, ErrConflict)
	}
	// Scope authorization is repeated by ExecuteTool and the publication owner.
	entry := factOnlyReplyCapability{service: newFactOnlyReplyService(a), run: run, publication: NewToolPublicationService(a)}
	registry, err := NewCapabilityRegistry(entry)
	if err != nil {
		return nil, err
	}
	ctx = withCapabilityRunRegistry(ctx, registry)
	refs := []string{}
	for _, state := range states {
		refs = append(refs, state.Ref)
	}
	input := stored.CurrentInput + "\n[已授权的同一回复续接] 原始认知已经结算。仅调用 conversation.reply，参数为 " + jsonString(map[string]any{"draft_ref": ref, "action_result_refs": refs}) + "。消费真实结果，完成最终合同，不执行其它行动、不新增用户消息。"
	result, err := a.RunConversationCognitionAgent(ctx, ConversationCognitionAgentInput{AuthorizationActorID: actor, SpeakerActorID: stored.Scope.SubjectActorID, FluctlightID: fluctlight, ConversationID: conversation, SourceFactID: source, RunID: "reply-resume:" + ref, CorrelationID: stored.Scope.CorrelationID, AuthorizationPolicy: stored.Scope.AuthorizationPolicy, Surface: stored.Scope.Surface, CurrentInput: input})
	// Final cognition failure after a committed Tool still leaves one truthful
	// message. Continuation retries must consume that stored result.
	outcome, outcomeErr := committedAgentOutcome(result.Trace)
	if outcomeErr != nil {
		return nil, outcomeErr
	}
	reply, committed := committedConversationReplyResult(outcome.Results)
	if !committed {
		if err != nil {
			return nil, err
		}
		return nil, errors.New("reply_continuation_output_missing")
	}
	message := stringValue(mapValue(reply.Output)["target_ref"])
	err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if source != "" {
			if _, err := tx.Exec(ctx, `UPDATE public.cognition_inbox SET status='processed',processed_at=now(),payload=jsonb_set(jsonb_set(payload,'{agent_result,message_id}',to_jsonb($2::text),true),'{agent_result,message_ids}',$3::jsonb,true) WHERE id=$1 AND status='waiting_reply'`, source, message, jsonBytes([]string{message})); err != nil {
				return err
			}
		}
		_, err := tx.Exec(ctx, `UPDATE public.conversation_reply_continuations SET status='completed',completed_at=now(),last_error=NULL WHERE draft_id=$1`, ref)
		return err
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"status": "completed", "draft_ref": ref, "message_id": message}, nil
}

// Bounded terminal cleanup also closes the original observer, while preserving
// already committed output. Never reset or regenerate successful components.
func (a *App) FailReplyContinuation(ctx context.Context, ref, code string) error {
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	return withTransaction(cleanup, a.DB.Pool(), func(tx pgx.Tx) error {
		var source, status string
		if err := tx.QueryRow(cleanup, `SELECT source_fact_id,status FROM public.conversation_reply_drafts WHERE id=$1 FOR UPDATE`, ref).Scan(&source, &status); err != nil {
			return err
		}
		if status == "published" {
			_, err := tx.Exec(cleanup, `UPDATE public.conversation_reply_continuations SET status='completed',completed_at=now() WHERE draft_id=$1`, ref)
			return err
		}
		if _, err := tx.Exec(cleanup, `UPDATE public.conversation_reply_drafts SET status='failed',last_error=$2,updated_at=now() WHERE id=$1`, ref, code); err != nil {
			return err
		}
		if _, err := tx.Exec(cleanup, `UPDATE public.cognition_inbox SET status='failed',error_code=$2,processed_at=now() WHERE id=$1 AND status='waiting_reply'`, source, code); err != nil {
			return err
		}
		_, err := tx.Exec(cleanup, `UPDATE public.conversation_reply_continuations SET status='failed',last_error=$2,completed_at=now() WHERE draft_id=$1`, ref, code)
		return err
	})
}

func (a *App) CompleteReplyContinuation(ctx context.Context, ref string) error {
	return withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error { return completeReplyContinuationTx(ctx, tx, ref) })
}
func completeReplyContinuationTx(ctx context.Context, tx pgx.Tx, ref string) error {
	var source, message string
	if err := tx.QueryRow(ctx, `SELECT source_fact_id,COALESCE(result_message_id,'') FROM public.conversation_reply_drafts WHERE id=$1 AND status='published' FOR UPDATE`, ref).Scan(&source, &message); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE public.cognition_inbox SET status='processed',processed_at=now(),payload=jsonb_set(jsonb_set(payload,'{agent_result,message_id}',to_jsonb($2::text),true),'{agent_result,message_ids}',$3::jsonb,true) WHERE id=$1 AND status='waiting_reply'`, source, message, jsonBytes([]string{message})); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `UPDATE public.conversation_reply_continuations SET status='completed',completed_at=now(),last_error=NULL WHERE draft_id=$1`, ref)
	return err
}

func replyContinuationSurface(value, fallback CapabilitySurface) CapabilitySurface {
	if value != "" {
		return value
	}
	return fallback
}

func authorizeWakeReplyContinuationTx(ctx context.Context, tx pgx.Tx, ref string) error {
	var source, conversation string
	var execution []byte
	if err := tx.QueryRow(ctx, `SELECT source_fact_id,conversation_id,execution_scope FROM public.conversation_reply_drafts WHERE id=$1 AND expires_at>now() FOR UPDATE`, ref).Scan(&source, &conversation, &execution); err != nil {
		return err
	}
	var wakeStatus string
	if err := tx.QueryRow(ctx, `SELECT status FROM public.cognition_wakeups WHERE id=$1`, source).Scan(&wakeStatus); err != nil {
		return err
	}
	if wakeStatus != "completed" && wakeStatus != "no_op" {
		return ErrUnauthorized
	}
	var currentSequence int
	if err := tx.QueryRow(ctx, `SELECT COALESCE(max(sequence),0) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='user'`, conversation).Scan(&currentSequence); err != nil {
		return err
	}
	if currentSequence != intValue(decodeObject(execution)["inbound_sequence"]) {
		return errCognitionTurnSuperseded
	}
	return nil
}
