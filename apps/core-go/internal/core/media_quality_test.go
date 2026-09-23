package core

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestMediaQualityAcceptanceSchemaHasStrictDecisionShape(t *testing.T) {
	schema := mediaQualityAcceptanceResponseSchema()
	if schema["additionalProperties"] != false {
		t.Fatalf("schema allows unknown top-level fields: %#v", schema)
	}
	required, ok := schema["required"].([]any)
	if !ok {
		t.Fatalf("schema required fields = %#v", schema["required"])
	}
	for _, field := range []string{"schema_version", "verdict", "violations", "observed_facts", "retry_guidance"} {
		found := false
		for _, value := range required {
			if value == field {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("schema missing required field %q: %#v", field, required)
		}
	}
}

func TestNormalizeMediaQualityAcceptanceRequiresBoundedVerdictShape(t *testing.T) {
	pass, err := normalizeMediaQualityAcceptance(map[string]any{
		"schema_version": 1,
		"verdict":        "pass",
		"violations":     []any{},
		"observed_facts": map[string]any{
			"subject_matches": true, "appearance_matches": true, "scene_matches": true,
			"capture_matches": true, "framing_matches": true,
		},
		"retry_guidance": "",
	})
	if err != nil || pass.Verdict != mediaQualityVerdictPass || len(pass.Violations) != 0 {
		t.Fatalf("pass = %#v, err=%v", pass, err)
	}

	_, err = normalizeMediaQualityAcceptance(map[string]any{
		"schema_version": 1,
		"verdict":        "retry",
		"violations":     []any{map[string]any{"code": "capture_mismatch", "severity": "hard", "detail": "phone visible"}},
		"observed_facts": map[string]any{
			"subject_matches": true, "appearance_matches": true, "scene_matches": true,
			"capture_matches": false, "framing_matches": true,
		},
		"retry_guidance": "Keep the phone out of frame.",
	})
	if err != nil {
		t.Fatalf("retry should be valid: %v", err)
	}

	_, err = normalizeMediaQualityAcceptance(map[string]any{
		"schema_version": 1,
		"verdict":        "retry",
		"violations":     []any{},
		"observed_facts": map[string]any{
			"subject_matches": true, "appearance_matches": true, "scene_matches": true,
			"capture_matches": false, "framing_matches": true,
		},
		"retry_guidance": "",
	})
	if err == nil {
		t.Fatal("retry without a violation/guidance should be rejected")
	}
	if _, err := normalizeMediaQualityAcceptance(map[string]any{
		"schema_version": 1,
		"verdict":        "pass",
		"violations":     []any{map[string]any{"code": "style", "severity": "soft", "detail": "too plain"}},
		"observed_facts": map[string]any{
			"subject_matches": true, "appearance_matches": true, "scene_matches": true,
			"capture_matches": true, "framing_matches": true,
		},
		"retry_guidance": "",
	}); err == nil {
		t.Fatal("soft/aesthetic violations should not be accepted")
	}
}

func TestMediaQualityImageDataURLBoundsAndMime(t *testing.T) {
	content := []byte("image-bytes")
	dataURL, err := mediaQualityImageDataURL("image/png; charset=binary", content)
	if err != nil {
		t.Fatalf("data URL error = %v", err)
	}
	if !strings.HasPrefix(dataURL, "data:image/png;base64,") {
		t.Fatalf("data URL prefix = %q", dataURL)
	}
	encoded := strings.TrimPrefix(dataURL, "data:image/png;base64,")
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || string(decoded) != string(content) {
		t.Fatalf("data URL payload = %q, err=%v", decoded, err)
	}
	if _, err := mediaQualityImageDataURL("text/plain", content); err == nil {
		t.Fatal("non-image MIME should be rejected")
	}
	if _, err := mediaQualityImageDataURL("image/png", nil); err == nil {
		t.Fatal("empty candidate should be rejected")
	}
}

func TestMediaQualityMessagesCarryFrozenPromptAndImageWithoutProviderURL(t *testing.T) {
	messages, err := mediaQualityMessages(mediaIntent{
		Kind: "image", Prompt: `{"scene":"library"}`, ProviderPrompt: "A library portrait", QualityRetryCount: 1,
	}, "image/png", []byte("candidate"))
	if err != nil {
		t.Fatalf("messages error = %v", err)
	}
	if len(messages) != 2 || messages[0]["role"] != "system" || messages[1]["role"] != "user" {
		t.Fatalf("messages = %#v", messages)
	}
	systemInstruction := stringValue(messages[0]["content"])
	if strings.Contains(systemInstruction, "should not be delivered") || !strings.Contains(systemInstruction, "delivers the second candidate") || !strings.Contains(systemInstruction, "never change a failing verdict to pass") {
		t.Fatalf("quality instruction conflicts with delivery policy: %s", systemInstruction)
	}
	parts, ok := messages[1]["content"].([]any)
	if !ok || len(parts) != 2 {
		t.Fatalf("multimodal parts = %#v", messages[1]["content"])
	}
	if !strings.Contains(stringValue(parts[0].(map[string]any)["text"]), "library") {
		t.Fatalf("text part = %#v", parts[0])
	}
	imagePart := parts[1].(map[string]any)
	imageURL := mapValue(imagePart["image_url"])
	if !strings.HasPrefix(stringValue(imageURL["url"]), "data:image/png;base64,") {
		t.Fatalf("image part = %#v", imagePart)
	}
}

func TestRedactDiagnosticRemovesVisionDataURLs(t *testing.T) {
	redacted := redactDiagnostic(map[string]any{
		"content": []any{map[string]any{
			"type":      "image_url",
			"image_url": map[string]any{"url": "data:image/png;base64,ZmFrZQ=="},
		}},
		"direct": "data:image/jpeg;base64,ZmFrZQ==",
	})
	encoded := jsonString(redacted)
	if strings.Contains(encoded, "ZmFrZQ==") || strings.Contains(encoded, "data:image/") {
		t.Fatalf("vision data leaked into diagnostic: %s", encoded)
	}
	if !strings.Contains(encoded, "REDACTED_IMAGE_DATA") {
		t.Fatalf("redaction marker missing: %s", encoded)
	}
}

func TestMediaPromptInputCarriesOnlyFrozenRetryFeedback(t *testing.T) {
	input := mediaPromptInput(mediaIntent{
		Prompt:               `{"scene":"library","people":["影者"]}`,
		ProviderPrompt:       "A previous library portrait",
		QualityRetryCount:    1,
		QualityRetryGuidance: "Keep the phone out of frame and preserve the library.",
		QualityRetryFeedback: map[string]any{
			"verdict":        "reject",
			"violations":     []any{map[string]any{"code": "capture_mismatch", "severity": "hard", "detail": "phone visible"}},
			"observed_facts": map[string]any{"capture_matches": false},
			"retry_guidance": "Keep the phone out of frame and preserve the library.",
		},
	})
	if !strings.Contains(input, "frozen_media_concept") || !strings.Contains(input, "quality_feedback") || !strings.Contains(input, "capture_mismatch") || !strings.Contains(input, "observed_facts") || !strings.Contains(input, "Keep the phone out of frame") {
		t.Fatalf("retry input lost frozen feedback: %s", input)
	}
	if !strings.Contains(input, "A previous library portrait") {
		t.Fatalf("retry input lost prior provider prompt: %s", input)
	}
}

func TestMediaPromptRetryInstructionUsesQualityFeedbackWithoutChangingFrozenFacts(t *testing.T) {
	initial := mediaPromptSystemInstruction(mediaIntent{})
	retry := mediaPromptSystemInstruction(mediaIntent{QualityRetryCount: 1})
	if strings.Contains(initial, "上一张图片未通过质量检查") {
		t.Fatalf("initial prompt unexpectedly contains retry instruction: %s", initial)
	}
	for _, required := range []string{"quality_feedback", "violations", "observed_facts", "retry_guidance", "frozen_media_concept", "不得新增人物"} {
		if !strings.Contains(retry, required) {
			t.Errorf("retry prompt instruction missing %q: %s", required, retry)
		}
	}
}

func TestMediaQualityDispositionRetriesEveryFirstNonPassAndAcceptsSecond(t *testing.T) {
	cases := []struct {
		verdict string
		count   int
		want    string
	}{
		{verdict: mediaQualityVerdictPass, count: 0, want: mediaQualityVerdictPass},
		{verdict: mediaQualityVerdictPass, count: 1, want: mediaQualityVerdictPass},
		{verdict: mediaQualityVerdictRetry, count: 0, want: mediaQualityActionRetry},
		{verdict: mediaQualityVerdictReject, count: 0, want: mediaQualityActionRetry},
		{verdict: mediaQualityVerdictRetry, count: 1, want: mediaQualityActionAcceptAfterRetry},
		{verdict: mediaQualityVerdictReject, count: 1, want: mediaQualityActionAcceptAfterRetry},
	}
	for _, testCase := range cases {
		if got := mediaQualityDisposition(testCase.count, testCase.verdict); got != testCase.want {
			t.Errorf("mediaQualityDisposition(%d, %q) = %q, want %q", testCase.count, testCase.verdict, got, testCase.want)
		}
	}
}

func TestMediaQualityRetryFeedbackIncludesAllBoundedReviewerFields(t *testing.T) {
	feedback := mediaQualityRetryFeedback(mediaQualityAcceptance{
		SchemaVersion: mediaQualitySchemaVersion,
		Verdict:       mediaQualityVerdictReject,
		Violations:    []mediaQualityViolation{{Code: "capture_mismatch", Severity: "hard", Detail: "phone visible"}},
		ObservedFacts: map[string]bool{"subject_matches": true, "capture_matches": false},
		RetryGuidance: "Keep the phone out of frame.",
	})
	if intValue(feedback["schema_version"]) != mediaQualitySchemaVersion || stringValue(feedback["verdict"]) != mediaQualityVerdictReject || stringValue(feedback["retry_guidance"]) != "Keep the phone out of frame." {
		t.Fatalf("quality feedback fields = %#v", feedback)
	}
	violations, ok := feedback["violations"].([]any)
	if !ok || len(violations) != 1 || stringValue(mapValue(violations[0])["detail"]) != "phone visible" {
		t.Fatalf("quality feedback violations = %#v", feedback["violations"])
	}
	observedFacts := mapValue(feedback["observed_facts"])
	if boolValue(observedFacts["subject_matches"]) != true || boolValue(observedFacts["capture_matches"]) != false {
		t.Fatalf("quality feedback observed facts = %#v", observedFacts)
	}
}

func TestMediaQualityRetryPersistsStructuredFeedbackAndAcceptsSecondCandidate(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	intentID := "media-quality-retry-structured"
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.media_intents(id,owner_fluctlight_id,kind,mime_type,prompt,provider_request_id,provider_job_id,workflow_id,status,quality_retry_count) VALUES($1,'owner-1','image','image/png','{"scene":"library"}','request-1','job-first','workflow-1','running',0)`, intentID); err != nil {
		t.Fatal(err)
	}
	app := &App{DB: repository}
	quality := mediaQualityAcceptance{
		SchemaVersion: mediaQualitySchemaVersion,
		Verdict:       mediaQualityVerdictReject,
		Violations:    []mediaQualityViolation{{Code: "capture_mismatch", Severity: "hard", Detail: "phone visible"}},
		ObservedFacts: map[string]bool{"subject_matches": true, "appearance_matches": true, "scene_matches": true, "capture_matches": false, "framing_matches": true},
	}
	if err := app.prepareMediaQualityRetry(ctx, intentID, "job-first", "", mediaQualityRetryFeedback(quality), "sha-first"); err != nil {
		t.Fatalf("prepareMediaQualityRetry() error = %v", err)
	}
	intent, err := app.readMediaIntent(ctx, intentID)
	if err != nil {
		t.Fatal(err)
	}
	if intent.QualityRetryCount != 1 || intent.QualityVerdict != mediaQualityVerdictRetry || stringValue(mapValue(intent.QualityRetryFeedback)["verdict"]) != mediaQualityVerdictReject {
		t.Fatalf("retry intent = %#v", intent)
	}
	violations, ok := mapValue(intent.QualityRetryFeedback)["violations"].([]any)
	if !ok || len(violations) != 1 || stringValue(mapValue(violations[0])["code"]) != "capture_mismatch" {
		t.Fatalf("persisted violations = %#v", mapValue(intent.QualityRetryFeedback)["violations"])
	}
	if _, err := repository.Pool().Exec(ctx, `UPDATE public.media_intents SET provider_job_id='job-second',status='running' WHERE id=$1`, intentID); err != nil {
		t.Fatal(err)
	}
	if err := app.acceptMediaQualityAfterRetry(ctx, intentID, "job-second", "sha-second"); err != nil {
		t.Fatalf("acceptMediaQualityAfterRetry() error = %v", err)
	}
	var verdict, candidateSHA, providerJobID, status string
	var retryCount int
	if err := repository.Pool().QueryRow(ctx, `SELECT quality_verdict,COALESCE(quality_candidate_sha256,''),COALESCE(provider_job_id,''),status,quality_retry_count FROM public.media_intents WHERE id=$1`, intentID).Scan(&verdict, &candidateSHA, &providerJobID, &status, &retryCount); err != nil {
		t.Fatal(err)
	}
	if verdict != mediaQualityVerdictRetryAccepted || candidateSHA != "sha-second" || providerJobID != "job-second" || status != "running" || retryCount != 1 {
		t.Fatalf("accepted second candidate = verdict:%q sha:%q job:%q status:%q retries:%d", verdict, candidateSHA, providerJobID, status, retryCount)
	}
}

func TestMediaPromptInputOmitsVisualIdentityWorkflowHistory(t *testing.T) {
	input := mediaPromptInput(mediaIntent{Prompt: `{"scene":"library","context_binding":{"visual_identity":{"status":"active","identity_snapshot":{"identity":{"name":"影者"}},"renderer_constraints":{"chest_cup":"B","chest_lora_weight":-3},"timeline":[{"stage":"seed_requested","summary":"工作流节点"}],"canonical_asset_id":"asset-1"}}}`})
	if strings.Contains(input, "timeline") || strings.Contains(input, "seed_requested") || strings.Contains(input, "canonical_asset_id") || strings.Contains(input, "identity_snapshot") {
		t.Fatalf("media prompt retained visual identity workflow metadata: %s", input)
	}
	if !strings.Contains(input, "chest_cup") || !strings.Contains(input, "chest_lora_weight") {
		t.Fatalf("media prompt lost renderer constraints: %s", input)
	}
}

func TestMediaPromptConceptFiltersRequestAndCharacterMetadata(t *testing.T) {
	input := compactMediaConceptForProvider(`{"user_request":"给我画一张她在窗边的照片","character":{"id":"character-1","description":"internal card"},"scene":"图书馆","action":"阅读","appearance":{"hair":"短发"},"context_binding":{"life_context":{"scene":"图书馆"}}}`)
	if strings.Contains(input, "user_request") || strings.Contains(input, "character") || strings.Contains(input, "internal card") {
		t.Fatalf("media prompt retained non-rendering metadata: %s", input)
	}
	for _, fragment := range []string{"scene", "图书馆", "action", "阅读", "appearance", "短发"} {
		if !strings.Contains(input, fragment) {
			t.Fatalf("media prompt lost semantic field %q: %s", fragment, input)
		}
	}
}

func TestMediaQualityConceptKeepsIdentitySemanticsWithoutTimeline(t *testing.T) {
	input := compactMediaConceptForProvider(`{"purpose":"visual_identity","visual_identity":{"identity_snapshot":{"identity":{"visible_text":"一位短发角色"}},"timeline":[{"stage":"vision_ready"}]},"renderer_constraints":{"chest_cup":"B"}}`)
	if !strings.Contains(input, "一位短发角色") || !strings.Contains(input, "chest_cup") {
		t.Fatalf("visual identity semantics were removed: %s", input)
	}
	if strings.Contains(input, "timeline") || strings.Contains(input, "vision_ready") {
		t.Fatalf("visual identity timeline leaked: %s", input)
	}
}
