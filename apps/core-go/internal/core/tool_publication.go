package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// ToolPublicationService owns the short PostgreSQL transaction work needed to
// publish user-visible Tool results. It deliberately contains no model,
// workflow, Redis, object-storage, or media-provider calls.
//
// Existing turn/autonomy publishers can migrate to these methods without
// changing the authoritative message, Moment, media-intent, workflow-intent,
// or outbox tables.
type ToolPublicationService struct {
	app *App
}

func NewToolPublicationService(app *App) *ToolPublicationService {
	return &ToolPublicationService{app: app}
}

type ConversationReplyPublication struct {
	SuppressRecentDuplicate bool
	AuthorizationActorID    string
	FluctlightID            string
	ConversationID          string
	OperationID             string
	CorrelationID           string
	Text                    string
}

type MomentPublication struct {
	FluctlightID  string
	OperationID   string
	CorrelationID string
	Text          string
}

type MediaPublicationTarget struct {
	AuthorizationActorID string
	FluctlightID         string
	ConversationID       string
	Kind                 string
	Ref                  string
}

type publishedResource struct {
	DuplicateSuppressed bool
	ID                  string
	Replayed            bool
}

func (service *ToolPublicationService) PublishConversationReplyTx(ctx context.Context, tx pgx.Tx, command ConversationReplyPublication) (publishedResource, error) {
	if service == nil || service.app == nil || tx == nil {
		return publishedResource{}, errors.New("conversation publication unavailable")
	}
	command.AuthorizationActorID = strings.TrimSpace(command.AuthorizationActorID)
	command.FluctlightID = strings.TrimSpace(command.FluctlightID)
	command.ConversationID = strings.TrimSpace(command.ConversationID)
	command.OperationID = strings.TrimSpace(command.OperationID)
	command.Text = strings.TrimSpace(command.Text)
	if command.AuthorizationActorID == "" || command.FluctlightID == "" || command.ConversationID == "" || command.OperationID == "" {
		return publishedResource{}, fmt.Errorf("%w: conversation publication scope is required", ErrInvalidArguments)
	}
	if command.Text == "" || len([]rune(command.Text)) > 32000 {
		return publishedResource{}, fmt.Errorf("%w: reply text is invalid", ErrInvalidArguments)
	}
	if err := requireConversationPublicationOwnershipTx(ctx, tx, command.AuthorizationActorID, command.FluctlightID, command.ConversationID); err != nil {
		return publishedResource{}, err
	}
	identity := stableDigest(strings.Join([]string{command.FluctlightID, "conversation.reply", command.OperationID}, "\x1f"))
	idempotency := "tool:conversation.reply:" + identity
	messageID := "message_" + identity
	correlationID := strings.TrimSpace(command.CorrelationID)
	if correlationID == "" || len([]rune(correlationID)) > 128 {
		correlationID = "tool-operation:" + stableDigest(command.OperationID)
	}
	correlationID = canonicalConversationPublicationCorrelation(correlationID)
	turnID, sourceFactID, err := resolveConversationPublicationSourceTx(ctx, tx, command.FluctlightID, command.ConversationID, correlationID)
	if err != nil {
		return publishedResource{}, err
	}
	var existingConversation, existingAuthor, existingText, existingIdempotency, existingTurnID, existingSourceFactID, existingCorrelationID string
	err = tx.QueryRow(ctx, `SELECT conversation_id,author_actor_id,text,idempotency_key,COALESCE(turn_id,''),COALESCE(source_fact_id,''),COALESCE(correlation_id,'') FROM public.conversation_messages WHERE id=$1`, messageID).Scan(&existingConversation, &existingAuthor, &existingText, &existingIdempotency, &existingTurnID, &existingSourceFactID, &existingCorrelationID)
	if err == nil {
		if existingConversation != command.ConversationID || existingAuthor != command.FluctlightID || existingText != command.Text || existingIdempotency != idempotency || existingTurnID != turnID || existingSourceFactID != sourceFactID || existingCorrelationID != correlationID {
			return publishedResource{}, fmt.Errorf("%w: conversation reply operation payload changed", ErrConflict)
		}
		return publishedResource{ID: messageID, Replayed: true}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return publishedResource{}, err
	}
	var conflictingID string
	err = tx.QueryRow(ctx, `SELECT id FROM public.conversation_messages WHERE conversation_id=$1 AND idempotency_key=$2`, command.ConversationID, idempotency).Scan(&conflictingID)
	if err == nil {
		return publishedResource{}, fmt.Errorf("%w: conversation reply operation identity changed", ErrConflict)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return publishedResource{}, err
	}
	var sequence int
	if err := tx.QueryRow(ctx, `SELECT next_sequence FROM public.conversation_heads WHERE conversation_id=$1 FOR UPDATE`, command.ConversationID).Scan(&sequence); err != nil {
		return publishedResource{}, err
	}
	if command.SuppressRecentDuplicate {
		id, duplicate, err := recentExactAssistantMessageTx(ctx, tx, command.ConversationID, command.FluctlightID, command.Text, proactiveMessageDuplicateWindow)
		if err != nil {
			return publishedResource{}, err
		}
		if duplicate {
			return publishedResource{ID: id, DuplicateSuppressed: true}, nil
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE public.conversation_heads SET next_sequence=$2 WHERE conversation_id=$1`, command.ConversationID, sequence+1); err != nil {
		return publishedResource{}, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.conversation_messages(id,conversation_id,sequence,author_actor_id,kind,text,attachment_refs,idempotency_key,turn_id,source_fact_id,correlation_id) VALUES($1,$2,$3,$4,'assistant',$5,'[]',$6,$7,$8,$9)`, messageID, command.ConversationID, sequence, command.FluctlightID, command.Text, idempotency, nullableString(turnID), nullableString(sourceFactID), correlationID); err != nil {
		return publishedResource{}, err
	}
	if err := service.app.enqueueConversationSummaryIntentTx(ctx, tx, command.FluctlightID, command.ConversationID, messageID); err != nil {
		return publishedResource{}, err
	}
	return publishedResource{ID: messageID}, nil
}

func canonicalConversationPublicationCorrelation(correlationID string) string {
	correlationID = strings.TrimSpace(correlationID)
	const agentPrefix = "conversation-cognition-agent:"
	if strings.HasPrefix(correlationID, agentPrefix) {
		candidate := strings.TrimSpace(strings.TrimPrefix(correlationID, agentPrefix))
		if strings.HasPrefix(candidate, "turn:") {
			return candidate
		}
	}
	return correlationID
}

func resolveConversationPublicationSourceTx(ctx context.Context, tx pgx.Tx, fluctlightID, conversationID, correlationID string) (string, string, error) {
	if !strings.HasPrefix(correlationID, "turn:") {
		return "", "", nil
	}
	turnID := strings.TrimSpace(strings.TrimPrefix(correlationID, "turn:"))
	if turnID == "" {
		return "", "", fmt.Errorf("%w: conversation turn correlation is invalid", ErrInvalidArguments)
	}
	var sourceFactID string
	err := tx.QueryRow(ctx, `SELECT id FROM public.cognition_inbox WHERE fluctlight_id=$1 AND event_type='conversation.turn' AND payload->>'conversation_id'=$2 AND payload->>'turn_id'=$3`, fluctlightID, conversationID, turnID).Scan(&sourceFactID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", fmt.Errorf("%w: conversation reply source fact does not exist", ErrConflict)
	}
	if err != nil {
		return "", "", err
	}
	return turnID, sourceFactID, nil
}

func (service *ToolPublicationService) PublishMomentTx(ctx context.Context, tx pgx.Tx, command MomentPublication) (publishedResource, error) {
	if service == nil || service.app == nil || tx == nil {
		return publishedResource{}, errors.New("moment publication unavailable")
	}
	command.FluctlightID = strings.TrimSpace(command.FluctlightID)
	command.OperationID = strings.TrimSpace(command.OperationID)
	command.Text = strings.TrimSpace(command.Text)
	if command.FluctlightID == "" || command.OperationID == "" {
		return publishedResource{}, fmt.Errorf("%w: Moment publication scope is required", ErrInvalidArguments)
	}
	if command.Text == "" || len([]rune(command.Text)) > 32000 {
		return publishedResource{}, fmt.Errorf("%w: Moment text is invalid", ErrInvalidArguments)
	}
	momentID := "moment_" + stableDigest(strings.Join([]string{command.FluctlightID, "moment.publish", command.OperationID}, "\x1f"))
	var existingOwner, existingAuthor, existingText, existingVisibility, existingStatus string
	err := tx.QueryRow(ctx, `SELECT owner_fluctlight_id,author_actor_id,text,visibility,status FROM public.moments WHERE id=$1`, momentID).Scan(&existingOwner, &existingAuthor, &existingText, &existingVisibility, &existingStatus)
	replayed := err == nil
	if err == nil {
		if existingOwner != command.FluctlightID || existingAuthor != command.FluctlightID || existingText != command.Text || existingVisibility != "participants" || existingStatus != "visible" {
			return publishedResource{}, fmt.Errorf("%w: Moment operation payload changed", ErrConflict)
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return publishedResource{}, err
	} else if _, err := tx.Exec(ctx, `INSERT INTO public.moments(id,owner_fluctlight_id,author_actor_id,text,visibility,status,media_asset_ids) VALUES($1,$2,$2,$3,'participants','visible','[]')`, momentID, command.FluctlightID, command.Text); err != nil {
		return publishedResource{}, err
	}
	correlationID := strings.TrimSpace(command.CorrelationID)
	if correlationID == "" {
		correlationID = "tool-operation:" + command.OperationID
	}
	if err := appendOutboxTx(ctx, tx, "moment.published", "moment", momentID, command.FluctlightID, command.OperationID, correlationID, "moment-outbox:tool:"+stableDigest(command.FluctlightID+"\x1f"+command.OperationID), map[string]any{
		"moment_id": momentID, "operation_id": command.OperationID, "aggregate_sequence": 1,
	}); err != nil {
		return publishedResource{}, err
	}
	return publishedResource{ID: momentID, Replayed: replayed}, nil
}

// ValidateMediaTargetTx resolves an explicit Tool target to the existing media
// intent columns and proves that the target belongs to the authorized
// Fluctlight. It never creates a compatibility message or Moment.
func (service *ToolPublicationService) ValidateMediaTargetTx(ctx context.Context, tx pgx.Tx, target MediaPublicationTarget) (conversationID, messageID, momentID string, err error) {
	target.AuthorizationActorID = strings.TrimSpace(target.AuthorizationActorID)
	target.FluctlightID = strings.TrimSpace(target.FluctlightID)
	target.ConversationID = strings.TrimSpace(target.ConversationID)
	target.Kind = strings.TrimSpace(target.Kind)
	target.Ref = strings.TrimSpace(target.Ref)
	if service == nil || service.app == nil || tx == nil || target.AuthorizationActorID == "" || target.FluctlightID == "" || target.Kind == "" || target.Ref == "" {
		return "", "", "", fmt.Errorf("%w: media target scope is required", ErrInvalidArguments)
	}
	switch target.Kind {
	case "conversation":
		if target.ConversationID == "" || target.Ref != target.ConversationID {
			return "", "", "", fmt.Errorf("%w: media conversation target does not match the request scope", ErrInvalidArguments)
		}
		if err := requireConversationPublicationOwnershipTx(ctx, tx, target.AuthorizationActorID, target.FluctlightID, target.ConversationID); err != nil {
			return "", "", "", err
		}
		return target.ConversationID, "", "", nil
	case "conversation_message":
		if target.ConversationID == "" {
			return "", "", "", fmt.Errorf("%w: media message target requires conversation scope", ErrInvalidArguments)
		}
		if err := requireConversationPublicationOwnershipTx(ctx, tx, target.AuthorizationActorID, target.FluctlightID, target.ConversationID); err != nil {
			return "", "", "", err
		}
		var ownerConversation string
		if err := tx.QueryRow(ctx, `SELECT conversation_id FROM public.conversation_messages WHERE id=$1`, target.Ref).Scan(&ownerConversation); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return "", "", "", ErrNotFound
			}
			return "", "", "", err
		}
		if ownerConversation != target.ConversationID {
			return "", "", "", ErrUnauthorized
		}
		return "", target.Ref, "", nil
	case "moment":
		var owner string
		if err := tx.QueryRow(ctx, `SELECT owner_fluctlight_id FROM public.moments WHERE id=$1`, target.Ref).Scan(&owner); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return "", "", "", ErrNotFound
			}
			return "", "", "", err
		}
		if owner != target.FluctlightID {
			return "", "", "", ErrUnauthorized
		}
		return "", "", target.Ref, nil
	default:
		return "", "", "", fmt.Errorf("%w: unsupported media target kind %q", ErrInvalidArguments, target.Kind)
	}
}

