package core

import "context"

// mediaLegacyDeliveryEvidenceSQL requires both the original target's durable
// reference and, where applicable, its actual attached asset. A ready asset
// alone does not establish delivery. The alias is fixed by both callers.
const mediaLegacyDeliveryEvidenceSQL = `EXISTS (
	SELECT 1 FROM public.media_assets a
	WHERE a.id='asset_'||t.id AND a.status='ready' AND a.owner_fluctlight_id=t.owner_fluctlight_id
	AND (
		(COALESCE(t.owner_request_id,'')<>'' AND EXISTS (
			SELECT 1 FROM public.media_references r WHERE r.asset_id=a.id AND r.owner_fluctlight_id=t.owner_fluctlight_id AND r.target_type='owner_media_request' AND r.target_id=t.owner_request_id))
		OR (COALESCE(t.owner_request_id,'')='' AND COALESCE(t.studio_job_id,'')=''
			AND NOT EXISTS (SELECT 1 FROM public.conversation_reply_actions d WHERE d.media_intent_id=t.id)
			AND (
				(COALESCE(t.message_id,'')<>'' AND EXISTS (
					SELECT 1 FROM public.media_references r JOIN public.conversation_messages m ON m.id=t.message_id
					WHERE r.asset_id=a.id AND r.owner_fluctlight_id=t.owner_fluctlight_id AND r.target_type='conversation_message' AND r.target_id=t.message_id AND m.attachment_refs @> jsonb_build_array(a.id)))
				OR (COALESCE(t.message_id,'')='' AND (COALESCE(t.conversation_id,'')<>'' OR COALESCE(t.moment_id,'')<>'')
					AND (COALESCE(t.conversation_id,'')='' OR EXISTS (
						SELECT 1 FROM public.media_references r JOIN public.conversation_messages m ON m.conversation_id=t.conversation_id
						WHERE r.asset_id=a.id AND r.owner_fluctlight_id=t.owner_fluctlight_id AND r.target_type='conversation' AND r.target_id=t.conversation_id AND m.idempotency_key='media:'||t.id||':conversation-result' AND m.attachment_refs @> jsonb_build_array(a.id)))
					AND (COALESCE(t.moment_id,'')='' OR EXISTS (
						SELECT 1 FROM public.media_references r JOIN public.moments m ON m.id=t.moment_id
						WHERE r.asset_id=a.id AND r.owner_fluctlight_id=t.owner_fluctlight_id AND r.target_type='moment' AND r.target_id=t.moment_id AND m.media_asset_ids @> jsonb_build_array(a.id)))
				)
			)
		)
	)
)`

func mediaLegacyDeliveryCommittedWith(ctx context.Context, q lifeContextQuerier, intentID string) (bool, error) {
	var delivered bool
	err := q.QueryRow(ctx, `SELECT `+mediaLegacyDeliveryEvidenceSQL+` FROM public.media_intents t WHERE t.id=$1`, intentID).Scan(&delivered)
	return delivered, err
}
