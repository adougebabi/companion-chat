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
		"framing":    enumStringSchema("closeup", "upper_body", "full_body", "body_detail", "scene"),
		"pose":       enumStringSchema("standing", "seated", "walking", "resting", "raising_hand", "holding_used_item", "leaning", "lying", "kneeling"),
		"expression": enumStringSchema("neutral", "smiling", "thoughtful"),
		"lighting":   enumStringSchema("ambient", "soft", "daylight"),
		"style":      enumStringSchema("photographic", "illustrated"),
	}, []string{"framing", "pose", "expression", "lighting", "style"}, false)
}
func hasCurrentCapture(concept map[string]any) bool {
	return stringValue(concept["purpose"]) != "visual_identity" && len(mapValue(mapValue(concept["context_binding"])["appearance"])) > 0
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
	if err := validateCapabilitySchemaValue(plan, currentCapturePlanSchema()); err != nil {
		return "", fmt.Errorf("current_capture_plan_invalid: %w", err)
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
func currentCaptureCameraPhrases(concept, plan map[string]any) ([]string, error) {
	capture := mapValue(concept["capture"])
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
		aliases := map[string]string{"closeup": "closeup", "close-up": "closeup", "close up": "closeup", "face close-up": "closeup", "upper body": "upper_body", "upper_body": "upper_body", "full body": "full_body", "full_body": "full_body", "full-length": "full_body", "body detail": "body_detail", "body_detail": "body_detail", "scene": "scene", "全身": "full_body", "半身": "upper_body", "特写": "closeup"}
		framing := aliases[strings.ToLower(strings.TrimSpace(raw))]
		if framing == "" || framing != stringValue(plan["framing"]) {
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
