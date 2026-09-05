package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

const (
	providerRunQueued    = "queued"
	providerRunRunning   = "running"
	providerRunCompleted = "completed"
	providerRunFailed    = "failed"
	providerRunCancelled = "cancelled"
	providerRunTimeout   = "timeout"
)

type providerScenarioContextKey struct{}
type providerCorrelationContextKey struct{}
type providerExecutionGuardContextKey struct{}

var (
	errProviderPaused   = errors.New("provider_suppressed_fluctlight_paused")
	errProviderInactive = errors.New("provider_suppressed_fluctlight_inactive")
)

// WithProviderScenario lets a domain operation retain its human-readable
// trigger while sharing the generic_llm provider binding.
func WithProviderScenario(ctx context.Context, scenario string) context.Context {
	return context.WithValue(ctx, providerScenarioContextKey{}, strings.TrimSpace(scenario))
}

// WithProviderCorrelation lets a domain operation connect provider diagnostics
// to its durable source (for example a media intent) instead of deriving a
// correlation from the rendered prompt payload.
func WithProviderCorrelation(ctx context.Context, correlationID string) context.Context {
	return context.WithValue(ctx, providerCorrelationContextKey{}, strings.TrimSpace(correlationID))
}

func providerCorrelation(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	value, _ := ctx.Value(providerCorrelationContextKey{}).(string)
	return strings.TrimSpace(value)
}

// WithProviderExecutionGuard attaches a last-moment domain-state check to a
// queued Provider call. It closes the race where an operation is queued while
// active and the Fluctlight is paused before a worker obtains a slot.
func WithProviderExecutionGuard(ctx context.Context, guard func(context.Context) error) context.Context {
	return context.WithValue(ctx, providerExecutionGuardContextKey{}, guard)
}

func providerExecutionGuard(ctx context.Context) func(context.Context) error {
	if ctx == nil {
		return nil
	}
	guard, _ := ctx.Value(providerExecutionGuardContextKey{}).(func(context.Context) error)
	return guard
}

func (a *App) providerGuardForFluctlight(fluctlightID string) func(context.Context) error {
	return func(ctx context.Context) error {
		var status string
		if err := a.DB.Pool().QueryRow(ctx, `SELECT status FROM public.fluctlights WHERE id=$1`, fluctlightID).Scan(&status); err != nil {
			return err
		}
		if status == "paused" {
			return errProviderPaused
		}
		if status != "active" {
			return errProviderInactive
		}
		return nil
	}
}

func providerBindingRole(role string) string {
	if role == "embedding" {
		return "embedding"
	}
	return "generic_llm"
}

func providerScenario(ctx context.Context, role, schemaName string) string {
	if ctx != nil {
		if value, ok := ctx.Value(providerScenarioContextKey{}).(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	switch schemaName {
	case "wake_up_response":
		return "wake_up"
	case "daily_review_response":
		return "daily_review"
	case "schedule_response":
		return "schedule_generation"
	case "media_quality_acceptance_response":
		return "media_quality_acceptance"
	}
	switch role {
	case "action_realization":
		return "reply"
	case "cognitive_assessment":
		return "cognitive_assessment"
	case "reflection":
		return "reflection"
	case "media_prompt":
		return "media_prompt"
	case "initialization":
		return "initialization"
	case "embedding":
		return "embedding"
	default:
		return role
	}
}

func providerPriority(scenario string) int {
	switch scenario {
	case "reply", "autonomy_reply":
		return 100
	case "cognitive_assessment", "native_cognition", "daily_review", "schedule_generation":
		return 90
	case "media_prompt":
		return 80
	case "media_quality_acceptance":
		return 80
	case "reflection", "wake_up":
		return 70
	case "initialization":
		return 60
	default:
		return 50
	}
}

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
			if normalized == "imageurl" {
				result[key] = map[string]any{"url": "[REDACTED_IMAGE_DATA]"}
				continue
			}
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
	case string:
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(typed)), "data:image/") {
			return "[REDACTED_IMAGE_DATA]"
		}
		return typed
	default:
		return value
	}
}

func (a *App) recordModelRun(ctx context.Context, role, endpointID, modelID, correlationID string, prompt any, response any, status, errorCode string) {
	scenario := providerScenario(ctx, role, "")
	a.recordModelRunLifecycle(ctx, role, endpointID, modelID, correlationID, scenario, providerPriority(scenario), prompt, response, status, errorCode)
}

func (a *App) recordQueuedModelRun(ctx context.Context, role, endpointID, modelID, correlationID, scenario string, priority int, prompt any) string {
	return a.recordModelRunLifecycle(ctx, role, endpointID, modelID, correlationID, scenario, priority, prompt, nil, providerRunQueued, "")
}

