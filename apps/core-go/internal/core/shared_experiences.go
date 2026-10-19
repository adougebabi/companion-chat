package core

import (
	"context"
	"net/url"
	"time"
)

// Shared communication is proved by the actual participants and messages.
// Location names, memories, diary prose and NPC encounters cannot place the
// human in the character's physical world.
func (a *App) OwnerSharedExperiences(ctx context.Context, owner, fid string, q url.Values) (map[string]any, error) {
	if _, err := a.DB.GetFluctlight(ctx, fid, owner); err != nil {
		return nil, err
	}
	limit, err := reflectionReadPage(q)
	if err != nil {
		return nil, err
	}
	scope := stableDigest(jsonString([]any{"shared-communication", owner, fid}))
	cursor, err := decodeOwnerReadCursor(q.Get("cursor"), scope)
	if err != nil || (q.Get("cursor") != "" && (cursor.ID == "" || cursor.Time.IsZero())) {
		return nil, invalid("invalid shared experience cursor")
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT m.id,m.conversation_id,m.author_actor_id,m.kind,m.text,m.created_at FROM public.conversation_messages m JOIN public.conversation_participants f ON f.conversation_id=m.conversation_id AND f.actor_id=$1 JOIN public.conversation_participants h ON h.conversation_id=m.conversation_id AND h.actor_id=$2 WHERE m.author_actor_id IN ($1,$2) AND m.kind IN ('user','assistant') AND m.created_at>=GREATEST(f.joined_at,h.joined_at) AND (f.left_at IS NULL OR m.created_at<=f.left_at) AND (h.left_at IS NULL OR m.created_at<=h.left_at) AND ($3='' OR (m.created_at,m.id)<($4,$3)) ORDER BY m.created_at DESC,m.id DESC LIMIT $5`, fid, owner, cursor.ID, cursor.Time, limit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []any{}
	next := ""
	last := ownerReadCursor{Scope: scope}
	for rows.Next() {
		var id, conversation, author, kind, text string
		var at time.Time
		if err := rows.Scan(&id, &conversation, &author, &kind, &text, &at); err != nil {
			return nil, err
		}
		if len(items) == limit {
			next = encodeOwnerReadCursor(last)
			break
		}
		items = append(items, map[string]any{"id": id, "kind": "remote_communication", "title": text, "author_actor_id": author, "message_kind": kind, "occurred_at": at.UTC().Format(time.RFC3339Nano), "source_ref": "message:" + id, "conversation_id": conversation, "copresent": false, "href": "#/c/" + url.PathEscape(fid) + "/chat?" + url.Values{"conversation": {conversation}, "message": {id}}.Encode()})
		last = ownerReadCursor{Scope: scope, ID: id, Time: at}
	}
	return map[string]any{"items": items, "next_cursor": next, "physical_presence_source": "requires_explicit_confirmed_joint_participation"}, rows.Err()
}
