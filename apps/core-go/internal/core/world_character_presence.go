package core

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// A confirmed current event or an actually started, still-effective activity
// establishes character presence. Schedule/routine estimates and a free-form
// inferred scene alone do not. Starting an activity proves no purchase/outcome.
func npcCharacterPresenceWith(ctx context.Context, q lifeContextQuerier, fid string, life map[string]any, at time.Time) (map[string]any, error) {
	if life["source"] != "event" || life["behavior_state"] == "sleep" || stringValue(life["place_id"]) == "" {
		return nil, nil
	}
	if life["authority_status"] == "confirmed" && life["event_kind"] != "life_activity" {
		return map[string]any{"kind": "confirmed_event"}, nil
	}
	if life["authority_status"] != "inferred" && !(life["authority_status"] == "confirmed" && life["event_kind"] == "life_activity") {
		return nil, nil
	}
	var id string
	var revision int
	var until time.Time
	err := q.QueryRow(ctx, `SELECT r.id,r.revision,r.active_until FROM public.fluctlight_life_activity_runs r WHERE r.fluctlight_id=$1 AND r.authority_event_id=$2 AND r.status IN ('in_progress','deferred') AND r.started_at<=$3 AND r.active_until>$3 AND (r.intention_id IS NULL OR EXISTS(SELECT 1 FROM public.fluctlight_intentions i WHERE i.id=r.intention_id AND i.fluctlight_id=r.fluctlight_id AND i.status IN ('qualified','due','in_progress'))) ORDER BY r.started_at DESC,r.id LIMIT 1`, fid, life["event_id"], at).Scan(&id, &revision, &until)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	valid, err := activityAuthorityActiveWith(ctx, q, id, fid, at)
	if err != nil {
		return nil, err
	}
	if !valid {
		return nil, nil
	}
	return map[string]any{"kind": "active_activity", "activity_id": id, "activity_revision": revision, "expires_at": until.UTC().Format(time.RFC3339Nano)}, nil
}
