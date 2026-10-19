package core

import "testing"

func TestExplicitPersonReferenceCarriesUnverifiedOwnerProvenanceWithoutActorContext(t *testing.T) {
	concept := map[string]any{"source": "studio_definition", "studio_owner_actor_id": "owner", "studio_source": map[string]any{"resource_id": "target", "source_digest": "actual-frozen-digest"}, "intent": "A fictional person image with an explicitly allowed visual reference, no Actor identity claims", "media_plan_version": mediaPlanVersion, "media_capture_plan": mediaTestPlan("other", "present", "no", "unknown"), "context_binding": map[string]any{}, "explicit_constraints": map[string]any{"self_in_frame": "no"}, "reference_asset": map[string]any{"asset_id": "original", "purpose": "person", "grant_id": "explicit-grant", "grant_revision": 1}}
	if err := validateStudioMediaConcept(concept); err != nil {
		t.Fatal(err)
	}
	packet, err := mediaNormalizationInput(concept)
	if err != nil {
		t.Fatal(err)
	}
	provenance := mapValue(packet["reference_provenance"])
	if provenance["content_verified"] != false || provenance["source"] != "owner_explicit_unverified_content" {
		t.Fatal("Owner authorization impersonated a visual inspection", packet)
	}
	if len(mapValue(packet["allowed_subject_data"])) != 0 || len(arrayValue(packet["other_subjects"])) != 0 {
		t.Fatal("reference imported private Actor context", packet)
	}
	noGrant := cloneMap(concept)
	noGrant["reference_asset"] = map[string]any{"asset_id": "original", "purpose": "person"}
	if validateBranchedMediaConcept(noGrant) == nil {
		t.Fatal("ungranted person path accepted")
	}
}
