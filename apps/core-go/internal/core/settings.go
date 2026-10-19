package core

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func (a *App) ReadSettings(ctx context.Context, actorID string) (map[string]any, error) {
	if _, ok := SurfaceDBTx(ctx); !ok {
		return a.configurationSnapshot(ctx, func(ctx context.Context) (map[string]any, error) { return a.ReadSettings(ctx, actorID) })
	}
	reader := a.configurationReader(ctx)
	var owner string
	if err := reader.QueryRow(ctx, `SELECT human_actor_id FROM public.owner_accounts LIMIT 1`).Scan(&owner); err != nil || owner != actorID {
		return nil, errors.New("forbidden")
	}
	rows, err := reader.Query(ctx, `SELECT key,value_json,revision FROM public.runtime_settings ORDER BY key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := map[string]any{}
	versions := map[string]any{}
	for rows.Next() {
		var key, raw string
		var revision int64
		if err := rows.Scan(&key, &raw, &revision); err != nil {
			return nil, err
		}
		var value any
		if json.Unmarshal([]byte(raw), &value) != nil {
			return nil, &SurfaceError{Status: 500, Code: "settings_value_invalid", Message: "已保存配置无法解析，请查看配置诊断", Details: map[string]any{"key": key}}
		}
		values[key], versions[key] = value, revision
	}
	readErr := rows.Err()
	rows.Close()
	if readErr != nil {
		return nil, readErr
	}
	wake := normalizeWakeUpSettings(mapValue(values["product.wakeup"]))
	values["product.wakeup"] = map[string]any{"enabled": wake.Enabled, "interval_seconds": wake.IntervalSeconds}
	if _, exists := values[conversationReplySettingKey]; !exists {
		values[conversationReplySettingKey] = defaultConversationReplySettings()
	}
	if _, exists := values[visionUnderstandingSettingKey]; !exists {
		values[visionUnderstandingSettingKey] = defaultVisionUnderstandingSettings()
	}
	values["ai.capability_status"] = a.replyVisionCapabilityStatus(ctx, values)
	secretRows, err := reader.Query(ctx, `SELECT purpose FROM public.setting_secrets ORDER BY purpose`)
	if err != nil {
		return nil, err
	}
	defer secretRows.Close()
	secrets := make([]string, 0)
	for secretRows.Next() {
		var purpose string
		if err := secretRows.Scan(&purpose); err != nil {
			return nil, err
		}
		secrets = append(secrets, purpose)
	}
	readErr = secretRows.Err()
	secretRows.Close()
	if readErr != nil {
		return nil, readErr
	}
	versionRows, err := reader.Query(ctx, `SELECT resource_key,revision FROM public.configuration_revisions`)
	if err != nil {
		return nil, err
	}
	for versionRows.Next() {
		var key string
		var revision int64
		if err := versionRows.Scan(&key, &revision); err != nil {
			versionRows.Close()
			return nil, err
		}
		versions[key] = revision
	}
	readErr = versionRows.Err()
	versionRows.Close()
	if readErr != nil {
		return nil, readErr
	}
	return map[string]any{"values": values, "configured_secrets": secrets, "versions": versions}, nil
}

func (a *App) UpdateSettings(ctx context.Context, actorID string, payload map[string]any) (map[string]any, error) {
	current, err := a.ReadSettings(ctx, actorID)
	if err != nil {
		return nil, err
	}
	originalPayload := cloneMap(payload)
	values := mapValue(payload["values"])
	if raw, exists := values[conversationReplySettingKey]; exists {
		config, decodeErr := decodeConversationReplySettings(jsonBytes(raw))
		if decodeErr != nil {
			return nil, decodeErr
		}
		values[conversationReplySettingKey] = config
	}
	if raw, exists := values[visionUnderstandingSettingKey]; exists {
		config, decodeErr := decodeVisionUnderstandingSettings(jsonBytes(raw))
		if decodeErr != nil {
			return nil, decodeErr
		}
		values[visionUnderstandingSettingKey] = config
	}
	if raw, exists := values["kev"]; exists {
		config, err := decodeKevConfig(jsonBytes(raw))
		if err != nil {
			return nil, err
		}
		values["kev"] = config
	}
	if raw, exists := values["product.summary"]; exists {
		setting := mapValue(raw)
		seconds, ok := intValueExact(setting["interval_seconds"])
		if !ok || seconds < 300 || seconds > 600 || len(setting) > 2 {
			return nil, ErrInvalidArguments
		}
	}
	if value, exists := mapValue(values["product.autonomy"])["topic_suppression_seconds"]; exists {
		seconds, ok := intValueExact(value)
		if !ok || seconds < 300 || seconds > 7*24*60*60 {
			return nil, ErrInvalidArguments
		}
	}
	if setting, exists := values["product.summary"]; exists {
		for key := range mapValue(setting) {
			if key != "interval_seconds" && key != "max_runes" {
				return nil, ErrInvalidArguments
			}
		}
		if value, exists := mapValue(setting)["max_runes"]; exists {
			budget, ok := intValueExact(value)
			if !ok || budget < 512 || budget > 4096 {
				return nil, ErrInvalidArguments
			}
		}
	}
	if raw, ok := values["media.comfyui"]; ok {
		if err := validateComfySetting(raw); err != nil {
			return nil, err
		}
	}
	if raw, ok := values["product.notifications"]; ok {
		if err := validNotificationPreferences(raw); err != nil {
			return nil, err
		}
	}
	previousWakeUp := normalizeWakeUpSettings(mapValue(mapValue(current["values"])["product.wakeup"]))
	var nextWakeUp *WakeUpSettings
	if raw, exists := values["product.wakeup"]; exists {
		normalized, mergeErr := mergeWakeUpSettings(previousWakeUp, raw)
		if mergeErr != nil {
			return nil, mergeErr
		}
		nextWakeUp = &normalized
		values["product.wakeup"] = map[string]any{
			"enabled":          normalized.Enabled,
			"interval_seconds": normalized.IntervalSeconds,
		}
	}
	secrets := mapValue(payload["secrets"])
	clear := arrayValue(payload["clear_secrets"])
	resources := []string{}
	for key := range values {
		resources = append(resources, key)
	}
	for purpose, raw := range secrets {
		if purpose == "" || len(purpose) > 128 {
			return nil, invalid("Invalid secret purpose")
		}
		if raw == nil {
			continue
		}
		plain, ok := raw.(string)
		if !ok {
			return nil, invalid("Secret values must be strings or null")
		}
		if plain != "" && plain != "configured" {
			resources = append(resources, "secret:"+purpose)
		}
	}
	for _, raw := range clear {
		purpose, ok := raw.(string)
		if !ok || purpose == "" || len(purpose) > 128 {
			return nil, invalid("Invalid secret clear purpose")
		}
		if value := stringValue(secrets[purpose]); value != "" && value != "configured" {
			return nil, invalid("Cannot replace and clear the same secret")
		}
		resources = append(resources, "secret:"+purpose)
	}
	sort.Strings(resources)
	var command map[string]any

	err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var replay bool
		var err error
		command, replay, err = beginConfigurationCommandTx(ctx, tx, actorID, "settings_update", originalPayload, resources)
		if err != nil || replay {
			return err
		}
		for key, value := range values {
			if key != "product.notifications" && key != "kev" && key != "media.comfyui" && key != "product.autonomy" && key != "product.wakeup" && key != "product.goal_retry" && key != "product.summary" && key != "diagnostics.retention" && key != "media.h3" && key != "llm.queue" && key != conversationReplySettingKey && key != visionUnderstandingSettingKey {
				return fmt.Errorf("unknown setting %s", key)
			}
			if key == "product.goal_retry" {
				policy := mapValue(value)
				if len(policy) != 3 || intValue(policy["max_attempts"]) < 1 || intValue(policy["max_attempts"]) > 20 || intValue(policy["base_seconds"]) < 30 || intValue(policy["base_seconds"]) > 3600 || intValue(policy["max_seconds"]) < intValue(policy["base_seconds"]) || intValue(policy["max_seconds"]) > 86400 {
					return errors.New("product_goal_retry_invalid")
				}
			}
			if key == "llm.queue" {
				value = normalizeProviderQueueSettings(value)
				if value == nil {
					return errors.New("llm_queue_invalid")
				}
			}
			if key == "product.autonomy" {
				next, err := applyGlobalAutonomyConfigurationTx(ctx, tx, mapValue(value))
				if err != nil {
					return err
				}
				value = next
			}
			if _, err := tx.Exec(ctx, `INSERT INTO public.runtime_settings (key,value_json,updated_at) VALUES ($1,$2,$3) ON CONFLICT (key) DO UPDATE SET value_json=excluded.value_json,updated_at=excluded.updated_at,revision=runtime_settings.revision+1`, key, jsonString(value), time.Now().UTC()); err != nil {
				return err
			}
			if key == "product.summary" {
				budget := intValue(mapValue(value)["max_runes"])
				if budget == 0 {
					budget = runtimeSummaryMaxRunes
				}
				if err := a.rebuildRuntimeSummariesForBudgetTx(ctx, tx, actorID, budget); err != nil {
					return err
				}
			}
		}
		for purpose, value := range secrets {
			plain, ok := value.(string)
			if !ok || plain == "" || plain == "configured" {
				continue
			}
			encrypted, err := encryptSecret(a.SettingsKey, purpose, plain)
			if err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO public.setting_secrets (purpose,ciphertext,nonce,updated_at) VALUES ($1,$2,$3,$4) ON CONFLICT (purpose) DO UPDATE SET ciphertext=excluded.ciphertext,nonce=excluded.nonce,updated_at=excluded.updated_at`, purpose, encrypted.ciphertext, encrypted.nonce, time.Now().UTC()); err != nil {
				return err
			}
		}
		for _, raw := range clear {
			purpose := stringValue(raw)
			if purpose == "" {
				continue
			}
			if _, err := tx.Exec(ctx, `DELETE FROM public.setting_secrets WHERE purpose=$1`, purpose); err != nil {
				return err
			}
		}
		if command != nil {
			command["wake_rearm_required"] = nextWakeUp != nil && wakeUpSettingsNeedsRearm(previousWakeUp, *nextWakeUp)
			return finishConfigurationCommandTx(ctx, tx, actorID, originalPayload, command, "settings_update")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if nextWakeUp != nil && wakeUpSettingsNeedsRearm(previousWakeUp, *nextWakeUp) || command != nil && command["wake_rearm_required"] == true {
		if _, err := a.EnsureWakeUpIntents(ctx); err != nil {
			return nil, fmt.Errorf("product_wakeup_rearm_failed: %w", err)
		}
		if _, err := a.ReleaseDueWakeUpIntents(ctx, wakeUpDueSweepLimit); err != nil {
			return nil, fmt.Errorf("product_wakeup_rearm_failed: %w", err)
		}
	}
	result, err := a.ReadSettings(ctx, actorID)
	if err != nil {
		return nil, err
	}
	if command != nil {
		result["command"] = command
	}
	return result, nil
}

