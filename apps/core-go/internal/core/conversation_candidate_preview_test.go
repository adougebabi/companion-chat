package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/cloudwego/eino/schema"
	"github.com/redis/go-redis/v9"
)

func TestCandidatePreviewJSONPrefix(t *testing.T) {
	for _, test := range []struct {
		source, want string
		valid        bool
	}{
		{`{"text":"你好`, "你好", true},
		{`{"decision":{"text":"hidden"},"text":"visible","decision2":"hidden"}`, "visible", true},
		{`{"text":"a\"b\\c\n`, "a\"b\\c\n", true},
		{`{"text":"a\uD83D`, "a", true},
		{`{"text":"a\uD83D\uDE00`, "a😀", true},
		{`{"text":"a\uDC00`, "", false},
		{`{"reasoning":{"text":"hidden"}`, "", false},
		{`{"text":42}`, "", false},
		{`{"text":"a\q`, "", false},
	} {
		got, ok := candidateReplyText(test.source)
		if got != test.want || ok != test.valid {
			t.Errorf("source=%q got=%q valid=%t want=%q valid=%t", test.source, got, ok, test.want, test.valid)
		}
	}
}

func TestCandidatePreviewObserverOnlyNativeReplyAndReplacesPhysicalCandidate(t *testing.T) {
	var snapshots []conversationCandidatePreview
	ctx := withFormalAgentDefinition(context.Background(), formalAgentRegistry[FormalAgentConversationCognition])
	ctx = withProviderDiagnosticSecrets(ctx, "secret-key")
	ctx = context.WithValue(ctx, conversationCandidateSinkKey{}, conversationCandidateSink(func(p conversationCandidatePreview) { snapshots = append(snapshots, p) }))
	observe := newConversationCandidateObserver(ctx)
	index := 0
	observe(&schema.Message{Content: "hidden content", ReasoningContent: "hidden reasoning", ToolCalls: []schema.ToolCall{{Index: &index, ID: "reply", Function: schema.FunctionCall{Name: "conversation_reply", Arguments: `{"text":"hello secret-`}}}}, nil)
	if len(snapshots) != 2 || snapshots[1].Text != "hello " {
		t.Fatalf("partial secret or hidden fields exposed: %#v", snapshots)
	}
	observe(&schema.Message{ToolCalls: []schema.ToolCall{{Index: &index, Function: schema.FunctionCall{Arguments: `key","decision":{"reasoning":"hidden"}}`}}}}, io.EOF)
	if got := snapshots[len(snapshots)-1].Text; got != "hello [REDACTED_SECRET]" {
		t.Fatal(got)
	}
	next := newConversationCandidateObserver(ctx)
	if snapshots[len(snapshots)-1].Text != "" || snapshots[len(snapshots)-1].ID == snapshots[0].ID {
		t.Fatal("new physical request did not replace prior preview")
	}
	next(&schema.Message{ToolCalls: []schema.ToolCall{{Index: &index, ID: "media", Function: schema.FunctionCall{Name: "media.image.generate", Arguments: `{"text":"hidden tool"}`}}}}, io.EOF)
	if snapshots[len(snapshots)-1].Text != "" {
		t.Fatal("non-reply tool exposed")
	}
}

func candidateTestRedis(t *testing.T, app *App) {
	t.Helper()
	server, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	app.Redis = client
	t.Cleanup(func() { client.Close(); server.Close() })
}

