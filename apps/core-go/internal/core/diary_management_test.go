package core

import (
	"context"
	"errors"
	"net/url"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// Empty-material handling, daily identity and annotations use the real DB
// authority without substituting a model result or creating diary prose.
func TestDiaryClosedDayNoMaterialRetryIdentityAndIndependentAnnotationCAS(t *testing.T) {
	ctx, repo, app, owner, fid, _ := setupDirectPublicationToolTest(t, "diary-empty")
	app.Provider = nil
	app.Clock = func() time.Time { return time.Date(2026, 10, 16, 2, 0, 0, 0, time.UTC) }
	command := func(body map[string]any) (map[string]any, error) {
		var result map[string]any
		err := newPostgresSurface(repo.Pool()).Transact(ctx, func(txCtx context.Context, _ surfaceTx) error {
			var err error
			result, err = app.RequestOwnerDiary(txCtx, owner, fid, body)
			return err
		})
		return result, err
	}
	accepted, err := command(map[string]any{"local_date": "2020-01-01", "timezone": "Asia/Shanghai", "reason": "closed day test", "idempotency_key": "diary-empty-first"})
	if err != nil {
		t.Fatal(err)
	}
	id := stringValue(accepted["diary_id"])
	var attempt string
	if err := repo.Pool().QueryRow(ctx, `SELECT id FROM public.fluctlight_diary_attempts WHERE diary_id=$1`, id).Scan(&attempt); err != nil {
		t.Fatal(err)
	}
	result, err := app.ProcessDiaryGeneration(ctx, attempt)
	if err != nil || result["status"] != "no_content" {
		t.Fatalf("empty material %#v %v", result, err)
	}
	again, err := command(map[string]any{"local_date": "2020-01-01", "timezone": "Asia/Shanghai", "reason": "same day test", "idempotency_key": "diary-empty-second"})
	if err != nil || again["diary_id"] != id || again["status"] != "no_content" {
		t.Fatalf("daily replay %#v %v", again, err)
	}
	if _, err := app.ProcessDiaryGeneration(ctx, attempt); err != nil {
		t.Fatal(err)
	}
	read, err := app.OwnerDiary(ctx, owner, fid, id)
	if err != nil || read["body"] != nil && stringValue(read["body"]) != "" || read["source"] != "agent" {
		t.Fatalf("diary read %#v %v", read, err)
	}
	annotate := func(expected int) (map[string]any, error) {
		var result map[string]any
		err := newPostgresSurface(repo.Pool()).Transact(ctx, func(txCtx context.Context, _ surfaceTx) error {
			var err error
			result, err = app.AnnotateOwnerDiary(txCtx, owner, fid, id, map[string]any{"expected_revision": expected, "favorite": true, "note": "This is an Owner note, never the character body", "idempotency_key": "annotation-test"})
			return err
		})
		return result, err
	}
	note, err := annotate(0)
	if err != nil || intValue(note["revision"]) != 1 {
		t.Fatalf("note %#v %v", note, err)
	}
	if _, err := annotate(0); err == nil {
		t.Fatal("stale annotation accepted")
	} else {
		var conflict *SurfaceError
		if !errors.As(err, &conflict) || conflict.Status != 409 {
			t.Fatal(err)
		}
	}
	page, err := app.OwnerDiaryList(ctx, owner, fid, url.Values{"favorite": {"true"}})
	if err != nil || len(arrayValue(page["items"])) != 1 {
		t.Fatalf("favorite page %#v %v", page, err)
	}
	if _, err := command(map[string]any{"local_date": "2026-10-16", "timezone": "Asia/Shanghai", "reason": "future material rejected", "idempotency_key": "open-day"}); err == nil {
		t.Fatal("unfinished local day accepted")
	}
	var count int
	if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.fluctlight_diary_attempts WHERE diary_id=$1`, id).Scan(&count); err != nil || count != 1 {
		t.Fatalf("duplicate attempts %d %v", count, err)
	}
	if err := withTransaction(ctx, repo.Pool(), func(tx pgx.Tx) error { return enqueueClosedDayDiaryTx(ctx, tx, fid, "2026-10-16", "America/New_York") }); err != nil {
		t.Fatal(err)
	}
	var due time.Time
	if err := repo.Pool().QueryRow(ctx, `SELECT next_attempt_at FROM public.platform_workflow_intents WHERE intent_type='diary.generate' AND payload->>'local_date'='2026-10-16' AND payload->>'timezone'='America/New_York'`).Scan(&due); err != nil {
		t.Fatal(err)
	}
	if !due.Equal(time.Date(2026, 10, 17, 4, 0, 0, 0, time.UTC)) {
		t.Fatalf("local midnight due %s", due)
	}
}
