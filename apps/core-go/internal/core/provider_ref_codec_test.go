package core

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"
)

func TestProviderContextRefCodecRoundTripsOnlyKnownScalarRefs(t *testing.T) {
	memory := "memory:ctx_0123456789abcdef0123456789abcdef"
	life := "life_context:ctx_0123456789abcdef0123456789abcdef"
	appearance := "appearance:ctx_0123456789abcdef0123456789abcdef"
	index := ContextReferenceIndex{ByRef: map[string]ContextReference{
		memory: {Ref: memory}, life: {Ref: life}, appearance: {Ref: appearance},
	}}
	codec, err := newProviderContextRefCodec(index)
	if err != nil {
		t.Fatal(err)
	}
	input := map[string]any{"retrieved_memory": []any{map[string]any{"ref": memory, "content": "原文 " + memory}},
		"current_state": map[string]any{"life_context": map[string]any{"ref": life}, "appearance": map[string]any{"ref": appearance}}}
	short := providerEncodeContextRefs(input, index).(map[string]any)
	encoded := jsonString(short)
	for _, ref := range []string{memory, life, appearance} {
		if !strings.Contains(encoded, providerShortRef(ref)) {
			t.Fatalf("missing alias for %s: %s", ref, encoded)
		}
	}
	if strings.Contains(encoded, `"ref":"`+memory+`"`) || strings.Contains(encoded, `"ref":"`+life+`"`) || strings.Contains(encoded, `"ref":"`+appearance+`"`) {
		t.Fatalf("long ref leaked as a field: %s", encoded)
	}
	if !strings.Contains(encoded, "原文 "+memory) {
		t.Fatalf("substring was rewritten: %s", encoded)
	}
	decoded, err := codec.decode(short)
	if err != nil || jsonString(decoded) != jsonString(input) {
		t.Fatalf("round trip: %v %#v", err, decoded)
	}
	if _, err := codec.decode(map[string]any{"evidence_refs": []any{"cr_ffffffffffff"}}); err == nil {
		t.Fatal("unknown alias accepted")
	}
}

func TestProviderContextRefCodecRegistersToolResultAndParallelReads(t *testing.T) {
	codec, err := newProviderContextRefCodec(ContextReferenceIndex{ByRef: map[string]ContextReference{}})
	if err != nil {
		t.Fatal(err)
	}
	ref := "memory:ctx_abcdef0123456789abcdef0123456789"
	result, err := codec.encodeResult(map[string]any{"output": map[string]any{"ref": ref}})
	if err != nil {
		t.Fatal(err)
	}
	short := mapValue(mapValue(result)["output"])["ref"]
	if short != providerShortRef(ref) {
		t.Fatalf("short = %#v", short)
	}
	var wg sync.WaitGroup
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			decoded, decodeErr := codec.decodeArguments(json.RawMessage(`{"evidence_refs":["` + providerShortRef(ref) + `"]}`))
			if decodeErr != nil || !strings.Contains(string(decoded), ref) {
				t.Errorf("parallel decode: %s %v", decoded, decodeErr)
			}
		}()
	}
	wg.Wait()
	newLife := "life_context:ctx_abcdef0123456789abcdef0123456789"
	if err := codec.registerIndex(ContextReferenceIndex{ByRef: map[string]ContextReference{newLife: {Ref: newLife}}}); err != nil {
		t.Fatal(err)
	}
	for _, known := range []string{ref, newLife} {
		decoded, decodeErr := codec.decode(map[string]any{"ref": providerShortRef(known)})
		if decodeErr != nil || mapValue(decoded)["ref"] != known {
			t.Fatalf("refresh lost alias %s: %#v %v", known, decoded, decodeErr)
		}
	}
}

func TestWorkingMemoryUsesShortRefsButKeepsLongSourceRefs(t *testing.T) {
	memory := "memory:ctx_0123456789abcdef0123456789abcdef"
	life := "life_context:ctx_0123456789abcdef0123456789abcdef"
	appearance := "appearance:ctx_0123456789abcdef0123456789abcdef"
	projection := ContextProjection{
		CurrentState: map[string]any{"data": map[string]any{
			"life_context": map[string]any{"ref": life, "scene": "书房"},
			"appearance":   map[string]any{"ref": appearance, "wearing_state": "known"},
		}},
		Memories: []map[string]any{{"ref": memory, "content": "喜欢读书", "importance": 0.7}},
		ReferenceIndex: ContextReferenceIndex{ByRef: map[string]ContextReference{
			memory:     {Ref: memory, Kind: ContextReferenceMemory},
			life:       {Ref: life, Kind: ContextReferenceLifeContext},
			appearance: {Ref: appearance, Kind: ContextReferenceAppearance},
		}},
	}
	input := workingMemoryInputFromProjectionForSurface(projection, ProviderContextSurfaceConversationMain, nil, nil)
	if len(input.RetrievedMemories) != 1 || input.RetrievedMemories[0].SourceRefs[0] != memory || mapValue(input.RetrievedMemories[0].Content)["ref"] != providerShortRef(memory) {
		t.Fatalf("memory ref/source mismatch: %#v", input.RetrievedMemories)
	}
	var current map[string]any
	for _, fact := range input.RuntimeFacts {
		if mapValue(fact.Content)["kind"] == "current_state" {
			current = mapValue(mapValue(fact.Content)["value"])
			break
		}
	}
	data := mapValue(current["data"])
	if mapValue(data["life_context"])["ref"] != providerShortRef(life) || mapValue(data["appearance"])["ref"] != providerShortRef(appearance) {
		t.Fatalf("current state refs = %#v", data)
	}
	if mapValue(mapValue(projection.CurrentState["data"])["life_context"])["ref"] != life {
		t.Fatal("Core projection was mutated")
	}
}
