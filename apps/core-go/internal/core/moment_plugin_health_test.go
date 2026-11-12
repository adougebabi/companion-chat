package core

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"testing"
	"time"
)

func TestMomentPluginHealthChecksRegistrationAndOwnerDataWithoutWriting(t *testing.T) {
	ctx, repo, app, owner, _, _ := setupDirectPublicationToolTest(t, "moment-health")
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES($1,'fixture','fixture')`, owner); err != nil {
		t.Fatal(err)
	}
	counts := func() [3]int {
		t.Helper()
		var values [3]int
		if err := repo.Pool().QueryRow(ctx, `SELECT (SELECT count(*) FROM public.moments), (SELECT count(*) FROM public.tool_executions), (SELECT count(*) FROM public.platform_outbox_events)`).Scan(&values[0], &values[1], &values[2]); err != nil {
			t.Fatal(err)
		}
		return values
	}
	before := counts()
	path := "/internal/settings/moment/test-health"
	if !MatchOwnerSurface("POST", path) {
		t.Fatal("Moment health route is not registered")
	}
	result, err := app.DispatchOwnerSurface(ctx, "POST", path, url.Values{}, owner, nil)
	if err != nil || result["state"] != "ready" || result["plugin_enabled"] != true || result["registry_checked"] != true || result["data_read_tested"] != true || result["publication_tested"] != false || result["model_tested"] != false || intValue(result["config_version"]) != 0 {
		t.Fatalf("default Moment health = %#v %v", result, err)
	}
	if after := counts(); after != before {
		t.Fatalf("read-only health check changed business state: before=%v after=%v", before, after)
	}
	if _, err := app.DispatchOwnerSurface(ctx, "POST", path, url.Values{}, "foreign-owner", nil); err == nil {
		t.Fatal("foreign owner accessed plugin health")
	}
	if _, err := repo.Pool().Exec(ctx, `ALTER TABLE public.moment_comments RENAME TO moment_comments_health_fixture`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = repo.Pool().Exec(context.Background(), `ALTER TABLE IF EXISTS public.moment_comments_health_fixture RENAME TO moment_comments`)
	})
	unreadable, err := app.TestMomentPluginHealth(ctx, owner)
	if err != nil || unreadable["state"] != "unavailable" || unreadable["error_code"] != "moment_data_read_failed" || unreadable["data_read_tested"] != false {
		t.Fatalf("unreadable feed was presented as healthy: %#v %v", unreadable, err)
	}
	if _, err := repo.Pool().Exec(ctx, `ALTER TABLE public.moment_comments_health_fixture RENAME TO moment_comments`); err != nil {
		t.Fatal(err)
	}
	if _, err := app.UpdateSettings(ctx, owner, map[string]any{"values": map[string]any{domainPluginsSettingKey: map[string]any{"image": false, "moment": false}}, "expected_versions": map[string]any{domainPluginsSettingKey: 0}, "idempotency_key": "moment-health-disabled"}); err != nil {
		t.Fatal(err)
	}
	before = counts()
	disabled, err := app.TestMomentPluginHealth(ctx, owner)
	if err != nil || disabled["state"] != "ready" || disabled["plugin_enabled"] != false || intValue(disabled["config_version"]) != 1 {
		t.Fatalf("disabled Moment health = %#v %v", disabled, err)
	}
	if after := counts(); after != before {
		t.Fatalf("disabled health check changed business state: before=%v after=%v", before, after)
	}
	empty, err := NewCapabilityRegistry()
	if err != nil {
		t.Fatal(err)
	}
	app.Capabilities = empty
	missing, err := app.TestMomentPluginHealth(ctx, owner)
	if err != nil || missing["state"] != "unavailable" || missing["error_code"] != "moment_capability_unavailable" || missing["data_read_tested"] != false {
		t.Fatalf("missing registry was presented as healthy: %#v %v", missing, err)
	}
}

func TestMomentPluginHealthRejectsStaleSuccessAndFailure(t *testing.T) {
	for _, failure := range []bool{false, true} {
		t.Run(fmt.Sprintf("old_probe_failed_%t", failure), func(t *testing.T) {
			ctx, repo, app, owner, _, _ := setupDirectPublicationToolTest(t, "moment-health-stale")
			if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES($1,'fixture','fixture')`, owner); err != nil {
				t.Fatal(err)
			}
			started, release := make(chan struct{}), make(chan struct{})
			var once sync.Once
			unblock := func() { once.Do(func() { close(release) }) }
			defer unblock()
			type outcome struct {
				value map[string]any
				err   error
			}
			done := make(chan outcome, 1)
			go func() {
				value, err := app.testMomentPluginHealth(ctx, owner, func(context.Context) momentPluginProbe {
					close(started)
					<-release
					if failure {
						return momentPluginProbe{registryChecked: true, errorCode: "moment_data_read_failed"}
					}
					return momentPluginProbe{registryChecked: true, dataReadTested: true}
				})
				done <- outcome{value, err}
			}()
			select {
			case <-started:
			case <-time.After(5 * time.Second):
				t.Fatal("probe did not start")
			}
			if _, err := app.UpdateSettings(ctx, owner, map[string]any{"values": map[string]any{domainPluginsSettingKey: map[string]any{"image": true, "moment": false}}, "expected_versions": map[string]any{domainPluginsSettingKey: 0}, "idempotency_key": "moment-health-change"}); err != nil {
				t.Fatal(err)
			}
			unblock()
			select {
			case got := <-done:
				if got.err != nil || got.value["state"] != "configuration_changed" || intValue(got.value["tested_config_version"]) != 0 || intValue(got.value["config_version"]) != 1 || got.value["publication_tested"] != false || got.value["model_tested"] != false {
					t.Fatalf("stale result = %#v %v", got.value, got.err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("probe did not finish")
			}
		})
	}
}
