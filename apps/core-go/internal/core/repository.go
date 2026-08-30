package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("resource not found")
var ErrUnauthorized = errors.New("resource is not authorized")

type Repository interface {
	Ping(context.Context) error
	ResolveSession(context.Context, string) (string, error)
	ListFluctlights(context.Context, string) ([]Fluctlight, error)
	GetFluctlight(context.Context, string, string) (Fluctlight, error)
	DirectConversationID(context.Context, string, string) (string, error)
	History(context.Context, string, string, *int, int) (ConversationPage, error)
}

type PostgresRepository struct{ pool *pgxpool.Pool }

func NewPostgresRepository(ctx context.Context, databaseURL string) (*PostgresRepository, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("create Core database pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping Core database: %w", err)
	}
	return &PostgresRepository{pool: pool}, nil
}

func (r *PostgresRepository) Close() { r.pool.Close() }

func (r *PostgresRepository) Ping(ctx context.Context) error { return r.pool.Ping(ctx) }

func (r *PostgresRepository) ResolveSession(ctx context.Context, token string) (string, error) {
	if strings.TrimSpace(token) == "" {
		return "", ErrUnauthorized
	}
	digest := sha256.Sum256([]byte(token))
	var actorID string
	err := r.pool.QueryRow(ctx, `
		SELECT human_actor_id
		FROM public.auth_sessions
		WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > now()
	`, hex.EncodeToString(digest[:])).Scan(&actorID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrUnauthorized
	}
	if err != nil {
		return "", fmt.Errorf("resolve Core session: %w", err)
	}
	return actorID, nil
}

func (r *PostgresRepository) ListFluctlights(ctx context.Context, ownerActorID string) ([]Fluctlight, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, identity, personality, behavioral_policy, life_profile, provenance, status, current_revision
		FROM public.fluctlights
		WHERE created_by_actor_id = $1 AND status <> 'retired'
		ORDER BY created_at
	`, ownerActorID)
	if err != nil {
		return nil, fmt.Errorf("list Core Fluctlights: %w", err)
	}
	defer rows.Close()
	result := make([]Fluctlight, 0)
	for rows.Next() {
		fluctlight, err := scanFluctlight(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, fluctlight)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate Core Fluctlights: %w", err)
	}
	return result, nil
}

func (r *PostgresRepository) GetFluctlight(ctx context.Context, id, ownerActorID string) (Fluctlight, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, identity, personality, behavioral_policy, life_profile, provenance, status, current_revision
		FROM public.fluctlights
		WHERE id = $1 AND created_by_actor_id = $2
	`, id, ownerActorID)
	fluctlight, err := scanFluctlight(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Fluctlight{}, ErrNotFound
	}
	return fluctlight, err
}

func (r *PostgresRepository) DirectConversationID(ctx context.Context, ownerActorID, fluctlightActorID string) (string, error) {
	var id string
	err := r.pool.QueryRow(ctx, `
		SELECT conversation_id
		FROM public.fluctlight_direct_conversations
		WHERE owner_actor_id = $1 AND fluctlight_actor_id = $2
	`, ownerActorID, fluctlightActorID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return id, err
}

func (r *PostgresRepository) History(ctx context.Context, conversationID, actorID string, before *int, limit int) (ConversationPage, error) {
	if limit < 1 {
		limit = 1
	}
	if limit > 200 {
		limit = 200
	}
	var conversation Conversation
	var title *string
	err := r.pool.QueryRow(ctx, `
		SELECT c.id, c.created_by_actor_id, c.title, c.revision, c.created_at, c.updated_at
		FROM public.conversations c
		JOIN public.conversation_participants p ON p.conversation_id = c.id
		WHERE c.id = $1 AND p.actor_id = $2 AND p.status = 'active'
	`, conversationID, actorID).Scan(&conversation.ID, &conversation.CreatedByActor, &title, &conversation.Revision, &conversation.CreatedAt, &conversation.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ConversationPage{}, ErrNotFound
	}
	if err != nil {
		return ConversationPage{}, fmt.Errorf("read Core conversation: %w", err)
	}
	conversation.Title = title
	participantRows, err := r.pool.Query(ctx, `
		SELECT conversation_id, actor_id, role, status, joined_at
		FROM public.conversation_participants WHERE conversation_id = $1 ORDER BY joined_at, actor_id
	`, conversationID)
	if err != nil {
		return ConversationPage{}, fmt.Errorf("read Core participants: %w", err)
	}
	participants := make([]Participant, 0)
	for participantRows.Next() {
		var item Participant
		if err := participantRows.Scan(&item.ConversationID, &item.ActorID, &item.Role, &item.Status, &item.JoinedAt); err != nil {
			participantRows.Close()
			return ConversationPage{}, fmt.Errorf("scan Core participant: %w", err)
		}
		participants = append(participants, item)
	}
	participantRows.Close()
	query := `SELECT id, conversation_id, sequence, author_actor_id, kind, text, attachment_refs, created_at FROM public.conversation_messages WHERE conversation_id = $1`
	args := []any{conversationID}
	if before != nil {
		query += " AND sequence < $2"
		args = append(args, *before)
	}
	query += fmt.Sprintf(" ORDER BY sequence DESC LIMIT %d", limit+1)
	messageRows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return ConversationPage{}, fmt.Errorf("read Core messages: %w", err)
	}
	defer messageRows.Close()
	messages := make([]Message, 0, limit)
	for messageRows.Next() {
		var item Message
		var attachmentJSON []byte
		if err := messageRows.Scan(&item.ID, &item.ConversationID, &item.Sequence, &item.AuthorActorID, &item.Kind, &item.Text, &attachmentJSON, &item.CreatedAt); err != nil {
			return ConversationPage{}, fmt.Errorf("scan Core message: %w", err)
		}
		if err := json.Unmarshal(attachmentJSON, &item.AttachmentRefs); err != nil {
			return ConversationPage{}, fmt.Errorf("decode Core attachments: %w", err)
		}
		messages = append(messages, item)
	}
	if err := messageRows.Err(); err != nil {
		return ConversationPage{}, fmt.Errorf("iterate Core messages: %w", err)
	}
	var nextBefore *int
	if len(messages) > limit {
		value := messages[limit-1].Sequence
		nextBefore = &value
		messages = messages[:limit]
	}
	for left, right := 0, len(messages)-1; left < right; left, right = left+1, right-1 {
		messages[left], messages[right] = messages[right], messages[left]
	}
	return ConversationPage{Conversation: conversation, Participants: participants, Messages: messages, NextBefore: nextBefore}, nil
}

type rowScanner interface{ Scan(...any) error }

func scanFluctlight(row rowScanner) (Fluctlight, error) {
	var result Fluctlight
	var identity, personality, policy, lifeProfile, provenance []byte
	if err := row.Scan(&result.ID, &identity, &personality, &policy, &lifeProfile, &provenance, &result.Status, &result.CurrentRevision); err != nil {
		return Fluctlight{}, err
	}
	for name, value := range map[string][]byte{"identity": identity, "personality": personality, "behavioral_policy": policy, "life_profile": lifeProfile, "provenance": provenance} {
		decoded := make(map[string]any)
		if err := json.Unmarshal(value, &decoded); err != nil {
			return Fluctlight{}, fmt.Errorf("decode Core %s: %w", name, err)
		}
		switch name {
		case "identity":
			result.Identity = decoded
		case "personality":
			result.Personality = decoded
		case "behavioral_policy":
			result.BehavioralPolicy = decoded
		case "life_profile":
			result.LifeProfile = decoded
		case "provenance":
			result.Provenance = decoded
		}
	}
	return result, nil
}
