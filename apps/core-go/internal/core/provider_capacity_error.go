package core

import (
	"encoding/json"
	"errors"
	"regexp"
	"strings"
)

var providerCapacityHTTP = regexp.MustCompile(`(?i)\bprovider HTTP (400|500|503|507):\s*`)
var providerGPUMemoryShortage = regexp.MustCompile(`(?i)\brequires ~?[0-9]+(?:\.[0-9]+)?\s*(?:MB|MiB|GB|GiB) GPU memory but only ~?[0-9]+(?:\.[0-9]+)?\s*(?:MB|MiB|GB|GiB) is available\b`)
var providerGPUOutOfMemory = regexp.MustCompile(`(?i)\b(?:CUDA|GPU)(?: error)?[: ]+out of memory\b|\btorch\.cuda\.OutOfMemoryError\b|\bCUDA_ERROR_OUT_OF_MEMORY\b`)

func providerCapacityDetail(detail string) bool {
	text := strings.ToLower(detail)
	// Billing, authentication, invalid schemas and context limits must retain
	// their own policy even if an error happens to mention GPU memory too.
	for _, excluded := range []string{"insufficient_quota", "insufficient_user_quota", "invalid_api_key", "invalid schema", "invalid_schema", "context_length_exceeded"} {
		if strings.Contains(text, excluded) {
			return false
		}
	}
	if providerGPUMemoryShortage.MatchString(detail) || providerGPUOutOfMemory.MatchString(detail) {
		return true
	}
	// Some local providers return only this exact message. Do not match generic
	// "memory", allocation failures, or an arbitrary occurrence of "oom".
	var body struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal([]byte(detail), &body) == nil && body.Error.Message != "" {
		text = strings.ToLower(strings.TrimSpace(body.Error.Message))
	}
	return strings.TrimSpace(text) == "out of memory"
}

func providerCapacityError(err error) bool {
	var response *providerHTTPResponseError
	if !errors.As(err, &response) {
		return false
	}
	switch response.StatusCode {
	case 400, 500, 503, 507:
		return providerCapacityDetail(response.Detail)
	}
	return false
}

// ProviderCapacityFailure recognizes the preserved HTTP failure after Temporal
// has serialized its Go type away. Require a provider HTTP boundary and a narrow
// capacity signature; ordinary business errors are never capacity evidence.
func ProviderCapacityFailure(message string) bool {
	location := providerCapacityHTTP.FindStringIndex(message)
	return location != nil && providerCapacityDetail(message[location[1]:])
}
