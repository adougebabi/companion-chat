package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/ai/decision"
)

type mediaCase struct {
	ID         string         `json:"id"`
	Intent     string         `json:"intent"`
	Paraphrase string         `json:"paraphrase"`
	Plan       map[string]any `json:"plan"`
	Error      string         `json:"error"`
}

func mediaCases(t *testing.T) []mediaCase {
	t.Helper()
	raw, err := os.ReadFile("testdata/media_capture_cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []mediaCase
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	return cases
}
func mediaTestPlan(subject, human, self, scope string) map[string]any {
	return map[string]any{"primary_subject": subject, "human_presence": human, "self_in_frame": self, "visible_scope": scope, "capture_method": "unspecified", "viewpoint": "unspecified", "framing": "unspecified", "camera_angle": "unspecified"}
}
func mediaTestContext(marker string) CapabilityContext {
	return CapabilityContext{Visual: &VisualIdentityContext{Data: map[string]any{"status": "active", "identity_snapshot": map[string]any{"identity": map[string]any{"name": marker + "_NAME", "face_shape": marker + "_FACE"}}, "canonical_asset_id": marker + "_REF"}}, Life: &CurrentLifeContext{Data: map[string]any{"scene": "测试房间", "context_revision": "life_1"}}, Outfit: &AppearanceContext{Data: map[string]any{"body_revision": 1, "wardrobe_revision": 2, "body_fields": map[string]any{"hair_color": map[string]any{"status": "known", "value": marker + "_HAIR"}, "skin_tone": map[string]any{"status": "known", "value": marker + "_SKIN"}, "chest_cup": map[string]any{"status": "known", "value": marker + "_BODY"}}, "worn_items": []any{map[string]any{"slot": "top", "description": marker + "_SHIRT"}, map[string]any{"slot": "shoes", "description": marker + "_BOOTS"}}}}, State: &CurrentStateContext{Data: map[string]any{"mood": marker + "_PERSONA"}}}
}
func mediaPreparedConcept(t *testing.T, intent string, plan map[string]any, resolved CapabilityContext) map[string]any {
	t.Helper()
	call := CapabilityInvocation{CallID: "media-test", CapabilityName: "media.image.generate", SourceFactID: "test-fact", ProviderRequestID: "test-provider", Arguments: jsonBytes(map[string]any{"intent": intent, "media_plan": plan})}
	prepared, err := (imageGenerateCapability{}).Prepare(context.Background(), call, resolved)
	if err != nil {
		t.Fatal(err)
	}
	raw, _, err := capabilityPreparedData(prepared, "media_concept")
	if err != nil {
		t.Fatal(err)
	}
	return mapValue(raw)
}
func mediaTestOutput(concept map[string]any) map[string]any {
	plan := mapValue(concept["media_capture_plan"])
	ids := []any{}
	if plan["self_in_frame"] == "yes" {
		ids = append(ids, "self")
	}
	return map[string]any{"declared_human_count": concept["human_count"], "prompt": stringValue(concept["intent"]), "declared_human_presence": plan["human_presence"], "self_in_frame": plan["self_in_frame"], "capture_method": plan["capture_method"], "used_reference_ids": []any{}, "used_subject_ids": ids}
}

func TestMediaBranchSceneMatrixM01ToM29(t *testing.T) {
	for _, fixture := range mediaCases(t) {
		t.Run(fixture.ID, func(t *testing.T) {
			for _, intent := range []string{fixture.Intent, fixture.Paraphrase} {
				plan, sources, err := ResolveMediaCapturePlan(context.Background(), nil, decision.Scope{}, intent, nil, func(_ context.Context, input map[string]any) (map[string]any, error) {
					if input["intent_text"] != intent {
						t.Fatal("raw intent changed")
					}
					return cloneMap(fixture.Plan), nil
				})
				if fixture.Error != "" {
					if err == nil || err.Error() != fixture.Error {
						t.Fatalf("got %v", err)
					}
					continue
				}
				if err != nil {
					t.Fatal(err)
				}
				if len(sources) != 8 {
					t.Fatalf("sources=%v", sources)
				}
				concept := mediaPreparedConcept(t, intent, plan, mediaTestContext("CURRENT"))
				packet, err := mediaNormalizationInput(concept)
				if err != nil {
					t.Fatal(err)
				}
				wire := jsonString(packet)
				if !mediaPlanNeedsAppearance(plan) && strings.Contains(wire, "CURRENT") {
					t.Fatalf("nonself pollution: %s", wire)
				}
				if !mediaPlanHasSelfIdentity(plan) {
					for _, sentinel := range []string{"CURRENT_FACE", "CURRENT_REF", "CURRENT_BODY"} {
						if strings.Contains(wire, sentinel) {
							t.Fatalf("forbidden %s: %s", sentinel, wire)
						}
					}
				}
				if plan["visible_scope"] == "feet" && (!strings.Contains(wire, "CURRENT_BOOTS") || strings.Contains(wire, "CURRENT_SHIRT")) {
					t.Fatalf("feet projection: %s", wire)
				}
				if plan["visible_scope"] == "hands" && strings.Contains(wire, "CURRENT_BOOTS") {
					t.Fatalf("hands projection: %s", wire)
				}
				if fixture.ID == "M13" && strings.Contains(wire, "CURRENT_BOOTS") {
					t.Fatal("half-body request retained shoes")
				}
				if strings.Contains(wire, "CURRENT_PERSONA") {
					t.Fatal("inner state leaked")
				}
				calls := 0
				_, repairs, err := normalizeMediaPrompt(context.Background(), concept, func(context.Context, map[string]any) (map[string]any, error) {
					calls++
					return mediaTestOutput(concept), nil
				})
				if err != nil || repairs != 0 || calls != 1 {
					t.Fatalf("normalize calls=%d repairs=%d err=%v", calls, repairs, err)
				}
			}
		})
	}
}

func TestMediaBranchM30M31SequenceAndActorIsolation(t *testing.T) {
	full := mediaTestPlan("person", "present", "yes", "full")
	none := mediaTestPlan("animal", "none", "no", "unknown")
	shared := mediaTestContext("A")
	before := jsonString(shared.Snapshot())
	for _, plan := range []map[string]any{full, none, mediaTestPlan("landscape", "none", "no", "unknown")} {
		concept := mediaPreparedConcept(t, "原始意图", plan, shared)
		wire := compactMediaConceptForProvider(jsonString(concept))
		if plan["human_presence"] == "none" && strings.Contains(wire, "A_FACE") {
			t.Fatal("prior identity reused")
		}
	}
	if before != jsonString(shared.Snapshot()) {
		t.Fatal("shared snapshot mutated")
	}
	var wg sync.WaitGroup
	for _, actor := range []string{"A", "B"} {
		wg.Add(1)
		go func(actor string) {
			defer wg.Done()
			plan := full
			if actor == "B" {
				plan = none
			}
			concept := mediaPreparedConcept(t, actor, plan, mediaTestContext(actor))
			wire := compactMediaConceptForProvider(jsonString(concept))
			if strings.Contains(wire, "A_FACE") && actor == "B" {
				t.Error("cross actor identity")
			}
		}(actor)
	}
	wg.Wait()
}

func TestMediaBranchM36RepairIsBoundedAndCancelled(t *testing.T) {
	concept := mediaPreparedConcept(t, "无人的街道", mediaTestPlan("landscape", "none", "no", "unknown"), mediaTestContext("FACE"))
	for _, succeed := range []bool{false, true} {
		calls := 0
		_, repairs, err := normalizeMediaPrompt(context.Background(), concept, func(_ context.Context, packet map[string]any) (map[string]any, error) {
			calls++
			if calls == 2 && packet["validation_error"] == nil {
				t.Fatal("missing repair reason")
			}
			out := mediaTestOutput(concept)
			if calls == 1 || !succeed {
				out["prompt"] = "A handheld phone self-capture; the phone camera defines the viewpoint."
			}
			return out, nil
		})
		if calls != 2 || repairs != 1 || (succeed != (err == nil)) {
			t.Fatalf("calls=%d repairs=%d err=%v", calls, repairs, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls := 0
	_, _, err := normalizeMediaPrompt(ctx, concept, func(context.Context, map[string]any) (map[string]any, error) { calls++; return nil, nil })
	if !errors.Is(err, context.Canceled) || calls != 0 {
		t.Fatal("cancel started generation")
	}
}

func TestMediaBranchM37FinalConditionsAndLegitimateStyleReference(t *testing.T) {
	concept := mediaPreparedConcept(t, "猫与咖啡", mediaTestPlan("mixed", "none", "no", "unknown"), mediaTestContext("FACE"))
	base := map[string]any{"1": map[string]any{"class_type": "CLIPTextEncode", "inputs": map[string]any{"text": "{{prompt}}"}}, "2": map[string]any{"class_type": "KSampler", "inputs": map[string]any{"positive": []any{"1", 0}}}}
	for _, polluted := range []map[string]any{
		{"class_type": "LoadImage", "inputs": map[string]any{"image": "old_self.png"}},
		{"class_type": "LoraLoader", "inputs": map[string]any{"lora_name": "body_identity.safetensors", "strength_model": 1}},
		{"class_type": "IPAdapterFaceID", "inputs": map[string]any{}},
		{"class_type": "CLIPTextEncode", "inputs": map[string]any{"text": "{{prompt}} wearing nonexistent boots"}},
	} {
		workflow := cloneMap(base)
		workflow["3"] = polluted
		if err := validateMediaWorkflowPolicy(nil, concept, workflow); err == nil {
			t.Fatalf("polluted graph accepted: %v", polluted)
		}
	}
	styled := cloneMap(base)
	styled["3"] = map[string]any{"class_type": "LoraLoader", "inputs": map[string]any{"lora_name": "art.safetensors"}}
	if err := validateMediaWorkflowPolicy(map[string]any{"style_loras": []any{"art.safetensors"}}, concept, styled); err != nil {
		t.Fatal(err)
	}
	concept["reference_asset"] = map[string]any{"asset_id": "cat_asset", "purpose": "animal"}
	styled["4"] = map[string]any{"class_type": "LoadImage", "inputs": map[string]any{"image": "{{media_reference_image}}"}}
	if err := validateMediaWorkflowPolicy(map[string]any{"style_loras": []any{"art.safetensors"}}, concept, styled); err != nil {
		t.Fatal(err)
	}
	output := mediaTestOutput(concept)
	concept["normalized_output"] = output
	submitted, err := replaceMediaPlaceholdersWithReference(replaceMediaReferenceAlias(styled, "cat-upload.png"), stringValue(output["prompt"]), nil, "cat-upload.png")
	if err != nil {
		t.Fatal(err)
	}
	if err := validateFinalMediaWorkflow(concept, submitted, stringValue(output["prompt"])); err != nil {
		t.Fatal(err)
	}
	if mediaRendererConstraints(concept)["chest_lora_weight"] != nil {
		t.Fatal("body condition survived")
	}
}

func TestMediaBranchNoMandatoryPersonSlotsAndDeferredReadCounts(t *testing.T) {
	if len(imageCapabilityDefinition().RequiredContext) != 0 || len((imageGenerateCapability{}).RequiredContext()) != 0 {
		t.Fatal("eager person slots")
	}
	for _, plan := range []map[string]any{mediaTestPlan("animal", "none", "no", "unknown"), mediaTestPlan("person", "partial", "yes", "hands"), mediaTestPlan("person", "present", "no", "unknown"), mediaTestPlan("person", "present", "yes", "full")} {
		counts := map[ContextSlot]int{}
		fixture := mediaTestContext("CURRENT")
		loaders := map[ContextSlot]ContextLoader{}
		for _, slot := range []ContextSlot{SlotVisualIdentity, SlotAppearance, SlotCurrentLife, SlotCurrentState} {
			slot := slot
			loaders[slot] = func(context.Context, ContextRequest) (any, error) {
				counts[slot]++
				switch slot {
				case SlotVisualIdentity:
					return fixture.Visual.Data, nil
				case SlotAppearance:
					return fixture.Outfit.Data, nil
				case SlotCurrentLife:
					return fixture.Life.Data, nil
				default:
					return fixture.State.Data, nil
				}
			}
		}
		c := imageGenerateCapability{contextResolver: NewStaticContextResolver(loaders)}
		call := CapabilityInvocation{Arguments: jsonBytes(map[string]any{"intent": "固定媒体", "media_plan": plan})}
		_, err := c.Prepare(context.Background(), call, CapabilityContext{})
		if err != nil {
			t.Fatal(err)
		}
		if !mediaPlanNeedsAppearance(plan) && counts[SlotAppearance] != 0 {
			t.Fatal("unneeded appearance query")
		}
		if !mediaPlanHasSelfIdentity(plan) && counts[SlotVisualIdentity] != 0 {
			t.Fatal("unneeded identity query")
		}
		if counts[SlotCurrentState] != 0 {
			t.Fatal("state query")
		}
	}
}

type mediaProbeStore struct {
	mu      sync.Mutex
	records []decision.Record
}

func (s *mediaProbeStore) Save(_ context.Context, r decision.Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = append(s.records, r)
	return nil
}
func mediaStubKev(t *testing.T, config decision.Config, answer func(decision.Request) map[string]any) (*decision.Service, *mediaProbeStore, *int) {
	t.Helper()
	calls := new(int)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*calls++
		var request decision.Request
		_ = json.NewDecoder(r.Body).Decode(&request)
		answers := answer(request)
		if answers["__status"] != nil {
			w.WriteHeader(500)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"answers": answers})
	}))
	t.Cleanup(server.Close)
	config.Endpoint = server.URL
	store := &mediaProbeStore{}
	return &decision.Service{Settings: explicitKevSettings{config: config, version: 1}, Store: store, HTTP: server.Client(), ID: func() string { return randomID("test_") }}, store, calls
}
func mediaChoiceResponse(choice string, q decision.Question) map[string]any {
	probabilities := map[string]float64{}
	for key := range q.Criteria {
		probabilities[key] = 0
	}
	probabilities[choice] = 1
	return map[string]any{"type": "choice", "choice": choice, "confidence": 1, "probabilities": probabilities}
}

