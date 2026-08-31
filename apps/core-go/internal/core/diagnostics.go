package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"
)

var diagnosticSecretKeys = map[string]struct{}{
	"token": {}, "password": {}, "secret": {}, "credential": {}, "authorization": {},
	"apikey": {}, "api_key": {}, "cookie": {}, "session": {}, "servicekey": {},
	"rawprompt": {}, "rawresponse": {}, "reasoning": {}, "hiddenreasoning": {},
}

func diagnosticCorrelation(messages []map[string]any, fallback string) string {
	for _, message := range messages {
		for _, key := range []string{"correlation_id", "correlationId", "turn_id", "turnId"} {
			if value := stringValue(message[key]); value != "" {
				return value
			}
		}
	}
	if strings.TrimSpace(fallback) != "" {
		return fallback
	}
	encoded, _ := json.Marshal(messages)
	return "provider:" + stableDigest(string(encoded))
}

func redactDiagnostic(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, child := range typed {
			normalized := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(key, "-", ""), "_", ""))
			if _, secret := diagnosticSecretKeys[normalized]; secret {
				result[key] = "[REDACTED]"
				continue
			}
			if normalized == "perception" || normalized == "appraisal" {
				continue
			}
			result[key] = redactDiagnostic(child)
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for index, child := range typed {
			result[index] = redactDiagnostic(child)
		}
		return result
	default:
		return value
	}
}

func (a *App) recordModelRun(ctx context.Context, role, endpointID, modelID, correlationID string, prompt any, response any, status, errorCode string) {
	if a == nil || a.DB == nil {
		return
	}
	if strings.TrimSpace(correlationID) == "" {
		correlationID = "provider:" + stableDigest(role+":"+modelID+":"+time.Now().UTC().Format(time.RFC3339Nano))
	}
	promptJSON, err := json.Marshal(redactDiagnostic(prompt))
	if err != nil {
		return
	}
	var responseJSON []byte
	if response != nil {
		responseJSON, err = json.Marshal(redactDiagnostic(response))
		if err != nil {
			return
		}
	}
	seed := role + ":" + endpointID + ":" + modelID + ":" + correlationID + ":" + status + ":" + string(promptJSON)
	digest := sha256.Sum256([]byte(seed))
	id := "model_run_" + hex.EncodeToString(digest[:])[:32]
	_, _ = a.DB.Pool().Exec(ctx, `INSERT INTO public.diagnostic_model_runs(id,role,endpoint_id,model_id,prompt,response,status,error_code,correlation_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT(id) DO UPDATE SET response=excluded.response,status=excluded.status,error_code=excluded.error_code`, id, role, nullableString(endpointID), modelID, promptJSON, responseJSON, status, nullableString(errorCode), correlationID)
	_, _ = a.DB.Pool().Exec(ctx, `INSERT INTO public.provider_provenance(id,role,endpoint_id,model_id,prompt_version,schema_version,correlation_id,token_budget) SELECT $1,$2,$3,$4,$5,$6,$7,COALESCE((SELECT token_budget FROM public.model_roles WHERE role=$2),0) ON CONFLICT(id) DO NOTHING`, "provider_provenance_"+hex.EncodeToString(digest[:])[:32], role, endpointID, modelID, "go-provider-v1", "fluctlight."+role+".v1", correlationID)
}

func (a *App) recordDiagnosticEvent(ctx context.Context, eventType, severity, fluctlightID, causationID, correlationID string, payload any) {
	if a == nil || a.DB == nil {
		return
	}
	if correlationID == "" {
		correlationID = eventType + ":" + stableDigest(time.Now().UTC().Format(time.RFC3339Nano))
	}
	encoded, err := json.Marshal(redactDiagnostic(payload))
	if err != nil {
		return
	}
	id := "diagnostic_" + stableDigest(eventType+":"+correlationID+":"+string(encoded))
	_, _ = a.DB.Pool().Exec(ctx, `INSERT INTO public.diagnostic_events(id,event_type,severity,fluctlight_id,causation_id,correlation_id,payload) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(id) DO NOTHING`, id, eventType, severity, nullableString(fluctlightID), nullableString(causationID), correlationID, encoded)
}
