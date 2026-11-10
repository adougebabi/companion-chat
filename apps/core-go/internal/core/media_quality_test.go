package core

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestDerivedVisualQualityPhysicalRequestUsesChosenAvatarSubject(t *testing.T) {
	for _, test := range []struct {
		stage, design, required string
	}{
		{"avatar", "A fox curled up beneath a moon.", "Do not require a person or face"},
		{"avatar", "A portrait of the character with black hair.", "Compare the confirmed reference only if the chosen design depicts the character"},
		{"character_sheet", "The character in three views.", "检查候选是否保持同一人的可识别特征"},
	} {
		t.Run(test.stage+"/"+test.design, func(t *testing.T) {
			intent := mediaIntent{Kind: "image", ProviderPrompt: test.design, Prompt: jsonString(map[string]any{"purpose": "visual_identity", "stage": test.stage, "subject_count": 1})}
			messages, err := mediaQualityMessages(intent, "image/png", []byte("candidate"))
			if err != nil {
				t.Fatal(err)
			}
			referenceURL, err := mediaQualityImageDataURL("image/png", []byte("reference"))
			if err != nil {
				t.Fatal(err)
			}
			addDerivedVisualReviewReference(messages, intent, referenceURL)
			var captured map[string]any
			router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
				captured = cloneMap(payload)
				return fakeProviderResult{Structured: map[string]any{"schema_version": 1, "verdict": "pass", "violations": []any{}, "retry_guidance": "", "observed_facts": map[string]any{"subject_matches": true, "appearance_matches": true, "scene_matches": true, "capture_matches": true, "framing_matches": true}}}
			})
			provider := &ProviderClient{HTTP: &http.Client{Transport: router}}
			ctx := context.WithValue(context.Background(), visionSettingsReaderKey{}, func(context.Context) (VisionUnderstandingSettings, error) {
				return VisionUnderstandingSettings{Enabled: true, ModelRef: visionUnderstandingModelRole}, nil
			})
			_, err = provider.generateWithEino(ctx, EinoModelCall{
				Assignment: providerAssignment{Role: visionUnderstandingModelRole, BaseURL: "http://controlled.invalid", ModelID: "reviewer", Timeout: time.Second},
				Role:       visionUnderstandingModelRole, Messages: formalPromptMessages(FormalAgentMediaQuality, formatProviderMessagesForRole(messages, "media_prompt")),
				SchemaName: "media_quality_acceptance_response", ResponseSchema: mediaQualityAcceptanceResponseSchema(),
			})
			if err != nil {
				t.Fatal(err)
			}
			wire := jsonString(captured["messages"])
			for _, required := range []string{test.design, test.required, "not automatically delivered", base64.StdEncoding.EncodeToString([]byte("reference")), base64.StdEncoding.EncodeToString([]byte("candidate"))} {
				if !strings.Contains(wire, required) {
					t.Fatalf("quality request missing %q: %s", required, wire)
				}
			}
			if test.stage == "avatar" && (strings.Contains(wire, "检查候选是否保持同一人的可识别特征") || strings.Contains(wire, `subject_count`)) {
				t.Fatalf("avatar review retained a forced-person assumption: %s", wire)
			}
			if router.totalRequests() != 1 || captured["max_tokens"] != nil || captured["max_completion_tokens"] != nil {
				t.Fatal("invalid physical review request")
			}
		})
	}
}

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
		Kind: "image", Prompt: `{"scene":"library","capture":{"mode":"mirror_selfie","mirror":true}}`, ProviderPrompt: "A library mirror selfie", QualityRetryCount: 1,
	}, "image/png", []byte("candidate"))
	if err != nil {
		t.Fatalf("messages error = %v", err)
	}
	if len(messages) != 2 || messages[0]["role"] != "system" || messages[1]["role"] != "user" {
		t.Fatalf("messages = %#v", messages)
	}
	systemInstruction := stringValue(messages[0]["content"])
	if strings.Contains(systemInstruction, "delivers the second candidate") || !strings.Contains(systemInstruction, "never change a failing verdict to pass") || !strings.Contains(systemInstruction, "not automatically delivered") {
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
	formatted := formatProviderMessagesForRole(messages, "media_prompt")
	formattedParts := arrayValue(formatted[1]["content"])
	formattedText := stringValue(mapValue(formattedParts[0])["text"])
	if !strings.Contains(formattedText, `"frozen_media_concept":`) || !strings.Contains(formattedText, `"mode":"mirror_selfie"`) || strings.Contains(formattedText, `\"`) || strings.Contains(formattedText, "frozen_media_concept: '{") {
		t.Fatalf("quality text part retained nested JSON or lost capture: %s", formattedText)
	}
	if jsonString(formattedParts[1]) != jsonString(imagePart) {
		t.Fatalf("quality formatter changed candidate image part: %#v", formattedParts[1])
	}
}

