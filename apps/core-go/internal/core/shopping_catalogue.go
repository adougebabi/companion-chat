package core

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

type shoppingCatalogueFeedback struct {
	code   string
	output map[string]any
}

func (e *shoppingCatalogueFeedback) Error() string { return e.code }

// Only this transactional admission may grant a catalogue provenance snapshot.
// Model arguments identify a record; they never choose source authority.
func freezeShoppingCatalogueTx(ctx context.Context, tx pgx.Tx, fid, place string, request map[string]any) error {
	items, err := shoppingRequestedItems(request)
	if err != nil {
		return err
	}
	for _, item := range items {
		id := stringValue(item["world_item_id"])
		if id == "" {
			continue
		}
		record, err := readWorldItemWith(ctx, tx, fid, id, true)
		if err != nil {
			return err
		}
		world, err := worldIDWith(ctx, tx, fid)
		if err != nil {
			return err
		}
		if record["world_id"] != world || record["available"] != true {
			return errors.New("shopping_catalogue_item_unavailable")
		}
		if required := stringValue(record["place_id"]); required != "" && required != place {
			return &shoppingCatalogueFeedback{code: "shopping_catalogue_place_required", output: map[string]any{"world_item_id": id, "required_place_id": required, "recovery": "Retry with this catalogue record's place_id. A location label or scene change does not supply a stable place. No purchase has started."}}
		}
		definition, err := normalizeShoppingItem(mapValue(record["definition"]))
		if err != nil {
			return err
		}
		for _, key := range []string{"item_kind", "category", "slot", "description"} {
			if item[key] != definition[key] {
				return &shoppingCatalogueFeedback{code: "shopping_catalogue_definition_mismatch", output: map[string]any{"world_item_id": id, "field": key, "expected_value": definition[key], "recovery": "Copy the exact definition from wardrobe.inspect catalogue_detail. No purchase has started."}}
			}
		}
		item["catalogue_source"] = map[string]any{"world_item_id": id, "world_item_revision": record["revision"], "world_id": world, "source_resource_id": record["source_resource_id"], "source_version": record["source_version"]}
	}
	if _, bundle := request["items"]; bundle {
		members := make([]any, len(items))
		for i, item := range items {
			members[i] = item
		}
		request["items"] = members
	} else {
		for key, value := range items[0] {
			request[key] = value
		}
	}
	return nil
}

func shoppingFrozenSources(request map[string]any) []map[string]any {
	if _, bundle := request["items"]; bundle {
		sources := []map[string]any{}
		for _, raw := range arrayValue(request["items"]) {
			sources = append(sources, mapValue(mapValue(raw)["catalogue_source"]))
		}
		return sources
	}
	return []map[string]any{mapValue(request["catalogue_source"])}
}

// Bounded model query through wardrobe.inspect. This is a world catalogue, not
// evidence of inventory. It uses the existing Tool's resource authorization.
func (s *WardrobeService) inspectWorldItems(ctx context.Context, fid string, args map[string]any) (map[string]any, error) {
	world, err := worldIDWith(ctx, s.repository.Pool(), fid)
	if err != nil {
		return nil, err
	}
	if stringValue(args["operation"]) == "catalogue_detail" && stringValue(args["item_id"]) != "" {
		item, err := readWorldItemWith(ctx, s.repository.Pool(), fid, stringValue(args["item_id"]), false)
		if err != nil {
			return nil, err
		}
		if item["world_id"] != world {
			return nil, ErrNotFound
		}
		return map[string]any{"item": item, "catalogue_only": true, "inventory_effect": false}, nil
	}
	limit := intValue(args["limit"])
	if limit == 0 {
		limit = 30
	}
	if limit < 1 || limit > 30 {
		return nil, ErrInvalidArguments
	}
	items, err := readWorldObjects(ctx, s.repository.Pool(), `SELECT to_jsonb(i) FROM public.world_items i WHERE fluctlight_id=$1 AND world_id=$2 AND ($3='' OR id>$3) AND ($4='' OR strpos(lower(name||' '||(definition->>'description')),lower($4))>0) ORDER BY id LIMIT $5`, fid, world, stringValue(args["cursor"]), stringValue(args["query"]), limit+1)
	if err != nil {
		return nil, err
	}
	next := ""
	if len(items) > limit {
		next = stringValue(mapValue(items[limit-1])["id"])
		items = items[:limit]
	}
	return map[string]any{"items": items, "next_cursor": next, "world_id": world, "catalogue_only": true, "inventory_effect": false, "has_more": next != ""}, nil
}
