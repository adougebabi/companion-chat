package core

import (
	"fmt"
	"testing"
)

func TestDirectInfluenceRecoveryPreservesValidDataAndStrictRefs(t *testing.T) {
	const valid = "memory:ctx_0123456789abcdef0123456789abcdef"
	index := ContextReferenceIndex{ByRef: map[string]ContextReference{valid: {Kind: ContextReferenceMemory}}}
	codec, err := newProviderContextRefCodec(index)
	if err != nil {
		t.Fatal(err)
	}
	args := map[string]any{"text": "正文原样 cr_ffffffffffff", "decision": map[string]any{"action_type": "reply", "response_intent": "保持原意", "influences": []any{
		map[string]any{"ref": "cr_ffffffffffff", "role": "grounds", "confidence": 1, "note": "无效"},
		map[string]any{"ref": providerShortRef(valid), "role": "grounds", "confidence": 0.7, "note": "保留原有事实"},
		map[string]any{"ref": "actor_user:invalid", "role": "grounds", "confidence": 1, "note": "无效"},
	}}}
	if _, err := codec.decodeDirectReplyArguments(jsonBytes(args), index, false); err == nil {
		t.Fatal("first correction opportunity was bypassed")
	}
	decoded, err := codec.decodeDirectReplyArguments(jsonBytes(args), index, true)
	if err != nil {
		t.Fatal(err)
	}
	actual := decodeObject(decoded)
	decision := mapValue(actual["decision"])
	items := arrayValue(decision["influences"])
	if actual["text"] != args["text"] || decision["response_intent"] != "保持原意" || len(items) != 1 || jsonString(items[0]) != jsonString(map[string]any{"ref": valid, "role": "grounds", "confidence": 0.7, "note": "保留原有事实"}) {
		t.Fatalf("recovery changed valid data: %s", decoded)
	}
	// Recovery must not discard executable references or other cognition data.
	mapValue(args["decision"])["evidence_refs"] = []any{"cr_eeeeeeeeeeee"}
	if _, err := codec.decodeDirectReplyArguments(jsonBytes(args), index, true); err == nil {
		t.Fatal("required evidence reference was relaxed")
	}
	delete(mapValue(args["decision"]), "evidence_refs")
	mapValue(arrayValue(mapValue(args["decision"])["influences"])[1])["role"] = "invalid-role"
	decoded, err = codec.decodeDirectReplyArguments(jsonBytes(args), index, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateCapabilitySchemaValue(decodeObject(decoded), (directMainReplyCapability{}).Definition().InputSchema); err == nil {
		t.Fatal("invalid role was silently repaired")
	}
}

// Requires PostgreSQL; all model responses use the controlled transport.
func TestDirectInvalidInfluenceRefStillPublishes(t *testing.T) {
	for _, ref := range []string{"cr_ffffffffffff", "cr_36665410e47d2016d999c79878883135", "actor_user:fixture", "用户消息原文"} {
		t.Run(ref, func(t *testing.T) {
			calls := 0
			router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
				calls++
				if calls > 1 {
					return fakeProviderResult{Status: 400}
				}
				args := map[string]any{"text": "当然记得，是芒果。", "decision": map[string]any{"action_type": "reply", "response_intent": "回应", "influences": []any{map[string]any{"ref": ref, "role": "grounds", "confidence": 1, "note": "记忆"}}}}
				return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall(fmt.Sprintf("influence-reply-%d", calls), conversationReplyCapabilityName, args)}}
			})
			ctx, repo, app, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, "influence-recovery", router)
			if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3)`, owner, fid, conversation); err != nil {
				t.Fatal(err)
			}
			persistConversationReplyMode(t, ctx, repo, "direct")
			payload := map[string]any{"fluctlight_id": fid, "text": "你记得我喜欢什么水果吗？", "turn_id": "influence-recovery", "idempotency_key": "influence-recovery", "attachment_refs": []any{}}
			turn, err := app.HandleTurn(ctx, owner, conversation, payload)
			if err != nil || stringValue(turn.Assistant["text"]) != "当然记得，是芒果。" || calls != 1 {
				t.Fatalf("reply=%#v calls=%d err=%v", turn, calls, err)
			}
			var messages, assessments int
			var influences []byte
			err = repo.Pool().QueryRow(ctx, `SELECT (SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant'), (SELECT count(*) FROM public.cognition_assessments WHERE fluctlight_id=$2), (SELECT payload->'influences' FROM public.cognition_assessments WHERE fluctlight_id=$2)`, conversation, fid).Scan(&messages, &assessments, &influences)
			if err != nil || messages != 1 || assessments != 1 || string(influences) != "[]" {
				t.Fatalf("messages=%d assessments=%d influences=%s err=%v", messages, assessments, influences, err)
			}
			if _, err := app.HandleTurn(ctx, owner, conversation, payload); err != nil || calls != 1 {
				t.Fatalf("replay called model: calls=%d err=%v", calls, err)
			}
		})
	}
}
