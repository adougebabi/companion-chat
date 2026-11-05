package migrations

import (
	"encoding/json"
	"testing"
)

func TestKevTimeoutUpgradePreservesOptionsAndLaterOwnerEdits(t *testing.T) {
	ctx, pool := isolatedMigrationPool(t)
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM public.runtime_settings WHERE key='kev'`).Scan(&count); err != nil || count != 0 {
		t.Fatal("fresh migration created a Kev setting", count, err)
	}
	for _, item := range []struct {
		name, previousHead                        string
		timeout, budget, wantBudget, wantRevision int
	}{
		{"old defaults", KevOneMinutePreviousHead, 1500, 3000, 60000, 8},
		{"longer stage", KevOneMinutePreviousHead, 10000, 90000, 90000, 8},
		{"already configured", KevOneMinutePreviousHead, 60000, 70000, 70000, 7},
		{"upgrade through old timeout", KevTimeoutPreviousHead, 1500, 3000, 60000, 9},
	} {
		t.Run(item.name, func(t *testing.T) {
			before := map[string]any{"enabled": false, "media_enabled": true, "endpoint": "http://kev.example/v1/systemone", "timeout_ms": item.timeout, "budget_ms": item.budget, "strategy": "choice_guarded", "points": map[string]bool{"tools.select": false}, "model": "preserve-model"}
			raw, _ := json.Marshal(before)
			if _, err := pool.Exec(ctx, `INSERT INTO public.runtime_settings(key,value_json,revision) VALUES('kev',$1,7) ON CONFLICT(key) DO UPDATE SET value_json=excluded.value_json,revision=7`, string(raw)); err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(ctx, `UPDATE public.alembic_version SET version_num=$1`, item.previousHead); err != nil {
				t.Fatal(err)
			}
			if err := New(pool).Apply(ctx); err != nil {
				t.Fatal(err)
			}
			var saved string
			var revision int
			if err := pool.QueryRow(ctx, `SELECT value_json,revision FROM public.runtime_settings WHERE key='kev'`).Scan(&saved, &revision); err != nil {
				t.Fatal(err)
			}
			var got map[string]any
			if err := json.Unmarshal([]byte(saved), &got); err != nil {
				t.Fatal(err)
			}
			before["timeout_ms"], before["budget_ms"] = 60000, item.wantBudget
			want, _ := json.Marshal(before)
			actual, _ := json.Marshal(got)
			if string(want) != string(actual) {
				t.Fatalf("other Kev options changed: got=%s want=%s", actual, want)
			}
			if revision != item.wantRevision {
				t.Fatal("configuration revision not fenced", revision, item.wantRevision)
			}
			if _, err := pool.Exec(ctx, `UPDATE public.runtime_settings SET value_json=jsonb_set(value_json::jsonb,'{timeout_ms}','5000'::jsonb)::text,revision=revision+1 WHERE key='kev'`); err != nil {
				t.Fatal(err)
			}
			if err := New(pool).Apply(ctx); err != nil {
				t.Fatal(err)
			}
			var timeout int
			if err := pool.QueryRow(ctx, `SELECT (value_json::jsonb->>'timeout_ms')::integer FROM public.runtime_settings WHERE key='kev'`).Scan(&timeout); err != nil || timeout != 5000 {
				t.Fatal("migration reapplied over a later Owner edit", timeout, err)
			}
		})
	}
}
