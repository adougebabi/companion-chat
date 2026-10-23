package core

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// NotificationSource is a committed domain event, independent of Redis/SDKs.
// The transport composition invokes this projection inside its durable inbox
// transaction; it never generates a notification from an uncommitted proposal.
type NotificationSource struct {
	EventID, Kind, FluctlightID, SourceID string
	OccurredAt                            time.Time
	Payload                               map[string]any
}

func notificationContent(event NotificationSource) (title, body, link string, handled bool) {
	base := "#/c/" + url.PathEscape(event.FluctlightID)
	switch event.Kind {
	case "conversation.reply.published":
		return "新回复", "有一条已保存的角色回复。", base + "/chat?" + url.Values{"conversation": {stringValue(event.Payload["conversation_id"])}, "message": {stringValue(event.Payload["message_id"])}}.Encode(), true
	case "moment.published":
		return "新动态", "角色发布了一条动态。", "#/feed/" + url.PathEscape(stringValue(event.Payload["moment_id"])), true
	case "moment.comment.created":
		return "动态有新评论", "有人回应了一条动态。", "#/feed/" + url.PathEscape(stringValue(event.Payload["moment_id"])), true
	case "moment.reaction.created":
		return "动态有新回应", "有人回应了一条动态。", "#/feed/" + url.PathEscape(stringValue(event.Payload["moment_id"])), true
	case "capability.requested":
		return "新的能力请求", "角色提出了一项需要审查的能力请求。", base + "/governance?tab=capabilities", true
	case "owner.request.settled":
		// Ordinary outcomes are already visible in chat, moments or life. Only
		// a genuine accepted invitation needs a separate personal notice.
		if event.Payload["request_kind"] == "promise" && event.Payload["status"] == "accepted" {
			return "她答应了你的邀请", "去看看你们的约定。", base + "/life/schedule?section=promises", true
		}
		return "", "", "", false
	case "media.asset.ready":
		return "有了新图片", "可以去相册看看。", base + "/media?asset=" + url.QueryEscape(stringValue(event.Payload["asset_id"])), true
	case "world.npc.encountered":
		return "人物相遇记录", "查看互动事件和关系来源。", base + "/people", true
	case "diary.generated":
		return "新的日记", "角色完成了一篇日记。", base + "/life/diary/" + url.PathEscape(stringValue(event.Payload["entry_id"])), true
	case "memory.embedding.failed":
		return "", "", "", false
	case "goal.evaluated":
		if event.Payload["status"] == "completed" {
			return "目标已完成", "目标完成评估已保存，请核对证据。", base + "/life/goal/" + url.PathEscape(stringValue(event.Payload["goal_id"])), true
		}
	}
	return "", "", "", false
}

