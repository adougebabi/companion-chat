package core

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"net/url"
	"strings"
)

// Filters belong before keyset pagination. A page of unrelated resources must
// never hide an older matching template or masquerade as an empty catalogue.
func (a *App) OwnerStudioResources(ctx context.Context, owner string, query url.Values) (map[string]any, error) {
	kind, search := query.Get("kind"), strings.TrimSpace(query.Get("q"))
	if v := query.Get("archived"); v != "" && v != "true" && v != "false" {
		return nil, invalid("Invalid archive filter")
	}
	if (kind != "" && !validResourceKind(kind)) || len([]rune(search)) > 200 || (query.Get("published") != "" && query.Get("published") != "true" && query.Get("published") != "false") {
		return nil, invalid("Invalid resource filters")
	}
	at, id, ok := decodeCursor(query.Get("cursor"))
	if !ok {
		return nil, &SurfaceError{Status: 400, Code: "studio_cursor_invalid", Message: "Cursor is invalid"}
	}
	var cursorAt any
	if query.Get("cursor") != "" {
		cursorAt = at
	}
	limit := pageLimit(query)
	rows, err := a.DB.Pool().Query(ctx, `SELECT id,owner_actor_id,fluctlight_id,kind,resource_key,status,
  payload||jsonb_build_object('referenced_by',COALESCE(payload->'referenced_by','[]'::jsonb)||(SELECT COALESCE(jsonb_agg(jsonb_build_object('fluctlight_id',b.fluctlight_id,'runtime_id',b.fluctlight_id,'running_version',b.source_version,'instance_revision',b.revision,'target_kind','visual_brief','status','applied','overrides',b.source_overrides) ORDER BY b.fluctlight_id),'[]'::jsonb) FROM public.fluctlight_visual_briefs b JOIN public.fluctlights f ON f.id=b.fluctlight_id WHERE b.source_resource_id=r.id AND f.created_by_actor_id=$1)||(SELECT COALESCE(jsonb_agg(jsonb_build_object('fluctlight_id',fr.fluctlight_id,'runtime_id',fr.id,'running_version',fr.source_version,'instance_revision',fr.revision,'target_kind','foundation','status',fr.status,'currently_effective',fr.status='accepted' AND f.current_revision=fr.revision) ORDER BY fr.created_at,fr.id),'[]'::jsonb) FROM public.fluctlight_foundation_revisions fr JOIN public.fluctlights f ON f.id=fr.fluctlight_id WHERE fr.source_resource_id=r.id AND f.created_by_actor_id=$1)||(SELECT COALESCE(jsonb_agg(jsonb_build_object('fluctlight_id',wi.fluctlight_id,'world_id',wi.world_id,'runtime_id',wi.id,'running_version',wi.source_version,'instance_revision',wi.revision,'target_kind','world_item','status','registered','available',wi.available) ORDER BY wi.id),'[]'::jsonb) FROM public.world_items wi JOIN public.fluctlights f ON f.id=wi.fluctlight_id WHERE wi.source_resource_id=r.id AND f.created_by_actor_id=$1)||(SELECT COALESCE(jsonb_agg(jsonb_build_object('fluctlight_id',o.fluctlight_id,'profile_id',o.profile_id,'runtime_id',o.id,'running_version',o.source_version,'instance_revision',o.revision,'target_kind','wardrobe_outfit','status','saved','overrides',o.source_overrides) ORDER BY o.id),'[]'::jsonb) FROM public.fluctlight_wardrobe_outfits o JOIN public.fluctlights f ON f.id=o.fluctlight_id WHERE o.source_resource_id=r.id AND f.created_by_actor_id=$1)) ,
  revision,created_at,updated_at
	 FROM public.owner_surface_records r WHERE owner_actor_id=$1 AND kind='studio_resource'
	 AND ((status='archived')=$8)
	 AND ($2='' OR payload->>'kind'=$2)
	 AND ($3='' OR strpos(lower(payload->>'name'),lower($3))>0)
	 AND ($4='' OR ($4='true')=EXISTS(SELECT 1 FROM public.studio_resource_versions v WHERE v.owner_actor_id=$1 AND v.resource_id=r.id AND v.version=r.payload->>'published_version' AND v.kind=r.payload->>'kind'))
	 AND ($5::timestamptz IS NULL OR (created_at,id)<($5,$6))
	 ORDER BY created_at DESC,id DESC LIMIT $7`, owner, kind, search, query.Get("published"), cursorAt, id, limit+1, query.Get("archived") == "true")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records, err := scanSurfaceRecords(rows)
	if err != nil {
		return nil, err
	}
	var next any
	if len(records) > limit {
		last := records[limit-1]
		next = encodeCursor(last.CreatedAt, last.ID)
		records = records[:limit]
	}
	items := make([]any, 0, len(records))
	for _, record := range records {
		items = append(items, (&OwnerSurface{}).resourceView(record, ""))
	}
	return map[string]any{"items": items, "next_cursor": next}, nil
}