func wakeUpSettingsNeedsRearm(previous, next WakeUpSettings) bool {
	return !previous.Enabled && next.Enabled
}

func mergeWakeUpSettings(previous WakeUpSettings, raw any) (WakeUpSettings, error) {
	input, ok := raw.(map[string]any)
	if !ok {
		return WakeUpSettings{}, errors.New("product_wakeup_invalid")
	}
	merged := map[string]any{
		"enabled":          previous.Enabled,
		"interval_seconds": previous.IntervalSeconds,
	}
	for key, value := range input {
		switch key {
		case "enabled":
			if _, ok := value.(bool); !ok {
				return WakeUpSettings{}, errors.New("product_wakeup_invalid")
			}
		case "interval_seconds":
			if _, isString := value.(string); isString {
				return WakeUpSettings{}, errors.New("product_wakeup_invalid")
			}
			parsed, ok := numberFloat(value)
			if !ok || parsed != float64(int(parsed)) {
				return WakeUpSettings{}, errors.New("product_wakeup_invalid")
			}
		default:
			return WakeUpSettings{}, errors.New("product_wakeup_invalid")
		}
		merged[key] = value
	}
	return normalizeWakeUpSettings(merged), nil
}

func normalizeProviderQueueSettings(value any) map[string]any {
	settings := map[string]any{
		"generated_concurrency": providerQueueDefaultConcurrency,
		"embedding_concurrency": providerQueueDefaultEmbedding,
	}
	input := mapValue(value)
	if len(input) == 0 && value != nil {
		return nil
	}
	for key := range input {
		if key != "generated_concurrency" && key != "embedding_concurrency" {
			return nil
		}
	}
	for key, fallback := range map[string]int{"generated_concurrency": providerQueueDefaultConcurrency, "embedding_concurrency": providerQueueDefaultEmbedding} {
		if raw, ok := input[key]; ok {
			parsed, valid := numberFloat(raw)
			if !valid || parsed != float64(int(parsed)) || int(parsed) < providerQueueMinConcurrency || int(parsed) > providerQueueMaxConcurrency {
				return nil
			}
			settings[key] = int(parsed)
		} else {
			settings[key] = fallback
		}
	}
	return settings
}

