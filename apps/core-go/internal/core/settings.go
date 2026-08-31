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
			if key != "media.comfyui" && key != "product.autonomy" && key != "diagnostics.retention" && key != "media.h3" {
				return fmt.Errorf("unknown setting %s", key)
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
	}
	return out, nil
}

func (a *App) ProviderBindings(ctx context.Context, actorID string) ([]map[string]any, error) {
	if _, err := a.ReadSettings(ctx, actorID); err != nil {
		return nil, err
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT role,provider_endpoint_id,model_id,token_budget,timeout_seconds FROM public.model_roles ORDER BY role`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]map[string]any, 0)
	for rows.Next() {
		var role, endpoint, model string
		var budget, timeout int
		if err := rows.Scan(&role, &endpoint, &model, &budget, &timeout); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"role": role, "endpoint_id": endpoint, "model_id": model, "token_budget": budget, "timeout_seconds": timeout})
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
	_, err := a.DB.Pool().Exec(ctx, `INSERT INTO public.provider_endpoints (id,kind,base_url,secret_purpose,capability_status) VALUES ($1,$2,$3,$4,'unknown') ON CONFLICT (id) DO UPDATE SET kind=excluded.kind,base_url=excluded.base_url,secret_purpose=excluded.secret_purpose,capability_status='unknown'`, id, kind, base, purpose)
	return err
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
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(base, "/")+"/models", nil)
	if secret != "" {
		req.Header.Set("Authorization", "Bearer "+secret)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("provider models returned HTTP %d", resp.StatusCode)
	}
	var data map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	models := make([]string, 0)
	if raw, ok := data["data"].([]any); ok {
		for _, item := range raw {
			switch value := item.(type) {
			case string:
				if strings.TrimSpace(value) != "" {
					models = append(models, value)
				}
			case map[string]any:
				if id := stringValue(value["id"]); id != "" {
					models = append(models, id)
				}
			}
		}
	}
	sort.Strings(models)
	return map[string]any{"endpoint_id": endpointID, "models": models}, nil
}
