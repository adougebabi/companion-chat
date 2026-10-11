package core

import (
	"context"
	"errors"
	"strings"
)

func mediaWorkflowBranch(plan map[string]any) string {
	if plan["human_presence"] == "none" {
		return "none"
	}
	if plan["human_presence"] == "unknown" || plan["self_in_frame"] == "unknown" {
		return "unknown"
	}
	if plan["human_presence"] == "partial" || plan["self_in_frame"] == "yes" && (plan["visible_scope"] == "hands" || plan["visible_scope"] == "feet") {
		return "partial"
	}
	if plan["self_in_frame"] == "no" {
		return "other"
	}
	if scope := stringValue(plan["visible_scope"]); scope == "face" || scope == "back" {
		return scope
	}
	return "self"
}

// Fixed workflow values are not inferred from their names. A style LoRA must
// be explicitly declared in the Owner configuration; unclassified adapters
// cannot silently apply self identity to a non-self photograph.
func validateMediaWorkflowPolicy(config, concept, workflow map[string]any) error {
	if err := validateBranchedMediaConcept(concept); err != nil {
		return err
	}
	if err := validateCurrentCaptureWorkflowText(workflow); err != nil {
		return err
	}
	if err := validateMediaNegativeWorkflow(config, workflow); err != nil {
		return err
	}
	identityAllowed := mediaPlanHasSelfIdentity(mapValue(concept["media_capture_plan"]))
	bodyAllowed := identityAllowed && mapValue(concept["media_capture_plan"])["visible_scope"] == "full"
	faceLoras := map[string]bool{}
	for _, raw := range arrayValue(config["face_identity_loras"]) {
		faceLoras[stringValue(raw)] = true
	}
	styles := map[string]bool{}
	for _, raw := range arrayValue(config["style_loras"]) {
		styles[stringValue(raw)] = true
	}
	for _, raw := range workflow {
		node := mapValue(raw)
		inputs := mapValue(node["inputs"])
		class := strings.ToLower(stringValue(node["class_type"]))
		if strings.Contains(class, "loadimage") {
			image := inputs["image"]
			if image == "{{visual_identity_reference_image}}" {
				if !identityAllowed {
					return errors.New("media_workflow_identity_reference_forbidden")
				}
			} else if image == "{{media_reference_image}}" {
				if len(mapValue(concept["reference_asset"])) == 0 {
					return errors.New("media_workflow_reference_missing")
				}
			} else {
				return errors.New("media_workflow_fixed_reference_forbidden")
			}
		}

		if bodyAllowed && strings.Contains(class, "lora") && !styles[stringValue(inputs["lora_name"])] && !faceLoras[stringValue(inputs["lora_name"])] {
			dynamicBody := false
			for _, value := range inputs {
				if text, ok := value.(string); ok && (text == "{{chest_lora_weight}}" || text == "{{renderer_constraints.chest_lora_weight}}") {
					dynamicBody = true
				}
			}
			if !dynamicBody {
				return errors.New("media_workflow_unclassified_lora_forbidden")
			}
			if mediaRendererConstraints(concept)["chest_lora_weight"] == nil {
				return errors.New("media_workflow_current_body_constraint_missing")
			}
		}
		if !bodyAllowed {
			if strings.Contains(class, "lora") && !styles[stringValue(inputs["lora_name"])] && !(identityAllowed && faceLoras[stringValue(inputs["lora_name"])]) {
				return errors.New("media_workflow_unclassified_lora_forbidden")
			}
			for _, value := range inputs {
				if text, ok := value.(string); ok && (strings.Contains(text, "{{chest_lora_weight}}") || strings.Contains(text, "{{renderer_constraints.chest_lora_weight}}")) {
					return errors.New("media_workflow_body_condition_forbidden")
				}
			}
		}
		if !identityAllowed {
			if strings.Contains(class, "ipadapter") || strings.Contains(class, "instantid") || strings.Contains(class, "pulid") || strings.Contains(class, "faceid") {
				return errors.New("media_workflow_identity_condition_forbidden")
			}
			for _, value := range inputs {
				if text, ok := value.(string); ok && (strings.Contains(text, "{{visual_identity_reference_image}}") || strings.Contains(text, "{{chest_lora_weight}}") || strings.Contains(text, "{{renderer_constraints.chest_lora_weight}}")) {
					return errors.New("media_workflow_self_condition_forbidden")
				}
			}
		}
	}
	return nil
}

