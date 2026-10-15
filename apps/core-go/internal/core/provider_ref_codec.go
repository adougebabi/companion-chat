package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
)

// Provider refs are local aliases for one Agent run. Authority remains with
// the full ContextReferenceIndex and is checked after decoding.
var providerShortRefPattern = regexp.MustCompile(`^cr_[a-f0-9]{12}$`)

type providerContextRefCodec struct {
	mu      sync.RWMutex
	byShort map[string]string
	known   map[string]ContextReferenceKind
}

func newProviderContextRefCodec(index ContextReferenceIndex) (*providerContextRefCodec, error) {
	codec := &providerContextRefCodec{byShort: map[string]string{}, known: map[string]ContextReferenceKind{}}
	return codec, codec.registerIndex(index)
}

func providerShortenableRef(ref string) bool {
	if !contextReferencePattern.MatchString(ref) {
		return false
	}
	for _, kind := range []ContextReferenceKind{ContextReferenceMemory, ContextReferenceActiveMemory, ContextReferenceLifeContext, ContextReferenceAppearance} {
		if strings.HasPrefix(ref, string(kind)+":ctx_") {
			return true
		}
	}
	return false
}

func providerShortRef(ref string) string { return "cr_" + stableDigest(ref)[:12] }

func (codec *providerContextRefCodec) register(ref string) error {
	if codec == nil || !providerShortenableRef(ref) {
		return nil
	}
	short := providerShortRef(ref)
	codec.mu.Lock()
	defer codec.mu.Unlock()
	if previous := codec.byShort[short]; previous != "" && previous != ref {
		return errors.New("provider_context_ref_alias_collision")
	}
	codec.byShort[short] = ref
	return nil
}

func (codec *providerContextRefCodec) registerIndex(index ContextReferenceIndex) error {
	if codec == nil {
		return nil
	}
	known := make(map[string]ContextReferenceKind, len(index.ByRef))
	for ref, entry := range index.ByRef {
		known[ref] = entry.Kind
		if err := codec.register(ref); err != nil {
			return err
		}
	}
	codec.mu.Lock()
	codec.known = known
	codec.mu.Unlock()
	return nil
}

func (codec *providerContextRefCodec) contains(ref string) bool {
	codec.mu.RLock()
	defer codec.mu.RUnlock()
	_, found := codec.known[ref]
	return found
}

// The initial prompt uses the same deterministic alias before the codec is
// attached to the ADK run, so budgeting and the actual wire agree.
func providerEncodeContextRefs(value any, index ContextReferenceIndex) any {
	result, _ := providerMapContextRefsChecked(value, "", func(ref string) (string, error) {
		if _, exists := index.ByRef[ref]; exists && providerShortenableRef(ref) {
			return providerShortRef(ref), nil
		}
		return ref, nil
	})
	return result
}

func providerRefField(key string) bool {
	return key == "ref" || strings.HasSuffix(key, "_ref") || strings.HasSuffix(key, "_refs")
}

func providerMapContextRefsChecked(value any, key string, scalar func(string) (string, error)) (any, error) {
	switch typed := value.(type) {
	case map[string]any:
		mapped := make(map[string]any, len(typed))
		for childKey, child := range typed {
			value, err := providerMapContextRefsChecked(child, childKey, scalar)
			if err != nil {
				return nil, err
			}
			mapped[childKey] = value
		}
		return mapped, nil
	case []any:
		mapped := make([]any, len(typed))
		for i, child := range typed {
			value, err := providerMapContextRefsChecked(child, key, scalar)
			if err != nil {
				return nil, err
			}
			mapped[i] = value
		}
		return mapped, nil
	case []map[string]any:
		mapped := make([]map[string]any, len(typed))
		for i, child := range typed {
			value, err := providerMapContextRefsChecked(child, key, scalar)
			if err != nil {
				return nil, err
			}
			mapped[i] = value.(map[string]any)
		}
		return mapped, nil
	case string:
		if !providerRefField(key) {
			return typed, nil
		}
		return scalar(typed)
	default:
		return value, nil
	}
}

func (codec *providerContextRefCodec) encodeResult(value any) (any, error) {
	return providerMapContextRefsChecked(value, "", func(ref string) (string, error) {
		if !providerShortenableRef(ref) {
			return ref, nil
		}
		if err := codec.register(ref); err != nil {
			return "", err
		}
		return providerShortRef(ref), nil
	})
}

func (codec *providerContextRefCodec) decode(value any) (any, error) {
	if codec == nil {
		return value, nil
	}
	return providerMapContextRefsChecked(value, "", func(ref string) (string, error) {
		if !providerShortRefPattern.MatchString(ref) {
			return ref, nil
		}
		codec.mu.RLock()
		long := codec.byShort[ref]
		codec.mu.RUnlock()
		if long == "" {
			return "", errors.New("provider_context_ref_alias_unknown")
		}
		return long, nil
	})
}

func (codec *providerContextRefCodec) decodeArguments(raw json.RawMessage) (json.RawMessage, error) {
	if codec == nil {
		return raw, nil
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return raw, nil
	}
	decoded, err := codec.decode(value)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(decoded)
	if err != nil {
		return nil, fmt.Errorf("provider_context_ref_decode: %w", err)
	}
	return encoded, nil
}

// Only optional Fact-only hints may remain unresolved for scoped reconciliation.
// Moments keeps evidence-only handling; private replies also allow draft hints.
// Action and target references retain strict decoding.
func (codec *providerContextRefCodec) decodeReplyArguments(raw json.RawMessage) (json.RawMessage, error) {
	return codec.decodeReplyHints(raw, false)
}

func (codec *providerContextRefCodec) decodePrivateReplyArguments(raw json.RawMessage) (json.RawMessage, error) {
	return codec.decodeReplyHints(raw, true)
}

func (codec *providerContextRefCodec) decodeReplyHints(raw json.RawMessage, allowDraftHint bool) (json.RawMessage, error) {
	var args map[string]any
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	draft, draftPresent := args["draft_ref"]
	if allowDraftHint {
		delete(args, "draft_ref")
	}
	refs, present := args["evidence_refs"]
	delete(args, "evidence_refs")
	decoded, err := codec.decode(args)
	if err != nil {
		return nil, err
	}
	if allowDraftHint && draftPresent {
		value, decodeErr := codec.decode(map[string]any{"draft_ref": draft})
		if decodeErr != nil {
			mapValue(decoded)["draft_ref"] = draft
		} else {
			mapValue(decoded)["draft_ref"] = mapValue(value)["draft_ref"]
		}
	}
	if present {
		values := []any{}
		for _, ref := range arrayValue(refs) {
			value, decodeErr := codec.decode(map[string]any{"evidence_ref": ref})
			if decodeErr != nil {
				values = append(values, ref)
			} else {
				values = append(values, mapValue(value)["evidence_ref"])
			}
		}
		mapValue(decoded)["evidence_refs"] = values
	}
	return json.Marshal(decoded)
}

func (codec *providerContextRefCodec) currentIndex() ContextReferenceIndex {
	index := ContextReferenceIndex{ByRef: map[string]ContextReference{}}
	if codec == nil {
		return index
	}
	codec.mu.RLock()
	defer codec.mu.RUnlock()
	for ref, kind := range codec.known {
		index.ByRef[ref] = ContextReference{Kind: kind}
	}
	return index
}
