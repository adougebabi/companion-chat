package core

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

func proactiveTopicWindowWith(ctx context.Context, q lifeContextQuerier) (time.Duration, error) {
	var raw string
	err := q.QueryRow(ctx, `SELECT value_json FROM public.runtime_settings WHERE key='product.autonomy'`).Scan(&raw)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return 0, err
	}
	seconds := intValue(decodeObject([]byte(raw))["topic_suppression_seconds"])
	if seconds == 0 {
		seconds = 12 * 60 * 60
	}
	if seconds < 300 || seconds > 7*24*60*60 {
		return 0, ErrInvalidArguments
	}
	return time.Duration(seconds) * time.Second, nil
}

func readCommunicationStateWith(ctx context.Context, q lifeContextQuerier, owner, conversation string) (map[string]any, error) {
	result := map[string]any{"recent_proactive": []map[string]any{}}
	if conversation == "" {
		return result, nil
	}
	var inbound, lastProactive int
	err := q.QueryRow(ctx, `SELECT COALESCE(max(sequence) FILTER (WHERE author_actor_id<>$2 AND kind='user'),0),COALESCE(max(sequence) FILTER (WHERE author_actor_id=$2 AND kind='assistant' AND EXISTS(SELECT 1 FROM public.conversation_proactive_deliveries d WHERE d.message_id=conversation_messages.id)),0) FROM public.conversation_messages WHERE conversation_id=$1`, conversation, owner).Scan(&inbound, &lastProactive)
	if err != nil {
		return nil, err
	}
	result["last_inbound_sequence"] = inbound
	result["awaiting_response"] = lastProactive > inbound
	rows, err := q.Query(ctx, `SELECT topic_key,purpose,occurred_at FROM public.conversation_proactive_deliveries WHERE fluctlight_id=$1 AND conversation_id=$2 ORDER BY occurred_at DESC,message_id DESC LIMIT 8`, owner, conversation)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var topic, purpose string
		var at time.Time
		if err := rows.Scan(&topic, &purpose, &at); err != nil {
			return nil, err
		}
		items = append(items, map[string]any{"topic_key": topic, "purpose": purpose, "sent_at": formatInstant(at)})
	}
	result["recent_proactive"] = items
	return result, rows.Err()
}