type encryptedValue struct{ ciphertext, nonce []byte }

func encryptSecret(key []byte, purpose, plain string) (encryptedValue, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return encryptedValue{}, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return encryptedValue{}, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return encryptedValue{}, err
	}
	return encryptedValue{ciphertext: gcm.Seal(nil, nonce, []byte(plain), []byte(purpose)), nonce: nonce}, nil
}

func (a *App) ProviderEndpoints(ctx context.Context, actorID string) ([]map[string]any, error) {
	if _, err := a.ReadSettings(ctx, actorID); err != nil {
		return nil, err
	}
	rows, err := a.configurationReader(ctx).Query(ctx, `SELECT jsonb_build_object('id',e.id,'kind',e.kind,'base_url',e.base_url,'secret_purpose',e.secret_purpose,'capability_status',e.capability_status,'secret_configured',EXISTS(SELECT 1 FROM public.setting_secrets s WHERE s.purpose=e.secret_purpose),'revision',COALESCE((SELECT revision FROM public.configuration_revisions WHERE resource_key='endpoint:'||e.id),0),'roles',COALESCE((SELECT jsonb_agg(jsonb_build_object('role',r.role,'model_id',r.model_id) ORDER BY r.role) FROM public.model_roles r WHERE r.provider_endpoint_id=e.id AND r.role IN ('generic_llm','embedding','conversation_reply','vision_understanding')),'[]'::jsonb)) FROM public.provider_endpoints e ORDER BY e.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		out = append(out, decodeObject(raw))
	}
	return out, rows.Err()
}

func (a *App) ProviderBindings(ctx context.Context, actorID string) ([]map[string]any, error) {
	if _, err := a.ReadSettings(ctx, actorID); err != nil {
		return nil, err
	}
	rows, err := a.configurationReader(ctx).Query(ctx, `SELECT r.role,r.provider_endpoint_id,r.model_id,r.token_budget,r.timeout_seconds,r.context_window_tokens,r.max_input_tokens,r.prompt_budget_policy_version,e.capability_status FROM public.model_roles r JOIN public.provider_endpoints e ON e.id=r.provider_endpoint_id WHERE r.role IN ('generic_llm','embedding','conversation_reply','vision_understanding') ORDER BY r.role`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]map[string]any, 0)
	for rows.Next() {
		var role, endpoint, model string
		var budget, timeout, contextWindowTokens, maxInputTokens int
		var promptBudgetPolicyVersion string
		var endpointStatus string
		if err := rows.Scan(&role, &endpoint, &model, &budget, &timeout, &contextWindowTokens, &maxInputTokens, &promptBudgetPolicyVersion, &endpointStatus); err != nil {
			return nil, err
		}
		margin, err := promptSafetyMargin(promptBudgetPolicyVersion)
		if err != nil {
			return nil, err
		}
		out = append(out, map[string]any{
			"role": role, "endpoint_id": endpoint, "model_id": model,
			"token_budget": budget, "output_reserve_tokens": budget, "timeout_seconds": timeout,
			"context_window_tokens": contextWindowTokens, "max_input_tokens": maxInputTokens,
			"prompt_budget_policy_version": promptBudgetPolicyVersion, "safety_margin_tokens": margin,
			"endpoint_status": endpointStatus,
		})
	}
	return out, nil
}

func (a *App) ConfigureProviderEndpoint(ctx context.Context, actorID string, payload map[string]any) error {
	if _, err := a.ReadSettings(ctx, actorID); err != nil {
		return err
	}
	id, kind, base, purpose := stringValue(payload["endpoint_id"]), stringValue(payload["kind"]), stringValue(payload["base_url"]), stringValue(payload["secret_purpose"])
	if id == "" || kind == "" || base == "" || purpose == "" {
		return errors.New("provider_endpoint_invalid")
	}
	parsed, err := url.Parse(base)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return errors.New("provider_endpoint_invalid")
	}
	return withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('owner-configuration',0))`); err != nil {
			return err
		}
		resources := []string{"endpoint:" + id}
		rows, err := tx.Query(ctx, `SELECT role FROM public.model_roles WHERE provider_endpoint_id=$1 ORDER BY role`, id)
		if err != nil {
			return err
		}
		for rows.Next() {
			var role string
			if err := rows.Scan(&role); err != nil {
				rows.Close()
				return err
			}
			resources = append(resources, "binding:"+role)
		}
		readErr := rows.Err()
		rows.Close()
		if readErr != nil {
			return readErr
		}
		receipt, replay, err := beginConfigurationCommandTx(ctx, tx, actorID, "provider_endpoint_update", payload, resources)
		if err != nil || replay {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM public.model_roles WHERE provider_endpoint_id=$1`, id); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO public.provider_endpoints (id,kind,base_url,secret_purpose,capability_status) VALUES ($1,$2,$3,$4,'unknown') ON CONFLICT (id) DO UPDATE SET kind=excluded.kind,base_url=excluded.base_url,secret_purpose=excluded.secret_purpose,capability_status='unknown',checked_at=NULL`, id, kind, strings.TrimRight(base, "/"), purpose)
		if err != nil {
			return err
		}
		if receipt != nil {
			return finishConfigurationCommandTx(ctx, tx, actorID, payload, receipt, "provider_endpoint_update")
		}
		return nil
	})
}

