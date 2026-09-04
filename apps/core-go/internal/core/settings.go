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
	var owner string
	if err := a.DB.Pool().QueryRow(ctx, `SELECT human_actor_id FROM public.owner_accounts LIMIT 1`).Scan(&owner); err != nil || owner != actorID {
		return nil, errors.New("forbidden")
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT key,value_json FROM public.runtime_settings ORDER BY key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := map[string]any{}
	for rows.Next() {
		var key, raw string
		if err := rows.Scan(&key, &raw); err != nil {
			return nil, err
		}
		var value any
		if json.Unmarshal([]byte(raw), &value) == nil {
			values[key] = value
		}
	}
	secretRows, err := a.DB.Pool().Query(ctx, `SELECT purpose FROM public.setting_secrets ORDER BY purpose`)
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
	return map[string]any{"values": values, "configured_secrets": secrets}, nil
}

func (a *App) UpdateSettings(ctx context.Context, actorID string, payload map[string]any) (map[string]any, error) {
	if _, err := a.ReadSettings(ctx, actorID); err != nil {
		return nil, err
	}
	values := mapValue(payload["values"])
	secrets := mapValue(payload["secrets"])
	clear := arrayValue(payload["clear_secrets"])
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		for key, value := range values {
			if key != "media.comfyui" && key != "product.autonomy" && key != "product.wakeup" && key != "diagnostics.retention" && key != "media.h3" && key != "llm.queue" {
				return fmt.Errorf("unknown setting %s", key)
			}
			if key == "llm.queue" {
				value = normalizeProviderQueueSettings(value)
				if value == nil {
					return errors.New("llm_queue_invalid")
				}
			}
			if _, err := tx.Exec(ctx, `INSERT INTO public.runtime_settings (key,value_json,updated_at) VALUES ($1,$2,$3) ON CONFLICT (key) DO UPDATE SET value_json=excluded.value_json,updated_at=excluded.updated_at`, key, jsonString(value), time.Now().UTC()); err != nil {
				return err
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
		return nil
	})
	if err != nil {
		return nil, err
	}
	return a.ReadSettings(ctx, actorID)
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
	rows, err := a.DB.Pool().Query(ctx, `SELECT id,kind,base_url,secret_purpose,capability_status FROM public.provider_endpoints ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]map[string]any, 0)
	for rows.Next() {
		var id, kind, base, purpose, status string
		if err := rows.Scan(&id, &kind, &base, &purpose, &status); err != nil {
			return nil, err
		}
		var configured bool
		_ = a.DB.Pool().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.setting_secrets WHERE purpose=$1)`, purpose).Scan(&configured)
		out = append(out, map[string]any{"id": id, "kind": kind, "base_url": base, "secret_configured": configured, "capability_status": status})
		rolesRows, roleErr := a.DB.Pool().Query(ctx, `SELECT role FROM public.model_roles WHERE provider_endpoint_id=$1 AND role IN ('generic_llm','embedding') ORDER BY role`, id)
		if roleErr != nil {
			return nil, roleErr
		}
		roles := make([]map[string]any, 0)
		for rolesRows.Next() {
			var role string
			if err := rolesRows.Scan(&role); err != nil {
				rolesRows.Close()
				return nil, err
			}
			var model string
			_ = a.DB.Pool().QueryRow(ctx, `SELECT model_id FROM public.model_roles WHERE role=$1`, role).Scan(&model)
			roles = append(roles, map[string]any{"role": role, "model_id": model})
		}
		rolesRows.Close()
		out[len(out)-1]["roles"] = roles
	}
	return out, nil
}

func (a *App) ProviderBindings(ctx context.Context, actorID string) ([]map[string]any, error) {
	if _, err := a.ReadSettings(ctx, actorID); err != nil {
		return nil, err
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT r.role,r.provider_endpoint_id,r.model_id,r.token_budget,r.timeout_seconds,e.capability_status FROM public.model_roles r JOIN public.provider_endpoints e ON e.id=r.provider_endpoint_id WHERE r.role IN ('generic_llm','embedding') ORDER BY r.role`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]map[string]any, 0)
	for rows.Next() {
		var role, endpoint, model string
		var budget, timeout int
		var endpointStatus string
		if err := rows.Scan(&role, &endpoint, &model, &budget, &timeout, &endpointStatus); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"role": role, "endpoint_id": endpoint, "model_id": model, "token_budget": budget, "timeout_seconds": timeout, "endpoint_status": endpointStatus})
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
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return errors.New("provider_endpoint_invalid")
	}
	return withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM public.model_roles WHERE provider_endpoint_id=$1`, id); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO public.provider_endpoints (id,kind,base_url,secret_purpose,capability_status) VALUES ($1,$2,$3,$4,'unknown') ON CONFLICT (id) DO UPDATE SET kind=excluded.kind,base_url=excluded.base_url,secret_purpose=excluded.secret_purpose,capability_status='unknown',checked_at=NULL`, id, kind, strings.TrimRight(base, "/"), purpose)
		return err
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
	req, _ := http.NewRequestWithContext(requestCtx, http.MethodGet, strings.TrimRight(base, "/")+"/models", nil)
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
