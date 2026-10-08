package core

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type GoalReviewPolicy struct {
	MissedOpportunityThreshold  int        `json:"missed_opportunity_threshold"`
	IneffectiveAttemptThreshold int        `json:"ineffective_attempt_threshold"`
	ResetAfter                  *time.Time `json:"reset_after,omitempty"`
}

func effectiveGoalReviewPolicy(policy *GoalReviewPolicy) GoalReviewPolicy {
	if policy == nil {
		return GoalReviewPolicy{MissedOpportunityThreshold: 3, IneffectiveAttemptThreshold: 3}
	}
	return *policy
}

type GoalReviewContext struct {
	DeadlineOverdue bool      `json:"deadline_overdue"`
	ID              string    `json:"id"`
	GoalID          string    `json:"goal_id"`
	LocalDate       string    `json:"local_date"`
	Timezone        string    `json:"timezone"`
	WindowStart     time.Time `json:"window_start"`
	WindowEnd       time.Time `json:"window_end"`
	SourceWatermark int64     `json:"source_watermark"`
	Revision        int       `json:"revision"`
}

type GoalReviewDecision struct {
	ReasonCategory      string   `json:"reason_category"`
	Decision            string   `json:"decision"`
	Explanation         string   `json:"explanation"`
	EvidenceRefs        []string `json:"evidence_refs"`
	StageID             string   `json:"stage_id"`
	FeasibleAlternative string   `json:"feasible_alternative"`
}

func actorGoalReviewWindow(at time.Time, timezone string) (string, time.Time, time.Time, error) {
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return "", time.Time{}, time.Time{}, err
	}
	local := at.In(location)
	start := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
	return start.Format("2006-01-02"), start.UTC(), start.AddDate(0, 0, 1).UTC(), nil
}

// The existing lifecycle schedule and explicit review Tool call this service.
// Same local cycle with unchanged journal waterline is a no-op; late arrivals
// revise that cycle instead of creating another daily commitment/quota.
func queueGoalReviewsTx(ctx context.Context, tx pgx.Tx, owner, profile, reason string, at time.Time) (string, error) {
	timezone, err := readLifeContextTimezoneWith(ctx, tx, owner)
	if err != nil {
		return "", err
	}
	date, start, end, err := actorGoalReviewWindow(at, timezone)
	if err != nil {
		return "", err
	}
	var watermark int64
	if err := tx.QueryRow(ctx, `SELECT COALESCE(max(id),0) FROM public.goal_source_events WHERE fluctlight_id=$1 AND ($2='' OR profile_id IS NULL OR profile_id=$2) AND occurred_at >= $3 AND occurred_at < $4`, owner, profile, start, end).Scan(&watermark); err != nil {
		return "", err
	}
	rows, err := tx.Query(ctx, `SELECT id FROM public.fluctlight_goals WHERE fluctlight_id=$1 AND (profile_id IS NULL OR profile_id=$2) AND status IN ('active','paused') ORDER BY updated_at DESC,id DESC`, owner, profile)
	if err != nil {
		return "", err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return "", err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return "", err
	}
	pending := []string{}
	for _, id := range ids {
		reviewID := "goal_review_" + stableDigest(id+"\x1f"+date)
		tag, err := tx.Exec(ctx, `INSERT INTO public.goal_reviews(id,fluctlight_id,goal_id,local_date,timezone,window_start,window_end,source_watermark) VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT(goal_id,local_date,strategy_version) DO UPDATE SET status='pending',source_watermark=EXCLUDED.source_watermark,revision=goal_reviews.revision+1,updated_at=now() WHERE goal_reviews.source_watermark<EXCLUDED.source_watermark`, reviewID, owner, id, date, timezone, start, end, watermark)
		if err != nil {
			return "", err
		}
		if tag.RowsAffected() > 0 {
			pending = append(pending, id)
		}
	}
	if len(pending) == 0 {
		return "", nil
	}
	requestID, err := queueGoalEvaluationTx(ctx, tx, owner, profile, "goal_review:"+reason, "review:"+date+":"+reason+":"+fmt.Sprint(watermark), pending)
	if err != nil {
		return "", err
	}
	_, err = tx.Exec(ctx, `UPDATE public.goal_reviews SET evaluation_request_id=$1 WHERE fluctlight_id=$2 AND goal_id=ANY($3::text[]) AND local_date=$4 AND status='pending'`, requestID, owner, pending, date)
	return requestID, err
}

