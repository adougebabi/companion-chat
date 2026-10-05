package core

import (
	"errors"
	"fmt"
	"strings"
)

// These are non-factual stylistic decisions. The model cannot supply body,
// clothing, objects or reference images through a free-text prompt section.
func currentCapturePlanSchema() map[string]any {
	return objectSchema(map[string]any{
		"framing":    enumStringSchema("closeup", "upper_body", "full_body", "body_detail", "scene", "full body", "upper body", "body detail", "close up", "close-up"),
		"pose":       enumStringSchema("standing", "seated", "walking", "resting", "raising_hand", "holding_used_item", "leaning", "lying", "kneeling"),
		"expression": enumStringSchema("neutral", "smiling", "thoughtful"),
		"lighting":   enumStringSchema("ambient", "soft", "daylight"),
		"style":      enumStringSchema("photographic", "illustrated"),
	}, []string{"framing", "pose", "expression", "lighting", "style"}, false)
}
func hasCurrentCapture(concept map[string]any) bool {
	return stringValue(concept["purpose"]) != "visual_identity" && len(mapValue(mapValue(concept["context_binding"])["appearance"])) > 0
}

func normalizeCurrentCapturePlan(concept, raw map[string]any) map[string]any {
	if raw == nil {
		raw = make(map[string]any)
	}
	plan := cloneMap(raw)
	// 1. Unwrap if wrapped in an outer object
	for _, wrapperKey := range []string{"current_capture_plan", "capture_plan", "capture", "plan", "data", "result", "properties"} {
		if inner, ok := plan[wrapperKey].(map[string]any); ok && len(inner) > 0 {
			for k, v := range inner {
				if _, exists := plan[k]; !exists {
					plan[k] = v
				}
			}
			delete(plan, wrapperKey)
		}
	}
	if plan["framing"] == nil && plan["pose"] == nil {
		for k, v := range plan {
			if inner, ok := v.(map[string]any); ok {
				if inner["framing"] != nil || inner["pose"] != nil {
					for ik, iv := range inner {
						if _, exists := plan[ik]; !exists {
							plan[ik] = iv
						}
					}
					delete(plan, k)
					break
				}
			}
		}
	}

	// 2. Normalize key casing and whitespace
	cleaned := make(map[string]any, len(plan))
	for k, v := range plan {
		cleaned[strings.ToLower(strings.TrimSpace(k))] = v
	}
	plan = cleaned

	framingAliases := map[string]string{
		"closeup": "closeup", "close-up": "closeup", "close up": "closeup", "face close-up": "closeup", "face closeup": "closeup", "特写": "closeup",
		"upper_body": "upper_body", "upper body": "upper_body", "upper-body": "upper_body", "半身": "upper_body", "上半身": "upper_body",
		"full_body": "full_body", "full body": "full_body", "full-body": "full_body", "full-length": "full_body", "full length": "full_body", "全身": "full_body",
		"body_detail": "body_detail", "body detail": "body_detail", "body-detail": "body_detail", "局部": "body_detail",
		"scene": "scene", "场景": "scene", "空镜": "scene",
	}

	// 3. Key alias mapping for framing
	if _, provided := plan["framing"]; !provided {
		for _, altKey := range []string{"frame", "shot", "composition", "framing_type", "view"} {
			if val := stringValue(plan[altKey]); val != "" {
				plan["framing"] = val
				break
			}
		}
	}
	if _, provided := plan["framing"]; !provided {
		for k := range plan {
			if canonical, ok := framingAliases[k]; ok {
				plan["framing"] = canonical
				break
			}
		}
	}

	// 4. Fallback from frozen concept if framing is still missing
	if _, provided := plan["framing"]; !provided {
		capture := mapValue(concept["capture"])
		if f := stringValue(capture["framing"]); f != "" {
			plan["framing"] = f
		} else if f := stringValue(concept["framing"]); f != "" {
			plan["framing"] = f
		}
	}

	// 5. Normalize framing value
	if rawFraming := stringValue(plan["framing"]); rawFraming != "" {
		normalized := strings.ToLower(strings.TrimSpace(rawFraming))
		if canonical, ok := framingAliases[normalized]; ok {
			plan["framing"] = canonical
		} else {
			cleanedVal := strings.ReplaceAll(strings.ReplaceAll(normalized, "-", "_"), " ", "_")
			if canonical, ok := framingAliases[cleanedVal]; ok {
				plan["framing"] = canonical
			}
		}
	}

	// 6. Normalize pose
	poseAliases := map[string]string{
		"standing": "standing", "stand": "standing", "站立": "standing",
		"seated": "seated", "sitting": "seated", "sit": "seated", "坐": "seated", "坐着": "seated",
		"walking": "walking", "walk": "walking", "走": "walking", "走路": "walking",
		"resting": "resting", "rest": "resting", "休息": "resting",
		"raising_hand": "raising_hand", "raising hand": "raising_hand", "raising-hand": "raising_hand", "举手": "raising_hand",
		"holding_used_item": "holding_used_item", "holding used item": "holding_used_item", "holding-used-item": "holding_used_item", "holding item": "holding_used_item",
		"leaning": "leaning", "lean": "leaning", "倚靠": "leaning", "靠着": "leaning",
		"lying": "lying", "lie": "lying", "躺": "lying", "躺着": "lying",
		"kneeling": "kneeling", "kneel": "kneeling", "跪": "kneeling", "跪着": "kneeling",
	}
	if rawPose := stringValue(plan["pose"]); rawPose != "" {
		normalized := strings.ToLower(strings.TrimSpace(rawPose))
		if canonical, ok := poseAliases[normalized]; ok {
			plan["pose"] = canonical
		} else {
			cleanedVal := strings.ReplaceAll(strings.ReplaceAll(normalized, "-", "_"), " ", "_")
			if canonical, ok := poseAliases[cleanedVal]; ok {
				plan["pose"] = canonical
			}
		}
	}
	if stringValue(plan["pose"]) == "" {
		plan["pose"] = "standing"
	}

	// 7. Normalize expression
	expressionAliases := map[string]string{
		"neutral": "neutral", "自然": "neutral", "平静": "neutral",
		"smiling": "smiling", "smile": "smiling", "微笑": "smiling", "笑": "smiling",
		"thoughtful": "thoughtful", "沉思": "thoughtful", "若有所思": "thoughtful",
	}
	if rawExpression := stringValue(plan["expression"]); rawExpression != "" {
		normalized := strings.ToLower(strings.TrimSpace(rawExpression))
		if canonical, ok := expressionAliases[normalized]; ok {
			plan["expression"] = canonical
		}
	}
	if stringValue(plan["expression"]) == "" {
		plan["expression"] = "neutral"
	}

	// 8. Normalize lighting
	lightingAliases := map[string]string{
		"ambient": "ambient", "ambient light": "ambient", "ambient_light": "ambient", "环境光": "ambient",
		"soft": "soft", "soft light": "soft", "soft_light": "soft", "柔光": "soft",
		"daylight": "daylight", "day light": "daylight", "day_light": "daylight", "自然光": "daylight", "日光": "daylight",
	}
	if rawLighting := stringValue(plan["lighting"]); rawLighting != "" {
		normalized := strings.ToLower(strings.TrimSpace(rawLighting))
		if canonical, ok := lightingAliases[normalized]; ok {
			plan["lighting"] = canonical
		}
	}
	if stringValue(plan["lighting"]) == "" {
		plan["lighting"] = "ambient"
	}

	// 9. Normalize style
	styleAliases := map[string]string{
		"photographic": "photographic", "photo": "photographic", "photography": "photographic", "写实": "photographic", "摄影": "photographic",
		"illustrated": "illustrated", "illustration": "illustrated", "插画": "illustrated",
	}
	if rawStyle := stringValue(plan["style"]); rawStyle != "" {
		normalized := strings.ToLower(strings.TrimSpace(rawStyle))
		if canonical, ok := styleAliases[normalized]; ok {
			plan["style"] = canonical
		}
	}
	if stringValue(plan["style"]) == "" {
		plan["style"] = "photographic"
	}

	// Consume only documented key aliases. Unexpected fields remain visible
	// to structural validation; a style fallback cannot hide body/outfit input.
	finalPlan := cloneMap(plan)
	for _, alias := range []string{"frame", "shot", "composition", "framing_type", "view"} {
		delete(finalPlan, alias)
	}
	for alias := range framingAliases {
		delete(finalPlan, alias)
	}
	// A canonical framing key is not a framing value alias.
	for _, key := range []string{"framing", "pose", "expression", "lighting", "style"} {
		if value, exists := plan[key]; exists {
			finalPlan[key] = value
		}
	}

	return finalPlan
}

