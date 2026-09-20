package conversation

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

type CanonicalVisibleReply struct {
	Text     string
	Source   string
	Conflict bool
	Digest   string
}

const (
	CanonicalVisibleSourceResponsePlan    = "response_plan"
	CanonicalVisibleSourceDecision        = "decision"
	CanonicalVisibleSourceReplyCapability = "reply_capability"

	CanonicalVisibleTextConflictCode = "visible_text_source_conflict"
)

type VisibleTextDiagnostic struct {
	Code   string
	Winner string
	Detail string
	Digest string
}

func (value VisibleTextDiagnostic) AsMap() map[string]any {
	result := map[string]any{"code": value.Code}
	if value.Winner != "" {
		result["winner"] = value.Winner
	}
	if value.Detail != "" {
		result["detail"] = value.Detail
	}
	if value.Digest != "" {
		result["digest"] = value.Digest
	}
	return result
}

func NormalizeVisibleReply(text string) string {
	return strings.TrimSpace(text)
}

func StableDigest(text string) string {
	h := sha256.Sum256([]byte(text))
	return hex.EncodeToString(h[:])
}

func DistinctVisibleTexts(texts ...string) []string {
	seen := make(map[string]struct{})
	var distinct []string
	for _, text := range texts {
		t := strings.TrimSpace(text)
		if t == "" {
			continue
		}
		if _, exists := seen[t]; !exists {
			seen[t] = struct{}{}
			distinct = append(distinct, t)
		}
	}
	return distinct
}