func (a *App) ProviderModels(ctx context.Context, actorID, endpointID string) (map[string]any, error) {
	if _, err := a.ReadSettings(ctx, actorID); err != nil {
		return nil, err
	}
	var base, purpose string
	if err := a.DB.Pool().QueryRow(ctx, `SELECT base_url,secret_purpose FROM public.provider_endpoints WHERE id=$1`, endpointID).Scan(&base, &purpose); err != nil {
		return nil, err
	}
	secret, err := a.Provider.secret(ctx, purpose)
	if err != nil {
		return nil, err
	}
	requestCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, requestErr := http.NewRequestWithContext(requestCtx, http.MethodGet, strings.TrimRight(base, "/")+"/models", nil)
	if requestErr != nil {
		return nil, errors.New("provider_endpoint_invalid")
	}
	if secret != "" {
		req.Header.Set("Authorization", "Bearer "+secret)
	}
	client := a.Provider.HTTP
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("provider models returned HTTP %d", resp.StatusCode)
	}
	var data any
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	recognized := false
	switch value := data.(type) {
	case []any:
		recognized = true
	case map[string]any:
		for _, key := range []string{"data", "models"} {
			if _, ok := value[key].([]any); ok {
				recognized = true
			}
		}
	}
	if !recognized {
		return nil, &SurfaceError{Status: 502, Code: "provider_models_invalid", Message: "服务返回了无法识别的模型目录格式"}
	}
	return map[string]any{"endpoint_id": endpointID, "models": providerModelIDs(data)}, nil
}