func TestMediaBranchM32M33ConflictsM34DisabledFallbackM38MixedM39Mirror(t *testing.T) {
	for _, disabled := range []string{"global", "media", "subject", "none"} {
		config := decision.DefaultConfig()
		config.Enabled = disabled != "global"
		config.MediaEnabled = disabled != "media"
		config.Points["media.subject_route"] = disabled != "subject"
		expected := mediaTestPlan("mixed", "none", "no", "unknown")
		service, store, calls := mediaStubKev(t, config, func(request decision.Request) map[string]any {
			answers := map[string]any{}
			for id, q := range request.Questions {
				selected := "unspecified"
				if _, ok := q.Criteria["person"]; ok {
					selected = "person"
				}
				if _, ok := q.Criteria["present"]; ok {
					selected = "present"
				}
				if _, ok := q.Criteria["yes"]; ok {
					selected = "yes"
				}
				if _, ok := q.Criteria["full"]; ok {
					selected = "full"
				}
				answers[id] = mediaChoiceResponse(selected, q)
			}
			return answers
		})
		plan, _, err := ResolveMediaCapturePlan(context.Background(), service, decision.Scope{}, "画面无人，猫与咖啡", expected, nil)
		if err != nil {
			t.Fatal(err)
		}
		if plan["human_presence"] != "none" || plan["self_in_frame"] != "no" {
			t.Fatal("explicit no-human replaced")
		}
		if *calls != 0 {
			t.Fatalf("fully explicit called Kev %d times", *calls)
		}
		_ = store
	}
	config := decision.DefaultConfig()
	config.Enabled = true
	explicit := map[string]any{"primary_subject": "landscape", "human_presence": "none"}
	service, store, _ := mediaStubKev(t, config, func(request decision.Request) map[string]any {
		answers := map[string]any{}
		for id, q := range request.Questions {
			choice := "unspecified"
			if _, ok := q.Criteria["yes"]; ok {
				choice = "yes"
			}
			if _, ok := q.Criteria["full"]; ok {
				choice = "full"
			}
			answers[id] = mediaChoiceResponse(choice, q)
		}
		return answers
	})
	plan, _, err := ResolveMediaCapturePlan(context.Background(), service, decision.Scope{}, "无人街道", explicit, nil)
	if err != nil || plan["self_in_frame"] != "no" {
		t.Fatalf("plan=%v err=%v", plan, err)
	}
	rejected := false
	for _, r := range store.records {
		if r.ApplicationStatus == "rejected" && r.Answer != nil && r.Answer.Choice == "yes" {
			rejected = true
		}
	}
	if !rejected {
		t.Fatal("raw conflicting answer not retained")
	}
	mirror := mediaTestPlan("person", "present", "yes", "full")
	mirror["capture_method"] = "mirror_selfie"
	mirror["viewpoint"] = "reflection"
	mirror["framing"] = "full_body"
	if _, _, err := ResolveMediaCapturePlan(context.Background(), nil, decision.Scope{}, "镜前全身", mirror, nil); err != nil {
		t.Fatal(err)
	}
	external := mediaTestPlan("person", "present", "yes", "full")
	external["capture_method"] = "external_photo"
	external["viewpoint"] = "observer"
	delete(external, "camera_angle")
	service, store, _ = mediaStubKev(t, config, func(request decision.Request) map[string]any {
		answers := map[string]any{}
		for id, q := range request.Questions {
			answers[id] = mediaChoiceResponse("unspecified", q)
		}
		return answers
	})
	if _, _, err := ResolveMediaCapturePlan(context.Background(), service, decision.Scope{}, "他拍", external, nil); err != nil {
		t.Fatal(err)
	}
	for _, r := range store.records {
		for _, q := range r.Request.Questions {
			if _, has := q.Criteria["mirror_selfie"]; has {
				t.Fatal("explicit method was reasked")
			}
		}
	}
}

