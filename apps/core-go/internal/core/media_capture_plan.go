package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/ai/decision"
)

const mediaPlanVersion = "media.capture.v1"

var errMediaStructuredResponseInvalid = errors.New("media_structured_response_invalid")

type mediaCaptureHintsKey struct{}
type mediaReferentsKey struct{}

func withMediaReferents(ctx context.Context, bindings map[string]any) context.Context {
	return context.WithValue(ctx, mediaReferentsKey{}, cloneMap(bindings))
}
func mediaReferents(ctx context.Context) map[string]any {
	value, _ := ctx.Value(mediaReferentsKey{}).(map[string]any)
	return cloneMap(value)
}

func withMediaCaptureHints(ctx context.Context, capture map[string]any) context.Context {
	return context.WithValue(ctx, mediaCaptureHintsKey{}, cloneMap(capture))
}
func mediaCaptureHints(ctx context.Context) map[string]any {
	value, _ := ctx.Value(mediaCaptureHintsKey{}).(map[string]any)
	return cloneMap(value)
}

var mediaPlanOptions = map[string][]string{
	"primary_subject": {"landscape", "animal", "object", "person", "mixed", "other", "unknown"},
	"human_presence":  {"none", "partial", "present", "unknown"},
	"self_in_frame":   {"yes", "no", "unknown"},
	"visible_scope":   {"full", "face", "back", "hands", "feet", "unknown"},
	"capture_method":  {"handheld_selfie", "mirror_selfie", "self_timer", "external_photo", "unspecified"},
	"viewpoint":       {"subject_facing", "pov_outward", "observer", "reflection", "unspecified"},
	"framing":         {"detail", "close_up", "half_body", "full_body", "wide", "unspecified"},
	"camera_angle":    {"eye_level", "high_angle", "low_angle", "top_down", "unspecified"},
}

func mediaCapturePlanSchema() map[string]any {
	properties := map[string]any{}
	required := []string{}
	for _, key := range []string{"primary_subject", "human_presence", "self_in_frame", "visible_scope", "capture_method", "viewpoint", "framing", "camera_angle"} {
		properties[key] = enumStringSchema(mediaPlanOptions[key]...)
		required = append(required, key)
	}
	return objectSchema(properties, required, false)
}

func validateMediaCapturePlan(plan map[string]any) error {
	if err := validateMediaExplicitSubset(plan); err != nil {
		return err
	}
	for key, options := range mediaPlanOptions {
		value := stringValue(plan[key])
		valid := false
		for _, option := range options {
			if value == option {
				valid = true
			}
		}
		if !valid {
			return fmt.Errorf("media_capture_plan_invalid:%s", key)
		}
	}
	if plan["human_presence"] == "none" && (plan["self_in_frame"] != "no" || plan["capture_method"] == "handheld_selfie" || plan["capture_method"] == "mirror_selfie" || plan["capture_method"] == "self_timer") {
		return errors.New("media_capture_plan_no_human_conflict")
	}
	if plan["human_presence"] == "unknown" && plan["self_in_frame"] == "yes" {
		return errors.New("media_capture_plan_unknown_self")
	}
	if plan["human_presence"] == "partial" && plan["visible_scope"] != "hands" && plan["visible_scope"] != "feet" {
		return errors.New("media_capture_plan_partial_scope_required")
	}
	if plan["capture_method"] == "handheld_selfie" && plan["viewpoint"] == "pov_outward" {
		return errors.New("media_capture_plan_camera_conflict")
	}
	if plan["capture_method"] == "mirror_selfie" && plan["viewpoint"] != "reflection" {
		return errors.New("media_capture_plan_reflection_required")
	}
	if len(plan) != len(mediaPlanOptions) {
		return errors.New("media_capture_plan_extra_fields")
	}
	return nil
}

