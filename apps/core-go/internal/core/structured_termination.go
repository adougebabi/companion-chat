package core

import (
	"errors"
	"strings"

	"github.com/cloudwego/eino/schema"
)

// Preserve a structured termination cause through final-contract wrappers.
type structuredResponseTruncatedError struct {
	role             string
	completionTokens int
	contentComplete  bool
}

func (e *structuredResponseTruncatedError) Error() string {
	if e.role == "initialization" {
		return "initialization_response_truncated"
	}
	return "structured_response_truncated"
}
func structuredTerminationError(message *schema.Message, role string) error {
	if message == nil || message.ResponseMeta == nil || strings.TrimSpace(message.ResponseMeta.FinishReason) != "length" {
		return nil
	}
	tokens := 0
	if message.ResponseMeta.Usage != nil {
		tokens = message.ResponseMeta.Usage.CompletionTokens
	}
	_, complete := parseStructuredCandidates(providerStructuredCandidates(einoMessageRaw(message)))
	return &structuredResponseTruncatedError{role: role, completionTokens: tokens, contentComplete: complete}
}
func initializationStructuredCause(err error) string {
	var truncated *structuredResponseTruncatedError
	if errors.As(err, &truncated) {
		return "initialization_response_truncated"
	}
	var visit func(error, int) string
	visit = func(cause error, depth int) string {
		if cause == nil || depth > 12 {
			return ""
		}
		switch cause.Error() {
		case "initialization_response_invalid_json", "initialization_response_semantic_empty":
			return cause.Error()
		}
		if many, ok := cause.(interface{ Unwrap() []error }); ok {
			for _, child := range many.Unwrap() {
				if code := visit(child, depth+1); code != "" {
					return code
				}
			}
		}
		return visit(errors.Unwrap(cause), depth+1)
	}
	return visit(err, 0)
}
