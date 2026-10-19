package core

import (
	"context"
	"time"
)

func (a *App) OwnerHarmSummary(ctx context.Context, owner string) (map[string]any, error) {
	if err := a.requireOwner(ctx, owner); err != nil {
		return nil, err
	}
	var count int64
	var first, last *time.Time
	if err := a.DB.Pool().QueryRow(ctx, `SELECT count(*),min(created_at),max(created_at) FROM public.permanent_harm_rejections WHERE owner_actor_id=$1`, owner).Scan(&count, &first, &last); err != nil {
		return nil, err
	}
	return map[string]any{"count": count, "first_recorded_at": first, "last_recorded_at": last, "coverage": "owner_browser_domain", "coverage_label": "已持久化的所有者浏览器域写入口拒绝；不是推算的内部Tool全局总数", "history_state": "recorded_since_feature_enabled"}, nil
}