func TestMediaBranchM34FailureUsesOriginalAndM40Cancel(t *testing.T) {
	for _, kind := range []string{"unknown", "missing", "500", "invalid"} {
		config := decision.DefaultConfig()
		config.Enabled = true
		service, _, _ := mediaStubKev(t, config, func(request decision.Request) map[string]any {
			out := map[string]any{}
			if kind == "500" {
				return map[string]any{"__status": 500}
			}
			if kind == "missing" {
				return out
			}
			for id, q := range request.Questions {
				choice := "unknown"
				if kind == "invalid" {
					choice = "not-an-option"
				}
				out[id] = mediaChoiceResponse(choice, q)
			}
			return out
		})
		calls := 0
		plan, _, err := ResolveMediaCapturePlan(context.Background(), service, decision.Scope{}, "我的猫", nil, func(context.Context, map[string]any) (map[string]any, error) {
			calls++
			return mediaTestPlan("animal", "none", "no", "unknown"), nil
		})
		if err != nil || calls != 1 || plan["human_presence"] != "none" {
			t.Fatalf("%s: plan=%v calls=%d err=%v", kind, plan, calls, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	called := false
	_, _, err := ResolveMediaCapturePlan(ctx, nil, decision.Scope{}, "照片", nil, func(ctx context.Context, _ map[string]any) (map[string]any, error) {
		called = true
		return nil, ctx.Err()
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if called {
		t.Fatal("fallback called after cancellation")
	}
}

func TestMediaBranchInvalidPlanAndForbiddenSource(t *testing.T) {
	plan := mediaTestPlan("animal", "none", "no", "unknown")
	plan["capture_method"] = "mirror_selfie"
	if validateMediaCapturePlan(plan) == nil {
		t.Fatal("none mirror")
	}
	concept := mediaPreparedConcept(t, "猫", mediaTestPlan("animal", "none", "no", "unknown"), mediaTestContext("CURRENT"))
	concept["context_binding"] = map[string]any{"visual_identity": map[string]any{"face": "sentinel"}}
	if _, err := mediaNormalizationInput(concept); err == nil {
		t.Fatal("pollution allowed")
	}
}

func TestMediaPlanProbe(t *testing.T) {
	endpoint := os.Getenv("FLUCTLIGHT_MEDIA_KEV_URL")
	if endpoint == "" {
		t.Skip("set FLUCTLIGHT_MEDIA_KEV_URL for real isolated Kev single-point JSON report")
	}
	config := decision.DefaultConfig()
	config.Enabled = true
	config.Endpoint = endpoint
	config.TimeoutMS = 60000
	config.BudgetMS = 120000
	store := &mediaProbeStore{}
	service := &decision.Service{Settings: explicitKevSettings{config: config, version: 1, secret: os.Getenv("FLUCTLIGHT_MEDIA_KEV_API_KEY")}, Store: store, HTTP: &http.Client{}, ID: func() string { return randomID("media_probe_") }}
	report := []map[string]any{}
	for _, fixture := range mediaCases(t) {
		for _, intent := range []string{fixture.Intent, fixture.Paraphrase} {
			plan, _, err := ResolveMediaCapturePlan(context.Background(), service, decision.Scope{ActorSelf: "probe_self", RunID: fixture.ID}, intent, nil, nil)
			entry := map[string]any{"id": fixture.ID, "intent": intent, "expected": fixture.Plan, "actual": plan}
			if err != nil {
				entry["error"] = err.Error()
			}
			report = append(report, entry)
		}
	}
	path := os.Getenv("FLUCTLIGHT_MEDIA_REPORT")
	if path == "" {
		path = "/tmp/media-kev-probe.json"
	}
	raw, _ := json.MarshalIndent(map[string]any{"cases": report, "records": store.records}, "", "  ")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	fmt.Printf("Kev report: %s (accuracy requires reviewing actual vs allowed expectations)\n", path)
}

func TestMediaBranchReviewRegressions(t *testing.T) {
	none := mediaTestPlan("landscape", "none", "no", "unknown")
	concept := mediaPreparedConcept(t, "无人街景", none, mediaTestContext("EXCLUDED"))
	out := mediaTestOutput(concept)
	out["prompt"] = "摇光举起手机，拍摄自己的前置自拍，脸部清晰可见"
	if validateMediaNormalization(concept, out) == nil {
		t.Fatal("Chinese selfie with fake none declaration passed")
	}
	contradictory := cloneMap(none)
	contradictory["self_in_frame"] = "yes"
	if _, _, err := ResolveMediaCapturePlan(context.Background(), nil, decision.Scope{}, "无人", contradictory, nil); err == nil {
		t.Fatal("explicit contradiction silently changed")
	}
	person := cloneMap(none)
	person["primary_subject"] = "person"
	if validateMediaCapturePlan(person) == nil {
		t.Fatal("person/none allowed")
	}
	source := mediaPreparedConcept(t, "本人", mediaTestPlan("person", "present", "yes", "full"), mediaTestContext("SELF"))
	ref := map[string]any{"asset_id": "self_asset", "purpose": "scene"}
	if validateMediaReferenceSource(ref, source) == nil {
		t.Fatal("self portrait accepted as scene reference")
	}
	cat := mediaPreparedConcept(t, "猫", mediaTestPlan("animal", "none", "no", "unknown"), CapabilityContext{})
	if err := validateMediaReferenceSource(map[string]any{"purpose": "animal"}, cat); err != nil {
		t.Fatal(err)
	}
	call := CapabilityInvocation{Arguments: jsonBytes(map[string]any{"intent": "猫", "media_plan": none})}
	prepared, err := (imageGenerateCapability{}).Prepare(context.Background(), call, CapabilityContext{})
	if err != nil {
		t.Fatal(err)
	}
	prepared.Arguments = jsonBytes(map[string]any{"intent": "猫", "media_plan": mediaTestPlan("person", "present", "yes", "full")})
	if _, err := (imageGenerateCapability{}).Prepare(context.Background(), prepared, mediaTestContext("NEW")); err == nil {
		t.Fatal("changed replay accepted")
	}
}

func TestMediaBranchM35CurrentAppearanceAndScopedRenderer(t *testing.T) {
	state := mediaTestContext("CURRENT")
	mapValue(mapValue(state.Visual.Data["identity_snapshot"])["identity"])["appearance"] = map[string]any{"hair_color": "OLD_HAIR", "clothing": "OLD_SHIRT"}
	mapValue(state.Outfit.Data["body_fields"])["chest_cup"] = map[string]any{"status": "known", "value": "B"}
	full := mediaPreparedConcept(t, "本人全身", mediaTestPlan("person", "present", "yes", "full"), state)
	packet, err := mediaNormalizationInput(full)
	if err != nil {
		t.Fatal(err)
	}
	wire := jsonString(packet)
	if strings.Contains(wire, "OLD_HAIR") || strings.Contains(wire, "OLD_SHIRT") || !strings.Contains(wire, "CURRENT_HAIR") {
		t.Fatalf("current authority lost: %s", wire)
	}
	if mediaRendererConstraints(full)["chest_lora_weight"] != float64(-3) {
		t.Fatal("renderer not from current body")
	}
	face := mediaPreparedConcept(t, "本人脸部近景", mediaTestPlan("person", "present", "yes", "face"), state)
	if len(mediaRendererConstraints(face)) != 0 {
		t.Fatal("face branch retained body weight")
	}
	if strings.Contains(compactMediaConceptForProvider(jsonString(face)), "CURRENT_BOOTS") {
		t.Fatal("face branch retained shoes")
	}
}

func TestMediaBranchKevTwoStagesAndMixedPointAudit(t *testing.T) {
	config := decision.DefaultConfig()
	config.Enabled = true
	plan := mediaTestPlan("person", "present", "yes", "full")
	service, store, calls := mediaStubKev(t, config, func(request decision.Request) map[string]any {
		answers := map[string]any{}
		for id, q := range request.Questions {
			value := "unspecified"
			for _, key := range []string{"primary_subject", "human_presence", "self_in_frame", "visible_scope"} {
				if strings.HasPrefix(q.Instructions, mediaQuestion(key).Instructions) {
					value = stringValue(plan[key])
				}
			}
			answers[id] = mediaChoiceResponse(value, q)
		}
		return answers
	})
	_, _, err := ResolveMediaCapturePlan(context.Background(), service, decision.Scope{}, "一张本人照片", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if *calls != 2 {
		t.Fatalf("calls=%d", *calls)
	}
	points := map[string]bool{}
	for _, r := range store.records {
		points[r.Point] = true
	}
	if !points["media.subject_route"] || !points["media.capture_plan"] || !points["media.view_plan"] {
		t.Fatalf("point audit=%v", points)
	}
}

type mediaRevokingSettings struct {
	mu        sync.Mutex
	config    decision.Config
	reads     int
	disableAt int
}

func (s *mediaRevokingSettings) Read(context.Context) (decision.Config, int64, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reads++
	c := s.config
	if s.disableAt > 0 && s.reads >= s.disableAt {
		c.Enabled = false
		return c, 2, "", nil
	}
	return c, 1, "", nil
}
func TestMediaBranchM40DisableBeforeFreezeRetainsRawAndFallsBack(t *testing.T) {
	config := decision.DefaultConfig()
	config.Enabled = true
	service, store, _ := mediaStubKev(t, config, func(request decision.Request) map[string]any {
		answers := map[string]any{}
		for id, q := range request.Questions {
			choice := "unknown"
			if _, ok := q.Criteria["landscape"]; ok {
				choice = "landscape"
			}
			if _, ok := q.Criteria["none"]; ok {
				choice = "none"
			}
			if _, ok := q.Criteria["no"]; ok {
				choice = "no"
			}
			if _, ok := q.Criteria["unspecified"]; ok {
				choice = "unspecified"
			}
			answers[id] = mediaChoiceResponse(choice, q)
		}
		return answers
	})
	configured, _, _, _ := service.Settings.Read(context.Background())
	settings := &mediaRevokingSettings{config: configured, disableAt: 10}
	service.Settings = settings
	calls := 0
	plan, _, err := ResolveMediaCapturePlan(context.Background(), service, decision.Scope{}, "窗外的猫", nil, func(context.Context, map[string]any) (map[string]any, error) {
		calls++
		return mediaTestPlan("animal", "none", "no", "unknown"), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || plan["primary_subject"] != "animal" {
		t.Fatalf("adopted revoked supplement: %v calls=%d", plan, calls)
	}
	retained := false
	for _, r := range store.records {
		if r.Answer != nil && r.Answer.Choice == "landscape" && r.ApplicationStatus == "original_used" {
			retained = true
		}
	}
	if !retained {
		t.Fatal("raw result discarded or falsely applied")
	}
}

func TestMediaBranchRawCaptureHintsSurviveToOriginalAndNormalizer(t *testing.T) {
	hints := map[string]any{"framing": "从头顶到鞋底", "angle": "明确的平视机位"}
	ctx := withMediaCaptureHints(context.Background(), hints)
	plan, _, err := ResolveMediaCapturePlan(ctx, nil, decision.Scope{}, "本人照片", nil, func(_ context.Context, packet map[string]any) (map[string]any, error) {
		if jsonString(packet["declared_capture"]) != jsonString(hints) {
			t.Fatal("raw capture lost")
		}
		result := mediaTestPlan("person", "present", "yes", "full")
		result["framing"] = "full_body"
		return result, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	concept := mediaPreparedConcept(t, "本人照片", plan, mediaTestContext("CURRENT"))
	concept["capture"] = hints
	packet, err := mediaNormalizationInput(concept)
	if err != nil || jsonString(packet["declared_capture"]) != jsonString(hints) {
		t.Fatal("normalizer lost raw capture")
	}
}

func TestMediaBranchKnownSubjectsUseTheirOwnDataAndReferences(t *testing.T) {
	self := mediaPreparedConcept(t, "我和B合影", mediaTestPlan("mixed", "present", "yes", "full"), mediaTestContext("A"))
	b := mediaTestContext("B")
	self["subject_contexts"] = []any{map[string]any{"actor_id": "actor_B", "visible_scope": "full", "appearance": b.Outfit.Data, "visual_identity": b.Visual.Data}}
	packet, err := mediaNormalizationInput(self)
	if err != nil {
		t.Fatal(err)
	}
	others := arrayValue(packet["other_subjects"])
	if len(others) != 1 || strings.Contains(jsonString(others[0]), "A_FACE") || !strings.Contains(jsonString(others[0]), "B_FACE") {
		t.Fatalf("subject partition=%v", packet)
	}
	output := mediaTestOutput(self)
	output["used_subject_ids"] = []any{"self", "actor_B"}
	if err := validateMediaNormalization(self, output); err != nil {
		t.Fatal(err)
	}
	output["used_subject_ids"] = []any{"unbound_actor_C"}
	if validateMediaNormalization(self, output) == nil {
		t.Fatal("invented actor accepted")
	}
	reference := map[string]any{"asset_id": "B_CANONICAL", "purpose": "subject", "subject_id": "actor_B"}
	if owner, err := mediaReferenceOwner("actor_A", reference, self); err != nil || owner != "actor_B" {
		t.Fatalf("reference owner=%s err=%v", owner, err)
	}
	reference["subject_id"] = "actor_C"
	if _, err := mediaReferenceOwner("actor_A", reference, self); err == nil {
		t.Fatal("unbound reference subject")
	}
	none := mediaPreparedConcept(t, "静物", mediaTestPlan("object", "none", "no", "unknown"), CapabilityContext{})
	none["subject_contexts"] = self["subject_contexts"]
	if validateBranchedMediaConcept(none) == nil {
		t.Fatal("none received other actors")
	}
	conflicting := mediaTestPlan("landscape", "none", "no", "unknown")
	if explicitMediaSubjects(map[string]any{"visible_subjects": []any{map[string]any{"actor_id": "A", "visible_scope": "full"}}}, "A", conflicting) == nil {
		t.Fatal("typed subject contradiction ignored")
	}
}

func TestMediaBranchBodyWeightsAndNegativeConditionsAreScoped(t *testing.T) {
	state := mediaTestContext("CURRENT")
	mapValue(state.Outfit.Data["body_fields"])["chest_cup"] = map[string]any{"status": "known", "value": "B"}
	concept := mediaPreparedConcept(t, "本人全身", mediaTestPlan("person", "present", "yes", "full"), state)
	workflow := map[string]any{"1": map[string]any{"class_type": "CLIPTextEncode", "inputs": map[string]any{"text": "{{prompt}}"}}, "2": map[string]any{"class_type": "LoraLoader", "inputs": map[string]any{"lora_name": "body.safetensors", "strength_model": "{{chest_lora_weight}}"}}}
	if err := validateMediaWorkflowPolicy(nil, concept, workflow); err != nil {
		t.Fatal(err)
	}
	final, err := replaceMediaPlaceholders(workflow, "本人全身的中文正文", mediaRendererConstraints(concept))
	if err != nil || mapValue(mapValue(final["2"])["inputs"])["strength_model"] != float64(-3) {
		t.Fatalf("weight=%v err=%v", final, err)
	}
	mapValue(mapValue(workflow["2"])["inputs"])["strength_model"] = 1.0
	if validateMediaWorkflowPolicy(nil, concept, workflow) == nil {
		t.Fatal("unclassified fixed body default")
	}
	none := mediaPreparedConcept(t, "静物", mediaTestPlan("object", "none", "no", "unknown"), CapabilityContext{})
	delete(workflow, "2")
	workflow["3"] = map[string]any{"class_type": "CLIPTextEncode", "inputs": map[string]any{"text": "fixed person body attributes"}}
	workflow["4"] = map[string]any{"class_type": "KSampler", "inputs": map[string]any{"positive": []any{"1", 0}, "negative": []any{"3", 0}}}
	if validateMediaWorkflowPolicy(nil, none, workflow) == nil {
		t.Fatal("unclassified negative condition")
	}
	mapValue(mapValue(workflow["3"])["inputs"])["text"] = "low quality, watermark"
	if err := validateMediaWorkflowPolicy(map[string]any{"quality_negative_prompts": []any{"low quality, watermark"}}, none, workflow); err != nil {
		t.Fatal(err)
	}
}

func TestMediaBranchExplicitCountAndStrictNormalizerShape(t *testing.T) {
	concept := mediaPreparedConcept(t, "无人街景", mediaTestPlan("landscape", "none", "no", "unknown"), CapabilityContext{})
	concept["human_count"] = 0
	output := mediaTestOutput(concept)
	output["declared_human_count"] = 0.5
	if validateMediaNormalization(concept, output) == nil {
		t.Fatal("fractional count")
	}
	output["declared_human_count"] = 1
	if validateMediaNormalization(concept, output) == nil {
		t.Fatal("count override")
	}
	output["declared_human_count"] = 0
	if err := validateMediaNormalization(concept, output); err != nil {
		t.Fatal(err)
	}
	delete(output, "declared_human_count")
	output["unexpected"] = nil
	if validateMediaNormalization(concept, output) == nil {
		t.Fatal("extra instead of required field")
	}
	_, err := (&App{}).RunMediaPromptTask(context.Background(), MediaPromptTaskInput{Intent: mediaIntent{Prompt: `{"context_binding":{"appearance":{"outfit":"legacy"}}}`}})
	if err == nil || err.Error() != "media_legacy_intent_missing" {
		t.Fatalf("legacy context-only request escaped: %v", err)
	}
}

func TestMediaBranchLatentReferenceUsesImmutableCaptureVersions(t *testing.T) {
	workflow := map[string]any{"image": map[string]any{"class_type": "LoadImage", "inputs": map[string]any{"image": "{{visual_identity_reference_image}}"}}, "encode": map[string]any{"class_type": "VAEEncode", "inputs": map[string]any{"pixels": []any{"image", 0}}}, "sampler": map[string]any{"class_type": "KSampler", "inputs": map[string]any{"latent_image": []any{"encode", 0}}}}
	if !mediaWorkflowReferenceIsLatent(workflow, "{{visual_identity_reference_image}}") {
		t.Fatal("latent reference path missed")
	}
	source := map[string]any{"purpose": "visual_identity", "visual_identity": map[string]any{"body_revision": 1, "wardrobe_revision": 2}}
	current := map[string]any{"body_revision": 1, "wardrobe_revision": 3}
	if mediaReferenceSnapshotMatches(source, current) {
		t.Fatal("historical clothing reference accepted")
	}
	current["wardrobe_revision"] = 2
	if !mediaReferenceSnapshotMatches(source, current) {
		t.Fatal("same capture rejected")
	}
}
