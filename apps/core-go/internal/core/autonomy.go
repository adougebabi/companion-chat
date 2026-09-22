package core

// StructuredAssembledWithToolsSchema remains the canonical assembled Eino task boundary.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func (a *App) tryDailyReviewExecutionLock(ctx context.Context, fluctlightID, localDate string) (func(), bool, error) {
	connection, err := a.DB.Pool().Acquire(ctx)
	if err != nil {
		return func() {}, false, err
	}
	key := "daily-review:" + strings.TrimSpace(fluctlightID) + ":" + strings.TrimSpace(localDate)
	var acquired bool
	if err := connection.QueryRow(ctx, `SELECT pg_try_advisory_lock(hashtextextended($1,0))`, key).Scan(&acquired); err != nil {
		connection.Release()
		return func() {}, false, err
	}
	if !acquired {
		connection.Release()
		return func() {}, false, nil
	}
	release := func() {
		var unlocked bool
		_ = connection.QueryRow(context.Background(), `SELECT pg_advisory_unlock(hashtextextended($1,0))`, key).Scan(&unlocked)
		connection.Release()
	}
	return release, true, nil
}

func (a *App) agencyProfile(ctx context.Context, fluctlightID string) ([]map[string]any, []map[string]any, error) {
	goals := make([]map[string]any, 0)
	rows, err := a.DB.Pool().Query(ctx, `SELECT id,profile_id,scope,target_actor_id,description,desired_outcome,success_criteria,motivation,needs_reflection,status,importance,urgency,progress,deadline,evidence_refs,revision FROM public.fluctlight_goals WHERE fluctlight_id=$1 ORDER BY created_at`, fluctlightID)
	if err != nil {
		return nil, nil, err
	}
	for rows.Next() {
		var id, scope, description, desiredOutcome, motivation, status string
		var profileID *string
		var targetActorID *string
		var successCriteria, importance, urgency, progress []byte
		var needsReflection bool
		var deadline *time.Time
		var evidenceRefs []byte
		var revision int
		if err := rows.Scan(&id, &profileID, &scope, &targetActorID, &description, &desiredOutcome, &successCriteria, &motivation, &needsReflection, &status, &importance, &urgency, &progress, &deadline, &evidenceRefs, &revision); err != nil {
			rows.Close()
			return nil, nil, err
		}
		item := map[string]any{"id": id, "scope": scope, "description": description, "desired_outcome": desiredOutcome, "success_criteria": decodeArray(successCriteria), "motivation": motivation, "needs_reflection": needsReflection, "status": status, "importance": jsonNumber(importance), "urgency": jsonNumber(urgency), "progress": jsonNumber(progress), "evidence_refs": decodeArray(evidenceRefs), "revision": revision}
		if deadline != nil {
			item["deadline"] = deadline.Format(time.RFC3339Nano)
		}
		if profileID != nil && strings.TrimSpace(*profileID) != "" {
			item["profile_id"] = *profileID
		}
		if targetActorID != nil {
			item["target_actor_id"] = *targetActorID
		}
		goals = append(goals, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, nil, err
	}
	rows.Close()
	intentions := make([]map[string]any, 0)
	intentRows, err := a.DB.Pool().Query(ctx, `SELECT i.id,i.profile_id,i.goal_id,COALESCE(g.desired_outcome,''),i.action_intent,i.expected_outcome,i.capability_constraints,i.status,i.confidence,i.preferred_time,i.expiration,i.trigger,i.evidence_refs,i.revision,COALESCE(i.current_attempt_id,'') FROM public.fluctlight_intentions i LEFT JOIN public.fluctlight_goals g ON g.id=i.goal_id AND g.fluctlight_id=i.fluctlight_id WHERE i.fluctlight_id=$1 AND i.status NOT IN ('cancelled','completed','expired') AND i.expiration > now() ORDER BY i.created_at`, fluctlightID)
	if err != nil {
		return nil, nil, err
	}
	for intentRows.Next() {
		var id, goalDescription, action, expectedOutcome, status, currentAttemptID string
		var profileID, goalID *string
		var capabilityConstraints, trigger, evidenceRefs []byte
		var confidence float64
		var revision int
		var preferredTime, expiration *time.Time
		if err := intentRows.Scan(&id, &profileID, &goalID, &goalDescription, &action, &expectedOutcome, &capabilityConstraints, &status, &confidence, &preferredTime, &expiration, &trigger, &evidenceRefs, &revision, &currentAttemptID); err != nil {
			intentRows.Close()
			return nil, nil, err
		}
		item := map[string]any{"id": id, "goal": goalDescription, "action": action, "action_intent": action, "expected_outcome": expectedOutcome, "capability_constraints": decodeArray(capabilityConstraints), "status": status, "confidence": confidence, "trigger": decodeObject(trigger), "evidence_refs": decodeArray(evidenceRefs), "revision": revision}
		if currentAttemptID != "" {
			item["current_attempt_id"] = currentAttemptID
		}
		if profileID != nil && strings.TrimSpace(*profileID) != "" {
			item["profile_id"] = *profileID
		}
		if goalID != nil && strings.TrimSpace(*goalID) != "" {
			item["goal_id"] = *goalID
		}
		triggerValue := decodeObject(trigger)
		if target := strings.TrimSpace(stringValue(triggerValue["target_actor_id"])); target != "" {
			item["target_actor_id"] = target
		}
		if preferredTime != nil {
			item["preferred_time"] = preferredTime.Format(time.RFC3339)
		}
		if expiration != nil {
			item["expiration"] = expiration.Format(time.RFC3339)
		}
		intentions = append(intentions, item)
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
func stableDigest(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])[:32]
}
