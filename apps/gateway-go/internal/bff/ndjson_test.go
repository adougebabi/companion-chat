package bff

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestTranslateCoreNDJSONPreservesSplitUTF8AndMapsTerminal(t *testing.T) {
	core := strings.Join([]string{
		coreFrame("token", "turn-1", 0, map[string]any{"text": "你"}),
		coreFrame("completed", "turn-1", 1, map[string]any{}),
	}, "")
	encoded := []byte(core)
	utf8Offset := bytes.Index(encoded, []byte("你")) + 1 // split inside the rune
	if utf8Offset <= 0 || utf8Offset >= len(encoded) {
		t.Fatalf("UTF-8 split offset = %d, body length = %d", utf8Offset, len(encoded))
	}

	var output bytes.Buffer
	response := &http.Response{Body: io.NopCloser(&chunkReader{data: encoded, chunkSize: utf8Offset})}
	if err := TranslateCoreNDJSON(context.Background(), response, &output); err != nil {
		t.Fatalf("TranslateCoreNDJSON() error = %v", err)
	}
	events := decodeFrames(t, output.Bytes())
	if got := []string{events[0].Type, events[1].Type}; !equalStrings(got, []string{"token", "completed"}) {
		t.Fatalf("event types = %#v", got)
	}
	if events[0].Sequence != 0 || events[1].Sequence != 1 || events[0].TurnID != "turn-1" {
		t.Fatalf("events = %#v", events)
	}
	if got := events[0].Payload["text"]; got != "你" {
		t.Fatalf("token text = %#v", got)
	}
}

func TestTranslateCoreNDJSONWorksWithHTTPTestServerReader(t *testing.T) {
	recorder := httptest.NewRecorder()
	upstream := responseBody(coreFrame("completed", "turn-http", 0, map[string]any{}))
	if err := TranslateCoreNDJSON(context.Background(), upstream, recorder); err != nil {
		t.Fatalf("TranslateCoreNDJSON() error = %v", err)
	}
	events := decodeFrames(t, recorder.Body.Bytes())
	if len(events) != 1 || events[0].Type != "completed" || events[0].TurnID != "turn-http" {
		t.Fatalf("events = %#v", events)
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/x-ndjson; charset=utf-8" {
		t.Fatalf("Content-Type = %q", got)
	}
}

func TestTranslateCoreNDJSONRejectsInvalidJSONAndUTF8(t *testing.T) {
	tests := []struct {
		name string
		body []byte
		code string
	}{
		{name: "invalid json", body: []byte(`{"type":"token","turn_id":"turn-1","sequence":0,"payload":` + "\n"), code: CoreStreamInvalid},
		{name: "invalid utf8", body: append([]byte{0xff, 0xfe}, '\n'), code: CoreStreamInvalid},
		{name: "wrong envelope shape", body: []byte(`{"type":"token","turn_id":"turn-1","sequence":0,"payload":[]}` + "\n"), code: InvalidCoreEvent},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			var output bytes.Buffer
			response := &http.Response{Body: io.NopCloser(bytes.NewReader(testCase.body))}
			if err := TranslateCoreNDJSON(context.Background(), response, &output); err != nil {
				t.Fatalf("TranslateCoreNDJSON() error = %v", err)
			}
			events := decodeFrames(t, output.Bytes())
			if len(events) != 1 || events[0].Type != "error" || events[0].Payload["code"] != testCase.code {
				t.Fatalf("events = %#v, want one %s error", events, testCase.code)
			}
		})
	}
}

func TestTranslateCoreNDJSONRejectsNestedHiddenPayload(t *testing.T) {
	body := coreFrame("token", "turn-hidden", 0, map[string]any{
		"safe": []any{map[string]any{"hidden-reasoning": "private"}},
	})
	var output bytes.Buffer
	if err := TranslateCoreNDJSON(context.Background(), responseBody(body), &output); err != nil {
		t.Fatalf("TranslateCoreNDJSON() error = %v", err)
	}
	events := decodeFrames(t, output.Bytes())
	if len(events) != 1 || events[0].Payload["code"] != HiddenCorePayload {
		t.Fatalf("events = %#v", events)
	}
	if events[0].TurnID != "turn-hidden" || events[0].Sequence != 0 {
		t.Fatalf("bounded error correlation = %#v", events[0])
	}
}

