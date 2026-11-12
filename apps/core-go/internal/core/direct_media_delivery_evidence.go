package core

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

// directMediaDelivery is an invocation-local projection of a committed media
// delivery. Storage identities remain in Core; Ref is only valid in the
// authorization/conversation scope from which this row was read.
type directMediaDelivery struct {
	Ref       string
	Intent    string
	Delivered time.Time
	intentID  string
	assetID   string
	messageID string
	refID     string
}

// Both legacy and draft-bound paths require an actual message attachment and
// its matching reference. A queued intent, ready asset or failed status alone
// is never proof of delivery. An already committed message remains evidence
// even if a later attempt failed, provided its actual attachment still exists.
func directMediaDeliveries(ctx context.Context, q lifeContextQuerier, owner, subject, fluctlight, conversation string, lock bool) ([]directMediaDelivery, error) {
	return directMediaDeliveriesForIntents(ctx, q, owner, subject, fluctlight, conversation, lock, nil)
}

func directMediaDeliveriesForIntents(ctx context.Context, q lifeContextQuerier, owner, subject, fluctlight, conversation string, lock bool, intentIDs []string) ([]directMediaDelivery, error) {
	if owner == "" || subject == "" || fluctlight == "" || conversation == "" {
		return nil, ErrUnauthorized
	}
	if lock {
		if err := lockDirectMediaDeliveryScope(ctx, q, owner, subject, fluctlight, conversation); err != nil {
			return nil, err
		}
	}
	const authorization = `
		EXISTS (SELECT 1 FROM public.fluctlights f WHERE f.id=$3 AND f.created_by_actor_id=$1)
		AND EXISTS (SELECT 1 FROM public.conversation_participants p WHERE p.conversation_id=$4 AND p.actor_id=$1 AND p.status='active')
		AND EXISTS (SELECT 1 FROM public.conversation_participants p WHERE p.conversation_id=$4 AND p.actor_id=$2 AND p.status='active')
		AND EXISTS (SELECT 1 FROM public.conversation_participants p WHERE p.conversation_id=$4 AND p.actor_id=$3 AND p.status='active')`
	// An attachment may be added to an existing legacy message much later than
	// the message was created. That path has no authoritative attachment time.
	const legacy = `SELECT t.id,a.id,m.id,r.id,t.prompt,CASE WHEN COALESCE(t.message_id,'')<>'' THEN NULL ELSE m.created_at END,t.revision,a.revision,a.sha256
		FROM public.media_intents t
		JOIN public.media_assets a ON a.id='asset_'||t.id AND a.owner_fluctlight_id=t.owner_fluctlight_id AND a.status='ready' AND a.kind='image'
		JOIN public.media_references r ON r.asset_id=a.id AND r.owner_fluctlight_id=t.owner_fluctlight_id
		JOIN public.conversation_messages m ON m.conversation_id=$4 AND m.author_actor_id=$3 AND m.kind IN ('assistant','media_reference') AND m.attachment_refs @> jsonb_build_array(a.id)
		WHERE t.owner_fluctlight_id=$3 AND t.kind='image' AND COALESCE(t.owner_request_id,'')='' AND COALESCE(t.studio_job_id,'')=''
		AND ($5::text[] IS NULL OR t.id=ANY($5::text[]))
		AND NOT EXISTS (SELECT 1 FROM public.conversation_reply_actions x WHERE x.media_intent_id=t.id)
		AND ((COALESCE(t.message_id,'')=m.id AND r.target_type='conversation_message' AND r.target_id=m.id)
		 OR (COALESCE(t.message_id,'')='' AND t.conversation_id=$4 AND r.target_type='conversation' AND r.target_id=$4
		     AND m.idempotency_key='media:'||t.id||':conversation-result' AND m.author_actor_id=$3 AND m.kind='media_reference'))
		AND ` + authorization
	const draft = `SELECT t.id,a.id,m.id,r.id,t.prompt,m.created_at,t.revision,a.revision,a.sha256
		FROM public.media_intents t
		JOIN public.conversation_reply_actions x ON x.media_intent_id=t.id AND x.kind='image' AND x.status='bound'
		JOIN public.conversation_reply_drafts d ON d.id=x.draft_id AND d.fluctlight_id=$3 AND d.conversation_id=$4
		 AND d.authorization_actor_id=$1 AND d.status='published'
		JOIN public.media_assets a ON a.id='asset_'||t.id AND a.owner_fluctlight_id=t.owner_fluctlight_id AND a.status='ready' AND a.kind='image'
		JOIN public.conversation_messages m ON m.id=d.result_message_id AND m.conversation_id=$4 AND m.author_actor_id=$3
		 AND m.attachment_refs @> jsonb_build_array(a.id)
		JOIN public.media_references r ON r.asset_id=a.id AND r.owner_fluctlight_id=$3 AND r.target_type='conversation_message' AND r.target_id=m.id
		WHERE t.owner_fluctlight_id=$3 AND t.kind='image' AND ($5::text[] IS NULL OR t.id=ANY($5::text[])) AND ` + authorization
	queries := []struct {
		sql   string
		locks string
	}{{legacy, " FOR SHARE OF t,a,r,m NOWAIT"}, {draft, " FOR SHARE OF t,x,d,a,m,r NOWAIT"}}
	byRef := map[string]directMediaDelivery{}
	for _, query := range queries {
		statement := query.sql
		if lock {
			statement += query.locks
		}
		rows, err := q.Query(ctx, statement, owner, subject, fluctlight, conversation, intentIDs)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var item directMediaDelivery
			var rawPrompt, assetDigest string
			var delivered *time.Time
			var intentRevision, assetRevision int64
			if err := rows.Scan(&item.intentID, &item.assetID, &item.messageID, &item.refID, &rawPrompt, &delivered, &intentRevision, &assetRevision, &assetDigest); err != nil {
				rows.Close()
				return nil, err
			}
			if delivered != nil {
				item.Delivered = *delivered
			}
			// The intent is a business value, not a command or Core identifier.
			// Keep it as supplied; do not recursively strip or rewrite user text.
			item.Intent = stringValue(decodeObject([]byte(rawPrompt))["intent"])
			item.Ref = "delivery:" + stableDigest(strings.Join([]string{owner, subject, fluctlight, conversation, item.intentID, item.assetID, item.messageID, item.refID, fmt.Sprint(intentRevision), fmt.Sprint(assetRevision), assetDigest, stableDigest(rawPrompt)}, "\x1f"))[:32]
			byRef[item.Ref] = item
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
	}
	result := make([]directMediaDelivery, 0, len(byRef))
	for _, item := range byRef {
		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Delivered.Equal(result[j].Delivered) {
			return result[i].Ref < result[j].Ref
		}
		return result[i].Delivered.Before(result[j].Delivered)
	})
	return result, nil
}