func commitGoalReviewTx(ctx context.Context, tx pgx.Tx, goal GoalAuthority, review GoalReviewContext, decision *GoalReviewDecision, sources map[string]GoalSource, nextStep string, nextReview *time.Time) error {
	if decision == nil {
		return errors.New("goal_review_decision_required")
	}
	switch decision.ReasonCategory {
	case "progressed", "no_opportunity", "not_yet_due", "waiting_external", "ineffective_attempt", "blocked", "missed_opportunity", "evaluation_incomplete", "refusal", "sleeping":
	default:
		return errors.New("goal_review_reason_invalid")
	}
	switch decision.Decision {
	case "continue", "wait", "adjust", "pause", "abandon":
	default:
		return errors.New("goal_review_decision_invalid")
	}
	if decision.Explanation == "" || len([]rune(decision.Explanation)) > 1000 {
		return errors.New("goal_review_explanation_required")
	}
	for _, ref := range decision.EvidenceRefs {
		source, ok := sources[ref]
		if !ok || !source.Valid || source.FluctlightID != goal.FluctlightID {
			return errors.New("goal_review_evidence_invalid")
		}
		if (goal.ProfileID != "" && source.ProfileID != "" && source.ProfileID != goal.ProfileID) || (len(source.GoalIDs) > 0 && !containsString(source.GoalIDs, goal.EntityID)) {
			return errors.New("goal_review_evidence_scope_invalid")
		}

	}
	if decision.ReasonCategory == "ineffective_attempt" || decision.ReasonCategory == "missed_opportunity" {
		inWindow := false
		for _, ref := range decision.EvidenceRefs {
			s := sources[ref]
			if s.OccurredAt.Before(review.WindowStart) || !s.OccurredAt.Before(review.WindowEnd) {
				continue
			}
			if decision.ReasonCategory == "missed_opportunity" {
				inWindow = true
				continue
			}
			if (s.Kind == "outcome" && s.CanSupportSuccess && stringValue(s.Data["status"]) == "completed") || (s.Kind == "message" && s.SubjectActorID == goal.FluctlightID) {
				inWindow = true
			}
		}
		if !inWindow {
			return errors.New("goal_review_actual_period_evidence_required")
		}
	}
	if decision.ReasonCategory == "missed_opportunity" && (len(decision.EvidenceRefs) == 0 || decision.StageID == "" || decision.StageID != goal.CurrentStageID || decision.FeasibleAlternative == "") {
		return errors.New("goal_review_opportunity_basis_required")
	}
	policy := effectiveGoalReviewPolicy(goal.ReviewPolicy)
	threshold := 0
	if decision.ReasonCategory == "missed_opportunity" {
		threshold = policy.MissedOpportunityThreshold
	}
	if decision.ReasonCategory == "ineffective_attempt" {
		threshold = policy.IneffectiveAttemptThreshold
	}
	if threshold > 0 {
		rows, err := tx.Query(ctx, `SELECT reason_category FROM public.goal_reviews WHERE goal_id=$1 AND status='succeeded' AND local_date<$2::date AND ($3::timestamptz IS NULL OR window_start>=$3) ORDER BY local_date DESC,id DESC LIMIT 30`, goal.EntityID, review.LocalDate, policy.ResetAfter)
		if err != nil {
			return err
		}
		streak := 1
		for rows.Next() {
			var category string
			if err := rows.Scan(&category); err != nil {
				rows.Close()
				return err
			}
			if category != decision.ReasonCategory {
				break
			}
			streak++
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		if streak >= threshold {
			if decision.ReasonCategory == "ineffective_attempt" && decision.Decision != "adjust" && decision.Decision != "pause" && decision.Decision != "abandon" {
				return errors.New("goal_review_strategy_change_required")
			}
			if decision.ReasonCategory == "missed_opportunity" && nextStep == "" && decision.Decision != "pause" && decision.Decision != "abandon" {
				return errors.New("goal_review_concrete_next_step_required")
			}
		}
	}
	tag, err := tx.Exec(ctx, `UPDATE public.goal_reviews SET status='succeeded',reason_category=$3,decision=$4,explanation=$5,next_step=$6,next_review_at=$7,evidence_refs=$8,deadline_overdue=$9,updated_at=now() WHERE id=$1 AND revision=$2 AND status='pending'`, review.ID, review.Revision, decision.ReasonCategory, decision.Decision, decision.Explanation, nextStep, nextReview, jsonBytes(decision.EvidenceRefs), review.DeadlineOverdue)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrConflict
	}
	return nil
}
