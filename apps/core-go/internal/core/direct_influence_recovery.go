package core

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/cloudwego/eino/schema"
	capabilitycontract "github.com/fluctlight/local-ai-companion/apps/core-go/internal/capability"
)

// Leave required reply data and fields used for claims, evidence, preference
// changes and goals on the normal validation path. These optional annotations
// can be omitted independently when the model has not corrected their shape.
func recoverDirectReplyOptionalFields(raw json.RawMessage, replySchema map[string]any) (json.RawMessage, bool) {
	args := decodeObject(raw)
	decision := mapValue(args["decision"])
	properties := mapValue(mapValue(mapValue(replySchema["properties"])["decision"])["properties"])
	changed := false
	for _, field := range []string{"internal_intent", "core_alignment", "state_expression", "self_evaluation"} {
		value, present := decision[field]
		fieldSchema := mapValue(properties[field])
		if !present || len(fieldSchema) == 0 {
			continue
		}
		wrapper := objectSchema(map[string]any{field: fieldSchema}, nil, false)
		if _, err := capabilitycontract.NormalizeModelArguments(jsonBytes(map[string]any{field: value}), wrapper); err != nil {
			delete(decision, field)
			changed = true
		}
	}
	if !changed {
		return raw, false
	}
	return jsonBytes(args), true
}

type directInfluenceRefError struct {
	position  int
	reason    string
	available []string
}

func (e *directInfluenceRefError) Error() string {
	return fmt.Sprintf("decision_influence_%d_ref_%s", e.position, e.reason)
}

// Only offer opaque refs already sent to the model, never the index's entity
// IDs, snapshots, or refs withheld by the task's projection. Assistant output
// cannot introduce a new offered ref. Refreshes replace the current input.
func (codec *providerContextRefCodec) noteOfferedInfluenceRefs(input []*schema.Message) {
	codec.mu.Lock()
	defer codec.mu.Unlock()
	codec.offered = map[string]string{}
	for _, message := range input {
		if message == nil || message.Role == schema.Assistant {
			continue
		}
		for ref := range codec.known {
			wire := ref
			if providerShortenableRef(ref) {
				wire = providerShortRef(ref)
			}
			if strings.Contains(message.Content, wire) {
				codec.offered[wire] = ref
			}
		}
	}
}

// A direct reply may settle with no influences (freezeDecisionInfluences uses
// required=false). In reply-only requests or after two ordinary correction
// opportunities, omit only unresolved influence refs. Other arguments,
// evidence, goals and the atomic publication contract retain normal validation.
func (codec *providerContextRefCodec) decodeDirectReplyArguments(raw json.RawMessage, index ContextReferenceIndex, discardInvalid bool) (json.RawMessage, error) {
	args := decodeObject(raw)
	decision := mapValue(args["decision"])
	items := arrayValue(decision["influences"])
	kept := make([]any, 0, len(items))
	for position, item := range items {
		ref := strings.TrimSpace(stringValue(mapValue(item)["ref"]))
		decoded, err := codec.decode(map[string]any{"ref": ref})
		long := stringValue(mapValue(decoded)["ref"])
		reason := ""
		if err != nil {
			reason = "unknown"
		} else if !contextReferencePattern.MatchString(long) {
			reason = "invalid"
		} else if _, ok := index.ByRef[long]; !ok {
			reason = "unknown"
		}
		if reason != "" {
			if discardInvalid {
				continue
			}
			available := []string{}
			if codec != nil {
				codec.mu.RLock()
				for wire, actual := range codec.offered {
					if _, ok := index.ByRef[actual]; ok {
						available = append(available, wire)
					}
				}
				codec.mu.RUnlock()
			}
			sort.Strings(available)
			return nil, &directInfluenceRefError{position: position, reason: reason, available: available}
		}
		kept = append(kept, item)
	}
	if discardInvalid && len(kept) != len(items) {
		decision["influences"] = kept
		raw = jsonBytes(args)
	}
	return codec.decodeArguments(raw)
}
