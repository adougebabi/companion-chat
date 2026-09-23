// Package browser contains the transport-only browser boundary for the public
// gateway.  This file deliberately has no dependency on the Core or domain
// packages: it translates the Core's visible NDJSON envelope into the
// browser envelope while it is being read.
package browser

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"sync"
	"unicode/utf8"
)

const (
	CoreStreamBodyMissing = "core_stream_body_missing"
	CoreStreamInvalid     = "core_stream_invalid"
	CoreStreamIncomplete  = "core_stream_incomplete"
	InvalidCoreEvent      = "invalid_core_event"
	HiddenCorePayload     = "hidden_core_payload"
	CoreSequenceInvalid   = "core_sequence_invalid"
	maxCoreFrameBytes     = 4 << 20
)

const browserTurnErrorMessage = "The turn could not be completed"

// CoreStreamEvent is the visible Core application stream envelope. The payload is
// intentionally untyped at this boundary; the event envelope and the hidden
// payload policy are the only semantics owned by the browser boundary.
type CoreStreamEvent struct {
	Type     string
	TurnID   string
	Sequence int
	Payload  map[string]any
}

// BrowserStreamEvent is the browser-facing stream envelope.  action_result
// events are represented as message or media events before they reach this
// type.
type BrowserStreamEvent struct {
	Type     string         `json:"type"`
	TurnID   string         `json:"turnId"`
	Sequence int            `json:"sequence"`
	Payload  map[string]any `json:"payload"`
}

// NdjsonTranslationError represents an error which prevents the translator
// from being started at all (for example, a missing upstream body).  Protocol
// violations after streaming starts are represented by one bounded browser
// error frame, matching the checked browser contract.
type NdjsonTranslationError struct {
	Code  string
	Cause error
}

func (err *NdjsonTranslationError) Error() string {
	if err == nil {
		return ""
	}
	return err.Code
}

func (err *NdjsonTranslationError) Unwrap() error { return err.Cause }

var errTranslationCanceled = errors.New("ndjson translation canceled")

var coreStreamTypes = map[string]struct{}{
	"token":         {},
	"action_result": {},
	"completed":     {},
	"error":         {},
	"heartbeat":     {},
}

var hiddenPayloadKeys = map[string]struct{}{
	"perception":      {},
	"appraisal":       {},
	"reasoning":       {},
	"hiddenreasoning": {},
	"credentials":     {},
	"authorization":   {},
	"apikey":          {},
	"rawprompt":       {},
	"rawresponse":     {},
}

