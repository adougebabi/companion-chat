package core

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type formalAgentE2ESpy struct {
	inner http.RoundTripper
	mu    sync.Mutex
	reqs  []map[string]any
}

func (spy *formalAgentE2ESpy) RoundTrip(request *http.Request) (*http.Response, error) {
	if spy == nil || spy.inner == nil {
		return nil, fmt.Errorf("formal Agent E2E transport is unavailable")
	}
	if request.Body != nil {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			return nil, err
		}
		request.Body = io.NopCloser(bytes.NewReader(body))
		var payload map[string]any
		if json.Unmarshal(body, &payload) == nil {
			spy.mu.Lock()
			spy.reqs = append(spy.reqs, payload)
			spy.mu.Unlock()
		}
	}
	return spy.inner.RoundTrip(request)
}

func (spy *formalAgentE2ESpy) snapshot() []map[string]any {
	spy.mu.Lock()
	defer spy.mu.Unlock()
	result := make([]map[string]any, len(spy.reqs))
	for index, request := range spy.reqs {
		result[index] = cloneMap(request)
	}
	return result
}

func (spy *formalAgentE2ESpy) count() int {
	spy.mu.Lock()
	defer spy.mu.Unlock()
	return len(spy.reqs)
}

type formalAgentE2EFixture struct {
	ctx            context.Context
	repository     *PostgresRepository
	app            *App
	spy            *formalAgentE2ESpy
	ownerID        string
	fluctlightID   string
	conversationID string
	runPrefix      string
}

func newFormalAgentE2EFixture(t *testing.T) *formalAgentE2EFixture {
	t.Helper()
	baseURL, model := liveProviderConfig(t)
	seedCtx, repository := isolatedCoreTestRepository(t)
	suffix := stableDigest(t.Name() + fmt.Sprintf("-%d", time.Now().UnixNano()))[:20]
	ownerID := "fae_owner_" + suffix
	fluctlightID := "fae_fluctlight_" + suffix
	conversationID := "fae_conversation_" + suffix
	seedTurnConversation(t, seedCtx, repository, ownerID, fluctlightID, conversationID)
	corePersona := map[string]any{
		"identity":     map[string]any{"name": "澄光", "self_description": "一位谨慎、温暖、尊重事实边界的本地 AI 伙伴"},
		"life_profile": map[string]any{"city": "上海", "timezone": "Asia/Shanghai"},
		"personality_system": map[string]any{
			"mode": "multiple", "active_profile_id": "day",
			"profiles": []any{
				map[string]any{"id": "day", "name": "晨光", "personality": map[string]any{"openness": 0.7}, "behavioral_policy": map[string]any{"directness": 0.7}},
				map[string]any{"id": "night", "name": "夜澜", "personality": map[string]any{"openness": 0.5}, "behavioral_policy": map[string]any{"gentleness": 0.9}},
			},
			"switching": map[string]any{"rules": []any{map[string]any{"id": "night-rule", "condition": "明确要求安静复盘时切换到夜澜", "target_profile_id": "night"}}},
		},
	}
	if _, err := repository.Pool().Exec(seedCtx, `UPDATE public.fluctlights SET core_persona=$2,identity=$3,personality=$4,behavioral_policy=$5,life_profile=$6 WHERE id=$1`,
		fluctlightID,
		jsonString(corePersona),
		jsonString(map[string]any{"name": "澄光", "timezone": "Asia/Shanghai", "appearance": map[string]any{"hair": "black shoulder-length hair", "outfit": "dark blue shirt"}}),
		jsonString(map[string]any{"openness": 0.7, "conscientiousness": 0.8}),
		jsonString(map[string]any{"response_style": "concise and warm", "directness": 0.7}),
		jsonString(map[string]any{"city": "上海", "timezone": "Asia/Shanghai"}),
	); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(seedCtx, `INSERT INTO public.fluctlight_personality_runtime(fluctlight_id,active_profile_id,revision) VALUES($1,'day',0) ON CONFLICT(fluctlight_id) DO NOTHING`, fluctlightID); err != nil {
		t.Fatal(err)
	}

	endpointID := "fae_endpoint_" + suffix
	secretPurpose := "fae_provider_" + suffix
	if _, err := repository.Pool().Exec(seedCtx, `INSERT INTO public.provider_endpoints(id,kind,base_url,secret_purpose,capability_status,checked_at) VALUES($1,'openai_compatible',$2,$3,'ready',now())`, endpointID, strings.TrimRight(baseURL, "/"), secretPurpose); err != nil {
		t.Fatal(err)
	}
	roles := []string{"initialization", "cognitive_assessment", "media_prompt", "reflection", "visual_identity_vision", "visual_identity_patch", takeoverJudgeRole}
	for _, role := range roles {
		required := "structured_output"
		tokenBudget := 4096
		if role == "cognitive_assessment" {
			required = "structured_output,tool_calling"
		}
		if role == "initialization" {
			tokenBudget = initializationMinimumOutputReserveTokens
		}
		if role == "visual_identity_vision" || role == "media_prompt" {
			required = "structured_output,multimodal_input"
		}
		if role == "visual_identity_vision" {
			required = "structured_output,tool_calling,multimodal_input"
		}
		if _, err := repository.Pool().Exec(seedCtx, `INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,required_capabilities,token_budget,timeout_seconds,retry_policy) VALUES($1,$2,$3,$4,$5,720,'{}')`, role, endpointID, model, required, tokenBudget); err != nil {
			t.Fatal(err)
		}
	}
	settingsKey := []byte("formal-agent-e2e-settings-key-32")
	if key := strings.TrimSpace(os.Getenv("FLUCTLIGHT_LIVE_PROVIDER_API_KEY")); key != "" {
		encrypted, err := encryptSecret(settingsKey, secretPurpose, key)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := repository.Pool().Exec(seedCtx, `INSERT INTO public.setting_secrets(purpose,ciphertext,nonce,updated_at) VALUES($1,$2,$3,now())`, secretPurpose, encrypted.ciphertext, encrypted.nonce); err != nil {
			t.Fatal(err)
		}
	}
	spy := &formalAgentE2ESpy{inner: http.DefaultTransport}
	app := newTestApp(t, repository, spy)
	app.Provider.SettingsKey = settingsKey
	runCtx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	t.Cleanup(cancel)
	return &formalAgentE2EFixture{
		ctx: runCtx, repository: repository, app: app, spy: spy,
		ownerID: ownerID, fluctlightID: fluctlightID, conversationID: conversationID,
		runPrefix: "formal-agent-e2e-" + suffix,
	}
}