func validateMediaExplicitSubset(plan map[string]any) error {
	for key, raw := range plan {
		options, known := mediaPlanOptions[key]
		valid := false
		for _, option := range options {
			if raw == option {
				valid = true
			}
		}
		if !known || !valid {
			return errors.New("media_explicit_constraint_invalid")
		}
	}
	if plan["human_presence"] == "none" {
		if plan["primary_subject"] == "person" || plan["self_in_frame"] == "yes" || plan["capture_method"] == "handheld_selfie" || plan["capture_method"] == "mirror_selfie" || plan["capture_method"] == "self_timer" {
			return errors.New("media_explicit_constraint_conflict")
		}
		if scope, ok := plan["visible_scope"]; ok && scope != "unknown" {
			return errors.New("media_explicit_scope_conflict")
		}
	}
	if plan["capture_method"] == "handheld_selfie" && plan["viewpoint"] == "pov_outward" {
		return errors.New("media_explicit_constraint_conflict")
	}
	if plan["capture_method"] == "mirror_selfie" && plan["viewpoint"] != nil && plan["viewpoint"] != "reflection" && plan["viewpoint"] != "unspecified" {
		return errors.New("media_explicit_constraint_conflict")
	}
	return nil
}

func mediaPlanHasSelfIdentity(plan map[string]any) bool {
	return plan["self_in_frame"] == "yes" && plan["human_presence"] == "present" && (plan["visible_scope"] == "full" || plan["visible_scope"] == "face")
}
func mediaPlanNeedsAppearance(plan map[string]any) bool {
	return plan["self_in_frame"] == "yes" && plan["human_presence"] != "none" && plan["visible_scope"] != "unknown"
}
func isBranchedMedia(concept map[string]any) bool {
	return concept["media_plan_version"] == mediaPlanVersion
}

// A structural Tool declaration is authority for this request. Natural text is
// data, never classified with word matching. The original model handles fields
// which Kev cannot reliably decide, through the same media task foundation.
func explicitMediaPlan(args map[string]any) map[string]any {
	result := cloneMap(mapValue(args["media_plan"]))
	if result == nil {
		result = map[string]any{}
	}
	for key, value := range result {
		if value == "unknown" || value == "unspecified" {
			delete(result, key)
		}
	}
	capture := mapValue(args["capture"])
	for source, target := range map[string]string{"mode": "capture_method", "framing": "framing", "angle": "camera_angle"} {
		value := stringValue(capture[source])
		switch value {
		case "selfie":
			value = "handheld_selfie"
		case "external_capture":
			value = "external_photo"
		case "operator_pov", "first_person":
			result["viewpoint"] = "pov_outward"
			continue
		case "closeup":
			value = "close_up"
		case "upper_body":
			value = "half_body"
		case "body_detail":
			value = "detail"
		case "scene":
			value = "wide"
		}
		for _, option := range mediaPlanOptions[target] {
			if value == option {
				result[target] = value
			}
		}
	}
	if capture["mirror"] == true || result["capture_method"] == "mirror_selfie" {
		result["viewpoint"] = "reflection"
	}
	return result
}

func mediaQuestion(key string) decision.Question {
	descriptions := map[string]string{
		"primary_subject": "Classify the requested visible image subject. Mixed objects need not contain humans. Dolls and posters are objects, not real people.",
		"human_presence":  "Classify humans actually depicted, not the author, photographer or owner. None excludes hands, silhouettes and human reflections; partial means hands or feet only. Negation and future plans must not add a person.",
		"self_in_frame":   "Is the task actor actually visible in this image? An author saying I take a photo, my cat, or my viewpoint is not in frame. Another person is not self. Use first_person referent bindings: actor_user is not task_actor; unknown cannot establish self identity. Do not invent people.",
		"visible_scope":   "Which human scope is visible? For an in-frame task actor classify that actor, otherwise classify the visible person. Hands/feet exclude the face and whole body. A back view excludes the face. Unknown means there is no evidence.",
		"capture_method":  "Choose only a compatible camera method; preserve explicit requests. Without evidence use unspecified. Never turn outward POV or an object photo into a selfie. External camera does not create a photographer Actor.",
		"viewpoint":       "Preserve the requested spatial relationship. Outward POV cannot face the photographer; mirror selfie needs reflection. Use unspecified without evidence.",
		"framing":         "Preserve explicit framing, including full body from head to feet. Objects and animals need not use human framing. Do not silently turn handheld full body into mirror selfie.",
		"camera_angle":    "Preserve explicit angle. No global high angle or 45-degree selfie default. Use unspecified without evidence.",
	}
	criteria := map[string]string{}
	for _, option := range mediaPlanOptions[key] {
		criteria[option] = option
	}
	return decision.Question{Type: "choice", Instructions: descriptions[key], Criteria: criteria}
}

