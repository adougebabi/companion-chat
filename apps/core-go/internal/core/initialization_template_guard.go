package core

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// Analysis is a source preview. A later archive may invalidate a not-yet-linked
// source; the activation transaction locks this same resource row before link.
func validateInitializationTemplate(ctx context.Context, q lifeContextQuerier, owner string, evidence map[string]any, lock bool) error {
	raw, present := evidence["template_source"]
	if !present {
		return nil
	}
	ref, ok := raw.(map[string]any)
	if !ok || stringValue(ref["resource_id"]) == "" || stringValue(ref["resource_version"]) == "" {
		return ErrActivationAnalysisInvalid
	}
	sql := `SELECT r.status FROM public.owner_surface_records r JOIN public.studio_resource_versions v ON v.resource_id=r.id AND v.owner_actor_id=r.owner_actor_id WHERE r.owner_actor_id=$1 AND r.id=$2 AND r.kind='studio_resource' AND v.version=$3 AND v.kind='character_template'`
	if lock {
		sql += " FOR SHARE OF r"
	}
	var status string
	if err := q.QueryRow(ctx, sql, owner, stringValue(ref["resource_id"]), stringValue(ref["resource_version"])).Scan(&status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrActivationAnalysisInvalid
		}
		return err
	}
	return activeStudioResource(status)
}
