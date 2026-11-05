package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/ai/decision"
)

func TestInitialAutomaticWakeUpConsultsKevBeforeMainModel(t *testing.T) {
	for _, outcome := range []string{"no", "no-after-limit", "unclear", "unavailable", "yes", "disabled", "manual-settings-error"} {
		t.Run(outcome, func(t *testing.T) {
			ctx, repository := isolatedCoreTestRepository(t)
			owner, character := "kev-wakeup-owner", "kev-wakeup-character"
			seedLifeContextFluctlight(t, ctx, repository, owner, character)
			base := &App{DB: repository}
			life := currentLifeForTest(t, ctx, base, character, time.Now().UTC())
			if _, err := base.AcceptSchedule(ctx, owner, character, fullDaySchedulePayloadForTest(time.Now().UTC(), "kev-wakeup-schedule", stringValue(life["context_revision"]))); err != nil {
				t.Fatal(err)
			}
			seedCognitiveProviderRole(t, ctx, repository, "kev-wakeup-provider")
			calls := []string{}
			router := newFakeProviderRouter().on("wake_up_response", func(payload map[string]any) fakeProviderResult {
				calls = append(calls, "main")
				return fakeProviderResult{Structured: map[string]any{"action_type": "no_op", "response_intent": "暂不打扰", "evidence_refs": []any{}, "influences": []any{}}}
			})
			app := newTestApp(t, repository, router)
			if _, err := app.EnsureDirectConversation(ctx, owner, character); err != nil {
				t.Fatal(err)
			}
			if _, err := app.EnsureWakeUpIntents(ctx); err != nil {
				t.Fatal(err)
			}
			if _, err := repository.Pool().Exec(ctx, `UPDATE public.platform_workflow_intents SET status='started',payload=jsonb_set(payload,'{cycle}','0'::jsonb),started_at=now() WHERE intent_id=$1`, "wake_up_intent:"+character); err != nil {
				t.Fatal(err)
			}
			if outcome == "manual-settings-error" {
				if _, err := repository.Pool().Exec(ctx, `UPDATE public.platform_workflow_intents SET payload=jsonb_set(payload,'{kev_manual_cycle}','0'::jsonb) WHERE intent_id=$1`, "wake_up_intent:"+character); err != nil {
					t.Fatal(err)
				}
			}
			config := decision.DefaultConfig()
			config.Enabled = outcome != "disabled"
			for point := range config.Points {
				config.Points[point] = point == "runtime.wakeup"
			}
			choice := outcome
			if choice == "disabled" {
				choice = "yes"
			}
			if choice == "no-after-limit" {
				choice = "no"
				wakeID := "wake_up_" + stableDigest(character+":"+jsonString(0))
				if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.kev_deferrals(decision_point,actor_self,candidate_id,deferral_count,deferred_until) VALUES('runtime.wakeup',$1,$2,$3,now()-interval '1 minute')`, character, wakeID, config.MaxDeferrals); err != nil {
					t.Fatal(err)
				}
			}
			transport := kevContextTestTransport(func(request *http.Request) (*http.Response, error) {
				calls = append(calls, "kev")
				var packet decision.Request
				if err := json.NewDecoder(request.Body).Decode(&packet); err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(packet.State, `"cycle":0`) || !strings.Contains(packet.State, `"trigger":"periodic_check"`) {
					t.Errorf("Kev did not receive the first wake-up: %s", packet.State)
				}
				if outcome == "unavailable" {
					return nil, errors.New("controlled Kev outage")
				}
				p := map[string]float64{"yes": 0.1, "no": 0.1, "unclear": 0.1}
				p[choice] = 0.8
				body := jsonString(map[string]any{"answers": map[string]any{"q_0001": map[string]any{"type": "choice", "choice": choice, "confidence": 0.8, "probabilities": p}}})
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
			})
			ids := 0
			app.Kev = &decision.Service{Settings: &kevFixedSettings{config}, Store: &kevMemoryStore{}, HTTP: &http.Client{Transport: transport}, ID: func() string { ids++; return fmt.Sprint(ids) }}
			if outcome == "manual-settings-error" {
				app.Kev.Settings = wakeGateUnavailableSettings{}
			}
			wakeCtx := WithWakeUpCycle(WithLifecycleIntentID(ctx, "wake_up_intent:"+character), 0)
			result, err := app.ProcessWakeUp(wakeCtx, character, 0)
			if err != nil {
				t.Fatal(err)
			}
			if outcome == "no" || outcome == "no-after-limit" || outcome == "unclear" || outcome == "unavailable" {
				if stringValue(result["status"]) != "deferred" || strings.Join(calls, ",") != "kev" {
					t.Fatalf("Kev no or unavailable decision must defer without main model: result=%v calls=%v", result, calls)
				}
			} else if outcome == "yes" {
				if len(calls) < 2 || calls[0] != "kev" || calls[1] != "main" {
					t.Fatalf("Kev must precede main model: result=%v calls=%v", result, calls)
				}
			} else if len(calls) == 0 || calls[0] != "main" {
				t.Fatalf("disabled Kev should use the normal path: result=%v calls=%v", result, calls)
			}
		})
	}
}