func TestCandidatePreviewFencesClaimsAndTerminalStates(t *testing.T) {
	ctx, repo, app, owner, fid, conversation := durableReplyFixture(t, "preview-fence")
	candidateTestRedis(t, app)
	accepted, err := app.AcceptTurn(ctx, owner, conversation, map[string]any{"fluctlight_id": fid, "text": "你好", "turn_id": "preview-fence", "idempotency_key": "preview-fence"})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = app.claimCognitionInbox(ctx, accepted.InboxID, "old-owner"); err != nil {
		t.Fatal(err)
	}
	oldCtx := context.WithValue(ctx, directTurnClaimKey{}, directTurnClaim{InboxID: accepted.InboxID, Owner: "old-owner"})
	oldCtx, enabled := app.withConversationCandidatePreview(oldCtx, accepted.InboxID)
	if !enabled {
		t.Fatal("preview not enabled")
	}
	oldSink := oldCtx.Value(conversationCandidateSinkKey{}).(conversationCandidateSink)
	oldSink(conversationCandidatePreview{ID: "old", Text: "old preview"})
	if got := app.readConversationCandidatePreview(ctx, accepted.InboxID); got.ID != "old" {
		t.Fatal(got)
	}
	if _, err = repo.Pool().Exec(ctx, `UPDATE public.cognition_inbox SET claimed_by='new-owner' WHERE id=$1`, accepted.InboxID); err != nil {
		t.Fatal(err)
	}
	oldSink(conversationCandidatePreview{ID: "late-old", Text: "must stay hidden"})
	if got := app.readConversationCandidatePreview(ctx, accepted.InboxID); got.ID != "" {
		t.Fatal("stale claim exposed", got)
	}
	newCtx := context.WithValue(ctx, directTurnClaimKey{}, directTurnClaim{InboxID: accepted.InboxID, Owner: "new-owner"})
	newCtx, _ = app.withConversationCandidatePreview(newCtx, accepted.InboxID)
	newCtx.Value(conversationCandidateSinkKey{}).(conversationCandidateSink)(conversationCandidatePreview{ID: "new", Text: "new preview"})
	if got := app.readConversationCandidatePreview(ctx, accepted.InboxID); got.ID != "new" {
		t.Fatal(got)
	}
	if err = app.CancelTurn(ctx, owner, conversation, accepted.TurnID); err != nil {
		t.Fatal(err)
	}
	if got := app.readConversationCandidatePreview(ctx, accepted.InboxID); got.ID != "" {
		t.Fatal("cancelled preview exposed", got)
	}
}

type candidateFrameWriter struct{ frames chan map[string]any }

func (w *candidateFrameWriter) Header() http.Header { return http.Header{} }
func (w *candidateFrameWriter) WriteHeader(int)     {}
func (w *candidateFrameWriter) Flush()              {}
func (w *candidateFrameWriter) Write(raw []byte) (int, error) {
	var frame map[string]any
	if err := json.Unmarshal(raw, &frame); err != nil {
		return 0, err
	}
	w.frames <- frame
	return len(raw), nil
}

