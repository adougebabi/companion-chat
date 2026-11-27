package core

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"
)

// The chat model sees media.image.generate from the first request (no pre-turn
// image check, no forced catalog step) and calls it itself on a follow-up.
func TestLiveDirectPhotoFollowupCallsImageToolDirectly(t *testing.T) {
	baseURL, model := liveProviderConfig(t)
	decision := map[string]any{"action_type": "reply", "response_intent": "回应", "influences": []any{}}
	prior := newFakeProviderRouter().on(directMediaDeliveryCheckSchema, func(map[string]any) fakeProviderResult {
		return fakeProviderResult{Structured: map[string]any{"claims": []any{}}}
	}).otherwise(func(map[string]any) fakeProviderResult {
		return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("prior-photo", conversationReplyCapabilityName,
			map[string]any{"text": "我给你拍一张自拍，拍好再发给你。", "decision": decision})}}
	})
	const suffix = "direct-live-photo-catalog"
	ctx, repo, app, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, suffix, prior)
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3)`, owner, fid, conversation); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_visual_identities(id,fluctlight_id,status,identity_snapshot) VALUES('fixture-live-photo-identity',$1,'active','{"identity":{"name":"Fixture identity"}}')`, fid); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.runtime_settings(key,value_json) VALUES('media.comfyui',$1)`, jsonString(map[string]any{
		"baseUrl": "http://comfy.invalid", "workflow": map[string]any{"prompt": "{{prompt}}"},
	})); err != nil {
		t.Fatal(err)
	}
	persistConversationReplyMode(t, ctx, repo, "direct")
	if _, err := app.HandleTurn(ctx, owner, conversation, map[string]any{"fluctlight_id": fid, "text": "能发张自拍吗？", "idempotency_key": "photo-prior", "attachment_refs": []any{}}); err != nil {
		t.Fatalf("set up prior turn: %v", err)
	}
	endpoint := "mixed-endpoint-" + suffix
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.provider_endpoints SET base_url=$2 WHERE id=$1`, endpoint, baseURL); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.model_roles SET model_id=$2,timeout_seconds=120 WHERE provider_endpoint_id=$1`, endpoint, model); err != nil {
		t.Fatal(err)
	}
	liveCtx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()
	wire := &r7LiveWire{}
	app = newTestApp(t, repo, wire)
	result, err := app.HandleTurn(liveCtx, owner, conversation, map[string]any{"fluctlight_id": fid, "text": "还没发哦", "idempotency_key": "photo-followup", "attachment_refs": []any{}})
	var firstNative map[string]any
	imageAccepted, replyAfterAcceptance := false, false
	for i, row := range wire.rows {
		req, response := mapValue(row["request"]), mapValue(row["response"])
		var offered, called []string
		for _, tool := range arrayValue(req["tools"]) {
			offered = append(offered, stringValue(mapValue(mapValue(tool)["function"])["name"]))
		}
		if len(offered) > 0 && firstNative == nil {
			firstNative = req
		}
		for _, choice := range arrayValue(response["choices"]) {
			for _, tool := range arrayValue(mapValue(mapValue(choice)["message"])["tool_calls"]) {
				called = append(called, stringValue(mapValue(mapValue(tool)["function"])["name"]))
			}
		}
		for _, raw := range arrayValue(req["messages"]) {
			message := mapValue(raw)
			if message["role"] != "tool" {
				continue
			}
			receipt := decodeObject([]byte(stringValue(message["content"])))
			data := mapValue(receipt["output"])
			if receipt["status"] == "accepted" {
				imageAccepted = true
			}
			t.Logf("round %d tool receipt: status=%v warning=%v detail=%v loaded=%v", i+1, receipt["status"], data["warning_code"], data["detail"], data["loaded"])
		}
		if imageAccepted && slices.ContainsFunc(called, func(name string) bool {
			return name == "conversation_reply" || name == conversationReplyCapabilityName
		}) {
			replyAfterAcceptance = true
		}
		var finish, bodyText string
		for _, choice := range arrayValue(response["choices"]) {
			finish = stringValue(mapValue(choice)["finish_reason"])
			bodyText = stringValue(mapValue(mapValue(choice)["message"])["content"])
		}
		if len(bodyText) > 300 {
			bodyText = bodyText[:300]
		}
		t.Logf("physical round %d: offered=%v called=%v finish=%q thinking=%v stop=%v content=%q", i+1, offered, called, finish, req["enable_thinking"], req["tool_choice"], bodyText)
	}
	var intents int
	queryErr := repo.Pool().QueryRow(liveCtx, `SELECT count(*) FROM public.media_intents WHERE conversation_id=$1 AND owner_fluctlight_id=$2 AND kind='image'`, conversation, fid).Scan(&intents)
	if err != nil || queryErr != nil || intents != 1 || stringValue(result.Assistant["id"]) == "" || !replyAfterAcceptance {
		t.Fatalf("live follow-up did not create one real image intent and reply: intents=%d calls=%d reply=%q turn=%v query=%v", intents, len(wire.rows), stringValue(result.Assistant["text"]), err, queryErr)
	}
	if !r6PayloadHasTool(firstNative, "media.image.generate") || r6PayloadHasTool(firstNative, "schedule.replan") {
		t.Fatal("the first request must offer the image tool directly and keep second-tier tools in the index")
	}

	ordinaryOffset := len(wire.rows)
	ordinary, err := app.HandleTurn(liveCtx, owner, conversation, map[string]any{"fluctlight_id": fid, "text": "晚安，我们下次再聊。", "idempotency_key": "ordinary-goodnight", "attachment_refs": []any{}})
	if err != nil || strings.TrimSpace(stringValue(ordinary.Assistant["text"])) == "" {
		t.Fatalf("ordinary chat did not reply: %v", err)
	}
	for _, row := range wire.rows[ordinaryOffset:] {
		response := mapValue(row["response"])
		for _, choice := range arrayValue(response["choices"]) {
			for _, raw := range arrayValue(mapValue(mapValue(choice)["message"])["tool_calls"]) {
				name := stringValue(mapValue(mapValue(raw)["function"])["name"])
				if name == "capability_catalog" || name == "media_image_generate" {
					t.Fatalf("ordinary chat unexpectedly called %s", name)
				}
			}
		}
	}
}
