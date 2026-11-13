package core

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/cloudwego/eino/schema"
	capabilitycontract "github.com/fluctlight/local-ai-companion/apps/core-go/internal/capability"
)

type conversationCandidatePreview struct {
	ID   string `json:"candidate_id"`
	Text string `json:"text"`
}
type conversationCandidateSinkKey struct{}
type conversationCandidateSink func(conversationCandidatePreview)

func candidatePreviewKey(inboxID, owner string) string {
	return "fluctlight:conversation:preview:" + inboxID + ":" + owner
}

// The optional cache carries unverified display snapshots across API/Worker
// processes. Claim-specific keys prevent a late Worker from replacing a new
// attempt. Neither these snapshots nor their expiry affect formal publication.
func (a *App) withConversationCandidatePreview(ctx context.Context, inboxID string) (context.Context, bool) {
	claim, _ := ctx.Value(directTurnClaimKey{}).(directTurnClaim)
	if a.Redis == nil || claim.InboxID != inboxID || claim.Owner == "" {
		return ctx, false
	}
	sink := conversationCandidateSink(func(preview conversationCandidatePreview) {
		raw, err := json.Marshal(preview)
		if err != nil {
			return
		}
		cacheCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
		defer cancel()
		_ = a.Redis.Set(cacheCtx, candidatePreviewKey(inboxID, claim.Owner), raw, 3*time.Minute).Err()
	})
	return context.WithValue(ctx, conversationCandidateSinkKey{}, sink), true
}

func (a *App) readConversationCandidatePreview(ctx context.Context, inboxID string) conversationCandidatePreview {
	if a.Redis == nil {
		return conversationCandidatePreview{}
	}
	var owner string
	// A committed message always supersedes the provisional display, even if
	// the Worker has not yet settled its inbox or this observer missed a poll.
	err := a.DB.Pool().QueryRow(ctx, `SELECT COALESCE(claimed_by,'') FROM public.cognition_inbox WHERE id=$1 AND status='claimed' AND NOT EXISTS(SELECT 1 FROM public.conversation_messages WHERE source_fact_id=$1 AND kind='assistant')`, inboxID).Scan(&owner)
	if err != nil || owner == "" {
		return conversationCandidatePreview{}
	}
	cacheCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	raw, err := a.Redis.Get(cacheCtx, candidatePreviewKey(inboxID, owner)).Bytes()
	if err != nil {
		return conversationCandidatePreview{}
	}
	var preview conversationCandidatePreview
	if json.Unmarshal(raw, &preview) != nil {
		return conversationCandidatePreview{}
	}
	return preview
}

// Only native conversation.reply.text is a display candidate. Content,
// reasoning, final cognition and other Tool arguments never enter the cache.
func newConversationCandidateObserver(ctx context.Context) func(*schema.Message, error) {
	sink, _ := ctx.Value(conversationCandidateSinkKey{}).(conversationCandidateSink)
	definition, formal := formalAgentDefinitionFromContext(ctx)
	if sink == nil || !formal || definition.ID != FormalAgentConversationCognition {
		return func(*schema.Message, error) {}
	}
	preview := conversationCandidatePreview{ID: randomID("candidate_")}
	sink(preview)
	var full *schema.Message
	var lastWrite time.Time
	var lastText string
	done := false
	return func(chunk *schema.Message, streamErr error) {
		if done {
			return
		}
		if chunk != nil && len(chunk.ToolCalls) > 0 {
			calls := append([]schema.ToolCall(nil), chunk.ToolCalls...)
			var err error
			full, err = concatEinoObservedMessages(full, &schema.Message{Role: schema.Assistant, ToolCalls: calls})
			if err != nil {
				full = nil
			}
		}
		text := ""
		replies := 0
		if full != nil {
			for _, call := range full.ToolCalls {
				if capabilitycontract.CapabilityNamesMatch(call.Function.Name, conversationReplyCapabilityName) {
					replies++
					text, _ = candidateReplyText(call.Function.Arguments)
				}
			}
		}
		if replies != 1 {
			text = ""
		}
		text = candidatePreviewSafeText(ctx, text)
		if streamErr != nil {
			done = true
			if !errors.Is(streamErr, io.EOF) {
				text = ""
			}
		}
		if text == lastText || streamErr == nil && time.Since(lastWrite) < 100*time.Millisecond {
			return
		}
		preview.Text = text
		sink(preview)
		lastText = text
		lastWrite = time.Now()
	}
}

func candidatePreviewSafeText(ctx context.Context, text string) string {
	secrets, _ := ctx.Value(providerDiagnosticSecretsKey{}).([]string)
	// Hold a partial trailing secret until it can be scrubbed as a whole.
	// Otherwise a credential split across chunks could flash in the preview.
	for _, secret := range secrets {
		if secret == "" {
			continue
		}
		text = strings.ReplaceAll(text, secret, "[REDACTED_SECRET]")
		for length := min(len(secret)-1, len(text)); length > 0; length-- {
			if strings.HasSuffix(text, secret[:length]) {
				text = text[:len(text)-length]
				break
			}
		}
	}
	return text
}

func candidateReplyText(arguments string) (string, bool) {
	decoder := json.NewDecoder(strings.NewReader(arguments))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return "", false
	}
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return "", false
		}
		if key == "text" {
			rest := strings.TrimLeft(arguments[decoder.InputOffset():], " \r\n\t")
			if !strings.HasPrefix(rest, ":") {
				return "", false
			}
			return candidateJSONStringPrefix(strings.TrimLeft(rest[1:], " \r\n\t"))
		}
		var ignored json.RawMessage
		if decoder.Decode(&ignored) != nil {
			return "", false
		}
	}
	return "", false
}

// Decode only complete JSON characters. Incomplete UTF-8, escapes and UTF-16
// surrogate pairs wait for the next chunk instead of exposing replacement text.
func candidateJSONStringPrefix(source string) (string, bool) {
	if len(source) == 0 || source[0] != '"' {
		return "", false
	}
	var text strings.Builder
	for index := 1; index < len(source); {
		if source[index] == '"' {
			return text.String(), true
		}
		if source[index] == '\\' {
			end := index + 2
			if end > len(source) {
				return text.String(), true
			}
			if source[index+1] == 'u' {
				end = index + 6
				if end > len(source) {
					return text.String(), true
				}
				value, err := strconv.ParseUint(source[index+2:end], 16, 16)
				if err != nil {
					return "", false
				}
				if utf16.IsSurrogate(rune(value)) {
					if value < 0xD800 || value > 0xDBFF {
						return "", false
					}
					end += 6
					if end > len(source) {
						return text.String(), true
					}
					if source[index+6:index+8] != "\\u" {
						return "", false
					}
					low, err := strconv.ParseUint(source[index+8:end], 16, 16)
					if err != nil || low < 0xDC00 || low > 0xDFFF {
						return "", false
					}
				}
			}
			var decoded string
			if json.Unmarshal([]byte("\""+source[index:end]+"\""), &decoded) != nil {
				return "", false
			}
			text.WriteString(decoded)
			index = end
			continue
		}
		char, size := utf8.DecodeRuneInString(source[index:])
		if char == utf8.RuneError && size == 1 {
			return text.String(), true
		}
		if char < 0x20 {
			return "", false
		}
		text.WriteRune(char)
		index += size
	}
	return text.String(), true
}