func currentCaptureEnumInstruction() string {
	return "Allowed values (use exactly one value for each field): " + jsonString(mapValue(currentCapturePlanSchema()["properties"])) + ". Prefer canonical framing tokens closeup, upper_body, full_body, body_detail, scene. If framing is unspecified and you are unsure, choose full_body; an explicit supported frozen framing takes priority. Never invent framing tokens or supply body, clothes, objects, references, prompt prose, or extra fields."
}

func resolveCurrentCapturePlan(concept, raw map[string]any) (map[string]any, map[string]any, error) {
	plan := normalizeCurrentCapturePlan(concept, raw)
	schema := currentCapturePlanSchema()
	properties := mapValue(schema["properties"])
	for key := range plan {
		if _, allowed := properties[key]; !allowed {
			return nil, nil, fmt.Errorf("current_capture_plan_invalid: unexpected field %q", key)
		}
	}
	defaults := map[string]any{"framing": "full_body", "pose": "standing", "expression": "neutral", "lighting": "ambient", "style": "photographic"}
	invalid := []any{}
	for _, key := range []string{"framing", "pose", "expression", "lighting", "style"} {
		valid := false
		if value, ok := plan[key].(string); ok {
			for _, choice := range arrayValue(mapValue(properties[key])["enum"]) {
				if value == stringValue(choice) {
					valid = true
					break
				}
			}
		}
		if !valid {
			invalid = append(invalid, key)
		} else if key != "framing" {
			defaults[key] = plan[key]
		}
	}
	fallback := mapValue(concept["capture_plan_fallback"])
	if len(invalid) > 0 {
		// The user-authorized recovery is specifically a framing fallback. A
		// free-text pose/style can contain factual overrides and must stay invalid.
		for _, key := range invalid {
			if key != "framing" {
				return nil, nil, fmt.Errorf("current_capture_plan_invalid: field %q: value is not in enum", key)
			}
		}
		plan = defaults
		fallback = map[string]any{"mode": "first_person", "framing": "full_body", "reason_code": "invalid_framing_enum", "invalid_fields": invalid}
	}
	if len(fallback) > 0 && (fallback["mode"] != "first_person" || fallback["framing"] != "full_body" || fallback["reason_code"] != "invalid_framing_enum" || plan["framing"] != "full_body") {
		return nil, nil, errors.New("current_capture_fallback_invalid")
	}
	if err := validateCapabilitySchemaValue(plan, schema); err != nil {
		return nil, nil, fmt.Errorf("current_capture_plan_invalid: %w", err)
	}
	return plan, fallback, nil
}