// browserTurnErrorCodes is the allowlist for application errors emitted by
// Core. Error payloads are a separate contract from progress payloads: retain
// only a stable code and a fixed message, even when an older Core uses
// payload.error or includes status/details beside it. Unknown values fall back
// to the generic turn failure so provider/database/model text cannot cross the
// browser boundary.
var browserTurnErrorCodes = map[string]struct{}{
	"active_memory_capability_unavailable":  {},
	"active_memory_plan_invalid":            {},
	"active_memory_plan_stale":              {},
	"affect_capability_unavailable":         {},
	"affect_idempotency_conflict":           {},
	"affect_profile_revision_conflict":      {},
	"affect_source_revision_conflict":       {},
	"affect_state_revision_conflict":        {},
	"appraisal_required":                    {},
	"capability_busy":                       {},
	"capability_execution_class_invalid":    {},
	"capability_execution_failed":           {},
	"capability_not_found":                  {},
	"capability_output_invalid":             {},
	"capability_prepare_failed":             {},
	"capability_prepared_payload_invalid":   {},
	"capability_request_arguments_invalid":  {},
	"capability_request_contract_invalid":   {},
	"capability_result_identity_mismatch":   {},
	"capability_result_missing":             {},
	"capability_result_status_invalid":      {},
	"capability_settlement_failed":          {},
	"adk_final_message_missing":             {},
	"adk_final_output_invalid":              {},
	"adk_final_output_missing":              {},
	"adk_final_text_missing":                {},
	"adk_run_failed":                        {},
	"agent_cognition_settlement_failed":     {},
	"agent_final_contract_invalid":          {},
	"agent_output_publication_failed":       {},
	"agent_run_failed":                      {},
	"cognition_visible_text_missing":        {},
	"conversation_not_found":                {},
	"conversation_settlement_failed":        {},
	"conversation_turn_conflict":            {},
	"conversation_turn_failed":              {},
	"conversation_turn_invalid":             {},
	"conversation_unauthorized":             {},
	"current_state_revision_stale":          {},
	"decision_effect_invalid":               {},
	"execution_cancelled":                   {},
	"execution_failed":                      {},
	"foundation_revision_stale":             {},
	"frozen_context_projection_missing":     {},
	"fluctlight_inactive":                   {},
	"fluctlight_paused":                     {},
	"life_context_stale":                    {},
	"media_arguments_invalid":               {},
	"media_capability_unavailable":          {},
	"media_context_stale":                   {},
	"media_intent_failed":                   {},
	"media_intent_invalid":                  {},
	"media_prepare_required":                {},
	"memory_capability_unavailable":         {},
	"memory_plan_invalid":                   {},
	"memory_plan_stale":                     {},
	"memory_recall_failed":                  {},
	"memory_recall_unavailable":             {},
	"moment_arguments_invalid":              {},
	"moment_target_invalid":                 {},
	"moment_text_invalid":                   {},
	"native_test_plan_invalid":              {},
	"native_test_retryable":                 {},
	"native_test_wrong_execution_path":      {},
	"personality_decision_plan_invalid":     {},
	"presence_capability_unavailable":       {},
	"presence_context_stale":                {},
	"presence_idempotency_conflict":         {},
	"presence_plan_expired":                 {},
	"presence_plan_invalid":                 {},
	"presence_prepared_context_mismatch":    {},
	"presence_replay_result_invalid":        {},
	"presence_resulting_context_invalid":    {},
	"provider_request_failed":               {},
	"relationship_lookup_arguments_invalid": {},
	"relationship_lookup_failed":            {},
	"relationship_lookup_not_found":         {},
	"relationship_lookup_owner_failed":      {},
	"relationship_lookup_scope_failed":      {},
	"relationship_lookup_target_forbidden":  {},
	"relationship_lookup_target_invalid":    {},
	"relationship_lookup_target_required":   {},
	"request_cancelled":                     {},
	"request_timeout":                       {},
	"required_capability_failed":            {},
	"scene_capability_unavailable":          {},
	"scene_context_stale":                   {},
	"scene_idempotency_conflict":            {},
	"scene_plan_expired":                    {},
	"scene_plan_invalid":                    {},
	"scene_prepared_context_mismatch":       {},
	"scene_replay_result_invalid":           {},
	"scene_resulting_context_invalid":       {},
	"schedule_prepared_context_mismatch":    {},
	"schedule_replan_arguments_invalid":     {},
	"schedule_replan_context_stale":         {},
	"schedule_replan_failed":                {},
	"schedule_replan_local_date_invalid":    {},
	"schedule_replan_owner_missing":         {},
	"schedule_replan_persist_failed":        {},
	"schedule_replan_planner_failed":        {},
	"schedule_replan_revision_invalid":      {},
	"schedule_replan_schedule_read_failed":  {},
	"schedule_replan_timezone_invalid":      {},
	"structured_turn_settlement_failed":     {},
	"takeover_failed":                       {},
	"takeover_frozen_turn_missing":          {},
	"takeover_reply_budget_exhausted":       {},
	"takeover_resume_decision_invalid":      {},
	"takeover_resume_rule_missing":          {},
	"takeover_target_profile_missing":       {},
	"tool_call_invalid":                     {},
	"tool_call_failed":                      {},
	"tool_execution_failed":                 {},
	"turn_stage_invalid":                    {},
	"turn_stage_not_executable":             {},
	"visible_text_source_conflict":          {},
}