func TestCandidatePreviewDurableWorkerEmitsBeforeCommit(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	transport := projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		raw, err := io.ReadAll(request.Body)
		if err != nil {
			return nil, err
		}
		payload := decodeObject(raw)
		if payload["stream"] != true {
			return nil, fmt.Errorf("expected native streaming request")
		}
		reader, writer := io.Pipe()
		go func() {
			defer writer.Close()
			send := func(delta map[string]any, finish any) bool {
				raw := jsonString(map[string]any{"id": "controlled-preview", "model": "controlled-model", "choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": finish}}})
				_, err := fmt.Fprintf(writer, "data: %s\n\n", raw)
				return err == nil
			}
			if !send(map[string]any{"role": "assistant", "reasoning_content": "hidden reasoning", "tool_calls": []any{map[string]any{"index": 0, "id": "native-preview", "type": "function", "function": map[string]any{"name": "conversation_reply", "arguments": `{"text":"正在生成`}}}}, nil) {
				return
			}
			select {
			case <-release:
			case <-request.Context().Done():
				return
			}
			if !send(map[string]any{"tool_calls": []any{map[string]any{"index": 0, "function": map[string]any{"arguments": `的回复。","decision":{"action_type":"reply","response_intent":"回应","influences":[]}}`}}}}, "tool_calls") {
				return
			}
			fmt.Fprint(writer, "data: [DONE]\n\n")
		}()
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: reader, Request: request}, nil
	})
	ctx, repo, app, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, "preview-worker", transport)
	candidateTestRedis(t, app)
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3)`, owner, fid, conversation); err != nil {
		t.Fatal(err)
	}
	persistConversationReplyMode(t, ctx, repo, "direct")
	payload := map[string]any{"fluctlight_id": fid, "text": "你好", "turn_id": "preview-worker", "idempotency_key": "preview-worker"}
	accepted, err := app.AcceptTurn(ctx, owner, conversation, payload)
	if err != nil {
		t.Fatal(err)
	}
	workerCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	observerCtx, disconnect := context.WithCancel(workerCtx)
	defer disconnect()
	defer cancel()
	frames := &candidateFrameWriter{frames: make(chan map[string]any, 32)}
	observerDone := make(chan error, 1)
	go func() { observerDone <- app.StreamDurableTurn(observerCtx, frames, owner, conversation, payload) }()
	workerDone := make(chan error, 1)
	go func() { _, err := app.ProcessCognitionInbox(workerCtx, accepted.InboxID); workerDone <- err }()
	sawPreview := false
	firstCandidateID := ""
	for !sawPreview {
		select {
		case frame := <-frames.frames:
			if frame["type"] == "candidate_preview" && mapValue(frame["payload"])["text"] == "正在生成" {
				sawPreview = true
				firstCandidateID = stringValue(mapValue(frame["payload"])["candidate_id"])
			}
			if frame["type"] == "error" || frame["type"] == "completed" {
				t.Fatal("terminal before preview", frame)
			}
		case err := <-workerDone:
			t.Fatal("worker ended before preview", err)
		case <-workerCtx.Done():
			t.Fatal("no incremental preview before provider completion")
		}
	}
	var committed int
	if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.conversation_messages WHERE source_fact_id=$1 AND kind='assistant'`, accepted.InboxID).Scan(&committed); err != nil || committed != 0 {
		t.Fatal("preview became formal message", committed, err)
	}
	disconnect()
	if err := <-observerDone; err != context.Canceled {
		t.Fatal("disconnect affected observer unexpectedly", err)
	}
	frames = &candidateFrameWriter{frames: make(chan map[string]any, 32)}
	go func() { observerDone <- app.StreamDurableTurn(workerCtx, frames, owner, conversation, payload) }()
	reconnected := false
	for !reconnected {
		select {
		case frame := <-frames.frames:
			if frame["type"] == "candidate_preview" && mapValue(frame["payload"])["text"] == "正在生成" {
				if mapValue(frame["payload"])["candidate_id"] != firstCandidateID {
					t.Fatal("reconnect replaced the physical candidate identity", frame)
				}
				reconnected = true
			}
		case err := <-workerDone:
			t.Fatal("observer disconnect cancelled Worker", err)
		case <-workerCtx.Done():
			t.Fatal("reconnect did not receive current snapshot")
		}
	}
	release <- struct{}{}
	if err := <-workerDone; err != nil {
		t.Fatal(err)
	}
	if err := <-observerDone; err != nil {
		t.Fatal(err)
	}
	var all bytes.Buffer
	sawClear, sawMessage, sawCompleted := false, false, false
	for len(frames.frames) > 0 {
		frame := <-frames.frames
		all.WriteString(jsonString(frame))
		switch frame["type"] {
		case "candidate_preview":
			if mapValue(frame["payload"])["text"] == "" {
				sawClear = true
			}
		case "action_result":
			if mapValue(mapValue(frame["payload"])["message"])["text"] == "正在生成的回复。" {
				sawMessage = true
			}
		case "completed":
			sawCompleted = true
		}
	}
	if !sawClear || !sawMessage || !sawCompleted || strings.Contains(all.String(), "hidden reasoning") {
		t.Fatalf("clear=%t message=%t complete=%t frames=%s", sawClear, sawMessage, sawCompleted, all.String())
	}
}
