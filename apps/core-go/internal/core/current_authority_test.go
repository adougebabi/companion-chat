package core

import (
	"errors"
	"testing"
)

func TestCurrentFactsRevisionRejectsStaleBodyWardrobeAndProfile(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID := "facts-owner", "facts-fluctlight"
	seedLifeContextFluctlight(t, ctx, repository, ownerID, fluctlightID)
	for _, statement := range []string{
		`INSERT INTO public.fluctlight_appearance_states(fluctlight_id,revision,state_json,source_kind) VALUES($1,0,'{}','initialization')`,
		`INSERT INTO public.fluctlight_wardrobe_states(fluctlight_id,revision) VALUES($1,0)`,
		`INSERT INTO public.fluctlight_personality_runtime(fluctlight_id,active_profile_id,revision) VALUES($1,'default',0)`,
		`INSERT INTO public.fluctlight_profile_habits(fluctlight_id,profile_id,revision) VALUES($1,'default',0) ON CONFLICT DO NOTHING`,
	} {
		if _, err := repository.Pool().Exec(ctx, statement, fluctlightID); err != nil {
			t.Fatal(err)
		}
	}
	app := &App{DB: repository}
	previous, err := app.readCurrentFactsRevision(ctx, fluctlightID)
	if err != nil || previous == "" {
		t.Fatalf("initial facts revision=%q err=%v", previous, err)
	}
	for _, mutation := range []struct {
		name string
		sql  string
	}{
		{"body", `UPDATE public.fluctlight_appearance_states SET revision=revision+1,state_json='{"hair_length":{"status":"known","value":"短发"}}' WHERE fluctlight_id=$1`},
		{"wardrobe", `UPDATE public.fluctlight_wardrobe_states SET revision=revision+1,wearing_state='known' WHERE fluctlight_id=$1`},
		{"profile", `UPDATE public.fluctlight_personality_runtime SET active_profile_id='alternate',revision=revision+1 WHERE fluctlight_id=$1`},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			if _, err := repository.Pool().Exec(ctx, mutation.sql, fluctlightID); err != nil {
				t.Fatal(err)
			}
			tx, err := repository.Pool().Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if err := app.requireCurrentFactsRevisionTx(ctx, tx, fluctlightID, previous); !errors.Is(err, ErrCurrentFactsStale) {
				_ = tx.Rollback(ctx)
				t.Fatalf("stale %s revision accepted: %v", mutation.name, err)
			}
			_ = tx.Rollback(ctx)
			current, err := app.readCurrentFactsRevision(ctx, fluctlightID)
			if err != nil || current == previous {
				t.Fatalf("%s did not change facts revision: before=%q after=%q err=%v", mutation.name, previous, current, err)
			}
			previous = current
		})
	}
	tx, err := repository.Pool().Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := app.requireCurrentFactsRevisionTx(ctx, tx, fluctlightID, previous); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("fresh facts revision rejected: %v", err)
	}
	_ = tx.Rollback(ctx)
}

func TestCurrentFactsRevisionTracksNewClaimsAndConversationMessages(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "facts-history-owner", "facts-history-fluctlight", "facts-history-conversation"
	seedTurnConversation(t, ctx, repository, ownerID, fluctlightID, conversationID)
	app := &App{DB: repository}
	previous, err := app.readCurrentFactsRevision(ctx, fluctlightID)
	if err != nil {
		t.Fatal(err)
	}
	for _, mutation := range []struct {
		name string
		sql  string
		args []any
	}{
		{"claim", `INSERT INTO public.cognition_claims(id,fluctlight_id,source_fact_id,claim_type,content,evidence_refs,confidence,repetition_key,status) VALUES('facts-history-claim',$1,'facts-history-source','observed_fact','用户纠正了事实','["facts-history-source"]',0.9,'facts-history-claim','active')`, []any{fluctlightID}},
		{"message", `INSERT INTO public.conversation_messages(id,conversation_id,sequence,author_actor_id,kind,text,attachment_refs,idempotency_key) VALUES('facts-history-message',$1,1,$2,'user','新消息','[]','facts-history-message')`, []any{conversationID, ownerID}},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			if _, err := repository.Pool().Exec(ctx, mutation.sql, mutation.args...); err != nil {
				t.Fatal(err)
			}
			current, err := app.readCurrentFactsRevision(ctx, fluctlightID)
			if err != nil || current == previous {
				t.Fatalf("%s did not advance current facts generation: before=%q after=%q err=%v", mutation.name, previous, current, err)
			}
			previous = current
		})
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.conversation_messages(id,conversation_id,sequence,author_actor_id,kind,text,attachment_refs,idempotency_key) VALUES('facts-history-assistant',$1,2,$2,'assistant','尚未结算的输出','[]','facts-history-assistant')`, conversationID, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if current, err := app.readCurrentFactsRevision(ctx, fluctlightID); err != nil || current != previous {
		t.Fatalf("pre-settlement assistant output advanced current facts: before=%q after=%q err=%v", previous, current, err)
	}
	if _, err := repository.Pool().Exec(ctx, `UPDATE public.conversation_messages SET text='已修订的助手输出' WHERE id='facts-history-assistant'`); err != nil {
		t.Fatal(err)
	}
	if current, err := app.readCurrentFactsRevision(ctx, fluctlightID); err != nil || current == previous {
		t.Fatalf("assistant source revision did not advance current facts: before=%q after=%q err=%v", previous, current, err)
	}
}