// TranslateCoreNDJSON reads response incrementally and writes one browser
// NDJSON frame at a time.  It never buffers the complete upstream response.
// The caller normally passes request.Context() so cancellation closes a
// blocking upstream body and suppresses subsequent browser writes.
//
// Protocol errors are deliberately converted into one bounded error event and
// return nil.  A non-nil return is reserved for setup failures and downstream
// writer failures, which the HTTP handler may use to stop its response.
func TranslateCoreNDJSON(ctx context.Context, response *http.Response, dst io.Writer) error {
	if response == nil || response.Body == nil {
		return &NdjsonTranslationError{Code: CoreStreamBodyMissing}
	}
	if dst == nil {
		return &NdjsonTranslationError{Code: "browser_stream_writer_missing"}
	}
	if ctx == nil {
		ctx = context.Background()
	}

	if responseWriter, ok := dst.(http.ResponseWriter); ok {
		if responseWriter.Header().Get("Content-Type") == "" {
			responseWriter.Header().Set("Content-Type", "application/x-ndjson; charset=utf-8")
		}
	}

	body := response.Body
	var closeOnce sync.Once
	closeBody := func() {
		closeOnce.Do(func() { _ = body.Close() })
	}
	defer closeBody()

	// io.Reader has no context-aware Read method.  Closing the HTTP response
	// body from this watcher is the standard-library way to unblock an in-flight
	// read when the browser request is canceled.
	watcherDone := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			closeBody()
		case <-watcherDone:
		}
	}()
	defer close(watcherDone)

	if ctx.Err() != nil {
		closeBody()
		return nil
	}

	reader := bufio.NewReader(body)
	turnID := ""
	expectedSequence := 0

	for {
		line, readErr := reader.ReadBytes('\n')
		if ctx.Err() != nil {
			return nil
		}

		if len(line) > 0 && line[len(line)-1] == '\n' {
			if len(line) > maxCoreFrameBytes {
				return emitProtocolError(ctx, dst, turnID, expectedSequence, CoreStreamInvalid)
			}
			line = line[:len(line)-1]
			line = bytes.TrimSpace(line)
			if len(line) == 0 {
				if readErr != nil {
					if errors.Is(readErr, io.EOF) {
						return emitIncomplete(ctx, dst, turnID, expectedSequence)
					}
					return emitProtocolError(ctx, dst, turnID, expectedSequence, CoreStreamInvalid)
				}
				continue
			}

			core, code := decodeCoreEvent(line)
			// Remember a usable turn_id before validating the rest of the event so
			// bounded errors correlate correctly for malformed frames too.
			if turnID == "" && core.TurnID != "" {
				turnID = core.TurnID
			}
			if code != "" {
				return emitProtocolError(ctx, dst, turnID, expectedSequence, code)
			}
			if turnID == "" {
				turnID = core.TurnID
			}
			if core.TurnID != turnID || core.Sequence != expectedSequence {
				return emitProtocolError(ctx, dst, turnID, expectedSequence, CoreSequenceInvalid)
			}

			mapped, code := browserEvent(core)
			if code != "" {
				return emitProtocolError(ctx, dst, turnID, expectedSequence, code)
			}
			if ctx.Err() != nil {
				return nil
			}
			if err := writeBrowserFrame(ctx, dst, mapped); err != nil {
				if errors.Is(err, errTranslationCanceled) || ctx.Err() != nil {
					return nil
				}
				return err
			}
			expectedSequence++

			if mapped.Type == "completed" || mapped.Type == "error" {
				// Stop at the first terminal event.  This prevents a Core that
				// violates terminal uniqueness from producing browser writes.
				closeBody()
				return nil
			}
			if readErr != nil {
				if errors.Is(readErr, io.EOF) {
					return emitIncomplete(ctx, dst, turnID, expectedSequence)
				}
				return emitProtocolError(ctx, dst, turnID, expectedSequence, CoreStreamInvalid)
			}
			continue
		}

		// A final line without a newline is not a complete NDJSON frame.  Even
		// if its JSON happens to be valid, the Node implementation reports the
		// bounded incomplete-stream error rather than guessing at the frame.
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return emitIncomplete(ctx, dst, turnID, expectedSequence)
			}
			return emitProtocolError(ctx, dst, turnID, expectedSequence, CoreStreamInvalid)
		}
	}
}

// TranslateCoreNdjson is a spelling alias for callers that use the conventional
// mixed-case acronym form.
func TranslateCoreNdjson(ctx context.Context, response *http.Response, dst io.Writer) error {
	return TranslateCoreNDJSON(ctx, response, dst)
}

// translateCoreNDJSON and translateCoreNdjson make the helper convenient to
// use from same-package HTTP route tests without forcing an export solely for
// the transport seam.
func translateCoreNDJSON(ctx context.Context, response *http.Response, dst io.Writer) error {
	return TranslateCoreNDJSON(ctx, response, dst)
}

func translateCoreNdjson(ctx context.Context, response *http.Response, dst io.Writer) error {
	return TranslateCoreNDJSON(ctx, response, dst)
}