func (a *App) OwnerStudioPublication(ctx context.Context, owner, id, version string) (map[string]any, error) {
	publication, err := readWorldPublication(ctx, a.DB.Pool(), owner, id, version)
	if err != nil {
		var surface *SurfaceError
		if errors.As(err, &surface) && surface.Code == "resource_not_published" {
			return nil, &SurfaceError{Status: 404, Code: "studio_publication_not_found", Message: "Published resource version is unavailable"}
		}
		return nil, err
	}
	return map[string]any{"id": publication.ID, "version": publication.Version, "kind": publication.Kind, "name": publication.Name, "body": publication.Body, "immutable": true}, nil
}

// Reusable creation sources carry no analysis credentials or runtime archive.
// Structured sources enter the existing canonical initializer (no model needed);
// prose sources enter the existing initialization Agent. Both get a fresh,
// Owner-scoped persisted analysis source and the normal activation checks.
func characterTemplateSource(body map[string]any) (string, error) {
	if body["schema"] != "character-template.v1" || containsPermanentHarm(body) {
		return "", invalid("Expected a safe character-template.v1 definition")
	}
	for key := range body {
		if key != "schema" && key != "description" && key != "initialization" {
			return "", invalid("Character template contains unsupported fields")
		}
	}
	description, hasDescription := body["description"]
	initialization, hasInitialization := body["initialization"]
	if hasDescription == hasInitialization {
		return "", invalid("Choose description or initialization for this template")
	}
	if hasDescription {
		text, ok := description.(string)
		if !ok || !validInitializationDescription(text) {
			return "", invalid("Template description must fit the 60000-byte UTF-8 limit")
		}
		return text, nil
	}
	value, ok := initialization.(map[string]any)
	if !ok || len(mapValue(value["core_persona"])) == 0 {
		return "", invalid("Template initialization requires core_persona")
	}
	for key := range value {
		switch key {
		case "schema_version", "core_persona", "developing_self", "extensions", "initial_goals", "initial_intentions", "initial_relationships":
		default:
			return "", invalid("Template initialization cannot include analysis credentials, Actor backgrounds or runtime state")
		}
	}
	text := jsonString(value)
	if !validInitializationDescription(text) {
		return "", invalid("Template initialization exceeds the 60000-byte UTF-8 limit")
	}
	// Validate the exact structured source at publication; no default template or
	// invented semantic data is substituted for an invalid definition.
	if _, err := prepareInitializationResponse(value); err != nil {
		return "", err
	}
	return text, nil
}

type initializationTemplateSourceKey struct{}

func (a *App) AnalyzePublishedCharacterTemplate(ctx context.Context, owner, id, version string) (map[string]any, error) {
	var status string
	if err := a.DB.Pool().QueryRow(ctx, `SELECT status FROM public.owner_surface_records WHERE owner_actor_id=$1 AND id=$2 AND kind='studio_resource'`, owner, id).Scan(&status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if err := activeStudioResource(status); err != nil {
		return nil, err
	}
	publication, err := readWorldPublication(ctx, a.DB.Pool(), owner, id, version)
	if err != nil {
		return nil, err
	}
	if publication.Kind != "character_template" {
		return nil, invalid("Select a published character template")
	}
	text, err := characterTemplateSource(publication.Body)
	if err != nil {
		return nil, err
	}
	ref := map[string]any{"resource_id": id, "resource_version": version, "definition_digest": stableDigest(jsonString(publication.Body))}
	return a.AnalyzeDescription(context.WithValue(ctx, initializationTemplateSourceKey{}, ref), owner, text)
}