func TestMediaQualityFinalProviderRequestKeepsImageAndReadableText(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	seedCognitiveProviderRole(t, ctx, repository, "media-quality-wire")
	enableControlledVision(t, ctx, repository)
	var modelText, imageURL string
	router := newFakeProviderRouter().on("media_quality_acceptance_response", func(payload map[string]any) fakeProviderResult {
		for _, raw := range arrayValue(payload["messages"]) {
			message := mapValue(raw)
			if stringValue(message["role"]) != "user" {
				continue
			}
			for _, rawPart := range arrayValue(message["content"]) {
				part := mapValue(rawPart)
				if stringValue(part["type"]) == "text" {
					modelText += stringValue(part["text"])
				}
				if stringValue(part["type"]) == "image_url" {
					imageURL = stringValue(mapValue(part["image_url"])["url"])
				}
			}
		}
		return fakeProviderResult{Structured: map[string]any{
			"schema_version": 1, "verdict": "pass", "violations": []any{}, "retry_guidance": "",
			"observed_facts": map[string]any{"subject_matches": true, "appearance_matches": true, "scene_matches": true, "capture_matches": true, "framing_matches": true},
		}}
	})
	app := newTestApp(t, repository, router)
	_, err := app.RunMediaQualityTask(ctx, MediaQualityTaskInput{Intent: mediaIntent{
		Kind: "image", ProviderPrompt: "A portrait in the library",
		Prompt: `{"purpose":"visual_identity","scene":"library","visual_identity":{"identity_snapshot":{"identity":{"name":"澄光","background_story":"private story","fluctlight_id":"fl-private"}}},"renderer_constraints":{"chest_cup":"B","chest_lora_weight":-3}}`,
	}, ContentType: "image/png", Content: []byte("candidate-image")})
	if err != nil {
		t.Fatal(err)
	}
	var diagnosticPrompt []byte
	if err := repository.Pool().QueryRow(ctx, `SELECT prompt FROM public.diagnostic_model_runs WHERE scenario='media_quality_acceptance' ORDER BY queued_at DESC LIMIT 1`).Scan(&diagnosticPrompt); err != nil {
		t.Fatal(err)
	}
	var diagnosticMessages []map[string]any
	if err := json.Unmarshal(diagnosticPrompt, &diagnosticMessages); err != nil {
		t.Fatal(err)
	}
	diagnosticText := ""
	for _, message := range diagnosticMessages {
		if message["role"] == "user" {
			for _, rawPart := range arrayValue(message["content"]) {
				part := mapValue(rawPart)
				if part["type"] == "text" {
					diagnosticText += stringValue(part["text"])
				}
			}
		}
	}
	if diagnosticText != modelText || !strings.Contains(string(diagnosticPrompt), "REDACTED_IMAGE_DATA") || strings.Contains(string(diagnosticPrompt), imageURL) {
		t.Fatalf("physical diagnostic does not match readable multimodal wire: prompt=%s image=%s", diagnosticPrompt, imageURL)
	}
	var taskData map[string]any
	if err := json.Unmarshal([]byte(modelText), &taskData); err != nil {
		t.Fatalf("media quality task text is not JSON: %v", err)
	}
	if !strings.HasPrefix(imageURL, "data:image/png;base64,") || len(mapValue(taskData["frozen_media_concept"])) == 0 || taskData["media_kind"] != "image" || !strings.Contains(modelText, "澄光") {
		t.Fatalf("media quality wire lost multimodal readable input: text=%s image=%s", modelText, imageURL)
	}
	for _, forbidden := range []string{"private story", "fl-private", "chest_lora_weight", `\"`} {
		if strings.Contains(modelText, forbidden) {
			t.Fatalf("media quality wire retained %q: %s", forbidden, modelText)
		}
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
	var decoded map[string]any
	if err := json.Unmarshal([]byte(input), &decoded); err != nil || len(mapValue(decoded["frozen_media_concept"])) == 0 {
		t.Fatalf("retry concept was serialized as a nested JSON string: %s (%v)", input, err)
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

func TestMediaQualityDispositionRetriesOnceAndStopsOnSecondNonPass(t *testing.T) {
	cases := []struct {
		verdict string
		count   int
		want    string
	}{
		{verdict: mediaQualityVerdictPass, count: 0, want: mediaQualityVerdictPass},
		{verdict: mediaQualityVerdictPass, count: 1, want: mediaQualityVerdictPass},
		{verdict: mediaQualityVerdictRetry, count: 0, want: mediaQualityActionRetry},
		{verdict: mediaQualityVerdictReject, count: 0, want: mediaQualityActionRetry},
		{verdict: mediaQualityVerdictRetry, count: 1, want: "stop"},
		{verdict: mediaQualityVerdictReject, count: 1, want: "stop"},
		{verdict: mediaQualityVerdictReject, count: 2, want: "stop"},
	}
	for _, testCase := range cases {
		if got := mediaQualityDisposition(testCase.count, testCase.verdict); got != testCase.want {
			t.Errorf("mediaQualityDisposition(%d, %q) = %q, want %q", testCase.count, testCase.verdict, got, testCase.want)
		}
	}
}

func TestTerminalMediaQualityFailureDoesNotInventVisualIdentityFailure(t *testing.T) {
	ordinary := mediaIntent{Status: "failed", ProviderJobID: "job", QualityCandidateSHA: "sha", QualityVerdict: mediaQualityVerdictReject}
	if !terminalMediaQualityFailure(ordinary) {
		t.Fatal("ordinary rejected candidate did not retain its terminal quality failure")
	}
	for _, stage := range []string{"avatar", "character_sheet"} {
		candidate := ordinary
		candidate.Prompt = jsonString(map[string]any{"purpose": "visual_identity", "stage": stage})
		if terminalMediaQualityFailure(candidate) {
			t.Fatalf("visual %s upload failure was incorrectly classified as quality failure", stage)
		}
	}
	incomplete := ordinary
	incomplete.QualityCandidateSHA = ""
	if terminalMediaQualityFailure(incomplete) {
		t.Fatal("failure without a checked candidate was classified as a review rejection")
	}
	legacy := mediaIntent{Status: "running", QualityVerdict: mediaQualityVerdictRetryAccepted}
	if !terminalMediaQualityFailure(legacy) || mediaQualityFailureCode(legacy.QualityVerdict) != "media_quality_legacy_review_required" {
		t.Fatal("unsettled legacy non-pass could be published automatically")
	}
	legacy.Status = "completed"
	if terminalMediaQualityFailure(legacy) {
		t.Fatal("completed historical media was reclassified")
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

func TestMediaQualityRetryPersistsFeedbackAndRejectsSecondCandidate(t *testing.T) {
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
	if err := app.rejectMediaQuality(ctx, intentID, "job-second", "sha-second", mediaQualityVerdictReject); err != nil {
		t.Fatalf("rejectMediaQuality() error = %v", err)
	}
	if err := app.rejectMediaQuality(ctx, intentID, "job-second", "sha-second", mediaQualityVerdictReject); err != nil {
		t.Fatalf("replaying the same rejection failed: %v", err)
	}
	if err := app.rejectMediaQuality(ctx, intentID, "job-second", "different-sha", mediaQualityVerdictReject); err == nil {
		t.Fatal("rejection replay accepted a different candidate")
	}
	var verdict, candidateSHA, providerJobID, status string
	var retryCount int
	if err := repository.Pool().QueryRow(ctx, `SELECT quality_verdict,COALESCE(quality_candidate_sha256,''),COALESCE(provider_job_id,''),status,quality_retry_count FROM public.media_intents WHERE id=$1`, intentID).Scan(&verdict, &candidateSHA, &providerJobID, &status, &retryCount); err != nil {
		t.Fatal(err)
	}
	if verdict != mediaQualityVerdictReject || candidateSHA != "sha-second" || providerJobID != "job-second" || status != "failed" || retryCount != 1 {
		t.Fatalf("rejected second candidate = verdict:%q sha:%q job:%q status:%q retries:%d", verdict, candidateSHA, providerJobID, status, retryCount)
	}
}

func TestRejectedMediaRecoveryDoesNotReplayOrReuseTerminalAction(t *testing.T) {
	ctx, repository, app, owner, fid, _ := setupDirectPublicationToolTest(t, "quality-recovery")
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES($1,'controlled','test')`, owner); err != nil {
		t.Fatal(err)
	}
	const intentID = "quality-recovery-intent"
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.media_intents(id,owner_fluctlight_id,kind,mime_type,prompt,provider_request_id,provider_job_id,workflow_id,status,revision,quality_retry_count,quality_verdict,quality_candidate_sha256) VALUES($1,$2,'image','image/png','{}','quality-recovery-request','quality-recovery-job','quality-recovery-workflow','failed',1,1,'reject','candidate-sha')`, intentID, fid); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.cognition_action_outcomes(id,fluctlight_id,action_id,call_id,capability_name,status,success_boundary,external_ref,request_digest) VALUES('quality-recovery-outcome',$1,'quality-recovery-action','quality-recovery-call','media.image.generate','failed','asset_ready',$2,$3)`, fid, intentID, stableDigest("quality-recovery")); err != nil {
		t.Fatal(err)
	}
	source, err := ownerDiagnosticSource("media-prompts")
	if err != nil {
		t.Fatal(err)
	}
	var rawDiagnostic []byte
	if err := repository.Pool().QueryRow(ctx, `SELECT data FROM (`+source+`) AS source WHERE key=$2`, owner, intentID).Scan(&rawDiagnostic); err != nil {
		t.Fatal(err)
	}
	diagnostic := decodeObject(rawDiagnostic)
	if diagnostic["quality_candidate_terminal"] != true || diagnostic["quality_terminal_action"] != true || diagnostic["quality_asset_ready"] != false {
		t.Fatalf("diagnostic did not expose real recovery restrictions: %#v", diagnostic)
	}
	for _, tc := range []struct{ mode, code string }{{"resume", "media_quality_candidate_terminal"}, {"regenerate", "media_quality_new_request_required"}} {
		t.Run(tc.mode, func(t *testing.T) {
			_, err := app.RetryOwnerDiagnosticMedia(ctx, owner, intentID, map[string]any{"expected_revision": 1, "mode": tc.mode, "idempotency_key": "quality-recovery-" + tc.mode, "reason": "test"})
			var surface *SurfaceError
			if !errors.As(err, &surface) || surface.Code != tc.code {
				t.Fatalf("recovery error = %v, want %s", err, tc.code)
			}
		})
	}
	var status string
	var retryIntents int
	if err := repository.Pool().QueryRow(ctx, `SELECT status FROM public.media_intents WHERE id=$1`, intentID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.platform_workflow_intents WHERE payload->>'intent_id'=$1`, intentID).Scan(&retryIntents); err != nil {
		t.Fatal(err)
	}
	if status != "failed" || retryIntents != 0 {
		t.Fatalf("terminal candidate changed: status=%s queued=%d", status, retryIntents)
	}
}

func TestLegacyNonPassRecoveryUsesCommittedDeliveryWithoutRepublishing(t *testing.T) {
	ctx, repository, app, owner, fid, conversation := setupDirectPublicationToolTest(t, "legacy-quality")
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES($1,'controlled','test')`, owner); err != nil {
		t.Fatal(err)
	}
	for index, tc := range []struct {
		id, status string
		delivered  bool
	}{{"legacy-delivered-running", "running", true}, {"legacy-delivered-failed", "failed", true}, {"legacy-not-delivered", "running", false}, {"legacy-completed-removed", "completed", false}} {
		asset := "asset_" + tc.id
		workflow := "workflow-" + tc.id
		if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.media_intents(id,owner_fluctlight_id,kind,mime_type,prompt,provider_request_id,provider_job_id,workflow_id,conversation_id,status,revision,quality_retry_count,quality_verdict,quality_candidate_sha256) VALUES($1,$2,'image','image/png','{}',$3,$4,$5,$6,$7,1,1,'retry_accepted','candidate-sha')`, tc.id, fid, "request-"+tc.id, "job-"+tc.id, workflow, conversation, tc.status); err != nil {
			t.Fatal(err)
		}
		if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.media_assets(id,owner_fluctlight_id,version,kind,mime_type,byte_size,sha256,bucket,object_key,provider_request_id,workflow_id,status) VALUES($1,$2,'v1','image','image/png',1,'candidate-sha','fixture','fixture',$3,$4,'ready')`, asset, fid, "request-"+tc.id, workflow); err != nil {
			t.Fatal(err)
		}
		if tc.delivered {
			if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.conversation_messages(id,conversation_id,sequence,author_actor_id,kind,text,attachment_refs,idempotency_key) VALUES($1,$2,$3,$4,'media_reference','图片已生成。',$5,$6)`, "message-"+tc.id, conversation, index+1, fid, jsonBytes([]string{asset}), "media:"+tc.id+":conversation-result"); err != nil {
				t.Fatal(err)
			}
			if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.media_references(id,asset_id,owner_fluctlight_id,target_type,target_id) VALUES($1,$2,$3,'conversation',$4)`, "reference-"+tc.id, asset, fid, conversation); err != nil {
				t.Fatal(err)
			}
		}
		if tc.status == "failed" {
			if _, err := app.RetryOwnerDiagnosticMedia(ctx, owner, tc.id, map[string]any{"expected_revision": 1, "mode": "resume", "idempotency_key": "resume-" + tc.id, "reason": "核对已交付结果"}); err != nil {
				t.Fatal(err)
			}
		}
		result, err := app.ProcessMediaIntent(ctx, tc.id)
		if tc.delivered {
			if err != nil || result["status"] != "completed" || result["recovered_existing_delivery"] != true {
				t.Fatalf("existing delivery did not settle: result=%#v err=%v", result, err)
			}
		} else if tc.status == "completed" {
			if err != nil || result["status"] != "completed" {
				t.Fatalf("completed historical task was reprocessed: result=%#v err=%v", result, err)
			}
		} else if err == nil || replyErrorCode(err) != "media_quality_legacy_review_required" {
			t.Fatalf("undelivered candidate was not held for review: result=%#v err=%v", result, err)
		}
		var messages int
		if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND idempotency_key=$2`, conversation, "media:"+tc.id+":conversation-result").Scan(&messages); err != nil {
			t.Fatal(err)
		}
		if tc.delivered && messages != 1 || !tc.delivered && messages != 0 {
			t.Fatalf("unexpected publication count for %s: %d", tc.id, messages)
		}
	}
}

