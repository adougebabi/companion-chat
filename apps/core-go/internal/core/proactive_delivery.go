package core

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// proactiveMessageDuplicateWindow bounds exact-text suppression to the recent
// conversation window. It prevents a failed/replayed wake-up cycle from
// filling the chat with the same sentence while still allowing a genuinely
// new message after a long enough interval.
const proactiveMessageDuplicateWindow = 12 * time.Hour

// recentExactAssistantMessageTx checks for a recent assistant message with
// exactly the same text. It locks the conversation head before reading so two
// autonomous deliveries cannot both observe a missing duplicate and then
// append the same text concurrently. All assistant message writers serialize
// through this head row.
func recentExactAssistantMessageTx(ctx context.Context, tx pgx.Tx, conversationID, actorID, text string, window time.Duration) (string, bool, error) {
	if conversationID == "" || actorID == "" || text == "" {
		return "", false, nil
	}
	if window <= 0 {
		window = proactiveMessageDuplicateWindow
	}
	var nextSequence int
	if err := tx.QueryRow(ctx, `SELECT next_sequence FROM public.conversation_heads WHERE conversation_id=$1 FOR UPDATE`, conversationID).Scan(&nextSequence); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", false, nil
		}
		return "", false, err
	}
	var messageID string
	err := tx.QueryRow(ctx, `
		SELECT id
		FROM public.conversation_messages
		WHERE conversation_id=$1
		  AND author_actor_id=$2
		  AND kind='assistant'
		  AND text=$3
		  AND created_at >= now() - ($4::double precision * interval '1 second')
		ORDER BY created_at DESC, id DESC
		LIMIT 1
	`, conversationID, actorID, text, window.Seconds()).Scan(&messageID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return messageID, true, nil
}
