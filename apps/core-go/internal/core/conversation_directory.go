package core

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

var ErrConversationInvalid = errors.New("conversation_request_invalid")

// Direct history is scoped by the actual two active participants, not by the
// mutable current-selection pointer. Old sessions remain addressable.
const directSessionScope = ` c.created_by_actor_id=$1
 AND EXISTS(SELECT 1 FROM public.conversation_participants p WHERE p.conversation_id=c.id AND p.actor_id=$1 AND p.status='active')
 AND EXISTS(SELECT 1 FROM public.conversation_participants p WHERE p.conversation_id=c.id AND p.actor_id=$2 AND p.status='active')
 AND (SELECT count(*) FROM public.conversation_participants p WHERE p.conversation_id=c.id AND p.status='active')=2 `

type ConversationDirectoryItem struct {
	Conversation          Conversation `json:"conversation"`
	UnreadCount           int          `json:"unread_count"`
	LastReadSequence      int          `json:"last_read_sequence"`
	LastDeliveredSequence int          `json:"last_delivered_sequence"`
	LastMessage           *Message     `json:"last_message,omitempty"`
}
type ConversationDirectoryPage struct {
	Items                 []ConversationDirectoryItem `json:"items"`
	CurrentConversationID string                      `json:"current_conversation_id"`
	NextCursor            string                      `json:"next_cursor"`
}
type ConversationSearchPage struct {
	Items      []Message `json:"items"`
	NextCursor string    `json:"next_cursor"`
}
type conversationCursor struct {
	Scope string    `json:"scope"`
	Time  time.Time `json:"time"`
	ID    string    `json:"id"`
}

func decodeConversationCursor(raw, scope string) (conversationCursor, error) {
	var c conversationCursor
	if raw == "" {
		return c, nil
	}
	if len(raw) > 2048 {
		return c, fmt.Errorf("%w: cursor", ErrConversationInvalid)
	}
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || json.Unmarshal(b, &c) != nil || c.Scope != scope || c.ID == "" || c.Time.IsZero() {
		return c, fmt.Errorf("%w: cursor", ErrConversationInvalid)
	}
	return c, nil
}
func encodeConversationCursor(scope string, at time.Time, id string) string {
	b, _ := json.Marshal(conversationCursor{Scope: scope, Time: at, ID: id})
	return base64.RawURLEncoding.EncodeToString(b)
}
func conversationLimit(limit int) int {
	if limit <= 0 {
		return 50
	}
	if limit > 200 {
		return 200
	}
	return limit
}

