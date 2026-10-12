package core

import (
	"errors"
	"io"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// Observation runs inside the existing bounded queue stream bridge. It never
// reads ahead, opens a second consumer, or changes Provider chunks/errors.
// Retained output is bounded independently of the native Runner's consumer.
type einoStreamObservation struct {
	chunks    []*schema.Message
	meta      *schema.Message
	bytes     int
	truncated bool
	done      bool
	finish    func(*schema.Message, error)
}

func newEinoStreamObservation(finish func(*schema.Message, error)) *einoStreamObservation {
	return &einoStreamObservation{finish: finish}
}

func (o *einoStreamObservation) observe(chunk *schema.Message, err error) {
	if o.done {
		return
	}
	if chunk != nil {
		size := len(chunk.Role) + len(chunk.Content) + len(chunk.ReasoningContent)
		for _, call := range chunk.ToolCalls {
			size += 128 + len(call.ID) + len(call.Type) + len(call.Function.Name) + len(call.Function.Arguments)
		}
		// Retain only text/calls and the metadata needed for diagnostics. Count
		// per-chunk overhead too, so empty deltas cannot grow the buffer forever.
		// Reserve space for the fixed terminal metadata and account for each
		// ToolCall's structural overhead, including calls with empty strings.
		if o.truncated || o.bytes+size+256 > 2*1024*1024-1024 {
			o.truncated = true
		} else {
			o.bytes += size + 256
			calls := make([]schema.ToolCall, 0, len(chunk.ToolCalls))
			for _, call := range chunk.ToolCalls {
				// Extra is arbitrary Provider data, outside this bounded projection.
				calls = append(calls, schema.ToolCall{Index: call.Index, ID: call.ID, Type: call.Type, Function: call.Function})
			}
			o.chunks = append(o.chunks, &schema.Message{Role: chunk.Role, Content: chunk.Content, ReasoningContent: chunk.ReasoningContent, ToolCalls: calls})
		}
		if chunk.ResponseMeta != nil {
			finishReason := chunk.ResponseMeta.FinishReason
			if len(finishReason) > 256 {
				finishReason = strings.Clone(finishReason[:256])
				o.truncated = true
			}
			meta := &schema.Message{ResponseMeta: &schema.ResponseMeta{FinishReason: finishReason, Usage: chunk.ResponseMeta.Usage}}
			if merged, mergeErr := concatEinoObservedMessages(o.meta, meta); mergeErr == nil {
				o.meta = merged
			}
		}
	}
	if err != nil {
		o.done = true
		full, mergeErr := concatEinoObservedMessages(append(o.chunks, o.meta)...)
		if mergeErr != nil && err == io.EOF {
			err = mergeErr
		}
		if o.truncated && full != nil {
			full.Extra = map[string]any{"diagnostic_output_truncated": true}
		}
		o.chunks = nil

		if errors.Is(err, io.EOF) {
			err = nil
		}
		o.finish(full, err)
	}
}

// Eino v0.7.37 ConcatMessages retains totals/cache but drops reasoning usage.
// Preserve that available Provider detail without summing cumulative counters.
func concatEinoObservedMessages(chunks ...*schema.Message) (*schema.Message, error) {
	messages := make([]*schema.Message, 0, len(chunks))
	for _, chunk := range chunks {
		if chunk != nil {
			messages = append(messages, chunk)
		}
	}
	if len(messages) == 0 {
		return nil, nil
	}
	full, err := schema.ConcatMessages(messages)
	if err != nil {
		return nil, err
	}
	for _, message := range messages {
		if message.ResponseMeta != nil && message.ResponseMeta.Usage != nil && full.ResponseMeta != nil && full.ResponseMeta.Usage != nil {
			full.ResponseMeta.Usage.CompletionTokensDetails.ReasoningTokens = maxInt(full.ResponseMeta.Usage.CompletionTokensDetails.ReasoningTokens, message.ResponseMeta.Usage.CompletionTokensDetails.ReasoningTokens)
		}
	}
	return full, nil
}

func einoResponseMetadata(message *schema.Message) map[string]any {
	result := map[string]any{}
	if message == nil {
		return result
	}
	result["content_chars"] = len([]rune(message.Content))
	result["reasoning_chars"] = len([]rune(message.ReasoningContent))
	result["tool_call_count"] = len(message.ToolCalls)
	if message.ResponseMeta != nil {
		result["finish_reason"] = message.ResponseMeta.FinishReason
		if message.ResponseMeta.Usage != nil {
			result["usage"] = einoUsage(message)
		}
	}
	return result
}

// The pinned native graph may concatenate stream chunks before exposing an
// Agent event, dropping reasoning usage. Restore only available physical
// metadata from this run; never derive usage from text or budget settings.
func preservePhysicalResponseMetadata(final *schema.Message, chat model.ToolCallingChatModel) {
	queued, ok := chat.(*queuedToolCallingChatModel)
	if !ok || final == nil || queued.lastPhysicalResponse == nil {
		return
	}
	physical := queued.lastPhysicalResponse.Load()
	if physical == nil || physical.ResponseMeta == nil {
		return
	}
	meta := *physical.ResponseMeta
	if meta.Usage != nil {
		usage := *meta.Usage
		meta.Usage = &usage
	}
	final.ResponseMeta = &meta
}
