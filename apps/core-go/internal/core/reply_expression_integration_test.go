package core

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestPrivateChatDurableStyleQuotaAcrossResume(t *testing.T) {
	ctx, repo, app, owner, self, conversation := durableReplyFixture(t, "style-quota")
	settings := defaultConversationReplySettings()
	settings.Mode = "fact_only"
	contract, _ := replyExpressionContract(settings.ExpressionVersion)
	ref := "quota-draft"
	_, err := repo.Pool().Exec(ctx, `INSERT INTO public.conversation_reply_drafts(id,logical_key,fluctlight_id,conversation_id,authorization_actor_id,subject_actor_id,profile_id,status,config_snapshot,binding_snapshot,generation_owner,generation_until) VALUES($1,$1,$2,$3,$4,$4,'profile','generating',$5,$6,'original-owner',now()+interval '1 minute')`, ref, self, conversation, owner, jsonBytes(settings), jsonBytes(map[string]any{"reply_contract": contract}))
	if err != nil {
		t.Fatal(err)
	}
	service := newFactOnlyReplyService(app)
	if err := service.claimReplyStyleRepair(ctx, ref, "foreign-owner", ReplyStyleTrace{}); err == nil {
		t.Fatal("foreign lease claimed quota")
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for n := 0; n < 2; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- service.claimReplyStyleRepair(ctx, ref, "original-owner", ReplyStyleTrace{FirstText: "PRIVATE_UNSENT_CANDIDATE"})
		}()
	}
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatal("quota CAS not atomic", success)
	}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.conversation_reply_drafts SET status='waiting_media',generation_owner='resumed-owner' WHERE id=$1`, ref); err != nil {
		t.Fatal(err)
	}
	if err := newFactOnlyReplyService(app).claimReplyStyleRepair(ctx, ref, "resumed-owner", ReplyStyleTrace{}); err == nil {
		t.Fatal("restart/media resume reset quota")
	}
	var snapshot []byte
	if err := repo.Pool().QueryRow(ctx, `SELECT binding_snapshot FROM public.conversation_reply_drafts WHERE id=$1`, ref).Scan(&snapshot); err != nil {
		t.Fatal(err)
	}
	binding := decodeObject(snapshot)
	if binding["style_repair_claimed"] != true || mapValue(binding["style_candidate"])["first_text"] != "PRIVATE_UNSENT_CANDIDATE" {
		t.Fatal("durable claim/candidate lost", binding)
	}
	restored, err := storedReplyExpressionContract(binding)
	if err != nil || restored != contract {
		t.Fatal("quota update changed expression", restored, err)
	}
	var messages int
	if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND text='PRIVATE_UNSENT_CANDIDATE'`, conversation).Scan(&messages); err != nil || messages != 0 {
		t.Fatal("candidate became history", messages, err)
	}
}

func TestPrivateChatCommittedStyleRepairAndMalformedSettlement(t *testing.T) {
	writerCalls, cognitionCalls, settlementRepairs := 0, 0, 0
	selfName := ""
	final := "我想周六见你，一起聊聊新书？"
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		if payload["model"] == "controlled-settlement-writer" {
			writerCalls++
			if writerCalls == 1 {
				return fakeProviderResult{Text: selfName + "抬起头，看向窗外。\n\n她轻声说：“周六见你。”\n\n她笑着问：“一起聊新书？”\n\n她转向柜台，说：“我想见你。”\n\n她低声答：“我愿意。”"}
			}
			if len(arrayValue(payload["tools"])) != 0 {
				t.Error("style repair regained tools")
			}
			return fakeProviderResult{Text: final}
		}
		if len(arrayValue(payload["tools"])) == 0 && providerWireSchemaName(payload) == "conversation_turn_response" {
			settlementRepairs++
			if !strings.Contains(jsonString(payload["messages"]), final) {
				t.Error("settlement repair lost actual final text")
			}
			return fakeProviderResult{Structured: map[string]any{"action_type": "reply", "response_intent": "真实回复已提交", "influences": []any{}}}
		}
		cognitionCalls++
		if payloadHasToolResult(payload) {
			if !strings.Contains(jsonString(payload["messages"]), final) {
				t.Error("Qwen read unrepaired candidate")
			}
			return fakeProviderResult{Text: `{"action_type":`}
		}
		return fakeProviderResult{ToolCalls: []map[string]any{{"call_id": "style-reply", "capability_name": conversationReplyCapabilityName, "arguments": map[string]any{}}}}
	})
	ctx, repo, app, owner, self, conversation := setupMixedMediaReplyTurnWithTransport(t, "style-settlement", router)
	resource, err := repo.GetFluctlight(ctx, self, owner)
	if err != nil {
		t.Fatal(err)
	}
	selfName = stringValue(resource.Identity["name"])
	turn, err := app.AcceptTurn(ctx, owner, conversation, map[string]any{"fluctlight_id": self, "text": "合成测试：继续聊近况。", "turn_id": "style-turn", "idempotency_key": "style-source", "attachment_refs": []any{}})
	if err != nil {
		t.Fatal(err)
	}
	settings := defaultConversationReplySettings()
	settings.Mode = "fact_only"
	settings.RetryLimit = 0
	run := &privateReplyRun{Settings: settings, ConfigVersion: 1, CurrentInput: "合成测试：继续聊近况。", Scope: ToolExecutionRequest{AuthorizationActorID: owner, SubjectActorID: owner, FluctlightID: self, ConversationID: conversation, EvidenceID: turn.InboxID, RunID: "style-run", Surface: CapabilitySurfaceConversation}, Assignment: providerAssignment{Role: conversationReplyModelRole, BaseURL: "http://controlled.invalid", ModelID: "controlled-settlement-writer", Timeout: time.Second, TokenBudget: 1024}}
	ctx = context.WithValue(ctx, privateReplyRunKey{}, run)
	registry, err := app.privateReplyRegistry(run)
	if err != nil {
		t.Fatal(err)
	}
	ctx = withCapabilityRunRegistry(ctx, registry)
	if _, err := app.ProcessCognitionInbox(ctx, turn.InboxID); err != nil {
		t.Fatal(err)
	}
	var count int
	var text string
	if err := repo.Pool().QueryRow(ctx, `SELECT count(*),COALESCE(min(text),'') FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant'`, conversation).Scan(&count, &text); err != nil || count != 1 || text != final {
		t.Fatal(count, text, err)
	}
	var payload []byte
	if err := repo.Pool().QueryRow(ctx, `SELECT payload FROM public.conversation_reply_drafts WHERE source_fact_id=$1`, turn.InboxID).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	if mapValue(decodeObject(payload)["style"])["repair_count"] != float64(1) {
		t.Fatal("repair trace missing", string(payload))
	}
	if decodeObject(payload)["committed_text"] != final || mapValue(decodeObject(payload)["committed_metadata"])["prompt_version"] != factOnlyReplyPromptVersion {
		t.Fatal("actual commit trace missing", string(payload))
	}
	if writerCalls != 2 || cognitionCalls != 2 || settlementRepairs != 1 {
		t.Fatal("unexpected calls", writerCalls, cognitionCalls, settlementRepairs)
	}
	if _, err := app.ProcessCognitionInbox(ctx, turn.InboxID); err != nil {
		t.Fatal(err)
	}
	if writerCalls != 2 || cognitionCalls != 2 || settlementRepairs != 1 {
		t.Fatal("published replay regenerated")
	}
}
