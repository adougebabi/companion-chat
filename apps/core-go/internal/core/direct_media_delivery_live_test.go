package core

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// This is a real-Provider test of the new typed checker, not a claim of real
// image generation or an end-to-end user conversation.
func TestLiveDirectMediaDeliveryChecker(t *testing.T) {
	base, primary, _, evidenceDir := r1LoopbackProvider(t)
	parsed, _ := url.Parse(base)
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	defer transport.CloseIdleConnections()
	for round := 1; round <= 2; round++ {
		round := round
		ok := t.Run(fmt.Sprintf("round_%d", round), func(t *testing.T) {
			seedCtx, repo, app, owner, fid, conversation := setupDirectPublicationToolTest(t, fmt.Sprintf("live-media-check-%d", round))
			if _, err := repo.Pool().Exec(seedCtx, `INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES($1,'fixture','fixture')`, owner); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
			defer cancel()
			roundDir := filepath.Join(evidenceDir, fmt.Sprintf("m1-check-round-%d", round))
			if err := os.Mkdir(roundDir, 0o700); err != nil {
				t.Fatal(err)
			}
			observer := &r1LiveTransport{inner: transport, host: parsed.Host, evidenceDir: roundDir}
			app.Provider.HTTP = &http.Client{Transport: observer}
			endpoint := fmt.Sprintf("live-media-check-endpoint-%d", round)
			endpointBody := map[string]any{"endpoint_id": endpoint, "kind": "openai_compatible", "base_url": base, "secret_purpose": "live-media-check-no-secret", "expected_versions": r1Versions(t, ctx, app, owner), "idempotency_key": "live-media-check-endpoint"}
			if err := app.ConfigureProviderEndpoint(ctx, owner, endpointBody); err != nil {
				t.Fatal(err)
			}
			r1SaveBinding(t, ctx, app, owner, endpoint, conversationReplyModelRole, primary, "live-media-check-default")
			projection, err := app.BuildContextProjectionFor(ctx, ContextProjectionRequest{AuthorizationActorID: owner, SpeakerActorID: owner, FluctlightID: fid, ConversationID: conversation, MemoryOperation: MemoryForReflection, MemoryConversationMode: MemoryConversationExact})
			if err != nil {
				t.Fatal(err)
			}
			cases := []struct {
				name, input, candidate string
				deliver                bool
				priorMedia             bool
				wantRef                bool
				wantRejection          bool
			}{
				{name: "unfounded", input: "照片发了吗？", candidate: "我已经把照片发给你了。", wantRejection: true},
				{name: "denial", input: "照片发了吗？", candidate: "我还没发照片，等会儿再发给你。"},
				{name: "future", input: "图片发了吗？", candidate: "等图片发送完成后，我会告诉你。"},
				{name: "cross_turn_unfounded", input: "弄好了没？", candidate: "已经贴到聊天里了，你往上翻就能看到。", priorMedia: true, wantRejection: true},
				{name: "history", input: "窗外日落的照片发过吗？", candidate: "我之前已经把窗外日落的照片发给你了。", deliver: true, wantRef: true},
				{name: "unrelated", input: "海边落日的照片发过吗？", candidate: "我已经把海边落日的照片发给你了。", wantRejection: true},
				{name: "cross_turn_matching", input: "窗外日落弄好了没？", candidate: "已经贴到聊天里了，你往上翻就能看到。", priorMedia: true, wantRef: true},
			}
			for _, tc := range cases {
				if tc.deliver {
					seedDirectMediaEvidence(t, ctx, repo, app, fid, conversation, fmt.Sprintf("live-history-%d", round), "completed", "窗外日落", true)
				}
				deliveries, err := directMediaDeliveries(ctx, repo.Pool(), owner, owner, fid, conversation, false)
				if err != nil {
					t.Fatal(err)
				}
				start := observer.begin(tc.name, "media-check")
				caseProjection := projection
				if tc.priorMedia {
					caseProjection.RecentMessages = []map[string]any{
						{"kind": "user", "text": "想看看窗外日落的照片。"},
						{"kind": "assistant", "text": "照片还没准备好。"},
						{"kind": "user", "text": tc.input},
					}
				}
				refs, checkErr := app.checkDirectMediaDeliveryClaims(ctx, caseProjection, tc.input, tc.candidate, deliveries, nil)
				requests, evidenceErr := observer.since(start)
				if evidenceErr != nil || len(requests) == 0 {
					t.Fatalf("real checker evidence unavailable: case=%s requests=%d err=%v", tc.name, len(requests), evidenceErr)
				}
				for _, request := range requests {
					t.Logf("M1_LIVE round=%d case=%s request_model=%s response_model=%s http=%d response_file=%s", round, tc.name, request.RequestModel, request.ResponseModel, request.HTTPStatus, request.ResponseFile)
					if request.RequestModel != primary || request.ResponseModel != primary || request.HTTPStatus != http.StatusOK {
						t.Fatalf("real checker model/HTTP mismatch: case=%s", tc.name)
					}
				}
				code, _ := capabilityErrorInfo(checkErr, "", false)
				if tc.wantRejection {
					if checkErr == nil || code != "direct_media_delivery_unverified" || len(refs) != 0 {
						t.Fatalf("real checker did not reject unsupported claim: case=%s refs=%d code=%s err=%v", tc.name, len(refs), code, checkErr)
					}
				} else if checkErr != nil || (len(refs) > 0) != tc.wantRef {
					t.Fatalf("real checker rejected a permitted case: case=%s refs=%d code=%s err=%v", tc.name, len(refs), code, checkErr)
				}
				t.Logf("M1_LIVE_RESULT round=%d case=%s contract=pass evidence_refs=%d", round, tc.name, len(refs))
			}
		})
		if !ok {
			t.FailNow()
		}
	}
}
