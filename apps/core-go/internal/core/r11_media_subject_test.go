package core

import (
	"context"
	"errors"
	"testing"
)

func TestMediaSelfAliasesUseOnlyTheAuthenticatedTaskActor(t *testing.T) {
	for _, alias := range []string{"self", "actor_self", "actual-self"} {
		args := map[string]any{"visible_subjects": []any{map[string]any{"actor_id": alias, "visible_scope": "face"}}}
		plan := map[string]any{"self_in_frame": "yes", "human_presence": "present", "visible_scope": "face"}
		if err := explicitMediaSubjects(args, "actual-self", plan); err != nil {
			t.Fatal(alias, err)
		}
		// A self-only request must not query another Actor's authority or context.
		invocation := CapabilityInvocation{}
		invocation.Metadata.FluctlightID = "actual-self"
		contexts, err := (&App{}).readMediaSubjectContexts(context.Background(), invocation, arrayValue(args["visible_subjects"]))
		if err != nil || len(contexts) != 0 {
			t.Fatal(alias, contexts, err)
		}
		conflict := map[string]any{"self_in_frame": "no"}
		if explicitMediaSubjects(args, "actual-self", conflict) == nil {
			t.Fatal("self visibility contradiction admitted", alias)
		}
	}
	if mediaSubjectActorID("actor_user", "actual-self") != "actor_user" || mediaSubjectActorID("foreign", "actual-self") != "foreign" || mediaSubjectActorID("actor_self", "") != "actor_self" {
		t.Fatal("self binding widened another subject or invented a task actor")
	}
	duplicates := map[string]any{"visible_subjects": []any{map[string]any{"actor_id": "self", "visible_scope": "face"}, map[string]any{"actor_id": "actor_self", "visible_scope": "face"}}}
	if explicitMediaSubjects(duplicates, "actual-self", map[string]any{}) == nil {
		t.Fatal("two aliases for the same subject admitted as two people")
	}
}

func TestMediaSubjectRejectionIsConsumableToolFeedback(t *testing.T) {
	err := newCapabilityError("media_subject_not_authorized", false, errors.New("media_subject_not_authorized"))
	code, retryable, _ := classifyToolPrepareError(err)
	if code != "media_subject_not_authorized" || retryable {
		t.Fatal("definite subject rejection would terminate the native loop as a transient prepare failure", code, retryable)
	}
	code, retryable, _ = classifyToolPrepareError(errors.New("database unavailable"))
	if code != "capability_prepare_failed" || !retryable {
		t.Fatal("dependency outage incorrectly treated as a correctable model argument", code, retryable)
	}
}

func TestMediaSelfAliasPreparesTheNativeNullableSelfieShape(t *testing.T) {
	args := map[string]any{
		"intent":    "Selfie regression with the task actor's confirmed face",
		"draft_ref": nil, "media_request_ref": nil, "reference_asset": nil,
		"human_count":       1,
		"capture":           map[string]any{"angle": "slightly above, natural", "camera": "front", "device_visibility": "visible", "framing": "upper_body", "mirror": false, "mode": "selfie"},
		"media_plan":        map[string]any{"camera_angle": "eye_level", "capture_method": "handheld_selfie", "framing": "half_body", "human_presence": "present", "primary_subject": "person", "self_in_frame": "yes", "viewpoint": "subject_facing", "visible_scope": "face"},
		"referent_bindings": map[string]any{"first_person": "task_actor"},
		"visible_subjects":  []any{map[string]any{"actor_id": "actor_self", "visible_scope": "face"}},
	}
	arguments, err := normalizeNativeModelArguments(jsonBytes(args), (replyImageCapability{}).Definition())
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"draft_ref", "media_request_ref", "reference_asset"} {
		if _, exists := decodeObject(arguments)[key]; exists {
			t.Fatal("optional null was not normalized", key)
		}
	}
	fixture := mediaTestContext("SELF")
	resolver := NewStaticContextResolver(map[ContextSlot]ContextLoader{
		SlotCurrentLife:    func(context.Context, ContextRequest) (any, error) { return fixture.Life.Data, nil },
		SlotAppearance:     func(context.Context, ContextRequest) (any, error) { return fixture.Outfit.Data, nil },
		SlotVisualIdentity: func(context.Context, ContextRequest) (any, error) { return fixture.Visual.Data, nil },
	})
	invocation := CapabilityInvocation{Arguments: arguments}
	invocation.Metadata.FluctlightID = "actual-self"
	prepared, err := (imageGenerateCapability{contextResolver: resolver}).Prepare(context.Background(), invocation, CapabilityContext{})
	if err != nil {
		t.Fatal(err)
	}
	concept, found, err := capabilityPreparedData(prepared, "media_concept")
	if err != nil || !found || mapValue(concept)["task_actor_id"] != "actual-self" || mapValue(mapValue(concept)["media_capture_plan"])["visible_scope"] != "face" {
		t.Fatal("prepared self identity was lost", concept, found, err)
	}
}