func validateCurrentCaptureSnapshot(concept map[string]any) error {
	if !hasCurrentCapture(concept) {
		return nil
	}
	appearance := mapValue(mapValue(concept["context_binding"])["appearance"])
	for _, key := range []string{"body_revision", "wardrobe_revision"} {
		value, ok := intValueExact(appearance[key])
		if !ok || value < 0 {
			return errors.New("current_capture_snapshot_missing")
		}
	}
	if appearance["wearing_state"] != "known" {
		return errors.New("current_capture_wearing_unknown")
	}
	if _, exists := appearance["worn_items"]; !exists {
		return errors.New("current_capture_wearing_unknown")
	}
	slots := map[string]bool{}
	for _, raw := range arrayValue(appearance["worn_items"]) {
		item := mapValue(raw)
		slot := stringValue(item["slot"])
		if stringValue(item["id"]) == "" || stringValue(item["description"]) == "" || slot == "" || slots[slot] || item["availability"] != "available" || item["source_verified"] != true {
			return errors.New("current_capture_wearing_conflict")
		}
		slots[slot] = true
	}
	return nil
}
func renderCurrentCapturePrompt(concept, plan map[string]any) (string, error) {
	if err := validateCurrentCaptureSnapshot(concept); err != nil {
		return "", err
	}
	var fallback map[string]any
	var err error
	plan, fallback, err = resolveCurrentCapturePlan(concept, plan)
	if err != nil {
		return "", err
	}
	if len(fallback) > 0 {
		concept = cloneMap(concept)
		concept["capture_plan_fallback"] = fallback
	}

	// Every phrase below is either a validated model enum or an authoritative
	// snapshot value. The user's wish and a model-written outfit never enter.
	phrases := []string{map[string]string{"photographic": "A photographic image.", "illustrated": "An illustrated image."}[stringValue(plan["style"])],
		map[string]string{"closeup": "A close-up composition.", "upper_body": "An upper-body composition.", "full_body": "A full-body composition.", "body_detail": "A partial-body detail composition.", "scene": "A composition of the current setting without a person."}[stringValue(plan["framing"])],
		map[string]string{"standing": "The subject is standing.", "seated": "The subject is seated.", "walking": "The subject is walking.", "resting": "The subject is resting.", "raising_hand": "The subject is raising a hand.", "holding_used_item": "The subject holds the object in actual use.", "leaning": "The subject is leaning.", "lying": "The subject is lying down.", "kneeling": "The subject is kneeling."}[stringValue(plan["pose"])],
		map[string]string{"neutral": "A neutral expression.", "smiling": "A smiling expression.", "thoughtful": "A thoughtful expression."}[stringValue(plan["expression"])],
		map[string]string{"ambient": "Ambient light.", "soft": "Soft light.", "daylight": "Daylight."}[stringValue(plan["lighting"])],
	}
	capture, err := currentCaptureCameraPhrases(concept, plan)
	if err != nil {
		return "", err
	}
	phrases = append(phrases, capture...)
	if plan["framing"] == "scene" {
		scene := stringValue(mapValue(mapValue(concept["context_binding"])["current_life"])["scene"])
		return strings.Join([]string{phrases[0], "A view of the current setting: " + scene + ". No person in the composition.", phrases[4]}, " "), nil
	}
	binding := mapValue(concept["context_binding"])
	safeVisual := compactVisualIdentityForCurrentMedia(mapValue(binding["visual_identity"]))
	identity := mapValue(mapValue(safeVisual["identity_snapshot"])["identity"])
	for _, key := range []string{"gender", "age", "ethnicity", "face_shape", "eye_color", "skin_tone"} {
		if value, exists := identity[key]; exists {
			phrases = append(phrases, fmt.Sprintf("Identity %s: %v.", key, value))
		}
	}
	for _, source := range []map[string]any{mapValue(identity["appearance"]), mapValue(mapValue(mapValue(safeVisual["identity_snapshot"])["life_profile"])["appearance"])} {
		for _, key := range []string{"face_shape", "facial_features", "eye_color", "skin_tone"} {
			if value, exists := source[key]; exists {
				phrases = append(phrases, fmt.Sprintf("Identity %s: %v.", key, value))
			}
		}
	}
	appearance := mapValue(binding["appearance"])
	clothes := make([]string, 0)
	for _, raw := range arrayValue(appearance["worn_items"]) {
		clothes = append(clothes, stringValue(mapValue(raw)["description"]))
	}
	if len(clothes) > 0 {
		phrases = append(phrases, "The subject wears "+strings.Join(clothes, " and ")+".")
	}
	fields := mapValue(appearance["body_fields"])
	for _, field := range []struct{ key, label string }{{"hair_length", "Hair length"}, {"hair_color", "Hair color"}, {"hair_style", "Hair arrangement"}, {"body_type", "Body type"}, {"height", "Height"}, {"chest_cup", "Chest cup"}, {"eye_color", "Eye color"}, {"skin_tone", "Skin tone"}} {
		state := mapValue(fields[field.key])
		if state["status"] == "known" {
			phrases = append(phrases, field.label+" is "+stringValue(state["value"])+".")
		}
	}
	life := mapValue(binding["current_life"])
	if scene := stringValue(life["scene"]); scene != "" {
		phrases = append(phrases, "The current setting is "+scene+".")
	}
	used := arrayValue(appearance["used_items"])
	if plan["pose"] == "holding_used_item" && len(used) == 0 {
		return "", errors.New("current_capture_used_item_missing")
	}
	for _, raw := range used {
		item := mapValue(raw)
		if stringValue(item["id"]) == "" || stringValue(item["description"]) == "" || item["source_verified"] != true {
			return "", errors.New("current_capture_used_item_invalid")
		}
		phrases = append(phrases, "The subject uses "+stringValue(item["description"])+".")
	}
	return strings.Join(phrases, " "), nil
}

