package core

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestDiaryModelSnapshotKeepsBindingAndHidesCredentialReferences(t *testing.T) {
	temperature, topP := 0.0, 0.72
	assignment := providerAssignment{Role: conversationReplyModelRole, SourceBindingRole: "agent:diary_expression", EndpointID: "private-endpoint", SecretPurpose: "private-purpose", BaseURL: "http://model.invalid", ModelID: "model-a", Secret: "secret-never-store", Timeout: 21 * time.Second, Temperature: &temperature, TopP: &topP}
	snapshot := newDiaryModelSnapshot(FormalAgentDiaryExpression, "diary_expression_text", conversationReplyModelRole, assignment, 7, 2)
	provenance := map[string]any{diaryModelSnapshotContractKey: diaryModelSnapshotVersion, diaryModelSnapshotsKey: map[string]any{"expression": snapshot}, "expression_model": "model-a"}
	encoded := jsonBytes(provenance)
	if strings.Contains(string(encoded), assignment.Secret) {
		t.Fatal("credential was persisted")
	}
	got, found, err := readDiaryModelSnapshot(decodeObject(encoded), "expression", FormalAgentDiaryExpression, "diary_expression_text", conversationReplyModelRole)
	if err != nil || !found || got.Assignment.ModelID != "model-a" || got.Assignment.Timeout != assignment.Timeout || got.Assignment.Temperature == nil || *got.Assignment.Temperature != 0 || got.Assignment.TopP == nil || *got.Assignment.TopP != topP || got.ConfigVersion != 7 || got.Concurrency != 2 {
		t.Fatalf("binding changed on round trip: %#v found=%v err=%v", got, found, err)
	}
	public := jsonString(publicDiaryModelProvenance(encoded))
	if strings.Contains(public, "private-purpose") || strings.Contains(public, "model.invalid") || strings.Contains(public, diaryModelSnapshotsKey) || !strings.Contains(public, "model-a") {
		t.Fatalf("public diary leaked private binding or lost model provenance: %s", public)
	}
	if _, found, err := readDiaryModelSnapshot(decodeObject(encoded), "expression", FormalAgentDiaryMaterial, "diary_material_v1", "cognitive_assessment"); !found || err == nil {
		t.Fatal("snapshot was reused for another Agent/variant")
	}
	tampered := snapshot
	tampered.Assignment.Secret = "must-not-replay"
	if _, found, err := readDiaryModelSnapshot(map[string]any{diaryModelSnapshotsKey: map[string]any{"expression": tampered}}, "expression", FormalAgentDiaryExpression, "diary_expression_text", conversationReplyModelRole); !found || err == nil {
		t.Fatal("persisted credential was accepted")
	}
	tampered = snapshot
	tampered.Assignment.Role = "embedding"
	if _, found, err := readDiaryModelSnapshot(map[string]any{diaryModelSnapshotsKey: map[string]any{"expression": tampered}}, "expression", FormalAgentDiaryExpression, "diary_expression_text", conversationReplyModelRole); !found || err == nil {
		t.Fatal("snapshot with incompatible assignment role was accepted")
	}
	if _, found, err := readDiaryModelSnapshot(map[string]any{diaryModelSnapshotsKey: []any{"corrupt"}}, "expression", FormalAgentDiaryExpression, "diary_expression_text", conversationReplyModelRole); !found || err == nil {
		t.Fatal("corrupt snapshot container was treated as an unsnapshotted stage")
	}
}

func TestDiarySnapshotReachesActualFormalAgentRequests(t *testing.T) {
	for _, sample := range []struct {
		name                       string
		agent                      FormalAgentID
		variant, role, bindingRole string
		structured                 bool
	}{
		{"material", FormalAgentDiaryMaterial, "diary_material_v1", "cognitive_assessment", "generic_llm", true},
		{"expression", FormalAgentDiaryExpression, "diary_expression_text", conversationReplyModelRole, conversationReplyModelRole, false},
	} {
		t.Run(sample.name, func(t *testing.T) {
			temperature, topP := 0.0, 0.74
			original := providerAssignment{Role: sample.bindingRole, EndpointID: "fixture-endpoint", SecretPurpose: "fixture-purpose", BaseURL: "http://fixture.invalid", ModelID: "frozen-" + sample.name, Secret: "private-secret", Timeout: time.Second, PromptBudgetPolicyVersion: promptBudgetPolicyVersionV1, Temperature: &temperature, TopP: &topP}
			snapshot := newDiaryModelSnapshot(sample.agent, sample.variant, sample.role, original, 4, 1)
			decoded, found, err := readDiaryModelSnapshot(map[string]any{diaryModelSnapshotsKey: map[string]any{sample.name: snapshot}}, sample.name, sample.agent, sample.variant, sample.role)
			if err != nil || !found {
				t.Fatalf("snapshot: %v", err)
			}
			decoded.Assignment.Secret = "private-secret" // simulated server-side credential restore
			var wire map[string]any
			router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
				wire = payload
				if sample.structured {
					return fakeProviderResult{Structured: map[string]any{"ok": true}}
				}
				return fakeProviderResult{Text: "受控日记正文"}
			})
			app := &App{Provider: &ProviderClient{HTTP: &http.Client{Transport: router}}}
			ctx := withAgentModelRoute(context.Background(), sample.agent, sample.variant)
			ctx = withFrozenProviderAssignment(ctx, sample.role, decoded.Assignment)
			input := FormalAgentRunInput{Prompt: PromptAssemblyResult{Messages: []map[string]any{{"role": "user", "content": "合成测试材料"}}}, SchemaName: sample.variant}
			if sample.structured {
				input.Prompt.ResponseFormat = objectSchema(map[string]any{"ok": booleanSchema()}, []string{"ok"}, false)
			}
			if _, err := app.RunFormalAgent(ctx, sample.agent, input); err != nil {
				t.Fatal(err)
			}
			if wire["model"] != original.ModelID || wire["temperature"] != 0.0 || wire["top_p"] != topP || wire["max_tokens"] != nil || wire["max_completion_tokens"] != nil || strings.Contains(jsonString(wire), "private-secret") || strings.Contains(jsonString(wire), "fixture-purpose") {
				t.Fatalf("physical request lost binding or exposed private data: %#v", wire)
			}
		})
	}
}

func TestDiaryExplicitRetryRetainsOnlyCompletedStageBindings(t *testing.T) {
	provenance := jsonBytes(map[string]any{diaryModelSnapshotContractKey: diaryModelSnapshotVersion, diaryModelSnapshotsKey: map[string]any{"material": map[string]any{"model": "a"}, "expression": map[string]any{"model": "b"}}, "other": "preserved"})
	body := "完成的正文"
	for _, sample := range []struct {
		name                 string
		plan                 []byte
		body                 *string
		material, expression bool
	}{
		{"neither completed", nil, nil, false, false},
		{"material completed", jsonBytes(map[string]any{"write": true}), nil, true, false},
		{"both completed", jsonBytes(map[string]any{"write": true}), &body, true, true},
	} {
		t.Run(sample.name, func(t *testing.T) {
			got := diaryRetryModelProvenance(provenance, sample.plan, sample.body)
			snapshots := mapValue(got[diaryModelSnapshotsKey])
			_, material := snapshots["material"]
			_, expression := snapshots["expression"]
			if material != sample.material || expression != sample.expression || got["other"] != "preserved" {
				t.Fatalf("retry provenance=%#v", got)
			}
		})
	}
}
