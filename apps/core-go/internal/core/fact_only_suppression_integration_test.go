package core

import (
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestFactOnlyProactiveSuppressionAcrossPurposeVersions(t *testing.T) {
	for _, test := range []struct {
		name, oldPurpose, nextPurpose, mode string
		bridge, want                        bool
	}{
		{"old_to_new", "care", factOnlyReplyPublicationPurpose, "fact_only", true, true},
		{"new_to_old_draft", factOnlyReplyPublicationPurpose, "care", "fact_only", true, true},
		{"fallback_to_new", "care", factOnlyReplyPublicationPurpose, "legacy_fallback", true, true},
		{"legacy_caller_unchanged", "care", factOnlyReplyPublicationPurpose, "fact_only", false, false},
		{"unrelated_legacy_delivery", "care", factOnlyReplyPublicationPurpose, "legacy", true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, repo, app, owner, self, conversation := durableReplyFixture(t, "purpose-"+test.name)
			service := NewToolPublicationService(app)
			command := ConversationReplyPublication{AuthorizationActorID: owner, FluctlightID: self, ConversationID: conversation, OperationID: "old-publication", Text: "先前已经发送的正文", ReplyMetadata: map[string]any{"reply_mode": test.mode}}
			var first publishedResource
			if err := withTransaction(ctx, repo.Pool(), func(tx pgx.Tx) error {
				var err error
				first, err = service.PublishConversationReplyTx(ctx, tx, command)
				return err
			}); err != nil {
				t.Fatal(err)
			}
			if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.conversation_proactive_deliveries(message_id,fluctlight_id,conversation_id,topic_key,purpose,inbound_sequence,occurred_at) VALUES($1,$2,$3,'check-in',$4,0,$5)`, first.ID, self, conversation, test.oldPurpose, app.now().UTC()); err != nil {
				t.Fatal(err)
			}
			command.OperationID = "new-publication"
			command.Text = "模型新生成的另一段正文，不能绕过去重"
			command.TopicKey = "check-in"
			command.Purpose = test.nextPurpose
			command.FactOnlyTopicSuppression = test.bridge
			if err := withTransaction(ctx, repo.Pool(), func(tx pgx.Tx) error {
				id, err := recentProactiveReplyTx(ctx, tx, command, 0, app.now().UTC().Add(-time.Hour))
				if test.want {
					if err != nil || id != first.ID {
						t.Fatalf("cross-version delivery missed: %q/%v", id, err)
					}
					command.SuppressRecentDuplicate = true
					second, err := service.PublishConversationReplyTx(ctx, tx, command)
					if err != nil || second.ID != first.ID || !second.DuplicateSuppressed {
						t.Fatalf("different generated prose published twice: %+v/%v", second, err)
					}
				} else if !errors.Is(err, pgx.ErrNoRows) {
					t.Fatalf("unrelated legacy behavior widened: %q/%v", id, err)
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			var count int
			if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant'`, conversation).Scan(&count); err != nil || count != 1 {
				t.Fatalf("publication count=%d/%v", count, err)
			}
		})
	}
}