// TranslateCoreStream adapts a body directly for callers that already own the
// response status and headers.
func TranslateCoreStream(ctx context.Context, body io.ReadCloser, dst io.Writer) error {
	return TranslateCoreNDJSON(ctx, &http.Response{Body: body}, dst)
}

func emitIncomplete(ctx context.Context, dst io.Writer, turnID string, sequence int) error {
	return emitProtocolError(ctx, dst, turnID, sequence, CoreStreamIncomplete)
}

func emitProtocolError(ctx context.Context, dst io.Writer, turnID string, sequence int, code string) error {
	if ctx.Err() != nil {
		return nil
	}
	return writeBrowserFrame(ctx, dst, BrowserStreamEvent{
		Type:     "error",
		TurnID:   nonEmptyTurnID(turnID),
		Sequence: sequence,
		Payload: map[string]any{
			"code":    code,
			"message": "The conversation stream is unavailable",
		},
	})
}

func nonEmptyTurnID(turnID string) string {
	if turnID == "" {
		return "turn-unknown"
	}
	return turnID
}

func writeBrowserFrame(ctx context.Context, dst io.Writer, event BrowserStreamEvent) error {
	if ctx.Err() != nil {
		return errTranslationCanceled
	}
	var frame bytes.Buffer
	encoder := json.NewEncoder(&frame)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(event); err != nil {
		return fmt.Errorf("encode browser stream event: %w", err)
	}
	if ctx.Err() != nil {
		return errTranslationCanceled
	}
	data := frame.Bytes()
	written, err := dst.Write(data)
	if err == nil && written != len(data) {
		err = io.ErrShortWrite
	}
	if err != nil {
		return err
	}
	if flusher, ok := dst.(http.Flusher); ok {
		flusher.Flush()
	}
	return nil
}

