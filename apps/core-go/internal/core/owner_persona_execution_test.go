package core

import (
	"errors"
	"net/url"
	"strings"
	"testing"
)

func TestOwnerPersonaAssessmentTerminalsAndRecovery(t *testing.T) {
	for _, scenario := range []string{"keep", "workflow_failure", "late_workflow_failure", "failure", "recover", "foundation_cas", "runtime_cas", "wrong_target", "wrong_rule", "disabled_rule", "retired"} {
		t.Run(scenario, func(t *testing.T) {
			ctx, repository := isolatedCoreTestRepository(t)
			owner, fid := "owner-request-test", "owner-request-fluctlight"
			takeoverChainSeed(t, ctx, repository, owner, fid, "owner-request-chat")
			calls := 0
			var app *App
			var requestID string
			router := newFakeProviderRouter().on(persistentSwitchAssessmentSchemaName, func(payload map[string]any) fakeProviderResult {
				calls++
				if !strings.Contains(jsonString(payload), "owner.persona_switch.requested") {
					t.Error("missing formal Owner source fact")
				}
				if scenario == "failure" || (scenario == "recover" && calls == 1) {
					return fakeProviderResult{Err: errors.New("controlled Provider outage secret-not-public")}
				}
				decision := map[string]any{"decision": "switch", "from_profile_id": "spark", "target_profile_id": "twilight", "trigger_id": "switch:safety", "reason": "controlled assessment"}
				switch scenario {
				case "late_workflow_failure":
					if _, err := app.FailOwnerPersonaSwitchIntent(ctx, requestID); err != nil {
						t.Fatal(err)
					}
				case "keep":
					decision = map[string]any{"decision": "keep"}
				case "foundation_cas":
					if _, err := repository.Pool().Exec(ctx, `UPDATE public.fluctlights SET current_revision=current_revision+1 WHERE id=$1`, fid); err != nil {
						t.Fatal(err)
					}
				case "runtime_cas":
					if _, err := repository.Pool().Exec(ctx, `UPDATE public.fluctlight_personality_runtime SET revision=revision+1 WHERE fluctlight_id=$1`, fid); err != nil {
						t.Fatal(err)
					}
				case "wrong_target":
					decision["target_profile_id"] = "spark"
				case "wrong_rule":
					decision["trigger_id"] = "switch:missing"
				case "retired":
					if _, err := repository.Pool().Exec(ctx, `UPDATE public.fluctlights SET status='retired' WHERE id=$1`, fid); err != nil {
						t.Fatal(err)
					}
				}
				return fakeProviderResult{Structured: map[string]any{"personality_decision": decision}}
			})
			app = newTestApp(t, repository, router)
			runtime, err := app.readPersonalityRuntime(ctx, fid, "spark")
			if err != nil {
				t.Fatal(err)
			}
			if scenario == "disabled_rule" {
				if _, err := repository.Pool().Exec(ctx, `UPDATE public.fluctlights SET core_persona=jsonb_set(core_persona,'{personality_system,switching,rules,0,enabled}','false') WHERE id=$1`, fid); err != nil {
					t.Fatal(err)
				}
			}
			accepted, err := app.DispatchOwnerSurface(ctx, "POST", "/internal/fluctlights/"+fid+"/persona-switch", url.Values{}, owner, map[string]any{"idempotency_key": "assessment-" + scenario, "target_persona_id": "twilight", "expected_revision": 0, "expected_runtime_revision": intValue(runtime["revision"]), "confirm": true, "reason": "Owner explicitly asks; unrelated conditions must still be evaluated"})
			if scenario == "disabled_rule" {
				var failure *SurfaceError
				if !errors.As(err, &failure) || failure.Code != "personality_trigger_not_found" {
					t.Fatalf("disabled admission=%v err=%v", accepted, err)
				}
				return
			}
			if err != nil || accepted["status"] != "pending" {
				t.Fatalf("admission=%v err=%v", accepted, err)
			}
			id := stringValue(accepted["request_id"])
			requestID = id
			var terminal map[string]any
			if scenario == "workflow_failure" {
				terminal, err = app.FailOwnerPersonaSwitchIntent(ctx, id)
			} else {
				terminal, err = app.ProcessOwnerPersonaSwitchIntent(ctx, id)
			}
			if scenario == "failure" || scenario == "recover" {
				// A recreated service consumes the same PostgreSQL identity and resumes
				// Temporal's retry without losing the pending request or duplicating effects.
				for attempt := 1; err != nil && attempt < 3; attempt++ {
					app = newTestApp(t, repository, router)
					terminal, err = app.ProcessOwnerPersonaSwitchIntent(ctx, id)
				}
			}
			want := "rejected"
			if scenario == "failure" || scenario == "workflow_failure" || scenario == "late_workflow_failure" {
				want = "failed"
			} else if scenario == "recover" {
				want = "completed"
			}
			if err != nil || terminal["status"] != want {
				t.Fatalf("terminal=%v err=%v calls=%d", terminal, err, calls)
			}
			stored, err := app.DispatchOwnerSurface(ctx, "GET", "/internal/owner-requests/"+id, nil, owner, nil)
			if err != nil || stored["status"] != want || strings.Contains(jsonString(stored), "secret-not-public") {
				t.Fatalf("public read=%v err=%v", stored, err)
			}
			list, err := app.DispatchOwnerSurface(ctx, "GET", "/internal/fluctlights/"+fid+"/owner-requests", url.Values{}, owner, nil)
			if err != nil || len(arrayValue(list["items"])) != 1 {
				t.Fatalf("list=%v err=%v", list, err)
			}
			history, _, err := app.readPersonaSwitchHistory(ctx, fid)
			expectedEffects := 0
			if want == "completed" {
				expectedEffects = 1
			}
			if err != nil || len(history) != expectedEffects {
				t.Fatalf("effects=%v err=%v", history, err)
			}
			before := router.totalRequests()
			again, err := app.ProcessOwnerPersonaSwitchIntent(ctx, id)
			if err != nil || again["status"] != want || router.totalRequests() != before {
				t.Fatalf("terminal replay=%v err=%v", again, err)
			}
			if _, err := app.DispatchOwnerSurface(ctx, "GET", "/internal/owner-requests/"+id, nil, "foreign-owner", nil); err == nil {
				t.Fatal("foreign Owner read request")
			}
		})
	}
}