func (a *App) ApplyNotificationSourceTx(ctx context.Context, tx pgx.Tx, event NotificationSource) error {
	title, body, link, handled := notificationContent(event)
	if !handled {
		return nil
	}
	if event.EventID == "" || event.FluctlightID == "" || event.SourceID == "" || event.OccurredAt.IsZero() {
		return ErrInvalidArguments
	}
	required := map[string][]string{"conversation.reply.published": {"message_id", "conversation_id"}, "moment.published": {"moment_id"}, "moment.comment.created": {"moment_id", "author_actor_id"}, "moment.reaction.created": {"moment_id", "author_actor_id"}, "owner.request.settled": {"request_id"}, "media.asset.ready": {"asset_id"}, "diary.generated": {"entry_id"}, "memory.embedding.failed": {"memory_id"}, "goal.evaluated": {"goal_id"}, "capability.requested": {"request_id"}}
	for _, key := range required[event.Kind] {
		if strings.TrimSpace(stringValue(event.Payload[key])) == "" {
			return ErrInvalidArguments
		}
	}
	owner, err := a.DB.FluctlightOwnerTx(ctx, tx, event.FluctlightID)
	if err != nil {
		return err
	}
	// A person's own comment is not an incoming interaction notification.
	if (event.Kind == "moment.comment.created" || event.Kind == "moment.reaction.created") && event.Payload["author_actor_id"] == owner {
		return nil
	}
	var quiet bool
	err = tx.QueryRow(ctx, `SELECT notifications_muted FROM public.autonomy_policies WHERE fluctlight_id=$1`, event.FluctlightID).Scan(&quiet)
	if errors.Is(err, pgx.ErrNoRows) {
		config, e := readGlobalAutonomyConfiguration(ctx, tx)
		if e != nil {
			return e
		}
		quiet, _ = config["notifications_muted"].(bool)
	} else if err != nil {
		return err
	}
	preferenceKey := ""
	switch event.Kind {
	case "conversation.reply.published":
		preferenceKey = "messages"
	case "goal.evaluated", "owner.request.settled":
		preferenceKey = "goals"
	case "media.asset.ready":
		preferenceKey = "media"
	}
	if preferenceKey != "" {
		var raw string
		err := tx.QueryRow(ctx, `SELECT value_json FROM public.runtime_settings WHERE key='product.notifications'`).Scan(&raw)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if err == nil {
			preferences := decodeObject([]byte(raw))
			if err := validNotificationPreferences(preferences); err != nil {
				return err
			}
			quiet = quiet || preferences[preferenceKey] == false
		}
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.owner_notifications(id,owner_actor_id,fluctlight_id,source_event_id,source_kind,source_id,source_occurred_at,title,body,link,quiet,metadata,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13) ON CONFLICT(owner_actor_id,source_event_id) DO NOTHING`, "notice_"+stableDigest(owner+":"+event.EventID), owner, event.FluctlightID, event.EventID, event.Kind, event.SourceID, event.OccurredAt, title, body, link, quiet, jsonBytes(map[string]any{"correlation_source": "committed_domain_event", "request_kind": event.Payload["request_kind"], "settlement_status": event.Payload["status"]}), a.now().UTC())
	return err
}

// This predicate has the same meaning for old and new stored notices.
const personalNotificationSQL = `source_kind<>'memory.embedding.failed' AND (source_kind<>'owner.request.settled' OR (metadata->>'request_kind'='promise' AND metadata->>'settlement_status'='accepted') OR EXISTS(SELECT 1 FROM public.platform_outbox_events e WHERE e.id=owner_notifications.source_event_id AND e.payload->>'request_kind'='promise' AND e.payload->>'status'='accepted'))`

func (a *App) OwnerNotifications(ctx context.Context, owner string, query url.Values) (map[string]any, error) {
	if err := a.requireOwner(ctx, owner); err != nil {
		return nil, err
	}
	limit, err := reflectionReadPage(query)
	if err != nil {
		return nil, err
	}
	unread := false
	if raw := query.Get("unread"); raw != "" {
		var e error
		unread, e = strconv.ParseBool(raw)
		if e != nil {
			return nil, invalid("unread must be boolean")
		}
	}
	fid, err := ownerQueryFluctlight(query)
	if err != nil {
		return nil, err
	}
	if fid != "" {
		if _, err := a.DB.GetFluctlight(ctx, fid, owner); err != nil {
			return nil, err
		}
	}
	scope := stableDigest(jsonString([]any{"notifications", owner, fid, unread}))
	cursor, err := decodeOwnerReadCursor(query.Get("cursor"), scope)
	if err != nil || (query.Get("cursor") != "" && (cursor.ID == "" || cursor.Time.IsZero() || cursor.Snapshot == nil)) {
		return nil, invalid("notification cursor is invalid")
	}
	asOf := a.now().UTC()
	if cursor.Snapshot != nil {
		asOf = *cursor.Snapshot
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT id,created_at,jsonb_build_object('id',id,'fluctlight_id',fluctlight_id,'title',title,'body',body,'link',link,'quiet',quiet,'read_at',read_at,'created_at',created_at,'source_kind',source_kind,'source_id',source_id,'source_event_id',source_event_id,'source_occurred_at',source_occurred_at) FROM public.owner_notifications WHERE `+personalNotificationSQL+` AND owner_actor_id=$1 AND ($2='' OR fluctlight_id=$2) AND (NOT $3 OR read_at IS NULL) AND created_at<=$4 AND ($5='' OR (created_at,id)<($6,$5)) ORDER BY created_at DESC,id DESC LIMIT $7`, owner, fid, unread, asOf, cursor.ID, cursor.Time, limit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []any{}
	next := ""
	last := ownerReadCursor{Scope: scope, Snapshot: &asOf}
	for rows.Next() {
		var id string
		var at time.Time
		var raw []byte
		if err := rows.Scan(&id, &at, &raw); err != nil {
			return nil, err
		}
		if len(items) == limit {
			next = encodeOwnerReadCursor(last)
			break
		}
		items = append(items, decodeObject(raw))
		last = ownerReadCursor{Scope: scope, ID: id, Time: at, Snapshot: &asOf}
	}
	return map[string]any{"items": items, "next_cursor": next, "as_of": asOf.Format(time.RFC3339Nano)}, rows.Err()
}

func (a *App) OwnerUnreadNotifications(ctx context.Context, owner string) (map[string]any, error) {
	if err := a.requireOwner(ctx, owner); err != nil {
		return nil, err
	}
	var count, alerts int
	err := a.DB.Pool().QueryRow(ctx, `SELECT count(*),count(*)FILTER(WHERE NOT quiet) FROM public.owner_notifications WHERE `+personalNotificationSQL+` AND owner_actor_id=$1 AND read_at IS NULL`, owner).Scan(&count, &alerts)
	return map[string]any{"count": count, "alert_count": alerts}, err
}

func (a *App) MarkOwnerNotificationRead(ctx context.Context, owner, id string, body map[string]any) (map[string]any, error) {
	if err := a.requireOwner(ctx, owner); err != nil {
		return nil, err
	}
	tx, ok := SurfaceDBTx(ctx)
	if !ok {
		return nil, errors.New("notification_transaction_required")
	}
	var at time.Time
	if err := tx.QueryRow(ctx, `UPDATE public.owner_notifications SET read_at=COALESCE(read_at,$3) WHERE id=$1 AND owner_actor_id=$2 RETURNING read_at`, id, owner, a.now().UTC()).Scan(&at); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &SurfaceError{Status: 404, Code: "notification_not_found", Message: "Notification is unavailable"}
		}
		return nil, err
	}
	return map[string]any{"id": id, "read_at": at.Format(time.RFC3339Nano)}, nil
}