// CreateMediaIntentTx delegates creation to the established media-intent
// authority after binding the stable operation identity to the full payload.
func (service *ToolPublicationService) CreateMediaIntentTx(ctx context.Context, tx pgx.Tx, media imageCapabilityService, fluctlightID string, concept map[string]any, intentID, workflowID, requestID, conversationID, messageID, momentID string) (replayed bool, status string, err error) {
	if service == nil || media == nil || tx == nil {
		return false, "", errors.New("media publication unavailable")
	}
	var existingOwner, existingPrompt, existingRequestID, existingWorkflowID, existingConversationID, existingMessageID, existingMomentID, existingStatus string
	err = tx.QueryRow(ctx, `SELECT owner_fluctlight_id,prompt,provider_request_id,workflow_id,COALESCE(conversation_id,''),COALESCE(message_id,''),COALESCE(moment_id,''),status FROM public.media_intents WHERE id=$1`, intentID).Scan(&existingOwner, &existingPrompt, &existingRequestID, &existingWorkflowID, &existingConversationID, &existingMessageID, &existingMomentID, &existingStatus)
	if err == nil {
		var existingConcept map[string]any
		if json.Unmarshal([]byte(existingPrompt), &existingConcept) != nil || strings.TrimSpace(stringValue(existingConcept["intent"])) != strings.TrimSpace(stringValue(concept["intent"])) || existingOwner != fluctlightID || existingRequestID != requestID || existingWorkflowID != workflowID || existingConversationID != conversationID || existingMessageID != messageID || existingMomentID != momentID {
			return false, "", fmt.Errorf("%w: media operation payload changed", ErrConflict)
		}
		return true, existingStatus, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return false, "", err
	}
	if err := media.createMediaIntentTargetTx(ctx, tx, fluctlightID, concept, intentID, workflowID, requestID, conversationID, messageID, momentID); err != nil {
		return false, "", err
	}
	return false, "pending", nil
}

func requireConversationPublicationOwnershipTx(ctx context.Context, tx pgx.Tx, authorizationActorID, fluctlightID, conversationID string) error {
	var participantCount int
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM public.conversation_participants WHERE conversation_id=$1 AND actor_id IN ($2,$3) AND status='active'`, conversationID, authorizationActorID, fluctlightID).Scan(&participantCount); err != nil {
		return err
	}
	if participantCount != 2 {
		return ErrUnauthorized
	}
	return nil
}
