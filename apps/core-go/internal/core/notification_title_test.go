package core

import (
	"net/url"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestReplyNotificationTitleUsesAuthoritativeCharacterName(t *testing.T) {
	ctx, repo, app, owner, fid, conversation := setupDirectPublicationToolTest(t, "named-notice")
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.fluctlights SET identity=identity||'{"name":"小桂"}'::jsonb WHERE id=$1`, fid); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES($1,'test','test')`, owner); err != nil {
		t.Fatal(err)
	}
	event := NotificationSource{EventID: "named-reply", Kind: "conversation.reply.published", FluctlightID: fid, SourceID: "message", OccurredAt: time.Now().UTC(), Payload: map[string]any{"conversation_id": conversation, "message_id": "message", "name": "不可采信的名字"}}
	for i := 0; i < 2; i++ {
		if err := withTransaction(ctx, repo.Pool(), func(tx pgx.Tx) error { return app.ApplyNotificationSourceTx(ctx, tx, event) }); err != nil {
			t.Fatal(err)
		}
	}
	page, err := app.OwnerNotifications(ctx, owner, url.Values{})
	if err != nil {
		t.Fatal(err)
	}
	items := arrayValue(page["items"])
	if len(items) != 1 || mapValue(items[0])["title"] != "小桂 · 新回复" {
		t.Fatalf("expected one named reply notice: %#v", items)
	}
}