// decodeCoreEvent classifies malformed JSON as core_stream_invalid and valid
// JSON whose envelope has the wrong shape as invalid_core_event.  It returns
// fields decoded before a shape failure so the caller can retain a useful
// turn_id on the bounded error frame.
func decodeCoreEvent(line []byte) (CoreStreamEvent, string) {
	var event CoreStreamEvent
	if !validUTF8(line) {
		return event, CoreStreamInvalid
	}
	// JSON.parse accepts arrays, strings, numbers, booleans, and null, but
	// none of them can satisfy the Core event envelope.  Keep those cases as
	// schema errors (rather than framing/JSON syntax errors) when the JSON is
	// otherwise valid.
	trimmed := bytes.TrimSpace(line)
	if len(trimmed) == 0 {
		return event, InvalidCoreEvent
	}
	if trimmed[0] != '{' {
		if json.Valid(trimmed) {
			return event, InvalidCoreEvent
		}
		return event, CoreStreamInvalid
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(line, &raw); err != nil {
		return event, CoreStreamInvalid
	}
	if raw == nil {
		return event, InvalidCoreEvent
	}

	shapeInvalid := false
	var present bool
	if value, ok := raw["type"]; ok {
		present = true
		if isJSONNull(value) || json.Unmarshal(value, &event.Type) != nil {
			shapeInvalid = true
		}
	} else {
		shapeInvalid = true
	}
	if !present || event.Type == "" {
		// Empty strings are represented in JSON correctly but are not valid
		// Core envelope values.
		shapeInvalid = true
	}

	if value, ok := raw["turn_id"]; ok {
		if isJSONNull(value) || json.Unmarshal(value, &event.TurnID) != nil {
			shapeInvalid = true
		}
	} else {
		shapeInvalid = true
	}
	if event.TurnID == "" {
		shapeInvalid = true
	}

	if value, ok := raw["sequence"]; ok {
		if isJSONNull(value) {
			shapeInvalid = true
		} else {
			var sequence float64
			if err := json.Unmarshal(value, &sequence); err != nil || math.IsNaN(sequence) || math.IsInf(sequence, 0) || math.Trunc(sequence) != sequence || sequence < 0 || sequence > float64(maxInt()) {
				shapeInvalid = true
			} else {
				event.Sequence = int(sequence)
			}
		}
	} else {
		shapeInvalid = true
	}

	if value, ok := raw["payload"]; ok {
		if isJSONNull(value) || json.Unmarshal(value, &event.Payload) != nil || event.Payload == nil {
			shapeInvalid = true
		}
	} else {
		shapeInvalid = true
	}

	if shapeInvalid {
		return event, InvalidCoreEvent
	}
	return event, ""
}

func maxInt() int {
	return int(^uint(0) >> 1)
}

func isJSONNull(value []byte) bool {
	return bytes.Equal(bytes.TrimSpace(value), []byte("null"))
}

func browserEvent(core CoreStreamEvent) (BrowserStreamEvent, string) {
	if _, ok := coreStreamTypes[core.Type]; !ok || core.TurnID == "" || core.Sequence < 0 || core.Payload == nil {
		return BrowserStreamEvent{}, InvalidCoreEvent
	}
	if core.Type == "error" {
		// Core currently has two error payload shapes: the newer `code` field
		// and the legacy `error` field emitted after visible output. Normalize
		// both at the public boundary and discard status/details/message so a
		// model, provider, or persistence failure cannot become browser data.
		return BrowserStreamEvent{
			Type:     "error",
			TurnID:   core.TurnID,
			Sequence: core.Sequence,
			Payload: map[string]any{
				"code":    browserTurnErrorCode(core.Payload),
				"message": browserTurnErrorMessage,
			},
		}, ""
	}
	if hasHiddenPayload(core.Payload) {
		return BrowserStreamEvent{}, HiddenCorePayload
	}

	eventType := core.Type
	if core.Type == "action_result" {
		eventType = "message"
		if message, ok := core.Payload["message"].(map[string]any); ok {
			if kind, ok := message["kind"].(string); ok && kind == "media_reference" {
				eventType = "media"
			}
			payload := make(map[string]any, len(core.Payload))
			for key, value := range core.Payload {
				payload[key] = value
			}
			payload["message"] = browserStreamMessage(message)
			return BrowserStreamEvent{Type: eventType, TurnID: core.TurnID, Sequence: core.Sequence, Payload: payload}, ""
		}
	}
	if core.Type == "completed" {
		// Only forward browser-visible completion metadata. Workflow/media
		// intent identifiers are Core internals and must not cross the browser boundary.
		payload := map[string]any{}
		if ids, ok := core.Payload["message_ids"]; ok {
			payload["message_ids"] = ids
		}
		return BrowserStreamEvent{Type: "completed", TurnID: core.TurnID, Sequence: core.Sequence, Payload: payload}, ""
	}

	return BrowserStreamEvent{Type: eventType, TurnID: core.TurnID, Sequence: core.Sequence, Payload: core.Payload}, ""
}

func browserTurnErrorCode(payload map[string]any) string {
	for _, key := range []string{"code", "error"} {
		value, ok := payload[key].(string)
		if !ok {
			continue
		}
		code := strings.TrimSpace(value)
		if _, allowed := browserTurnErrorCodes[code]; allowed {
			return code
		}
	}
	return "conversation_turn_failed"
}

func browserStreamMessage(message map[string]any) map[string]any {
	result := make(map[string]any, 8)
	for source, target := range map[string]string{
		"id":              "id",
		"conversation_id": "conversationId",
		"sequence":        "sequence",
		"author_actor_id": "authorActorId",
		"kind":            "kind",
		"text":            "text",
		"created_at":      "createdAt",
	} {
		if value, ok := message[source]; ok {
			result[target] = value
		}
	}
	if refs, ok := message["attachment_refs"].([]any); ok {
		result["attachmentRefs"] = refs
	} else {
		result["attachmentRefs"] = []any{}
	}
	return result
}

func hasHiddenPayload(value any) bool {
	switch current := value.(type) {
	case []any:
		for _, child := range current {
			if hasHiddenPayload(child) {
				return true
			}
		}
	case map[string]any:
		for key, child := range current {
			normalized := strings.ToLower(strings.NewReplacer("-", "", "_", "").Replace(key))
			if _, hidden := hiddenPayloadKeys[normalized]; hidden || hasHiddenPayload(child) {
				return true
			}
		}
	}
	return false
}

// Keep utf8 imported and make strict UTF-8 validation explicit at the frame
// boundary.  encoding/json replaces invalid UTF-8 by default, which would
// violate the Core/browser contract if it were allowed to do so silently.
func validUTF8(value []byte) bool { return utf8.Valid(value) }