func currentCaptureReferenceCompatible(concept map[string]any) bool {
	binding := mapValue(concept["context_binding"])
	visual := mapValue(binding["visual_identity"])
	snapshot := mapValue(visual["identity_snapshot"])
	appearance := mapValue(binding["appearance"])
	if snapshot["source"] != "effective_life" {
		return false
	}
	for _, key := range []string{"body_revision", "wardrobe_revision"} {
		expected, ok := intValueExact(snapshot[key])
		actual, exists := intValueExact(appearance[key])
		if !ok || !exists || expected != actual {
			return false
		}
	}
	return true
}

func appearanceSnapshotIdentity(value map[string]any) string {
	return jsonString(compactStateMap(value, []string{"body_revision", "wardrobe_revision", "body_fields", "wearing_state", "worn_items", "used_items"}))
}

// Explicit camera semantics come from the frozen Tool DTO. Free framing/angle
// strings are never concatenated into a renderer instruction.
func effectiveCurrentCaptureCamera(concept map[string]any) (map[string]any, error) {
	capture := mapValue(concept["capture"])
	fallback := mapValue(concept["capture_plan_fallback"])
	if len(fallback) == 0 {
		return capture, nil
	}
	if fallback["mode"] != "first_person" || fallback["framing"] != "full_body" || fallback["reason_code"] != "invalid_framing_enum" {
		return nil, errors.New("current_capture_fallback_invalid")
	}
	capture = cloneMap(capture)
	if capture == nil {
		capture = map[string]any{}
	}
	capture["mode"] = "first_person"
	capture["framing"] = "full_body"
	capture["camera"] = "rear"
	capture["device_visibility"] = "hidden"
	delete(capture, "mirror")
	return capture, nil
}