// A reply mutation already holds the lifecycle/Life locks. Never wait for a
// media row or authority row here: a media completion may hold that row while
// it waits for Life. NOWAIT turns that ordering conflict into a safe retry.
func lockDirectMediaDeliveryScope(ctx context.Context, q lifeContextQuerier, owner, subject, fluctlight, conversation string) error {
	var actualOwner string
	if err := q.QueryRow(ctx, `SELECT created_by_actor_id FROM public.fluctlights WHERE id=$1 FOR SHARE NOWAIT`, fluctlight).Scan(&actualOwner); err != nil {
		return err
	}
	if actualOwner != owner {
		return ErrUnauthorized
	}
	wanted := map[string]bool{owner: false, subject: false, fluctlight: false}
	actors := make([]string, 0, len(wanted))
	for actor := range wanted {
		actors = append(actors, actor)
	}
	rows, err := q.Query(ctx, `SELECT actor_id,status FROM public.conversation_participants WHERE conversation_id=$1 AND actor_id=ANY($2::text[]) ORDER BY actor_id FOR SHARE NOWAIT`, conversation, actors)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var actor, status string
		if err := rows.Scan(&actor, &status); err != nil {
			return err
		}
		if status != "active" {
			return ErrUnauthorized
		}
		wanted[actor] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, found := range wanted {
		if !found {
			return ErrUnauthorized
		}
	}
	return nil
}

func directMediaDeliveryByRef(deliveries []directMediaDelivery, ref string) (directMediaDelivery, error) {
	for _, delivery := range deliveries {
		if delivery.Ref == ref {
			return delivery, nil
		}
	}
	return directMediaDelivery{}, fmt.Errorf("media_delivery_reference_unavailable: %w", ErrConflict)
}