// providerModelIDs normalizes the model-list envelopes used by the common
// OpenAI-compatible and Ollama-compatible endpoints. The endpoint contract
// remains read-only: an unrecognized or empty envelope produces no model IDs,
// so role activation still refuses to persist an unverifiable model.
func providerModelIDs(data any) []string {
	seen := make(map[string]struct{})
	models := make([]string, 0)
	var visit func(any)
	visit = func(value any) {
		switch current := value.(type) {
		case string:
			addProviderModel(&models, seen, current)
		case []any:
			for _, item := range current {
				visit(item)
			}
		case map[string]any:
			for _, field := range []string{"id", "name", "model"} {
				if candidate := stringValue(current[field]); candidate != "" {
					addProviderModel(&models, seen, candidate)
					return
				}
			}
			for _, field := range []string{"data", "models"} {
				if nested, ok := current[field]; ok {
					visit(nested)
				}
			}
		}
	}
	if root, ok := data.(map[string]any); ok {
		for _, field := range []string{"data", "models"} {
			if nested, exists := root[field]; exists {
				visit(nested)
			}
		}
	} else {
		visit(data)
	}
	sort.Strings(models)
	return models
}

func addProviderModel(models *[]string, seen map[string]struct{}, value string) {
	model := strings.TrimSpace(value)
	if model == "" {
		return
	}
	if _, duplicate := seen[model]; duplicate {
		return
	}
	seen[model] = struct{}{}
	*models = append(*models, model)
}
