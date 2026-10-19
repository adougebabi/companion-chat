package core

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// This exercises the actual DB projection and commands, without a model.
// Redis crash/ACK delivery remains covered by the platform consumer suite.
func TestOwnerNotificationsDurableDedupScopePagingMuteAndObservedReadAll(t *testing.T) {
	ctx, repo, app, owner, fid, conversation := setupDirectPublicationToolTest(t, "notifications")
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES($1,'test-hash','test-revision')`, owner); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 16, 1, 0, 0, 0, time.UTC)
	app.Clock = func() time.Time { return now }
	if err := withTransaction(ctx, repo.Pool(), func(tx pgx.Tx) error { return ensureAutonomyPolicyTx(ctx, tx, fid) }); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.autonomy_policies SET notifications_muted=true WHERE fluctlight_id=$1`, fid); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		event := NotificationSource{EventID: fmt.Sprint("event-", i), Kind: "conversation.reply.published", FluctlightID: fid, SourceID: fmt.Sprint("message-", i), OccurredAt: now, Payload: map[string]any{"conversation_id": conversation, "message_id": fmt.Sprint("message-", i)}}
		for j := 0; j < 2; j++ {
			if err := withTransaction(ctx, repo.Pool(), func(tx pgx.Tx) error { return app.ApplyNotificationSourceTx(ctx, tx, event) }); err != nil {
				t.Fatal(err)
			}
		}
	}
	count, err := app.OwnerUnreadNotifications(ctx, owner)
	if err != nil || intValue(count["count"]) != 5 || intValue(count["alert_count"]) != 0 {
		t.Fatalf("quiet count %#v %v", count, err)
	}
	cursor := ""
	seen := map[string]bool{}
	snapshot := ""
	for {
		page, err := app.OwnerNotifications(ctx, owner, url.Values{"limit": {"2"}, "cursor": {cursor}})
		if err != nil {
			t.Fatal(err)
		}
		snapshot = stringValue(page["as_of"])
		for _, raw := range arrayValue(page["items"]) {
			item := mapValue(raw)
			id := stringValue(item["id"])
			if seen[id] || item["quiet"] != true || item["fluctlight_id"] != fid {
				t.Fatalf("duplicate or wrong scope %#v", item)
			}
			seen[id] = true
		}
		cursor = stringValue(page["next_cursor"])
		if cursor == "" {
			break
		}
	}
	if len(seen) != 5 {
		t.Fatal("incomplete notification pagination")
	}
	now = now.Add(time.Second)
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.autonomy_policies SET notifications_muted=false WHERE fluctlight_id=$1`, fid); err != nil {
		t.Fatal(err)
	}
	event := NotificationSource{EventID: "event-later", Kind: "conversation.reply.published", FluctlightID: fid, SourceID: "message-later", OccurredAt: now, Payload: map[string]any{"conversation_id": conversation, "message_id": "message-later"}}
	if err := withTransaction(ctx, repo.Pool(), func(tx pgx.Tx) error { return app.ApplyNotificationSourceTx(ctx, tx, event) }); err != nil {
		t.Fatal(err)
	}
	var updated map[string]any
	if err := newPostgresSurface(repo.Pool()).Transact(ctx, func(txCtx context.Context, _ surfaceTx) error {
		var err error
		updated, err = app.MarkAllOwnerNotificationsRead(txCtx, owner, map[string]any{"before": snapshot, "idempotency_key": "read-observed"})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if updated["updated"] != int64(5) {
		t.Fatalf("observed snapshot %#v", updated)
	}
	count, err = app.OwnerUnreadNotifications(ctx, owner)
	if err != nil || intValue(count["count"]) != 1 || intValue(count["alert_count"]) != 1 {
		t.Fatalf("new arrival swallowed %#v %v", count, err)
	}
	if _, err := app.OwnerNotifications(ctx, "foreign-owner", url.Values{}); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("foreign owner %v", err)
	}
}
