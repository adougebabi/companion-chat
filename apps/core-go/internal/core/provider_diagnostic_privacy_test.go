package core

import (
	"context"
	"strings"
	"testing"
)

type capturedSuccessDiagnostics struct {
	ProviderRuntimeSupport
	Prompt, Response any
}

func (s *capturedSuccessDiagnostics) RecordModelRun(_ context.Context, _, _, _, _ string, prompt, response any, _, _ string) {
	s.Prompt = prompt
	s.Response = response
}

func TestProviderSuccessDiagnosticsRedactKeysAndCredentialEcho(t *testing.T) {
	secret := "controlled-provider-secret"
	capture := &capturedSuccessDiagnostics{}
	provider := &ProviderClient{runtime: capture}
	response := map[string]any{"text": "模型正文回显 " + secret, "nested": map[string]any{"api_key": "another-secret", "Authorization": "Bearer header-secret"}, "content": `{"password":"password-value","normal":"🙂 e6"}`}
	provider.recordProviderSuccess(context.Background(), providerAssignment{Secret: secret}, conversationReplyModelRole, "reply", []map[string]any{{"role": "user", "content": "plain"}}, response)
	encoded := jsonString(capture.Response)
	for _, value := range []string{secret, "another-secret", "header-secret", "password-value"} {
		if strings.Contains(encoded, value) {
			t.Fatalf("success diagnostic contains %s", value)
		}
	}
	if !strings.Contains(encoded, "🙂 e6") {
		t.Fatal("ordinary model text lost")
	}
}

func TestProviderPhysicalDiagnosticsRedactNestedKnownCredentialsAndImages(t *testing.T) {
	ctx := withProviderDiagnosticSecrets(context.Background(), "first-key", "second-key")
	response := map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": "echo first-key and second-key"}}}, "assignment": map[string]any{"Secret": "hidden"}, "image_url": map[string]any{"url": "data:image/png;base64,secret-pixels"}}
	safe := jsonString(redactProviderDiagnostic(ctx, response))
	for _, value := range []string{"first-key", "second-key", "hidden", "secret-pixels"} {
		if strings.Contains(safe, value) {
			t.Fatalf("physical diagnostic contains %s", value)
		}
	}
	if !strings.Contains(safe, "REDACTED_SECRET") || !strings.Contains(safe, "REDACTED_IMAGE_DATA") {
		t.Fatal("redaction markers missing")
	}
	if strings.Contains(jsonString(response), "REDACTED_SECRET") {
		t.Fatal("diagnostic scrub mutated Provider result")
	}
}

func TestReplyEvidenceUsesExactlyOfferedAuthorizedRevision(t *testing.T) {
	entry := ContextReference{Ref: "fact:ctx_123", Kind: ContextReferenceActorFact, EntityID: "actual", Revision: 2, Snapshot: jsonBytes(map[string]any{"value": "explicit"})}
	index := ContextReferenceIndex{FluctlightID: "self", ConversationID: "conversation", ByRef: map[string]ContextReference{entry.Ref: entry}}
	inv := CapabilityInvocation{Metadata: InvocationMetadata{Source: "model_tool", FluctlightID: "self", ConversationID: "conversation"}, ContextSnapshot: map[string]any{"context_reference_index": index}}
	if _, ok := replyEvidenceEntry(inv, ContextProjection{ReferenceIndex: index}, entry.Ref); !ok {
		t.Fatal("offered revision rejected")
	}
	changed := entry
	changed.EntityID = "foreign"
	live := index
	live.ByRef = map[string]ContextReference{entry.Ref: changed}
	if _, ok := replyEvidenceEntry(inv, ContextProjection{ReferenceIndex: live}, entry.Ref); ok {
		t.Fatal("new fact substituted for offered ref")
	}
	inv.ContextSnapshot = nil
	if _, ok := replyEvidenceEntry(inv, ContextProjection{ReferenceIndex: index}, entry.Ref); ok {
		t.Fatal("model invented unoffered ref")
	}
}
