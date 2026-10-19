package core

import (
	"context"
	"time"
)

// A bounded source window is explicit: it never claims to be all history.
// Messages are remote communication claims, not proof of physical presence.
func readDiarySourcesWith(ctx context.Context, query lifeContextQuerier, fid string, start, end, asOf time.Time) (map[string]any, error) {
	sources := []any{}
	hasMore := false
	rows, err := query.Query(ctx, `SELECT m.id,m.conversation_id,m.author_actor_id,m.kind,m.text,m.created_at FROM public.conversation_messages m JOIN public.conversation_participants p ON p.conversation_id=m.conversation_id AND p.actor_id=$1 WHERE m.created_at>=p.joined_at AND (p.left_at IS NULL OR m.created_at<=p.left_at) AND m.created_at >=$2 AND m.created_at<$3 AND m.created_at<=$4 AND m.kind IN ('user','assistant') ORDER BY m.created_at DESC,m.id DESC LIMIT 61 FOR SHARE OF m`, fid, start, end, asOf)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id, conversation, author, kind, text string
		var at time.Time
		if err := rows.Scan(&id, &conversation, &author, &kind, &text, &at); err != nil {
			rows.Close()
			return nil, err
		}
		if len(sources) == 60 {
			hasMore = true
			break
		}
		sources = append(sources, map[string]any{"ref": "message:" + id, "kind": "remote_communication", "conversation_id": conversation, "author_actor_id": author, "message_kind": kind, "text": text, "at": at.UTC().Format(time.RFC3339Nano), "presence": "remote_not_copresent"})
	}
	readErr := rows.Err()
	rows.Close()
	if readErr != nil {
		return nil, readErr
	}
	rows, err = query.Query(ctx, `SELECT id,kind,start_at,end_at,scene,activity,location,revision,result FROM public.life_events WHERE fluctlight_id=$1 AND start_at<$3 AND end_at>$2 AND created_at<=$4 AND start_at<=$4 AND status='confirmed' ORDER BY start_at DESC,id DESC LIMIT 61 FOR SHARE`, fid, start, end, asOf)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var id, kind string
		var scene, activity, location *string
		var from, to time.Time
		var revision int
		var result []byte
		if err := rows.Scan(&id, &kind, &from, &to, &scene, &activity, &location, &revision, &result); err != nil {
			return nil, err
		}
		if count == 60 {
			hasMore = true
			break
		}
		count++
		sources = append(sources, map[string]any{"ref": "event:" + id, "kind": "confirmed_life_event", "event_kind": kind, "start_at": from.UTC().Format(time.RFC3339Nano), "end_at": to.UTC().Format(time.RFC3339Nano), "scene": scene, "activity": activity, "location": location, "revision": revision, "result": decodeObject(result), "presence": "participants_only_if_explicit_in_source"})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return map[string]any{"sources": sources, "window_start": start.UTC().Format(time.RFC3339Nano), "window_end": end.UTC().Format(time.RFC3339Nano), "as_of": asOf.UTC().Format(time.RFC3339Nano), "has_more_sources": hasMore, "message_limit": 60, "event_limit": 60}, nil
}
