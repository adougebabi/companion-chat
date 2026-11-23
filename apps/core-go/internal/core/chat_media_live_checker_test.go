package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
)

type chatLiveCheckerTransport func(*http.Request) (*http.Response, error)

func (f chatLiveCheckerTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// The main candidate is controlled; only the publication checker calls the real
// model. PostgreSQL, native Tool execution and message publication are real.
// This is deliberately not evidence of a full live conversation generation.
func TestLiveChatMediaCheckerPublication(t *testing.T) {
	base, model := os.Getenv("CHAT_DELIVERY_LIVE_CHECKER_URL"), os.Getenv("CHAT_DELIVERY_LIVE_CHECKER_MODEL")
	if base == "" || model == "" {
		t.Skip("explicit live checker URL/model required")
	}
	u, err := url.Parse(base)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		t.Fatal("explicit credential-free checker base URL required")
	}
	db, err := url.Parse(os.Getenv("GO_CORE_TEST_DATABASE_URL"))
	if err != nil || net.ParseIP(db.Hostname()) == nil || !net.ParseIP(db.Hostname()).IsLoopback() {
		t.Fatal("live checker validation requires loopback disposable PostgreSQL")
	}
	for _, tc := range []struct {
		name, text string
		publish    bool
	}{
		{"intention", "是张配色图，想分享给你看。（发完后，看着对话框，又补了一句。）", true},
		{"unsupported_completion", "图片已经发给你了。", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			router := newFakeProviderRouter().otherwise(func(map[string]any) fakeProviderResult {
				calls++
				return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall(fmt.Sprintf("live-main-%d", calls), conversationReplyCapabilityName, map[string]any{"text": tc.text, "decision": map[string]any{"action_type": "reply", "response_intent": "回答", "influences": []any{}}})}}
			})
			ctx, repo, app, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, "live-chat-check-"+tc.name, router)
			if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3)`, owner, fid, conversation); err != nil {
				t.Fatal(err)
			}
			persistConversationReplyMode(t, ctx, repo, "direct")
			fallback := app.Provider.HTTP.Transport
			transport := http.DefaultTransport.(*http.Transport).Clone()
			transport.Proxy = nil
			t.Cleanup(transport.CloseIdleConnections)
			checks := 0
			app.Provider.HTTP = &http.Client{Transport: chatLiveCheckerTransport(func(r *http.Request) (*http.Response, error) {
				raw, err := io.ReadAll(r.Body)
				if err != nil {
					return nil, err
				}
				r.Body = io.NopCloser(bytes.NewReader(raw))
				var payload map[string]any
				if err = json.Unmarshal(raw, &payload); err != nil {
					return nil, err
				}
				if providerWireSchemaName(payload) != directMediaDeliveryCheckSchema {
					return fallback.RoundTrip(r)
				}
				checks++
				payload["model"] = model
				wire, err := json.Marshal(payload)
				if err != nil {
					return nil, err
				}
				clone := r.Clone(r.Context())
				target := *u
				target.Path = strings.TrimRight(u.Path, "/") + "/chat/completions"
				clone.URL = &target
				clone.Host = u.Host
				clone.Header = r.Header.Clone()
				clone.Header.Del("Authorization")
				clone.Header.Del("X-API-Key")
				clone.Body = io.NopCloser(bytes.NewReader(wire))
				clone.ContentLength = int64(len(wire))
				return transport.RoundTrip(clone)
			})}
			_, turnErr := app.HandleTurn(ctx, owner, conversation, map[string]any{"fluctlight_id": fid, "text": "你说的是什么图片？", "turn_id": "live-chat-" + tc.name, "idempotency_key": "live-chat-" + tc.name, "attachment_refs": []any{}})
			var messages int
			if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant' AND text=$2`, conversation, tc.text).Scan(&messages); err != nil {
				t.Fatal(err)
			}
			if tc.publish {
				if turnErr != nil || messages != 1 || checks < 1 || checks > 3 {
					t.Fatal(messages, checks, turnErr)
				}
			} else if ConversationTurnFailureCode(turnErr) != "direct_media_delivery_unresolved" || messages != 0 || checks != 3 {
				t.Fatal(messages, checks, turnErr)
			}
			t.Logf("real checker calls=%d; committed messages=%d", checks, messages)
		})
	}
}