// Reuse the existing positive graph validation without its assumption that
// every LoadImage is necessarily the task actor's identity reference.
func validateCurrentCaptureWorkflowText(workflow map[string]any) error {
	textOnly := cloneMap(workflow)
	for _, raw := range textOnly {
		node := mapValue(raw)
		if strings.Contains(strings.ToLower(stringValue(node["class_type"])), "loadimage") {
			inputs := cloneMap(mapValue(node["inputs"]))
			inputs["image"] = "{{visual_identity_reference_image}}"
			node["inputs"] = inputs
		}
	}
	return validateCurrentCaptureWorkflow(textOnly)
}

func validateFinalMediaWorkflow(concept, workflow map[string]any, prompt string) error {
	if !isBranchedMedia(concept) {
		return nil
	}
	if err := validateMediaNormalization(concept, mapValue(concept["normalized_output"])); err != nil {
		return err
	}
	if prompt != stringValue(mapValue(concept["normalized_output"])["prompt"]) {
		return errors.New("media_prompt_cache_conflict")
	}
	found := false
	var walk func(any) error
	walk = func(value any) error {
		switch v := value.(type) {
		case string:
			if strings.Contains(v, "{{") {
				return errors.New("media_workflow_placeholder_unresolved")
			}
			if v == prompt {
				found = true
			}
		case map[string]any:
			for _, child := range v {
				if err := walk(child); err != nil {
					return err
				}
			}
		case []any:
			for _, child := range v {
				if err := walk(child); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := walk(workflow); err != nil {
		return err
	}
	if !found {
		return errors.New("media_workflow_prompt_missing")
	}
	return nil
}

func replaceMediaReferenceAlias(workflow map[string]any, filename string) map[string]any {
	result := cloneMap(workflow)
	for _, raw := range result {
		node := mapValue(raw)
		inputs := mapValue(node["inputs"])
		if inputs["image"] == "{{media_reference_image}}" {
			inputs["image"] = filename
		}
	}
	return result
}

func validateMediaReferenceSource(reference, source map[string]any) error {
	purpose := stringValue(reference["purpose"])
	if purpose != "scene" && purpose != "animal" && purpose != "object" {
		return errors.New("media_reference_purpose_invalid")
	}
	if !isBranchedMedia(source) || source["purpose"] == "visual_identity" {
		return errors.New("media_reference_subject_unverified")
	}
	plan := mapValue(source["media_capture_plan"])
	if plan["human_presence"] != "none" || plan["self_in_frame"] != "no" {
		return errors.New("media_reference_person_forbidden")
	}
	subject := stringValue(plan["primary_subject"])
	if purpose == "scene" && (subject == "landscape" || subject == "other") || purpose == "animal" && subject == "animal" || purpose == "object" && (subject == "object" || subject == "mixed") {
		return nil
	}
	return errors.New("media_reference_subject_mismatch")
}

func (a *App) validateMediaReferenceAsset(ctx context.Context, owner string, reference, concept map[string]any) error {
	referenceOwner, err := mediaReferenceOwner(owner, reference, concept)
	if err != nil {
		return err
	}
	var raw string
	err = a.DB.Pool().QueryRow(ctx, `SELECT i.prompt FROM public.media_assets a JOIN public.media_intents i ON a.id='asset_'||i.id WHERE a.id=$1 AND a.owner_fluctlight_id=$2 AND a.status='ready'`, stringValue(reference["asset_id"]), referenceOwner).Scan(&raw)
	if err != nil {
		return errors.New("media_reference_source_unavailable")
	}
	source := decodeObject([]byte(raw))
	if reference["purpose"] == "subject" {
		if source["purpose"] == "visual_identity" {
			return nil
		}
		plan := mapValue(source["media_capture_plan"])
		if !isBranchedMedia(source) || !mediaPlanHasSelfIdentity(plan) || len(arrayValue(source["subject_contexts"])) > 0 {
			return errors.New("media_subject_reference_unverified")
		}
		for _, id := range arrayValue(mapValue(source["normalized_output"])["used_subject_ids"]) {
			if id != "self" {
				return errors.New("media_subject_reference_group_forbidden")
			}
		}
		return nil
	}
	return validateMediaReferenceSource(reference, source)
}

func mediaReferenceOwner(owner string, reference, concept map[string]any) (string, error) {
	if reference["purpose"] != "subject" {
		return owner, nil
	}
	id := stringValue(reference["subject_id"])
	if (id == "self" || id != "" && (id == owner || id == stringValue(concept["task_actor_id"]))) && mediaPlanHasSelfIdentity(mapValue(concept["media_capture_plan"])) {
		return owner, nil
	}
	for _, raw := range arrayValue(concept["subject_contexts"]) {
		subject := mapValue(raw)
		if subject["actor_id"] == id && (subject["visible_scope"] == "full" || subject["visible_scope"] == "face") {
			return id, nil
		}
	}
	return "", errors.New("media_reference_subject_not_visible")
}

func validateMediaNegativeWorkflow(config, workflow map[string]any) error {
	quality := map[string]bool{"": true}
	for _, raw := range arrayValue(config["quality_negative_prompts"]) {
		quality[stringValue(raw)] = true
	}
	seen := map[string]bool{}
	var check func(string) error
	check = func(id string) error {
		if seen[id] {
			return nil
		}
		seen[id] = true
		inputs := mapValue(mapValue(workflow[id])["inputs"])
		for key, value := range inputs {
			if key == "text" || key == "prompt" {
				if text, ok := value.(string); ok && !quality[text] {
					return errors.New("media_workflow_negative_unclassified")
				}
			}
			if edge := arrayValue(value); len(edge) == 2 {
				if upstream, ok := edge[0].(string); ok {
					if err := check(upstream); err != nil {
						return err
					}
				}
			}
		}
		return nil
	}
	for _, raw := range workflow {
		inputs := mapValue(mapValue(raw)["inputs"])
		if edge := arrayValue(inputs["negative"]); len(edge) == 2 {
			if err := check(stringValue(edge[0])); err != nil {
				return err
			}
		}
	}
	return nil
}

// A canonical face reference may remain useful as identity, but using the
// entire image as sampler latent also carries historical hair and clothing.
func mediaWorkflowReferenceIsLatent(workflow map[string]any, placeholder string) bool {
	seen := map[string]bool{}
	var visit func(string) bool
	visit = func(id string) bool {
		if seen[id] {
			return false
		}
		seen[id] = true
		node := mapValue(workflow[id])
		inputs := mapValue(node["inputs"])
		if strings.Contains(strings.ToLower(stringValue(node["class_type"])), "loadimage") && inputs["image"] == placeholder {
			return true
		}
		for _, value := range inputs {
			if edge := arrayValue(value); len(edge) == 2 {
				if upstream, ok := edge[0].(string); ok && visit(upstream) {
					return true
				}
			}
		}
		return false
	}
	for _, raw := range workflow {
		inputs := mapValue(mapValue(raw)["inputs"])
		if edge := arrayValue(inputs["latent_image"]); len(edge) == 2 && visit(stringValue(edge[0])) {
			return true
		}
	}
	return false
}
func mediaReferenceSnapshotMatches(source, appearance map[string]any) bool {
	snapshot := mapValue(mapValue(source["context_binding"])["appearance"])
	if source["purpose"] == "visual_identity" {
		snapshot = mapValue(source["visual_identity"])
		if nested := mapValue(snapshot["identity_snapshot"]); len(nested) > 0 {
			snapshot = nested
		}
	}
	for _, key := range []string{"body_revision", "wardrobe_revision"} {
		expected, known := intValueExact(snapshot[key])
		actual, present := intValueExact(appearance[key])
		if !known || !present || expected != actual {
			return false
		}
	}
	return true
}
func (a *App) validateLatentIdentityReference(ctx context.Context, owner, asset string, appearance map[string]any) error {
	var raw string
	if err := a.DB.Pool().QueryRow(ctx, `SELECT i.prompt FROM public.media_assets a JOIN public.media_intents i ON a.id='asset_'||i.id WHERE a.id=$1 AND a.owner_fluctlight_id=$2 AND a.status='ready'`, asset, owner).Scan(&raw); err != nil {
		return errors.New("media_reference_capture_unverified")
	}
	if !mediaReferenceSnapshotMatches(decodeObject([]byte(raw)), appearance) {
		return errors.New("media_reference_current_state_conflict")
	}
	return nil
}
