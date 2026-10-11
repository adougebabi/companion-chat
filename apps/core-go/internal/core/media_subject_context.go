package core

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
)

func mediaVisibleSubjectsSchema() map[string]any {
	return map[string]any{"type": "array", "maxItems": 8, "items": objectSchema(map[string]any{
		"actor_id":      map[string]any{"type": "string", "minLength": 1, "maxLength": 128},
		"visible_scope": enumStringSchema(mediaPlanOptions["visible_scope"]...),
	}, []string{"actor_id", "visible_scope"}, false)}
}

// Known Actor IDs are supplied explicitly, never invented by a scorer. An
// anonymous passer-by or friend can remain described only by the raw intent.
func explicitMediaSubjects(args map[string]any, self string, explicit map[string]any) error {
	subjects := arrayValue(args["visible_subjects"])
	if len(subjects) == 0 {
		return nil
	}
	human := "partial"
	selfVisible := false
	seen := map[string]bool{}
	for _, raw := range subjects {
		subject := mapValue(raw)
		id := stringValue(subject["actor_id"])
		if id == "self" && self != "" {
			id = self
		}
		scope := stringValue(subject["visible_scope"])
		if id == "" || seen[id] {
			return errors.New("media_subject_binding_invalid")
		}
		seen[id] = true
		if scope != "hands" && scope != "feet" {
			human = "present"
		}
		if id == self || id == "self" {
			selfVisible = true
			if existing, ok := explicit["visible_scope"]; ok && existing != scope {
				return errors.New("media_subject_scope_conflict")
			}
			explicit["visible_scope"] = scope
		}
	}
	if existing, ok := explicit["human_presence"]; ok && existing != human {
		return errors.New("media_subject_presence_conflict")
	}
	explicit["human_presence"] = human
	if selfVisible {
		if existing, ok := explicit["self_in_frame"]; ok && existing != "yes" {
			return errors.New("media_subject_self_conflict")
		}
		explicit["self_in_frame"] = "yes"
	}
	return nil
}

func (a *App) readMediaSubjectContexts(ctx context.Context, invocation CapabilityInvocation, subjects []any) ([]any, error) {
	result := []any{}
	for _, raw := range subjects {
		subject := mapValue(raw)
		id := stringValue(subject["actor_id"])
		if id == invocation.Metadata.FluctlightID || id == "self" {
			continue
		}
		scope := stringValue(subject["visible_scope"])
		// Match ordinary detail authorization without reading a persona snapshot.
		// Human/unknown IDs have no visual authority here and must remain in intent.
		var authorized bool
		if err := a.DB.Pool().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.fluctlights WHERE id=$1 AND created_by_actor_id=$2)`, id, invocation.Metadata.AuthorizationActorID).Scan(&authorized); err != nil {
			return nil, err
		}
		if !authorized {
			return nil, errors.New("media_subject_not_authorized")
		}
		if scope == "unknown" {
			result = append(result, map[string]any{"actor_id": id, "visible_scope": scope})
			continue
		}
		slots := []ContextSlot{SlotAppearance}
		if scope == "full" || scope == "face" {
			slots = append(slots, SlotVisualIdentity)
		}
		resolved, err := NewAppContextResolver(a).Resolve(ctx, ContextRequest{FluctlightID: id}, slots)
		if err != nil {
			return nil, err
		}
		entry := map[string]any{"actor_id": id, "visible_scope": scope, "appearance": projectMediaAppearance(resolved.Outfit.Data, scope), "appearance_snapshot_digest": stableDigest(appearanceSnapshotIdentity(resolved.Outfit.Data))}
		if resolved.Visual != nil {
			if resolved.Visual.Data["status"] != "active" {
				return nil, errors.New("media_subject_identity_pending")
			}
			entry["visual_identity"] = projectMediaVisualIdentity(resolved.Visual.Data, scope)
		}
		result = append(result, entry)
	}
	return result, nil
}

func validateMediaSubjectContexts(concept map[string]any) error {
	if raw, present := concept["subject_contexts"]; present {
		entries, ok := raw.([]any)
		if !ok || len(entries) > 8 {
			return errors.New("media_subject_context_invalid")
		}
	}
	seen := map[string]bool{}
	for _, raw := range arrayValue(concept["subject_contexts"]) {
		subject := mapValue(raw)
		id := stringValue(subject["actor_id"])
		scope := stringValue(subject["visible_scope"])
		if id == "" || seen[id] || mapValue(concept["media_capture_plan"])["human_presence"] == "none" {
			return errors.New("media_subject_context_forbidden")
		}
		seen[id] = true
		valid := false
		for _, value := range mediaPlanOptions["visible_scope"] {
			if scope == value {
				valid = true
			}
		}
		if !valid {
			return errors.New("media_subject_scope_invalid")
		}
		appearance := mapValue(subject["appearance"])
		if scope == "unknown" && len(appearance) > 0 {
			return errors.New("media_unknown_subject_appearance_forbidden")
		}
		if scope != "full" && jsonString(projectMediaAppearance(appearance, scope)) != jsonString(appearance) && len(appearance) > 0 {
			return errors.New("media_subject_projection_invalid")
		}
		if scope != "full" && scope != "face" && len(mapValue(subject["visual_identity"])) > 0 {
			return errors.New("media_subject_identity_forbidden")
		}
	}
	return nil
}

func mediaOtherSubjectProviderData(concept map[string]any) []any {
	result := []any{}
	for _, raw := range arrayValue(concept["subject_contexts"]) {
		subject := mapValue(raw)
		entry := map[string]any{"subject_id": subject["actor_id"], "visible_scope": subject["visible_scope"]}
		if appearance := mapValue(subject["appearance"]); len(appearance) > 0 {
			entry["appearance"] = compactMediaAppearance(appearance)
		}
		if visual := mapValue(subject["visual_identity"]); len(visual) > 0 {
			entry["visual_identity"] = mediaStableIdentityForNormalizer(visual)
		}
		result = append(result, entry)
	}
	return result
}

func lockMediaSubjectSnapshots(ctx context.Context, tx pgx.Tx, self string, concept map[string]any, at time.Time) error {
	ids := []string{}
	if mediaPlanNeedsAppearance(mapValue(concept["media_capture_plan"])) {
		ids = append(ids, self)
	}
	for _, raw := range arrayValue(concept["subject_contexts"]) {
		subject := mapValue(raw)
		if subject["appearance_snapshot_digest"] != nil {
			ids = append(ids, stringValue(subject["actor_id"]))
		}
	}
	sort.Strings(ids)
	for _, id := range ids {
		if err := lockEffectiveLifeSnapshotTx(ctx, tx, id); err != nil {
			return err
		}
	}
	for _, raw := range arrayValue(concept["subject_contexts"]) {
		subject := mapValue(raw)
		if subject["appearance_snapshot_digest"] == nil {
			continue
		}
		live, _, _, err := readEffectiveLifeSnapshotWith(ctx, tx, stringValue(subject["actor_id"]), at)
		if err != nil {
			return err
		}
		if stableDigest(appearanceSnapshotIdentity(live)) != subject["appearance_snapshot_digest"] {
			return ErrConflict
		}
	}
	return nil
}