// ResolveMediaCapturePlan is shared by the real Tool and the independent Kev
// probe. It reads no persona, appearance, history, life facts or image assets.
func ResolveMediaCapturePlan(ctx context.Context, service *decision.Service, scope decision.Scope, intent string, explicit map[string]any, original func(context.Context, map[string]any) (map[string]any, error)) (map[string]any, map[string]any, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	if original != nil {
		call := original
		parseCalls := 0
		original = func(ctx context.Context, packet map[string]any) (map[string]any, error) {
			if parseCalls >= 1 {
				return nil, errors.New("media_subject_parse_invalid")
			}
			parseCalls++
			packet = cloneMap(packet)
			if bindings := mediaReferents(ctx); len(bindings) > 0 {
				packet["referent_bindings"] = bindings
			}
			if hints := mediaCaptureHints(ctx); len(hints) > 0 {
				packet["declared_capture"] = hints
			}
			return call(ctx, packet)
		}
	}
	if err := validateMediaExplicitSubset(explicit); err != nil {
		return nil, nil, err
	}
	plan := cloneMap(explicit)
	if plan == nil {
		plan = map[string]any{}
	}
	sources := map[string]any{}
	adopted := []*decision.Result{}
	for key := range plan {
		sources[key] = "explicit"
	}
	if service != nil && service.Settings != nil {
		if config, _, _, err := service.Settings.Read(ctx); err == nil {
			ctx = decision.WithStageBudget(ctx, time.Duration(config.BudgetMS)*time.Millisecond)
		}
	}
	questions := func(point string, keys []string) error {
		if service == nil || !service.Enabled(ctx, point) {
			return ctx.Err()
		}
		candidates := []decision.Candidate{}
		for _, key := range keys {
			if _, locked := explicit[key]; locked {
				continue
			}
			candidates = append(candidates, decision.Candidate{ID: key, Question: mediaQuestion(key)})
		}
		if len(candidates) == 0 {
			return nil
		}
		results, err := service.Decide(ctx, point, scope, jsonString(map[string]any{"intent_text": intent, "explicit_constraints": explicit, "accepted_plan": plan, "declared_capture": mediaCaptureHints(ctx), "referent_bindings": mediaReferents(ctx)}), candidates)
		if err != nil {
			return err
		}
		for i := range results {
			r := &results[i]
			outcome, err := r.Admit(ctx)
			if err != nil {
				return err
			}
			if outcome == decision.Original {
				_ = r.Finish(ctx, "original_used", r.Record.Reason, scope.RunID)
				continue
			}
			key := r.Record.CandidateID
			value := string(outcome)
			proposal := cloneMap(plan)
			proposal[key] = value
			conflict := validateMediaExplicitSubset(proposal) != nil || proposal["human_presence"] == "none" && ((key == "self_in_frame" && value != "no") || (key == "capture_method" && value != "unspecified" && value != "external_photo"))
			if conflict {
				_ = r.Finish(ctx, "rejected", "media_plan_conflict", scope.RunID)
				continue
			}
			plan[key] = value
			adopted = append(adopted, r)
			sources[key] = "kev"
			_ = r.Finish(ctx, "applied", "", scope.RunID)
		}
		return nil
	}
	if err := questions("media.subject_route", []string{"primary_subject", "human_presence", "self_in_frame", "visible_scope"}); err != nil {
		return nil, nil, err
	}
	if plan["human_presence"] == "none" {
		if _, locked := explicit["self_in_frame"]; !locked {
			plan["self_in_frame"] = "no"
			sources["self_in_frame"] = "neutral_default"
		}
		if _, locked := explicit["visible_scope"]; !locked {
			plan["visible_scope"] = "unknown"
			sources["visible_scope"] = "neutral_default"
		}
	}
	// An unavailable/abstaining subject decision must not load self context. Only
	// the existing model's bounded parsing continuation may resolve that ambiguity.
	needOriginal := plan["primary_subject"] == nil || plan["primary_subject"] == "unknown" || plan["human_presence"] == nil || plan["human_presence"] == "unknown" || plan["self_in_frame"] == nil || plan["self_in_frame"] == "unknown" || (plan["self_in_frame"] == "yes" && (plan["visible_scope"] == nil || plan["visible_scope"] == "unknown"))
	if needOriginal && original != nil {
		parsed, err := original(ctx, map[string]any{"intent_text": intent, "explicit_constraints": explicit, "accepted_plan": plan})
		if err != nil {
			return nil, nil, err
		}
		for key, value := range parsed {
			if _, locked := explicit[key]; !locked {
				plan[key] = value
				sources[key] = "original_llm"
			}
		}
	}
	// Capture and view are two logical points in one physical second stage.
	if service != nil {
		candidates := []decision.Candidate{}
		point := ""
		for _, key := range []string{"capture_method", "viewpoint", "framing", "camera_angle"} {
			route := "media.view_plan"
			if key == "capture_method" {
				route = "media.capture_plan"
				if plan["human_presence"] != "present" {
					continue
				}
			}
			if _, locked := explicit[key]; locked || sources[key] == "original_llm" {
				continue
			}
			if !service.Enabled(ctx, route) {
				continue
			}
			point = route
			candidates = append(candidates, decision.Candidate{Point: route, ID: key, Question: mediaQuestion(key)})
		}
		if len(candidates) > 0 {
			results, err := service.Decide(ctx, point, scope, jsonString(map[string]any{"intent_text": intent, "explicit_constraints": explicit, "accepted_plan": plan, "declared_capture": mediaCaptureHints(ctx), "referent_bindings": mediaReferents(ctx)}), candidates)
			if err != nil {
				return nil, nil, err
			}
			for i := range results {
				r := &results[i]
				out, err := r.Admit(ctx)
				if err != nil {
					return nil, nil, err
				}
				if out == decision.Original {
					_ = r.Finish(ctx, "original_used", r.Record.Reason, scope.RunID)
					continue
				}
				proposal := cloneMap(plan)
				proposal[r.Record.CandidateID] = string(out)
				if len(proposal) == len(mediaPlanOptions) && validateMediaCapturePlan(proposal) != nil {
					_ = r.Finish(ctx, "rejected", "media_plan_conflict", scope.RunID)
					continue
				}
				plan[r.Record.CandidateID] = string(out)
				adopted = append(adopted, r)
				sources[r.Record.CandidateID] = "kev"
				_ = r.Finish(ctx, "applied", "", scope.RunID)
			}
		}
	}

	revoked := false
	for _, r := range adopted {
		out, err := r.Admit(ctx)
		if err != nil {
			return nil, nil, err
		}
		if out == decision.Original {
			revoked = true
		}
	}
	if revoked {
		for key, source := range sources {
			if source == "kev" {
				delete(plan, key)
				delete(sources, key)
			}
		}
		for _, r := range adopted {
			_ = r.Finish(ctx, "original_used", "config_changed_before_freeze", scope.RunID)
		}
		if original != nil && (plan["primary_subject"] == nil || plan["human_presence"] == nil || plan["self_in_frame"] == nil) {
			parsed, err := original(ctx, map[string]any{"intent_text": intent, "explicit_constraints": explicit})
			if err != nil {
				return nil, nil, err
			}
			for key, value := range parsed {
				if _, locked := explicit[key]; !locked {
					plan[key] = value
					sources[key] = "original_llm"
				}
			}
		}
	}

	for key := range mediaPlanOptions {
		if plan[key] == nil {
			value := "unspecified"
			if key == "primary_subject" || key == "human_presence" || key == "self_in_frame" || key == "visible_scope" {
				value = "unknown"
			}
			plan[key] = value
			sources[key] = "neutral_default"
		}
	}
	if plan["human_presence"] == "none" {
		plan["self_in_frame"] = "no"
		plan["visible_scope"] = "unknown"
		if _, locked := explicit["capture_method"]; !locked {
			plan["capture_method"] = "unspecified"
		}
	}
	if err := validateMediaCapturePlan(plan); err != nil {
		if original == nil {
			return nil, nil, err
		}
		parsed, parseErr := original(ctx, map[string]any{"intent_text": intent, "explicit_constraints": explicit, "validation_error": err.Error()})
		if parseErr != nil {
			return nil, nil, parseErr
		}
		for key, value := range parsed {
			if _, locked := explicit[key]; !locked {
				plan[key] = value
				sources[key] = "original_llm"
			}
		}
		if err := validateMediaCapturePlan(plan); err != nil {
			return nil, nil, err
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	if plan["primary_subject"] == "unknown" {
		return nil, nil, newCapabilityError("media_subject_unresolved", false, nil)
	}
	return plan, sources, nil
}

func (a *App) parseMediaPlan(ctx context.Context, packet map[string]any) (map[string]any, error) {
	return a.mediaStructuredCall(ctx, "media_subject_parse", "解析本次原始图像意图与明确要求，仅返回所给摄影方案结构，不决定新的主题。拍摄者、所属Actor和入镜者不同。遵守first_person指代绑定：actor_user是用户而不是task_actor，unknown不能自动假定本人；task_actor绑定也不代表入镜。没有人物不加载本人；我拍猫、我的猫、从我的视角都不等于本人入镜。不要用未来计划或引用的自拍词改变本次主体。明确约束优先；可用unknown/unspecified。镜面与景别独立，人偶/海报是物品，背影/剪影/路人属于人物。局部手/脚只用hands/feet范围。不要虚构生活事实。", packet, mediaCapturePlanSchema())
}

func (a *App) mediaStructuredCall(ctx context.Context, stage, instruction string, packet map[string]any, schema map[string]any) (map[string]any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// The parent conversation may require publication through its own Tool.
	// This independent, tool-free media task keeps cancellation/correlation but
	// must not inherit that parent's physical tool-choice or catalog policy.
	ctx = withoutPhysicalModelRequestPolicy(ctx)
	prompt := PromptAssemblyResult{ResponseFormat: providerResponseFormatForSchema("media_prompt", stage, schema), Messages: formatProviderMessagesForRole([]map[string]any{{"role": "system", "content": instruction}, {"role": "user", "content": jsonString(packet)}}, "media_prompt")}
	run, err := a.RunFormalAgent(WithProviderScenario(ctx, "media_prompt"), FormalAgentMediaPrompt, FormalAgentRunInput{Prompt: prompt, SchemaName: stage})
	if err != nil {
		return nil, err
	}
	if parsed, ok := parseStructuredCandidates([]string{run.Completion.Text}); ok {
		return parsed, nil
	}
	return nil, errMediaStructuredResponseInvalid
}

// projectMediaAppearance exposes only the visible scope. Revision/digest stay
// durable Core evidence; the normalizer never receives the excluded raw fields.
func projectMediaAppearance(appearance map[string]any, scope string) map[string]any {
	if scope == "full" {
		return cloneMap(appearance)
	}
	result := map[string]any{"body_revision": appearance["body_revision"], "wardrobe_revision": appearance["wardrobe_revision"]}
	if state, known := appearance["wearing_state"]; known {
		result["wearing_state"] = state
	}
	fields := map[string]any{}
	allowed := []string{"skin_tone"}
	if scope == "face" || scope == "back" || scope == "upper_body" {
		allowed = append(allowed, "hair_length", "hair_color", "hair_style")
	}
	if scope == "face" || scope == "upper_body" {
		allowed = append(allowed, "eye_color")
		if scope == "upper_body" {
			allowed = append(allowed, "body_type", "chest_cup")
		}
	}
	for _, key := range allowed {
		if value, ok := mapValue(appearance["body_fields"])[key]; ok {
			fields[key] = value
		}
	}
	result["body_fields"] = fields
	worn := []any{}
	for _, raw := range arrayValue(appearance["worn_items"]) {
		item := mapValue(raw)
		slot := stringValue(item["slot"])
		if scope == "back" || (scope == "face" || scope == "upper_body") && (slot == "top" || slot == "outerwear" || slot == "head") || scope == "feet" && (slot == "shoes" || slot == "footwear" || slot == "bottom" || slot == "legs") || scope == "hands" && (slot == "hands" || slot == "gloves") {
			worn = append(worn, cloneMap(item))
		}
	}
	result["worn_items"] = worn
	return result
}

func mediaContextForPlan(plan map[string]any, resolved CapabilityContext) map[string]any {
	binding := map[string]any{}
	if mediaPlanNeedsAppearance(plan) {
		if resolved.Outfit != nil {
			binding["appearance"] = projectMediaAppearance(resolved.Outfit.Data, mediaAppearanceProjectionScope(plan))
		}
		if resolved.Life != nil {
			binding["current_life"] = cloneMap(resolved.Life.Data)
		}
	}
	if mediaPlanHasSelfIdentity(plan) && resolved.Visual != nil {
		binding["visual_identity"] = projectMediaVisualIdentity(resolved.Visual.Data, stringValue(plan["visible_scope"]))
	}
	return binding
}

func validateBranchedMediaConcept(concept map[string]any) error {
	if !isBranchedMedia(concept) {
		return errors.New("media_plan_version_invalid")
	}
	if concept["source"] == "studio_definition" {
		if err := validateStudioMediaConcept(concept); err != nil {
			return err
		}
	}
	plan := mapValue(concept["media_capture_plan"])
	if count, known := concept["human_count"]; known {
		n, validCount := mediaHumanCountValue(count)
		if !validCount || n < 0 || n > 8 || n == 0 && plan["human_presence"] != "none" || n > 0 && plan["human_presence"] == "none" || len(arrayValue(concept["subject_contexts"])) > n {
			return errors.New("media_human_count_conflict")
		}
	}
	if err := validateMediaCapturePlan(plan); err != nil {
		return err
	}
	binding := mapValue(concept["context_binding"])
	for key := range binding {
		if key != "appearance" && key != "visual_identity" && key != "current_life" {
			return errors.New("media_context_source_forbidden")
		}
	}
	if !mediaPlanNeedsAppearance(plan) && len(binding) > 0 {
		return errors.New("media_context_person_forbidden")
	}
	if !mediaPlanHasSelfIdentity(plan) && len(mapValue(binding["visual_identity"])) > 0 {
		return errors.New("media_context_identity_forbidden")
	}
	if mediaPlanNeedsAppearance(plan) && len(mapValue(binding["appearance"])) == 0 {
		return errors.New("media_appearance_missing")
	}
	if mediaPlanHasSelfIdentity(plan) {
		if len(mapValue(binding["visual_identity"])) == 0 {
			return errors.New("media_identity_missing")
		}
		if mapValue(binding["visual_identity"])["status"] != "active" {
			return errors.New("identity_pending")
		}
	}
	if reference := mapValue(concept["reference_asset"]); len(reference) > 0 {
		if reference["purpose"] == "subject" {
			if _, err := mediaReferenceOwner("self", reference, concept); err != nil {
				return err
			}
		} else if purpose := reference["purpose"]; purpose != "animal" && purpose != "scene" && purpose != "object" && !(purpose == "person" && stringValue(reference["grant_id"]) != "" && (concept["source"] == "studio_definition" || stringValue(mapValue(concept["owner_media_request"])["request_id"]) != "" && !mediaPlanHasSelfIdentity(plan))) {
			return errors.New("media_reference_purpose_invalid")
		}
	}
	if err := validateMediaSubjectContexts(concept); err != nil {
		return err
	}
	if scope := mediaAppearanceProjectionScope(plan); scope != "full" && mediaPlanNeedsAppearance(plan) {
		if jsonString(projectMediaAppearance(mapValue(binding["appearance"]), scope)) != jsonString(mapValue(binding["appearance"])) {
			return errors.New("media_context_scope_forbidden")
		}
	}
	for _, key := range []string{"visual_identity", "renderer_constraints", "appearance", "current_state", "core_persona"} {
		if _, exists := concept[key]; exists {
			return errors.New("media_context_root_forbidden")
		}
	}
	if strings.TrimSpace(stringValue(concept["intent"])) == "" {
		return errors.New("media_intent_missing")
	}
	return nil
}

// Only an unsubmitted legacy job is upgraded. Accepted provider jobs retain
// their original frozen execution and assets; completed history is immutable.
func (a *App) upgradeLegacyMediaConcept(ctx context.Context, intent mediaIntent, legacy map[string]any) (map[string]any, error) {
	text := strings.TrimSpace(stringValue(legacy["intent"]))
	if version := stringValue(legacy["media_plan_version"]); version != "" {
		return nil, newCapabilityError("media_plan_version_unsupported", false, nil)
	}
	if text == "" {
		return nil, errors.New("media_legacy_intent_missing")
	}
	explicit := explicitMediaPlan(map[string]any{"capture": legacy["capture"]})
	plan, sources, err := ResolveMediaCapturePlan(withMediaCaptureHints(ctx, mapValue(legacy["capture"])), a.kevService(), decision.Scope{ActorSelf: intent.Owner, Agent: "media_prompt", RunID: intent.ID, CorrelationID: "media:" + intent.ID}, text, explicit, a.parseMediaPlan)
	if err != nil {
		return nil, err
	}
	binding := mapValue(legacy["context_binding"])
	resolved := CapabilityContext{}
	if len(mapValue(binding["appearance"])) > 0 {
		resolved.Outfit = &AppearanceContext{Data: mapValue(binding["appearance"])}
	}
	if len(mapValue(binding["visual_identity"])) > 0 {
		resolved.Visual = &VisualIdentityContext{Data: mapValue(binding["visual_identity"])}
	}
	if len(mapValue(binding["current_life"])) > 0 {
		resolved.Life = &CurrentLifeContext{Data: mapValue(binding["current_life"])}
	}
	concept := map[string]any{"intent": text, "media_plan_version": mediaPlanVersion, "media_capture_plan": plan, "explicit_constraints": explicit, "field_sources": sources, "context_binding": mediaContextForPlan(plan, resolved), "source": "legacy_unsubmitted", "capture": cloneMap(mapValue(legacy["capture"]))}
	if err := validateBranchedMediaConcept(concept); err != nil {
		return nil, err
	}
	return concept, nil
}

func projectMediaVisualIdentity(visual map[string]any, scope string) map[string]any {
	result := cloneMap(visual)
	if scope == "full" {
		return result
	}
	delete(result, "renderer_constraints")
	snapshot := mapValue(result["identity_snapshot"])
	identity := mapValue(snapshot["identity"])
	filtered := compactStateMap(identity, []string{"name", "face_shape", "facial_features", "eye_color", "skin_tone"})
	if appearance := mapValue(identity["appearance"]); len(appearance) > 0 {
		filtered["appearance"] = compactStateMap(appearance, []string{"face_shape", "facial_features", "eye_color", "skin_tone"})
	}
	result["identity_snapshot"] = map[string]any{"identity": filtered}
	return result
}

func mediaAppearanceProjectionScope(plan map[string]any) string {
	scope := stringValue(plan["visible_scope"])
	if scope == "full" && plan["framing"] == "half_body" {
		return "upper_body"
	}
	return scope
}