func TestLegacyNonPassPartialMultiTargetDeliveryDoesNotBecomeComplete(t *testing.T) {
	ctx, repository, app, _, fid, conversation := setupDirectPublicationToolTest(t, "legacy-quality-partial")
	const intentID, momentID = "legacy-partial-media", "legacy-partial-moment"
	asset := "asset_" + intentID
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.moments(id,owner_fluctlight_id,author_actor_id,text,visibility,status,media_asset_ids) VALUES($1,$2,$2,'正文','participants','visible','[]')`, momentID, fid); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.media_intents(id,owner_fluctlight_id,kind,mime_type,prompt,provider_request_id,provider_job_id,workflow_id,conversation_id,moment_id,status,quality_retry_count,quality_verdict,quality_candidate_sha256) VALUES($1,$2,'image','image/png','{}','legacy-partial-request','legacy-partial-job','legacy-partial-workflow',$3,$4,'running',1,'retry_accepted','candidate-sha')`, intentID, fid, conversation, momentID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.media_assets(id,owner_fluctlight_id,version,kind,mime_type,byte_size,sha256,bucket,object_key,provider_request_id,workflow_id,status) VALUES($1,$2,'v1','image','image/png',1,'candidate-sha','fixture','fixture','legacy-partial-request','legacy-partial-workflow','ready')`, asset, fid); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.conversation_messages(id,conversation_id,sequence,author_actor_id,kind,text,attachment_refs,idempotency_key) VALUES('legacy-partial-message',$1,1,$2,'media_reference','图片已生成。',$3,$4)`, conversation, fid, jsonBytes([]string{asset}), "media:"+intentID+":conversation-result"); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.media_references(id,asset_id,owner_fluctlight_id,target_type,target_id) VALUES('legacy-partial-reference',$1,$2,'conversation',$3)`, asset, fid, conversation); err != nil {
		t.Fatal(err)
	}
	_, err := app.ProcessMediaIntent(ctx, intentID)
	if err == nil || replyErrorCode(err) != "media_quality_legacy_review_required" {
		t.Fatalf("partially delivered candidate was treated as complete: %v", err)
	}
	var status string
	var momentAssets, messageAssets []byte
	if err := repository.Pool().QueryRow(ctx, `SELECT status FROM public.media_intents WHERE id=$1`, intentID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT media_asset_ids FROM public.moments WHERE id=$1`, momentID).Scan(&momentAssets); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT attachment_refs FROM public.conversation_messages WHERE id='legacy-partial-message'`).Scan(&messageAssets); err != nil {
		t.Fatal(err)
	}
	if status == "completed" || len(decodeArray(momentAssets)) != 0 || !containsStringValue(decodeArray(messageAssets), asset) {
		t.Fatalf("partial effects changed: status=%s moment=%s message=%s", status, momentAssets, messageAssets)
	}
	// Reconstruct the distinct historical state in which the second target had
	// also committed before settlement; the retry must only settle it.
	if _, err := repository.Pool().Exec(ctx, `UPDATE public.moments SET media_asset_ids=$2 WHERE id=$1`, momentID, jsonBytes([]string{asset})); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.media_references(id,asset_id,owner_fluctlight_id,target_type,target_id) VALUES('legacy-partial-moment-reference',$1,$2,'moment',$3)`, asset, fid, momentID); err != nil {
		t.Fatal(err)
	}
	result, err := app.ProcessMediaIntent(ctx, intentID)
	if err != nil || result["status"] != "completed" || result["recovered_existing_delivery"] != true {
		t.Fatalf("fully delivered targets did not settle: result=%#v err=%v", result, err)
	}
	var references int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.media_references WHERE asset_id=$1`, asset).Scan(&references); err != nil || references != 2 {
		t.Fatalf("settlement changed already committed references: count=%d err=%v", references, err)
	}
}

func TestMediaPromptInputOmitsVisualIdentityWorkflowHistory(t *testing.T) {
	input := mediaPromptInput(mediaIntent{Prompt: `{"scene":"library","context_binding":{"visual_identity":{"status":"active","identity_snapshot":{"identity":{"name":"影者","background":"private background","background_story":"private story"}},"renderer_constraints":{"chest_cup":"B","chest_lora_weight":-3},"timeline":[{"stage":"seed_requested","summary":"工作流节点"}],"canonical_asset_id":"asset-1"}}}`})
	if strings.Contains(input, "timeline") || strings.Contains(input, "seed_requested") || strings.Contains(input, "canonical_asset_id") || strings.Contains(input, "private background") || strings.Contains(input, "private story") || strings.Contains(input, "chest_lora_weight") {
		t.Fatalf("media prompt retained visual identity workflow metadata: %s", input)
	}
	if !strings.Contains(input, "chest_cup") || !strings.Contains(input, "影者") {
		t.Fatalf("media prompt lost visual semantics: %s", input)
	}
}

func TestMediaPromptRejectsInvalidFrozenConceptBeforeProvider(t *testing.T) {
	_, err := (&App{}).RunMediaPromptTask(context.Background(), MediaPromptTaskInput{Intent: mediaIntent{Prompt: `{"scene":`}})
	if err == nil || !strings.Contains(err.Error(), "media_prompt_frozen_concept_invalid") {
		t.Fatalf("invalid frozen concept reached Provider boundary: %v", err)
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
	input := compactMediaConceptForProvider(`{"purpose":"visual_identity","visual_identity":{"identity_snapshot":{"identity":{"visible_text":"一位短发角色","background_story":"private story"}},"timeline":[{"stage":"vision_ready"}]},"renderer_constraints":{"chest_cup":"B","chest_lora_weight":-3}}`)
	if !strings.Contains(input, "一位短发角色") || !strings.Contains(input, "chest_cup") {
		t.Fatalf("visual identity semantics were removed: %s", input)
	}
	if strings.Contains(input, "timeline") || strings.Contains(input, "vision_ready") || strings.Contains(input, "private story") || strings.Contains(input, "chest_lora_weight") {
		t.Fatalf("visual identity timeline leaked: %s", input)
	}
}
