package core

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// A prior Moment delivery requires the visible character-authored Moment,
// the exact attached ready image and its reference, and the matching image
// intent. Uploads or other assets without such an intent remain unknown.
func momentMediaDeliveriesForIntents(ctx context.Context, q lifeContextQuerier, owner, fluctlight string, intentIDs []string, lock bool) ([]directMediaDelivery, error) {
	if owner == "" || fluctlight == "" {
		return nil, ErrUnauthorized
	}
	if lock {
		var id string
		if err := q.QueryRow(ctx, `SELECT id FROM public.fluctlights WHERE id=$2 AND created_by_actor_id=$1 FOR SHARE NOWAIT`, owner, fluctlight).Scan(&id); err != nil {
			return nil, err
		}
	}
	statement := `SELECT t.id,a.id,m.id,r.id,t.prompt,t.revision,a.revision,a.sha256,a.version,a.mime_type,m.revision
		FROM public.media_intents t
		JOIN public.media_assets a ON a.id='asset_'||t.id AND a.owner_fluctlight_id=t.owner_fluctlight_id AND a.status='ready' AND a.kind='image' AND lower(a.mime_type) LIKE 'image/%'
		JOIN public.moments m ON m.id=t.moment_id AND m.owner_actor_id=$1 AND m.owner_fluctlight_id=$2 AND m.author_actor_id=$2 AND m.status='visible' AND m.media_asset_ids @> jsonb_build_array(a.id)
		JOIN public.media_references r ON r.asset_id=a.id AND r.owner_fluctlight_id=$2 AND r.target_type='moment' AND r.target_id=m.id
		WHERE t.owner_fluctlight_id=$2 AND t.kind='image' AND COALESCE(t.owner_request_id,'')='' AND COALESCE(t.studio_job_id,'')=''
		AND ($3::text[] IS NULL OR t.id=ANY($3::text[]))
		AND EXISTS (SELECT 1 FROM public.fluctlights f WHERE f.id=$2 AND f.created_by_actor_id=$1)
		ORDER BY t.id,a.id,m.id,r.id`
	if lock {
		statement += ` FOR SHARE OF t,a,m,r NOWAIT`
	}
	rows, err := q.Query(ctx, statement, owner, fluctlight, intentIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []directMediaDelivery{}
	for rows.Next() {
		var item directMediaDelivery
		var prompt, digest, version, mime string
		var intentRevision, assetRevision, momentRevision int64
		if err := rows.Scan(&item.intentID, &item.assetID, &item.momentID, &item.refID, &prompt, &intentRevision, &assetRevision, &digest, &version, &mime, &momentRevision); err != nil {
			return nil, err
		}
		item.Intent = stringValue(decodeObject([]byte(prompt))["intent"])
		item.Ref = "moment-delivery:" + stableDigest(strings.Join([]string{owner, fluctlight, item.intentID, item.assetID, item.momentID, item.refID, fmt.Sprint(intentRevision), fmt.Sprint(assetRevision), fmt.Sprint(momentRevision), digest, version, mime, stableDigest(prompt)}, "\x1f"))[:32]
		// Moment creation time is not an attachment time: media can arrive later.
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Ref < result[j].Ref })
	return result, nil
}

func requireMomentMediaDeliveriesTx(ctx context.Context, q lifeContextQuerier, owner, fluctlight string, refs, rawIntentIDs []any) error {
	if len(refs) == 0 {
		return nil
	}
	if len(refs) != len(rawIntentIDs) {
		return fmt.Errorf("moment_media_delivery_intents_invalid: %w", ErrConflict)
	}
	ids := make([]string, 0, len(rawIntentIDs))
	for _, raw := range rawIntentIDs {
		id := stringValue(raw)
		if id == "" {
			return fmt.Errorf("moment_media_delivery_intent_invalid: %w", ErrConflict)
		}
		ids = append(ids, id)
	}
	deliveries, err := momentMediaDeliveriesForIntents(ctx, q, owner, fluctlight, ids, true)
	if err != nil {
		return err
	}
	for _, raw := range refs {
		if _, err := directMediaDeliveryByRef(deliveries, stringValue(raw)); err != nil {
			return err
		}
	}
	return nil
}