func TestTranslateCoreNDJSONMapsActionResultMessageAndMedia(t *testing.T) {
	message := map[string]any{
		"id":              "message-1",
		"conversation_id": "conversation-1",
		"sequence":        3,
		"author_actor_id": "actor-1",
		"kind":            "assistant",
		"text":            "hello",
		"created_at":      "2026-08-29T00:00:00Z",
	}
	media := map[string]any{
		"id":              "message-2",
		"conversation_id": "conversation-1",
		"sequence":        4,
		"author_actor_id": "actor-1",
		"kind":            "media_reference",
		"text":            "image",
		"attachment_refs": []any{"attachment-1"},
	}
	body := coreFrame("action_result", "turn-actions", 0, map[string]any{"message": message}) +
		coreFrame("action_result", "turn-actions", 1, map[string]any{"message": media}) +
		coreFrame("completed", "turn-actions", 2, map[string]any{})
	var output bytes.Buffer
	if err := TranslateCoreNDJSON(context.Background(), responseBody(body), &output); err != nil {
		t.Fatalf("TranslateCoreNDJSON() error = %v", err)
	}
	events := decodeFrames(t, output.Bytes())
	if len(events) != 3 || events[0].Type != "message" || events[1].Type != "media" {
		t.Fatalf("events = %#v", events)
	}
	if got := events[0].Payload["message"].(map[string]any); got["conversationId"] != "conversation-1" || got["attachmentRefs"].([]any) == nil {
		t.Fatalf("message mapping = %#v", got)
	}
	if got := events[1].Payload["message"].(map[string]any); got["kind"] != "media_reference" || got["attachmentRefs"].([]any)[0] != "attachment-1" {
		t.Fatalf("media mapping = %#v", got)
	}
}

func TestTranslateCoreNDJSONRejectsSequenceAndTurnViolations(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "sequence skip", body: coreFrame("token", "turn-1", 1, map[string]any{})},
		{name: "turn mismatch", body: coreFrame("token", "turn-1", 0, map[string]any{}) + coreFrame("completed", "turn-2", 1, map[string]any{})},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			var output bytes.Buffer
			if err := TranslateCoreNDJSON(context.Background(), responseBody(testCase.body), &output); err != nil {
				t.Fatalf("TranslateCoreNDJSON() error = %v", err)
			}
			events := decodeFrames(t, output.Bytes())
			last := events[len(events)-1]
			if last.Type != "error" || last.Payload["code"] != CoreSequenceInvalid {
				t.Fatalf("events = %#v", events)
			}
		})
	}
}

func TestTranslateCoreNDJSONReportsIncompleteEOFAndStopsAfterTerminal(t *testing.T) {
	tests := []struct {
		name string
		body string
		want int
	}{
		{name: "empty", body: "", want: 1},
		{name: "final frame has no newline", body: strings.TrimSuffix(coreFrame("completed", "turn-eof", 0, map[string]any{}), "\n"), want: 1},
		{name: "nonterminal then EOF", body: coreFrame("token", "turn-eof", 0, map[string]any{}), want: 2},
		{name: "terminal suppresses later frames", body: coreFrame("completed", "turn-eof", 0, map[string]any{}) + coreFrame("token", "turn-eof", 1, map[string]any{}), want: 1},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			var output bytes.Buffer
			if err := TranslateCoreNDJSON(context.Background(), responseBody(testCase.body), &output); err != nil {
				t.Fatalf("TranslateCoreNDJSON() error = %v", err)
			}
			events := decodeFrames(t, output.Bytes())
			if len(events) != testCase.want {
				t.Fatalf("len(events) = %d, want %d (%#v)", len(events), testCase.want, events)
			}
			if testCase.name != "terminal suppresses later frames" && events[len(events)-1].Type != "error" {
				t.Fatalf("last event = %#v, want error", events[len(events)-1])
			}
			if testCase.name == "nonterminal then EOF" && events[1].Payload["code"] != CoreStreamIncomplete {
				t.Fatalf("incomplete event = %#v", events[1])
			}
		})
	}
}