func currentCaptureCameraPhrases(concept, plan map[string]any) ([]string, error) {
	capture, err := effectiveCurrentCaptureCamera(concept)
	if err != nil {
		return nil, err
	}
	mode := firstString(capture["mode"], "selfie")
	modes := map[string]string{
		"selfie":           "A handheld phone self-capture; the phone camera defines the viewpoint.",
		"mirror_selfie":    "A self-capture through a mirror, showing the subject's reflection.",
		"external_capture": "An external photograph; the photographer and their camera remain outside the image.",
		"operator_pov":     "The operator's point of view; the operator remains outside the image.",
		"first_person":     "The subject's first-person view, without an invented observer.",
	}
	phrase, valid := modes[mode]
	if !valid {
		return nil, errors.New("current_capture_camera_invalid")
	}
	result := []string{phrase}
	if raw := stringValue(capture["framing"]); raw != "" {
		aliases := map[string]string{
			"closeup": "closeup", "close-up": "closeup", "close up": "closeup", "face close-up": "closeup", "face closeup": "closeup", "特写": "closeup",
			"upper_body": "upper_body", "upper body": "upper_body", "upper-body": "upper_body", "半身": "upper_body", "上半身": "upper_body",
			"full_body": "full_body", "full body": "full_body", "full-body": "full_body", "full-length": "full_body", "full length": "full_body", "全身": "full_body",
			"body_detail": "body_detail", "body detail": "body_detail", "body-detail": "body_detail", "局部": "body_detail",
			"scene": "scene", "场景": "scene", "空镜": "scene",
		}
		framing := aliases[strings.ToLower(strings.TrimSpace(raw))]
		planFraming := aliases[strings.ToLower(strings.TrimSpace(stringValue(plan["framing"])))]
		if planFraming == "" {
			planFraming = stringValue(plan["framing"])
		}
		if framing == "" || framing != planFraming {
			return nil, errors.New("current_capture_framing_conflict")
		}
	}
	if raw := stringValue(capture["angle"]); raw != "" {
		angles := map[string]string{"front": "Viewed from the front.", "side": "Viewed from the side.", "rear": "Viewed from behind.", "high": "A high camera angle.", "low": "A low camera angle.", "eye_level": "An eye-level camera angle."}
		phrase, ok := angles[strings.ToLower(strings.TrimSpace(raw))]
		if !ok {
			return nil, errors.New("current_capture_angle_unsupported")
		}
		result = append(result, phrase)
	}
	camera := stringValue(capture["camera"])
	if camera == "front" {
		result = append(result, "The phone's front camera is used.")
	} else if camera == "rear" {
		result = append(result, "The phone's rear camera is used.")
	} else if camera != "" && camera != "external" {
		return nil, errors.New("current_capture_camera_invalid")
	}
	if capture["device_visibility"] == "visible" {
		result = append(result, "The capture device is visible only where physically consistent with this camera relationship.")
	} else {
		result = append(result, "Keep the capture device outside the visible composition.")
	}
	return result, nil
}