func (fixture *formalAgentE2EFixture) projection(t *testing.T, currentInput string, operation MemoryRetrievalOperation) ContextProjection {
	t.Helper()
	projection, err := fixture.app.BuildContextProjectionFor(fixture.ctx, ContextProjectionRequest{
		AuthorizationActorID: fixture.ownerID, SpeakerActorID: fixture.ownerID,
		FluctlightID: fixture.fluctlightID, ConversationID: fixture.conversationID,
		CurrentUserText: currentInput, MemoryOperation: operation,
		MemoryConversationMode: MemoryConversationExact,
	})
	if err != nil {
		t.Fatal(err)
	}
	return projection
}

func formalAgentE2EImage(t *testing.T) ([]byte, map[string]any) {
	t.Helper()
	path := filepath.Join("..", "..", "..", "..", ".gomodcache", "github.com", "klauspost", "compress@v1.19.2", "zip", "testdata", "gophercolor16x16.png")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read real PNG fixture %s: %v", path, err)
	}
	return content, map[string]any{
		"type": "image_url",
		"image_url": map[string]any{
			"url":    "data:image/png;base64," + base64.StdEncoding.EncodeToString(content),
			"detail": "high",
		},
	}
}

func requireFormalAgentE2E(t *testing.T) {
	t.Helper()
	if strings.TrimSpace(os.Getenv("FLUCTLIGHT_LIVE_PROVIDER_TEST")) != "1" {
		t.Skip("set FLUCTLIGHT_LIVE_PROVIDER_TEST=1 to run real FormalAgent E2E")
	}
	for _, name := range []string{"GO_CORE_TEST_DATABASE_URL", "FLUCTLIGHT_LIVE_PROVIDER_URL", "FLUCTLIGHT_LIVE_PROVIDER_MODEL"} {
		if strings.TrimSpace(os.Getenv(name)) == "" {
			t.Fatalf("%s is required for real FormalAgent E2E", name)
		}
	}
}

func requireFormalAgentStructured(t *testing.T, id FormalAgentID, value map[string]any) {
	t.Helper()
	if len(value) == 0 {
		t.Fatalf("%s returned an empty final DTO", id)
	}
}

func formalAgentRequestContains(payload map[string]any, needle string) bool {
	encoded, err := json.Marshal(payload["messages"])
	return err == nil && strings.Contains(string(encoded), needle)
}
