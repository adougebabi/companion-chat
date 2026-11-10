package core

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestStudioSnapshotsReachTaskOwnedRequestsWithoutSecrets(t *testing.T) {
	for _, sample := range []struct {
		name                       string
		agent                      FormalAgentID
		variant, role, bindingRole string
		structured                 bool
	}{
		{"assistant", FormalAgentStudioAssistant, "studio_assistant_v1", "cognitive_assessment", "generic_llm", true},
		{"text_preview", FormalAgentStudioTextPreview, "studio_preview_text", conversationReplyModelRole, conversationReplyModelRole, false},
		{"initialization", FormalAgentInitialization, "initialization_response", "initialization", "generic_llm", true},
		{"media_subject_parse", FormalAgentMediaPrompt, "media_subject_parse", "media_prompt", "generic_llm", true},
	} {
		t.Run(sample.name, func(t *testing.T) {
			temperature, topP := 0.0, 0.75
			original := providerAssignment{Role: sample.bindingRole, EndpointID: "private-endpoint", SecretPurpose: "private-purpose", BaseURL: "http://fixture.invalid", ModelID: "frozen-" + sample.name, Secret: "private-secret", Timeout: time.Second, PromptBudgetPolicyVersion: promptBudgetPolicyVersionV1, Temperature: &temperature, TopP: &topP}
			snapshot := newWorkflowModelSnapshot(sample.agent, sample.variant, sample.role, original, 3, 1)
			provenance := map[string]any{studioModelSnapshotContractKey: workflowModelSnapshotVersion, studioModelSnapshotsKey: map[string]any{sample.name: snapshot}}
			if strings.Contains(jsonString(provenance), original.Secret) {
				t.Fatal("credential persisted")
			}
			decoded, found, err := readWorkflowModelSnapshot(decodeObject(jsonBytes(provenance)), studioModelSnapshotsKey, sample.name, sample.agent, sample.variant, sample.role, errStudioModelSnapshotInvalid)
			if err != nil || !found {
				t.Fatalf("snapshot %v", err)
			}
			decoded.Assignment.Secret = original.Secret
			var wire map[string]any
			router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
				wire = payload
				if sample.structured {
					return fakeProviderResult{Structured: map[string]any{"ok": true}}
				}
				return fakeProviderResult{Text: "受控创作预览"}
			})
			app := &App{Provider: &ProviderClient{HTTP: &http.Client{Transport: router}}}
			ctx := withAgentModelRoute(context.Background(), sample.agent, sample.variant)
			ctx = withFrozenProviderAssignment(ctx, sample.role, decoded.Assignment)
			input := FormalAgentRunInput{Prompt: PromptAssemblyResult{Messages: []map[string]any{{"role": "user", "content": "虚构测试资料"}}}, SchemaName: sample.variant}
			if sample.structured {
				input.Prompt.ResponseFormat = objectSchema(map[string]any{"ok": booleanSchema()}, []string{"ok"}, false)
				input.OutputKind = FormalAgentOutputStructured
			}
			if _, err := app.RunFormalAgent(ctx, sample.agent, input); err != nil {
				t.Fatal(err)
			}
			if wire["model"] != original.ModelID || wire["temperature"] != 0.0 || wire["top_p"] != topP || wire["max_tokens"] != nil || wire["max_completion_tokens"] != nil || strings.Contains(jsonString(wire), "private-secret") || strings.Contains(jsonString(wire), "private-purpose") {
				t.Fatalf("snapshot request=%#v", wire)
			}
		})
	}
}