// Guard the workflow as well as the model plan: an owner template cannot
// append arbitrary positive clothing text or select an unverified input image.
func validateCurrentCaptureWorkflow(workflow map[string]any) error {
	found := false
	var check func(any) error
	check = func(value any) error {
		switch v := value.(type) {
		case string:
			if strings.Contains(v, "{{prompt}}") {
				if v != "{{prompt}}" {
					return errors.New("current_capture_workflow_prompt_override")
				}
				found = true
			}
		case map[string]any:
			if strings.Contains(strings.ToLower(stringValue(v["class_type"])), "loadimage") {
				if image := stringValue(mapValue(v["inputs"])["image"]); image != "{{visual_identity_reference_image}}" {
					return errors.New("current_capture_workflow_reference_override")
				}
			}
			for _, child := range v {
				if err := check(child); err != nil {
					return err
				}
			}
		case []any:
			for _, child := range v {
				if err := check(child); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := check(workflow); err != nil {
		return err
	}
	if !found {
		return errors.New("current_capture_workflow_prompt_missing")
	}
	// Follow actual sampler positive conditioning edges. A separate negative
	// encoder retains the configured quality exclusions, but cannot become a
	// positive encoder with hard-coded apparel.
	visited := map[string]bool{}
	var positive func(string) error
	positive = func(id string) error {
		if visited[id] {
			return nil
		}
		visited[id] = true
		node := mapValue(workflow[id])
		inputs := mapValue(node["inputs"])
		for key, value := range inputs {
			if key == "text" || key == "prompt" {
				if text, ok := value.(string); ok && text != "{{prompt}}" {
					return errors.New("current_capture_workflow_positive_override")
				}
			}
			if edge := arrayValue(value); len(edge) == 2 {
				if upstream, ok := edge[0].(string); ok {
					if err := positive(upstream); err != nil {
						return err
					}
				}
			}
		}
		return nil
	}
	for _, raw := range workflow {
		node := mapValue(raw)
		inputs := mapValue(node["inputs"])
		if edge := arrayValue(inputs["positive"]); len(edge) == 2 {
			if err := positive(stringValue(edge[0])); err != nil {
				return err
			}
		}
	}
	return nil
}