func (a *App) updateModelRunState(ctx context.Context, id, status string, runErr error) {
	if a == nil || a.DB == nil || strings.TrimSpace(id) == "" {
		return
	}
	errorCode := ""
	if runErr != nil {
		errorCode = providerRunErrorCode(runErr)
	}
	diagnosticCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, _ = a.DB.Pool().Exec(diagnosticCtx, `UPDATE public.diagnostic_model_runs SET status=$2,error_code=CASE WHEN $3='' OR COALESCE(error_code,'')<>'' THEN error_code ELSE $3 END,started_at=CASE WHEN $2='running' THEN COALESCE(started_at,now()) ELSE started_at END,completed_at=CASE WHEN $2 IN ('completed','failed','cancelled','timeout') THEN COALESCE(completed_at,now()) ELSE completed_at END WHERE id=$1`, id, status, nullableString(errorCode))
}

func providerRunErrorCode(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, errProviderPaused) {
		return "fluctlight_paused"
	}
	if errors.Is(err, errProviderInactive) {
		return "fluctlight_inactive"
	}
	if errors.Is(err, context.Canceled) {
		return "request_cancelled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "request_timeout"
	}
	return "provider_request_failed"
}

func providerSuppressionStatus(err error) (string, bool) {
	if errors.Is(err, errProviderPaused) {
		return "paused", true
	}
	if errors.Is(err, errProviderInactive) {
		return "inactive", true
	}
	return "", false
}

func (a *App) recordModelRunLifecycle(ctx context.Context, role, endpointID, modelID, correlationID, scenario string, priority int, prompt any, response any, status, errorCode string) string {
	if a == nil || a.DB == nil {
		return ""
	}
	if strings.TrimSpace(correlationID) == "" {
		correlationID = "provider:" + stableDigest(role+":"+modelID+":"+time.Now().UTC().Format(time.RFC3339Nano))
	}
	promptJSON, err := json.Marshal(redactDiagnostic(prompt))
	if err != nil {
		return ""
	}
	var responseJSON []byte
	if response != nil {
		responseJSON, err = json.Marshal(redactDiagnostic(response))
		if err != nil {
			return ""
		}
	}
	bindingRole := providerBindingRole(role)
	if strings.TrimSpace(scenario) == "" {
		scenario = providerScenario(ctx, role, "")
	}
	if priority <= 0 {
		priority = providerPriority(scenario)
	}
	seed := bindingRole + ":" + endpointID + ":" + modelID + ":" + scenario + ":" + correlationID + ":" + string(promptJSON)
	digest := sha256.Sum256([]byte(seed))
	id := "model_run_" + hex.EncodeToString(digest[:])[:32]
	startedAt := any(nil)
	completedAt := any(nil)
	if status == providerRunRunning {
		startedAt = time.Now().UTC()
	}
	if status == providerRunCompleted || status == providerRunFailed || status == providerRunCancelled || status == providerRunTimeout {
		completedAt = time.Now().UTC()
	}
	_, _ = a.DB.Pool().Exec(ctx, `INSERT INTO public.diagnostic_model_runs(id,role,binding_role,scenario,priority,endpoint_id,model_id,prompt,response,status,error_code,correlation_id,queued_at,started_at,completed_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,now(),$13,$14) ON CONFLICT(id) DO UPDATE SET role=excluded.role,binding_role=excluded.binding_role,scenario=excluded.scenario,priority=excluded.priority,response=COALESCE(excluded.response,public.diagnostic_model_runs.response),status=excluded.status,error_code=excluded.error_code,started_at=COALESCE(public.diagnostic_model_runs.started_at,excluded.started_at),completed_at=COALESCE(excluded.completed_at,public.diagnostic_model_runs.completed_at)`, id, role, bindingRole, scenario, priority, nullableString(endpointID), modelID, promptJSON, responseJSON, status, nullableString(errorCode), correlationID, startedAt, completedAt)
	_, _ = a.DB.Pool().Exec(ctx, `INSERT INTO public.provider_provenance(id,role,endpoint_id,model_id,prompt_version,schema_version,correlation_id,token_budget) SELECT $1,$2,$3,$4,$5,$6,$7,COALESCE((SELECT token_budget FROM public.model_roles WHERE role=$2),0) ON CONFLICT(id) DO NOTHING`, "provider_provenance_"+hex.EncodeToString(digest[:])[:32], bindingRole, endpointID, modelID, "go-provider-v1", "fluctlight."+scenario+".v1", correlationID)
	return id
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
