package core

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// This observer reads only bytes already consumed by Eino. It neither buffers
// the response ahead of the caller nor substitutes model data. Evidence is
// counts of actual SSE delta/DONE records, never raw reasoning or credentials.
type productionStreamWireSpy struct {
	inner                   http.RoundTripper
	mu                      sync.Mutex
	responses, deltas, done int
}

func (s *productionStreamWireSpy) RoundTrip(request *http.Request) (*http.Response, error) {
	response, err := s.inner.RoundTrip(request)
	if err != nil {
		return response, err
	}
	if strings.Contains(response.Header.Get("Content-Type"), "text/event-stream") {
		s.mu.Lock()
		s.responses++
		s.mu.Unlock()
		response.Body = &productionStreamObservedBody{ReadCloser: response.Body, spy: s}
	}
	return response, nil
}

type productionStreamObservedBody struct {
	io.ReadCloser
	spy     *productionStreamWireSpy
	pending []byte
}

func (b *productionStreamObservedBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	b.pending = append(b.pending, p[:n]...)
	for {
		index := bytes.IndexByte(b.pending, '\n')
		if index < 0 {
			break
		}
		line := strings.TrimSpace(string(b.pending[:index]))
		b.pending = b.pending[index+1:]
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		b.spy.mu.Lock()
		if data == "[DONE]" {
			b.spy.done++
		} else {
			var payload map[string]any
			if json.Unmarshal([]byte(data), &payload) == nil {
				for _, raw := range arrayValue(payload["choices"]) {
					delta := mapValue(mapValue(raw)["delta"])
					if stringValue(delta["content"]) != "" || len(arrayValue(delta["tool_calls"])) > 0 {
						b.spy.deltas++
					}
				}
			}
		}
		b.spy.mu.Unlock()
	}
	return n, err
}

func TestLiveStreamTurnFormalAgentNDJSON(t *testing.T) {
	fixture := newFormalAgentE2EFixture(t) // live-only configuration gate, isolated PG
	wire := &productionStreamWireSpy{inner: http.DefaultTransport}
	fixture.spy.inner = wire
	turnID := fixture.runPrefix + "-production-stream"
	writer := httptest.NewRecorder()
	err := fixture.app.StreamTurn(fixture.ctx, writer, fixture.ownerID, fixture.conversationID, map[string]any{
		"fluctlight_id": fixture.fluctlightID, "turn_id": turnID, "idempotency_key": turnID,
		"text": "请只用一句简短的中文确认你已收到这条消息，不需要查询信息或生成图片。", "attachment_refs": []any{},
	})
	if err != nil {
		t.Fatalf("production StreamTurn failed: %v", err)
	}
	scanner := bufio.NewScanner(writer.Body)
	terminal, assistantFrames := 0, 0
	sequence := 0
	var tokenText strings.Builder
	var assistant map[string]any
	for scanner.Scan() {
		var frame map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &frame); err != nil {
			t.Fatal(err)
		}
		if intValue(frame["sequence"]) != sequence {
			t.Fatalf("NDJSON sequence=%v expected=%d", frame["sequence"], sequence)
		}
		sequence++
		payload := mapValue(frame["payload"])
		switch stringValue(frame["type"]) {
		case "error":
			t.Fatalf("production stream terminal error: %s", stringValue(payload["code"]))
		case "token":
			tokenText.WriteString(stringValue(payload["text"]))
		case "action_result":
			if message := mapValue(payload["message"]); stringValue(message["kind"]) == "assistant" {
				assistantFrames++
				assistant = message
			}
		case "completed":
			terminal++
			if len(arrayValue(payload["message_ids"])) != 1 {
				t.Fatal("completed stream must name its committed reply")
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if terminal != 1 || assistantFrames != 1 || tokenText.Len() == 0 || tokenText.String() != stringValue(assistant["text"]) {
		t.Fatalf("NDJSON completion=%d assistant=%d visible bytes=%d", terminal, assistantFrames, tokenText.Len())
	}
	var text, storedTurn, source string
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT text,turn_id,source_fact_id FROM public.conversation_messages WHERE id=$1 AND author_actor_id=$2 AND conversation_id=$3`, stringValue(assistant["id"]), fixture.fluctlightID, fixture.conversationID).Scan(&text, &storedTurn, &source); err != nil {
		t.Fatal(err)
	}
	if text != tokenText.String() || storedTurn != turnID || source == "" {
		t.Fatal("visible stream does not match the committed reply/source identity")
	}
	requests := fixture.spy.snapshot()
	if len(requests) == 0 {
		t.Fatal("production stream made no Provider requests")
	}
	for _, request := range requests {
		if !boolValue(request["stream"]) {
			t.Fatal("production stream fell back to a non-stream Provider request")
		}
	}
	wire.mu.Lock()
	responses, deltas, done := wire.responses, wire.deltas, wire.done
	wire.mu.Unlock()
	if responses != len(requests) || deltas < responses || done != responses {
		t.Fatalf("actual SSE evidence: requests=%d responses=%d delta records=%d DONE=%d", len(requests), responses, deltas, done)
	}
	t.Logf("production StreamTurn: actual streaming requests=%d SSE delta records=%d terminal_frames=%d committed_replies=%d", len(requests), deltas, terminal, assistantFrames)
}
