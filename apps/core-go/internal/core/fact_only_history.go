package core

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// The bounded recent window can begin inside a long run of proactive replies.
// Follow the real stored source, never infer a missing user from assistant text.
func (a *App) backfillFactOnlyHistory(ctx context.Context, projection *ContextProjection) error {
	if len(projection.RecentMessages) == 0 || stringValue(projection.RecentMessages[0]["kind"]) != "assistant" || projection.ConversationID == "" {
		return nil
	}
	first := projection.RecentMessages[0]
	var id, author, text, turn string
	var sequence int64
	var created time.Time
	var senderZone *string
	var senderOffset *int
	var senderTime *time.Time
	err := a.DB.Pool().QueryRow(ctx, `SELECT u.id,u.author_actor_id,u.text,u.created_at,COALESCE(u.turn_id,''),u.sequence,u.sender_timezone,u.sender_utc_offset_minutes,u.sender_sent_at FROM public.conversation_messages a JOIN public.conversation_messages u ON u.conversation_id=a.conversation_id AND u.source_fact_id=a.source_fact_id AND u.kind='user' AND u.sequence<a.sequence WHERE a.id=$1 AND a.conversation_id=$2 ORDER BY u.sequence DESC LIMIT 1`, stringValue(first["id"]), projection.ConversationID).Scan(&id, &author, &text, &created, &turn, &sequence, &senderZone, &senderOffset, &senderTime)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	row := map[string]any{"id": id, "author_actor_id": author, "text": text, "kind": "user", "turn_id": turn, "sequence": sequence, "created_at": formatInstant(created), "history_context": "trigger_backfill_intermediate_messages_may_be_outside_window"}
	if senderZone != nil {
		row["sender_timezone"] = *senderZone
	}
	if senderOffset != nil {
		row["sender_utc_offset_minutes"] = *senderOffset
	}
	if senderTime != nil {
		row["sender_sent_at"] = formatInstant(*senderTime)
	}
	projection.RecentMessages = append([]map[string]any{row}, projection.RecentMessages...)
	return nil
}

// Real roles/text stay intact. Metadata is moved into a separate runtime data
// block by the assembler, after final whole-group selection.
func factOnlyHistory(projection ContextProjection, currentInput string) []PromptFragment {
	skip := currentInputRecentMessageIndex(projection.RecentMessages, currentInput)
	result := []PromptFragment{}
	group := ""
	previousRole := ""
	for index, row := range projection.RecentMessages {
		role := stringValue(row["kind"])
		if role != "user" && role != "assistant" {
			continue
		}
		text, _ := row["text"].(string)
		if strings.TrimSpace(text) == "" {
			continue
		}
		// A new real user run starts an interaction. Explicit turn IDs cannot
		// split consecutive unsolicited assistant replies from their trigger.
		if role == "user" && previousRole != "user" || group == "" {
			group = firstString(row["turn_id"], "interaction:"+stableDigest(jsonString(row)))
		}
		previousRole = role
		if index == skip {
			continue
		}
		actor := actorRefForID(projection.Actors, stringValue(row["author_actor_id"]))
		if len(actor) == 0 {
			if role == "assistant" {
				actor = projection.SelfActor
			} else {
				actor = projection.CurrentSpeaker
			}
		}
		metadata := map[string]any{"sender": factOnlyActor(actor), "source_kind": "conversation_message", "created_at": row["created_at"]}
		if context := row["history_context"]; context != nil {
			metadata["context_coverage"] = context
		}
		for _, key := range []string{"sender_sent_at", "sender_timezone", "sender_utc_offset_minutes"} {
			if raw, ok := row[key]; ok {
				metadata[key] = raw
			}
		}
		if stringValue(row["turn_id"]) == "" {
			metadata["lineage"] = "real_contiguous_interaction"
		}
		if role == "assistant" && len(result) == 0 {
			metadata["trigger_context"] = "not_available_in_loaded_history"
		}
		ref := "message:" + firstString(row["id"], stableDigest(jsonString(row)))
		result = append(result, PromptFragment{Kind: PromptFragmentRecentMessage, Priority: index, GroupKey: group,
			Content: map[string]any{"role": role, "content": text, "metadata": metadata}, SourceRefs: []string{ref}})
	}
	groups, last := 0, ""
	for index := len(result) - 1; index >= 0; index-- {
		if result[index].GroupKey != last {
			groups++
			last = result[index].GroupKey
		}
		result[index].Required = groups <= factOnlyReplyProtectedTurns
	}
	return result
}
