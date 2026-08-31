package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

func (a *App) ProcessDailyReview(ctx context.Context, fluctlightID, localDate string) (map[string]any, error) {
	fluctlight, err := a.DB.GetFluctlight(ctx, fluctlightID, "")
	if err != nil {
		// The internal job is authorized by the workflow intent; re-read without
		// owner filtering for worker execution.
		fluctlight, err = a.readFluctlightByID(ctx, fluctlightID)
		if err != nil {
			return nil, err
		}
	}
	if fluctlight.Status != "active" {
		return map[string]any{"fluctlight_id": fluctlightID, "local_date": localDate, "timezone": stringValue(fluctlight.Identity["timezone"]), "status": "inactive"}, nil
	}
	location, zoneErr := time.LoadLocation(canonicalTimezone(stringValue(fluctlight.Identity["timezone"])))
	if zoneErr != nil {
		return nil, zoneErr
	}
	if localDate == "" {
		localDate = time.Now().In(location).Format("2006-01-02")
	}
	var scheduleReady bool
	if err := a.DB.Pool().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.life_schedules WHERE fluctlight_id=$1 AND local_date=$2 AND status='accepted')`, fluctlightID, localDate).Scan(&scheduleReady); err != nil {
		return nil, err
	}
	if !scheduleReady {
		return map[string]any{"fluctlight_id": fluctlightID, "local_date": localDate, "timezone": location.String(), "status": "pending", "error_code": "schedule_pending"}, nil
	}
	ownerID, conversationID, err := a.directTarget(ctx, fluctlightID)
	if err != nil {
		return nil, err
	}
	goals, intentions, err := a.agencyProfile(ctx, fluctlightID)
	if err != nil {
		return nil, err
	}
	projection, err := a.BuildContextProjection(ctx, ownerID, fluctlightID, conversationID, "daily-review:"+fluctlightID+":"+localDate, "")
	if err != nil {
		return nil, err
	}
	workflowID := fmt.Sprintf("go-autonomy:%s:%s", fluctlightID, localDate)
	actionID := "autonomy_" + stableDigest(workflowID)
	providerID := "provider_" + stableDigest(actionID)
	var existingStatus, existingType string
	if err := a.DB.Pool().QueryRow(ctx, `SELECT status,action_type FROM public.autonomy_actions WHERE id=$1`, actionID).Scan(&existingStatus, &existingType); err == nil {
		return map[string]any{"action_id": actionID, "action_type": existingType, "local_date": localDate, "timezone": location.String(), "status": existingStatus}, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	decision, err := a.Provider.Structured(ctx, "cognitive_assessment", []map[string]any{
		{"role": "system", "content": "Choose one action for a daily life review. Return JSON with action_type (proactive_message, moment, or no_op), response_intent, and optional moment_media_request. Never return visible text. Honor the persona's explicit goals, intentions, and behavioral policy; an intention to publish a dynamic should be represented as action_type=moment, while an intention to contact the Owner should be represented as action_type=proactive_message."},
		{"role": "user", "content": jsonString(map[string]any{"fluctlight_id": fluctlightID, "local_date": localDate, "conversation_id": conversationID, "context": projection, "persona_profile": map[string]any{"identity": fluctlight.Identity, "personality": fluctlight.Personality, "behavioral_policy": fluctlight.BehavioralPolicy, "goals": goals, "intentions": intentions}})},
	})
	if err != nil {
		return nil, err
	}
	actionType := firstString(decision["action_type"], "")
	if actionType != "proactive_message" && actionType != "moment" && actionType != "no_op" {
		return nil, errors.New("daily_review_decision_invalid")
	}
	visible := ""
	if actionType != "no_op" {
		visible, err = a.Provider.Text(ctx, "action_realization", []map[string]any{{"role": "system", "content": "Write one concise Chinese message for the Owner. For proactive_message, address the Owner directly. For moment, write one concise public Moment."}, {"role": "user", "content": jsonString(map[string]any{"action_type": actionType, "response_intent": decision["response_intent"], "persona_profile": map[string]any{"identity": fluctlight.Identity, "personality": fluctlight.Personality, "behavioral_policy": fluctlight.BehavioralPolicy}})}})
		if err != nil {
			return nil, err
		}
	}
	err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		payload := map[string]any{"text": visible, "conversation_id": conversationID, "response_intent": decision["response_intent"], "decision": decision}
		if media := mediaConceptValue(decision["moment_media_request"]); len(media) > 0 {
			payload["moment_media_request"] = media
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.autonomy_actions (id,fluctlight_id,action_type,payload,policy_snapshot,expected_revisions,status,workflow_id,provider_request_id,created_at,settled_at) VALUES ($1,$2,$3,$4,'{}','{}','completed',$5,$6,now(),now())`, actionID, fluctlightID, actionType, jsonBytes(payload), workflowID, providerID); err != nil {
			return err
		}
		if actionType == "proactive_message" {
			return appendAssistantTx(ctx, tx, conversationID, fluctlightID, visible, "proactive:"+actionID)
		}
		if actionType == "moment" {
			if len([]rune(visible)) == 0 || len([]rune(visible)) > 32000 {
				return errors.New("moment_text_invalid")
			}
			momentID := "moment_" + stableDigest(actionID)
			if _, err := tx.Exec(ctx, `INSERT INTO public.moments (id,owner_fluctlight_id,author_actor_id,text,visibility,status,media_asset_ids) VALUES ($1,$2,$3,$4,'participants','visible','[]') ON CONFLICT DO NOTHING`, momentID, fluctlightID, fluctlightID, visible); err != nil {
				return err
			}
			if media := mediaConceptValue(decision["moment_media_request"]); len(media) > 0 {
				intentID := "media_intent_" + stableDigest(actionID)
				mediaWorkflowID := "media_workflow_" + stableDigest(actionID)
				mediaRequestID := "media_request_" + stableDigest(actionID)
				if _, err := tx.Exec(ctx, `INSERT INTO public.media_intents(id,owner_fluctlight_id,kind,mime_type,prompt,provider_request_id,workflow_id,moment_id,status,revision) VALUES($1,$2,'image','image/png',$3,$4,$5,$6,'pending',0) ON CONFLICT DO NOTHING`, intentID, fluctlightID, jsonString(media), mediaRequestID, mediaWorkflowID, momentID); err != nil {
					return err
				}
				if _, err := tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload) VALUES($1,$2,'media','media.generation',$3) ON CONFLICT DO NOTHING`, "media_workflow_intent:"+intentID, mediaWorkflowID, jsonBytes(map[string]any{"intent_id": intentID, "provider_request_id": mediaRequestID})); err != nil {
					return err
				}
			}
		}
		return appendOutboxTx(ctx, tx, "autonomy.action.completed", "autonomy_action", actionID, fluctlightID, actionID, workflowID, "autonomy-outbox:"+actionID, map[string]any{"action_type": actionType, "status": "completed"})
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"action_id": actionID, "action_type": actionType, "local_date": localDate, "timezone": location.String(), "status": "completed", "owner_actor_id": ownerID}, nil
}

func (a *App) agencyProfile(ctx context.Context, fluctlightID string) ([]map[string]any, []map[string]any, error) {
	goals := make([]map[string]any, 0)
	rows, err := a.DB.Pool().Query(ctx, `SELECT id,description,status,importance,urgency,progress FROM public.fluctlight_goals WHERE fluctlight_id=$1 AND status <> 'forgotten' ORDER BY created_at`, fluctlightID)
	if err != nil {
		return nil, nil, err
	}
	for rows.Next() {
		var id, description, status string
		var importance, urgency, progress []byte
		if err := rows.Scan(&id, &description, &status, &importance, &urgency, &progress); err != nil {
			rows.Close()
			return nil, nil, err
		}
		goals = append(goals, map[string]any{"id": id, "description": description, "status": status, "importance": jsonNumber(importance), "urgency": jsonNumber(urgency), "progress": jsonNumber(progress)})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, nil, err
	}
	rows.Close()
	intentions := make([]map[string]any, 0)
	intentRows, err := a.DB.Pool().Query(ctx, `SELECT id,goal_id,action,status,confidence,expiration FROM public.fluctlight_intentions WHERE fluctlight_id=$1 AND status NOT IN ('cancelled','completed') ORDER BY created_at`, fluctlightID)
	if err != nil {
		return nil, nil, err
	}
	for intentRows.Next() {
		var id, goalID, action, status string
		var confidence float64
		var expiration time.Time
		if err := intentRows.Scan(&id, &goalID, &action, &status, &confidence, &expiration); err != nil {
			intentRows.Close()
			return nil, nil, err
		}
		intentions = append(intentions, map[string]any{"id": id, "goal_id": goalID, "action": action, "status": status, "confidence": confidence, "expiration": expiration.Format(time.RFC3339)})
	}
	if err := intentRows.Err(); err != nil {
		intentRows.Close()
		return nil, nil, err
	}
	intentRows.Close()
	return goals, intentions, nil
}

func (a *App) directTarget(ctx context.Context, fluctlightID string) (string, string, error) {
	var owner, conversation string
	err := a.DB.Pool().QueryRow(ctx, `SELECT owner_actor_id,conversation_id FROM public.fluctlight_direct_conversations WHERE fluctlight_actor_id=$1`, fluctlightID).Scan(&owner, &conversation)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", ErrNotFound
	}
	return owner, conversation, err
}
func (a *App) readFluctlightByID(ctx context.Context, id string) (Fluctlight, error) {
	var f Fluctlight
	var i, p, b, l, pr []byte
	err := a.DB.Pool().QueryRow(ctx, `SELECT id,identity,personality,behavioral_policy,life_profile,provenance,status,current_revision FROM public.fluctlights WHERE id=$1`, id).Scan(&f.ID, &i, &p, &b, &l, &pr, &f.Status, &f.CurrentRevision)
	if errors.Is(err, pgx.ErrNoRows) {
		return f, ErrNotFound
	}
	if err != nil {
		return f, err
	}
	f.Identity = decodeObject(i)
	f.Personality = decodeObject(p)
	f.BehavioralPolicy = decodeObject(b)
	f.LifeProfile = decodeObject(l)
	f.Provenance = decodeObject(pr)
	return f, nil
}
func appendAssistantTx(ctx context.Context, tx pgx.Tx, conversationID, actorID, text, idempotency string) error {
	var existing string
	if err := tx.QueryRow(ctx, `SELECT id FROM public.conversation_messages WHERE conversation_id=$1 AND idempotency_key=$2`, conversationID, idempotency).Scan(&existing); err == nil {
		return nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	var seq int
	if err := tx.QueryRow(ctx, `SELECT next_sequence FROM public.conversation_heads WHERE conversation_id=$1 FOR UPDATE`, conversationID).Scan(&seq); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE public.conversation_heads SET next_sequence=$2 WHERE conversation_id=$1`, conversationID, seq+1); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `INSERT INTO public.conversation_messages (id,conversation_id,sequence,author_actor_id,kind,text,attachment_refs,idempotency_key) VALUES ($1,$2,$3,$4,'assistant',$5,'[]',$6)`, randomID("message_"), conversationID, seq, actorID, text, idempotency)
	return err
}
func stableDigest(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])[:32]
}