// NewDirectConversation creates only transport history and selects it atomically.
// The durable command receipt makes a retry return the same session, without
// re-selecting it after a later explicit selection.
func (a *App) NewDirectConversation(ctx context.Context, owner, fid, title, key string) (ConversationPage, error) {
	if strings.TrimSpace(key) == "" || len(key) > 256 || utf8.RuneCountInString(title) > 200 {
		return ConversationPage{}, fmt.Errorf("%w: command", ErrConversationInvalid)
	}
	if _, err := a.DB.GetFluctlight(ctx, fid, owner); err != nil {
		return ConversationPage{}, err
	}
	digest := stableDigest(jsonString([]any{"new_direct_conversation", fid, title}))
	var id string
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "conversation-command:"+owner+":"+key); err != nil {
			return err
		}
		receipts := postgresSurfaceTx{tx: tx}
		prior, body, found, err := receipts.Idempotency(ctx, owner, key)
		if err != nil {
			return err
		}
		if found {
			if prior != digest {
				return ErrConflict
			}
			id = stringValue(body["conversation_id"])
			return nil
		}
		// Serialize creation and selection for this Owner/Fluctlight pair.
		if _, err := tx.Exec(ctx, `SELECT id FROM public.fluctlights WHERE id=$1 AND created_by_actor_id=$2 FOR UPDATE`, fid, owner); err != nil {
			return err
		}
		id = randomID("conversation_")
		if _, err := tx.Exec(ctx, `INSERT INTO public.conversations(id,created_by_actor_id,title,revision) VALUES($1,$2,$3,0)`, id, owner, nullableString(title)); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.conversation_heads(conversation_id,next_sequence) VALUES($1,1)`, id); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.conversation_participants(conversation_id,actor_id,role,status) VALUES($1,$2,'owner','active'),($1,$3,'member','active')`, id, owner, fid); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.conversation_read_positions(conversation_id,actor_id) VALUES($1,$2),($1,$3)`, id, owner, fid); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3) ON CONFLICT(owner_actor_id,fluctlight_actor_id) DO UPDATE SET conversation_id=EXCLUDED.conversation_id`, owner, fid, id); err != nil {
			return err
		}
		return receipts.SaveIdempotency(ctx, owner, key, digest, map[string]any{"conversation_id": id})
	})
	if err != nil {
		return ConversationPage{}, err
	}
	return a.DB.History(ctx, id, owner, nil, 50)
}
func (a *App) SelectDirectConversation(ctx context.Context, owner, fid, id string) (ConversationPage, error) {
	if _, err := a.DB.GetFluctlight(ctx, fid, owner); err != nil {
		return ConversationPage{}, err
	}
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT id FROM public.fluctlights WHERE id=$1 AND created_by_actor_id=$2 FOR UPDATE`, fid, owner); err != nil {
			return err
		}
		var valid bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.conversations c WHERE `+directSessionScope+` AND c.id=$3)`, owner, fid, id).Scan(&valid); err != nil {
			return err
		}
		if !valid {
			return ErrNotFound
		}
		_, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3) ON CONFLICT(owner_actor_id,fluctlight_actor_id) DO UPDATE SET conversation_id=EXCLUDED.conversation_id`, owner, fid, id)
		return err
	})
	if err != nil {
		return ConversationPage{}, err
	}
	return a.DB.History(ctx, id, owner, nil, 50)
}
func (a *App) ConversationDirectory(ctx context.Context, owner, fid string, limit int, cursor string) (ConversationDirectoryPage, error) {
	result := ConversationDirectoryPage{Items: []ConversationDirectoryItem{}}
	if _, err := a.DB.GetFluctlight(ctx, fid, owner); err != nil {
		return result, err
	}
	scope := stableDigest(jsonString([]any{"conversation_directory", owner, fid}))
	c, err := decodeConversationCursor(cursor, scope)
	if err != nil {
		return result, err
	}
	limit = conversationLimit(limit)
	current, err := a.DB.DirectConversationID(ctx, owner, fid)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return result, err
	}
	result.CurrentConversationID = current
	rows, err := a.DB.Pool().Query(ctx, `SELECT c.id,c.created_by_actor_id,c.title,c.revision,c.created_at,c.updated_at,
 COALESCE(r.last_read_sequence,0),COALESCE(r.last_delivered_sequence,0),
 (SELECT count(*) FROM public.conversation_messages m WHERE m.conversation_id=c.id AND m.author_actor_id<>$1 AND m.sequence>COALESCE(r.last_read_sequence,0))
 FROM public.conversations c LEFT JOIN public.conversation_read_positions r ON r.conversation_id=c.id AND r.actor_id=$1
 WHERE `+directSessionScope+` AND ($3::timestamptz IS NULL OR (c.created_at,c.id)<($3,$4)) ORDER BY c.created_at DESC,c.id DESC LIMIT $5`, owner, fid, nullableCursorTime(c), c.ID, limit+1)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var item ConversationDirectoryItem
		if err := rows.Scan(&item.Conversation.ID, &item.Conversation.CreatedByActor, &item.Conversation.Title, &item.Conversation.Revision, &item.Conversation.CreatedAt, &item.Conversation.UpdatedAt, &item.LastReadSequence, &item.LastDeliveredSequence, &item.UnreadCount); err != nil {
			rows.Close()
			return result, err
		}
		result.Items = append(result.Items, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, err
	}
	if len(result.Items) > limit {
		last := result.Items[limit-1].Conversation
		result.NextCursor = encodeConversationCursor(scope, last.CreatedAt, last.ID)
		result.Items = result.Items[:limit]
	}
	for i := range result.Items {
		page, err := a.DB.History(ctx, result.Items[i].Conversation.ID, owner, nil, 1)
		if err != nil {
			return result, err
		}
		if len(page.Messages) > 0 {
			result.Items[i].LastMessage = &page.Messages[0]
		}
	}
	return result, nil
}
func nullableCursorTime(c conversationCursor) any {
	if c.Time.IsZero() {
		return nil
	}
	return c.Time
}

// Search uses literal text over all persisted sessions. The tuple cursor is
// scoped to the owner, persona and query; it is independent of loaded UI pages.
func (a *App) SearchConversationMessages(ctx context.Context, owner, fid, query string, limit int, cursor string) (ConversationSearchPage, error) {
	result := ConversationSearchPage{Items: []Message{}}
	if _, err := a.DB.GetFluctlight(ctx, fid, owner); err != nil {
		return result, err
	}
	if strings.TrimSpace(query) == "" || utf8.RuneCountInString(query) > 2000 {
		return result, fmt.Errorf("%w: search", ErrConversationInvalid)
	}
	scope := stableDigest(jsonString([]any{"conversation_search", owner, fid, query}))
	c, err := decodeConversationCursor(cursor, scope)
	if err != nil {
		return result, err
	}
	limit = conversationLimit(limit)
	rows, err := a.DB.Pool().Query(ctx, `SELECT m.id,m.conversation_id,m.sequence,m.created_at FROM public.conversation_messages m JOIN public.conversations c ON c.id=m.conversation_id WHERE `+directSessionScope+` AND strpos(lower(m.text),lower($3))>0 AND ($4::timestamptz IS NULL OR (m.created_at,m.id)<($4,$5)) ORDER BY m.created_at DESC,m.id DESC LIMIT $6`, owner, fid, query, nullableCursorTime(c), c.ID, limit+1)
	if err != nil {
		return result, err
	}
	type hit struct {
		id, conversation string
		sequence         int
		at               time.Time
	}
	hits := []hit{}
	for rows.Next() {
		var h hit
		if err := rows.Scan(&h.id, &h.conversation, &h.sequence, &h.at); err != nil {
			rows.Close()
			return result, err
		}
		hits = append(hits, h)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, err
	}
	if len(hits) > limit {
		h := hits[limit-1]
		result.NextCursor = encodeConversationCursor(scope, h.at, h.id)
		hits = hits[:limit]
	}
	for _, h := range hits {
		message, err := a.ConversationMessage(ctx, owner, h.conversation, h.id)
		if err != nil {
			return result, err
		}
		result.Items = append(result.Items, message)
	}
	return result, nil
}
func (a *App) ConversationMessage(ctx context.Context, owner, conversation, id string) (Message, error) {
	var sequence int
	err := a.DB.Pool().QueryRow(ctx, `SELECT m.sequence FROM public.conversation_messages m JOIN public.conversation_participants p ON p.conversation_id=m.conversation_id AND p.actor_id=$1 AND p.status='active' WHERE m.conversation_id=$2 AND m.id=$3`, owner, conversation, id).Scan(&sequence)
	if errors.Is(err, pgx.ErrNoRows) {
		return Message{}, ErrNotFound
	}
	if err != nil {
		return Message{}, err
	}
	before := sequence + 1
	page, err := a.DB.History(ctx, conversation, owner, &before, 1)
	if err != nil {
		return Message{}, err
	}
	if len(page.Messages) != 1 || page.Messages[0].ID != id {
		return Message{}, fmt.Errorf("%w: message", ErrNotFound)
	}
	return page.Messages[0], nil
}