func (a *App) MarkAllOwnerNotificationsRead(ctx context.Context, owner string, body map[string]any) (map[string]any, error) {
	if err := a.requireOwner(ctx, owner); err != nil {
		return nil, err
	}
	tx, ok := SurfaceDBTx(ctx)
	if !ok {
		return nil, errors.New("notification_transaction_required")
	}
	before, err := time.Parse(time.RFC3339Nano, stringValue(body["before"]))
	if err != nil || before.After(a.now().UTC().Add(time.Minute)) {
		return nil, invalid("before must be a valid observed snapshot timestamp")
	}
	if strings.TrimSpace(stringValue(body["idempotency_key"])) == "" {
		return nil, invalid("idempotencyKey is required")
	}
	tag, err := tx.Exec(ctx, `UPDATE public.owner_notifications SET read_at=$3 WHERE owner_actor_id=$1 AND created_at<=$2 AND read_at IS NULL`, owner, before, a.now().UTC())
	if err != nil {
		return nil, err
	}
	return map[string]any{"updated": tag.RowsAffected(), "before": before.Format(time.RFC3339Nano)}, nil
}

func appendOwnerRequestNotificationTx(ctx context.Context, tx pgx.Tx, fid, id, kind, status, source string, failureReason ...string) error {
	payload := map[string]any{"request_id": id, "request_kind": kind, "status": status}
	if status == "failed" && len(failureReason) > 0 {
		payload["failure_reason"] = failureReason[0]
	}
	return appendOutboxTx(ctx, tx, "owner.request.settled", "owner_request", id, fid, source, "owner-request:"+id, "owner-request-settled:"+id+":"+status+":"+stableDigest(source), payload)
}
