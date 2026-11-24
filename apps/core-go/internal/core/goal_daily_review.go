package core

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

// At local 03:00 review the preceding complete local day. A personality
// created later that morning waits until its first real daily boundary.
func dailyGoalReviewClock(now, created time.Time, timezone string) (string, time.Time, bool, error) {
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return "", time.Time{}, false, err
	}
	local := now.In(location)
	due := time.Date(local.Year(), local.Month(), local.Day(), 3, 0, 0, 0, location)
	day := due.AddDate(0, 0, -1)
	return day.Format("2006-01-02"), day, !now.Before(due) && created.Before(due), nil
}

func queueDailyGoalReviewsTx(ctx context.Context, tx pgx.Tx, now time.Time, limit int) (int, error) {
	// Filter the persisted date before LIMIT so already scheduled personalities
	// cannot starve later rows. Queueing and recording the actual merged request
	// share the same transaction and Life lock.
	rows, err := tx.Query(ctx, `SELECT f.id,f.created_at,COALESCE(NULLIF(f.identity->>'timezone',''),'Asia/Shanghai') FROM public.fluctlights f LEFT JOIN public.goal_agent_cadence c ON c.fluctlight_id=f.id WHERE f.status='active' AND (c.daily_review_date IS NULL OR c.daily_review_date < ($1::timestamptz AT TIME ZONE COALESCE(NULLIF(f.identity->>'timezone',''),'Asia/Shanghai'))::date-1) AND ($1::timestamptz AT TIME ZONE COALESCE(NULLIF(f.identity->>'timezone',''),'Asia/Shanghai'))::time >= time '03:00' AND f.created_at < ((($1::timestamptz AT TIME ZONE COALESCE(NULLIF(f.identity->>'timezone',''),'Asia/Shanghai'))::date+time '03:00') AT TIME ZONE COALESCE(NULLIF(f.identity->>'timezone',''),'Asia/Shanghai')) AND EXISTS(SELECT 1 FROM public.fluctlight_goals g WHERE g.fluctlight_id=f.id AND g.status IN ('active','paused') AND (g.profile_id IS NULL OR g.profile_id=COALESCE((SELECT active_profile_id FROM public.fluctlight_personality_runtime WHERE fluctlight_id=f.id),'default'))) AND NOT EXISTS(SELECT 1 FROM public.goal_evaluation_requests r WHERE r.fluctlight_id=f.id AND r.status IN ('processing','retry')) ORDER BY f.id LIMIT $2`, now, limit)
	if err != nil {
		return 0, err
	}
	type candidate struct {
		id, zone string
		created  time.Time
	}
	var candidates []candidate
	for rows.Next() {
		var c candidate
		if err := rows.Scan(&c.id, &c.created, &c.zone); err != nil {
			rows.Close()
			return 0, err
		}
		candidates = append(candidates, c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	queued := 0
	for _, c := range candidates {
		if err := lockLifeContextTx(ctx, tx, c.id); err != nil {
			return queued, err
		}
		date, at, due, err := dailyGoalReviewClock(now, c.created, c.zone)
		if err != nil {
			return queued, err
		}
		if !due {
			continue
		}
		var already bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.goal_agent_cadence WHERE fluctlight_id=$1 AND daily_review_date >= $2::date)`, c.id, date).Scan(&already); err != nil {
			return queued, err
		}
		if already {
			continue
		}
		profile, err := (&intentionService{}).resolveProfile(ctx, tx, c.id, "")
		if err != nil {
			return queued, err
		}
		request, err := queueGoalReviewsTx(ctx, tx, c.id, profile, "daily_03", at)
		if err != nil {
			return queued, err
		}
		// A completed review with unchanged evidence also settles this date.
		// Always advance the marker so it cannot starve later LIMIT candidates.
		if _, err := tx.Exec(ctx, `INSERT INTO public.goal_agent_cadence(fluctlight_id,daily_review_date,daily_review_request_id) VALUES($1,$2,$3) ON CONFLICT(fluctlight_id) DO UPDATE SET daily_review_date=EXCLUDED.daily_review_date,daily_review_request_id=EXCLUDED.daily_review_request_id,updated_at=now()`, c.id, date, nullableString(request)); err != nil {
			return queued, err
		}
		queued++
	}
	return queued, nil
}