func TestTranslateCoreNDJSONAbortClosesReaderAndSuppressesLaterWrites(t *testing.T) {
	body := newBlockingBody([]byte(coreFrame("token", "turn-abort", 0, map[string]any{"text": "first"})))
	controller, cancel := context.WithCancel(context.Background())
	defer cancel()
	output := newSignalBuffer()
	done := make(chan error, 1)
	go func() { done <- TranslateCoreNDJSON(controller, &http.Response{Body: body}, output) }()

	select {
	case <-output.ready:
	case <-time.After(2 * time.Second):
		t.Fatal("translator did not emit first frame")
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("TranslateCoreNDJSON() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("translator did not finish after cancellation")
	}
	if !body.wasClosed() {
		t.Fatal("upstream body was not closed on cancellation")
	}
	events := decodeFrames(t, output.Bytes())
	if len(events) != 1 || events[0].Type != "token" {
		t.Fatalf("events after abort = %#v", events)
	}
}

func TestTranslateCoreNDJSONAlreadyAbortedEmitsNoFrame(t *testing.T) {
	body := newBlockingBody(nil)
	controller, cancel := context.WithCancel(context.Background())
	cancel()
	var output bytes.Buffer
	if err := TranslateCoreNDJSON(controller, &http.Response{Body: body}, &output); err != nil {
		t.Fatalf("TranslateCoreNDJSON() error = %v", err)
	}
	if output.Len() != 0 || !body.wasClosed() {
		t.Fatalf("output length = %d, closed = %v", output.Len(), body.wasClosed())
	}
}

func TestTranslateCoreNDJSONMissingBodyReturnsBoundedSetupError(t *testing.T) {
	err := TranslateCoreNDJSON(context.Background(), &http.Response{}, io.Discard)
	var translationErr *NdjsonTranslationError
	if !errors.As(err, &translationErr) || translationErr.Code != CoreStreamBodyMissing {
		t.Fatalf("error = %v, want %s", err, CoreStreamBodyMissing)
	}
}

type frame struct {
	Type     string         `json:"type"`
	TurnID   string         `json:"turnId"`
	Sequence int            `json:"sequence"`
	Payload  map[string]any `json:"payload"`
}

func decodeFrames(t *testing.T, value []byte) []frame {
	t.Helper()
	lines := bytes.Split(bytes.TrimSpace(value), []byte("\n"))
	if len(lines) == 1 && len(lines[0]) == 0 {
		return nil
	}
	frames := make([]frame, 0, len(lines))
	for _, line := range lines {
		var current frame
		if err := json.Unmarshal(line, &current); err != nil {
			t.Fatalf("invalid output frame %q: %v", line, err)
		}
		frames = append(frames, current)
	}
	return frames
}

func coreFrame(eventType, turnID string, sequence int, payload map[string]any) string {
	value, err := json.Marshal(map[string]any{
		"type":     eventType,
		"turn_id":  turnID,
		"sequence": sequence,
		"payload":  payload,
	})
	if err != nil {
		panic(err)
	}
	return string(value) + "\n"
}

func responseBody(value string) *http.Response {
	return &http.Response{Body: io.NopCloser(strings.NewReader(value))}
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

type chunkReader struct {
	data      []byte
	position  int
	chunkSize int
}

func (reader *chunkReader) Read(value []byte) (int, error) {
	if reader.position == len(reader.data) {
		return 0, io.EOF
	}
	count := reader.chunkSize
	if count <= 0 || count > len(value) {
		count = len(value)
	}
	if remaining := len(reader.data) - reader.position; count > remaining {
		count = remaining
	}
	copy(value[:count], reader.data[reader.position:reader.position+count])
	reader.position += count
	return count, nil
}

type blockingBody struct {
	first  []byte
	once   sync.Once
	closed chan struct{}
	mu     sync.Mutex
	isDone bool
}

type signalBuffer struct {
	mu    sync.Mutex
	value bytes.Buffer
	ready chan struct{}
	once  sync.Once
}

func newSignalBuffer() *signalBuffer {
	return &signalBuffer{ready: make(chan struct{})}
}

func (buffer *signalBuffer) Write(value []byte) (int, error) {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	count, err := buffer.value.Write(value)
	buffer.once.Do(func() { close(buffer.ready) })
	return count, err
}

func (buffer *signalBuffer) Bytes() []byte {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	return append([]byte(nil), buffer.value.Bytes()...)
}

func newBlockingBody(first []byte) *blockingBody {
	return &blockingBody{first: first, closed: make(chan struct{})}
}

func (body *blockingBody) Read(value []byte) (int, error) {
	var count int
	body.once.Do(func() {
		count = copy(value, body.first)
	})
	if count > 0 {
		return count, nil
	}
	<-body.closed
	return 0, io.EOF
}

func (body *blockingBody) Close() error {
	body.mu.Lock()
	if !body.isDone {
		body.isDone = true
		close(body.closed)
	}
	body.mu.Unlock()
	return nil
}

func (body *blockingBody) wasClosed() bool {
	body.mu.Lock()
	defer body.mu.Unlock()
	return body.isDone
}
