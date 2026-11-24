package core

import (
	"encoding/json"
	"errors"
)

// The direct API accepts canonical IDs. Models only receive frozen references;
// resolve those before linked Goal admission, never by querying arbitrary IDs.
func (c lifeActivityStartCapability) NativeDefinition() CapabilityDefinition {
	d := c.Definition()
	d.InputSchema = copyPromptData(d.InputSchema).(map[string]any)
	properties := mapValue(d.InputSchema["properties"])
	// Keep legacy field spellings visible to normalization so they cannot be
	// silently discarded and turn a linked action into an unlinked one.
	properties["intention_id"] = map[string]any{"type": "string", "description": "Compatibility spelling of intention_ref: offered reference only, never a database ID."}
	properties["schedule_item_id"] = map[string]any{"type": "string", "description": "Compatibility spelling of schedule_item_ref: offered reference only, never a database ID."}
	properties["intention_ref"] = stringSchema()
	properties["schedule_item_ref"] = stringSchema()
	return d
}

func (c lifeActivityStartCapability) BindNativeArguments(raw json.RawMessage, projection ContextProjection) (json.RawMessage, error) {
	args := decodeObject(raw)
	for _, field := range []struct {
		ref, id string
		kind    ContextReferenceKind
	}{{"intention_ref", "intention_id", ContextReferenceIntention}, {"schedule_item_ref", "schedule_item_id", ContextReferenceScheduleItem}} {
		ref := stringValue(args[field.ref])
		if legacy := stringValue(args[field.id]); legacy != "" {
			if ref != "" && ref != legacy {
				return nil, errors.New(field.ref + " conflicts with its compatibility field")
			}
			ref = legacy
		}
		if ref == "" {
			continue
		}
		entry, found := projection.ReferenceIndex.ByRef[ref]
		profile := stringValue(decodeObject(entry.Snapshot)["profile_id"])
		if !found || entry.Kind != field.kind || entry.EntityID == "" || entry.Revision <= 0 || (profile != "" && profile != projection.ReferenceIndex.ActiveProfileID) {
			return nil, errors.New(field.ref + " must be a currently offered reference in the active profile")
		}
		delete(args, field.ref)
		args[field.id] = entry.EntityID
	}
	return jsonBytes(args), nil
}

func (c lifeActivityStartCapability) NativeOutput(value any, _ ContextProjection) any { return value }
