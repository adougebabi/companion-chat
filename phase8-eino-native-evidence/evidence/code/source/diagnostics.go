package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
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
type providerAttemptIdentityContextKey struct{}
type providerExecutionGuardContextKey struct{}
type providerPromptDiagnosticsContextKey struct{}

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

// WithProviderAttemptIdentity binds every diagnostic write made by one
// Provider invocation to the same attempt. Callers with a durable execution
// attempt (for example a Temporal Activity) may provide it explicitly; the
// Provider boundary creates one otherwise.
func WithProviderAttemptIdentity(ctx context.Context, attemptID string) context.Context {
	return context.WithValue(ctx, providerAttemptIdentityContextKey{}, strings.TrimSpace(attemptID))
}

func providerAttemptIdentity(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	value, _ := ctx.Value(providerAttemptIdentityContextKey{}).(string)
	return strings.TrimSpace(value)
}

func ensureProviderAttemptIdentity(ctx context.Context) context.Context {
	if providerAttemptIdentity(ctx) != "" {
		return ctx
	}
	return WithProviderAttemptIdentity(ctx, randomID("provider_attempt_"))
}

func WithPromptDiagnostics(ctx context.Context, trace map[string]any) context.Context {
	return context.WithValue(ctx, providerPromptDiagnosticsContextKey{}, boundedPromptDiagnostics(trace))
}

func providerPromptDiagnostics(ctx context.Context) map[string]any {
	if ctx == nil {
		return map[string]any{}
	}
	value, _ := ctx.Value(providerPromptDiagnosticsContextKey{}).(map[string]any)
	result := cloneMap(value)
	if result == nil {
		return map[string]any{}
	}
	return result
}

func boundedPromptDiagnostics(trace map[string]any) map[string]any {
	encoded, err := json.Marshal(trace)
	if err != nil {
		return map[string]any{}
	}
	var normalized map[string]any
	if json.Unmarshal(encoded, &normalized) != nil {
		return map[string]any{}
	}
	redacted, _ := redactDiagnostic(normalized).(map[string]any)
	if redacted == nil {
		return map[string]any{}
	}
	var bound func(any) any
	bound = func(value any) any {
		switch typed := value.(type) {
		case map[string]any:
			result := make(map[string]any, len(typed))
			for key, child := range typed {
				result[key] = bound(child)
			}
			return result
		case []any:
			if len(typed) > 64 {
				typed = typed[:64]
			}
			result := make([]any, len(typed))
			for index, child := range typed {
				result[index] = bound(child)
			}
			return result
		default:
			return value
		}
	}
	bounded, _ := bound(redacted).(map[string]any)
	return bounded
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
	case takeoverJudgeRole:
		// The Judge sits on the critical path of a user-visible reply, so it
		// gets its own scenario (and therefore its own metering/priority)
		// instead of being folded into cognitive_assessment.
		return "takeover_judge"
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
	case "reply", "autonomy_reply", "cognitive_assessment", "takeover_judge":
		return 100
	case "native_cognition", "daily_review", "schedule_generation":
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
	newProviderRuntimeSupport(a.DB).RecordModelRun(ctx, role, endpointID, modelID, correlationID, prompt, response, status, errorCode)
}

func (a *App) recordQueuedModelRun(ctx context.Context, role, endpointID, modelID, correlationID, scenario string, priority int, prompt any) string {
	return newProviderRuntimeSupport(a.DB).RecordQueuedModelRun(ctx, role, endpointID, modelID, correlationID, scenario, priority, prompt)
}

func (a *App) updateModelRunState(ctx context.Context, id, status string, runErr error) {
	newProviderRuntimeSupport(a.DB).UpdateModelRunState(ctx, id, status, runErr)
}

func (a *App) updateModelRunPromptMetrics(ctx context.Context, id string, usage map[string]any, latency time.Duration) {
	newProviderRuntimeSupport(a.DB).UpdateModelRunPromptMetrics(ctx, id, usage, latency)
}

func (a *App) recordDiagnosticEvent(ctx context.Context, eventType, severity, fluctlightID, causationID, correlationID string, payload any) {
	newProviderRuntimeSupport(a.DB).RecordDiagnosticEvent(ctx, eventType, severity, fluctlightID, causationID, correlationID, payload)
}

func (a *App) persistModelRunLifecycle(ctx context.Context, role, endpointID, modelID, correlationID, scenario string, priority int, prompt any, response any, status, errorCode string) (string, error) {
	return newProviderRuntimeSupport(a.DB).PersistModelRunLifecycle(ctx, role, endpointID, modelID, correlationID, scenario, priority, prompt, response, status, errorCode)
}

func (s providerRuntimeSupport) RecordModelRun(ctx context.Context, role, endpointID, modelID, correlationID string, prompt any, response any, status, errorCode string) {
	scenario := providerScenario(ctx, role, "")
	if _, err := s.PersistModelRunLifecycle(ctx, role, endpointID, modelID, correlationID, scenario, providerPriority(scenario), prompt, response, status, errorCode); err != nil {
		recordDiagnosticPersistenceFailure("model_run", "create", correlationID, err)
	}
}

func (s providerRuntimeSupport) RecordQueuedModelRun(ctx context.Context, role, endpointID, modelID, correlationID, scenario string, priority int, prompt any) string {
	id, err := s.PersistModelRunLifecycle(ctx, role, endpointID, modelID, correlationID, scenario, priority, prompt, nil, providerRunQueued, "")
	if err != nil {
		recordDiagnosticPersistenceFailure("model_run", "queue", correlationID, err)
		return ""
	}
	if scenario == "wake_up" || scenario == "reflection" {
		s.RecordLifecycleDiagnosticBestEffort(ctx, LifecycleDiagnostic{
			Surface: scenario, Transition: LifecycleTransitionProviderQueued,
			FluctlightID:  strings.TrimSpace(stringValue(providerPromptDiagnostics(ctx)["fluctlight_id"])),
			CorrelationID: correlationID, ProviderRequestID: providerDiagnosticRequestID(role, correlationID),
			ProviderAttemptID: providerAttemptIdentity(ctx), ModelRunID: id,
			Stage: "provider_queue", Status: providerRunQueued, ReasonCode: "provider_request_queued",
		})
	}
	return id
}

func (s providerRuntimeSupport) UpdateModelRunState(ctx context.Context, id, status string, runErr error) {
	if s.DB == nil || strings.TrimSpace(id) == "" {
		return
	}
	errorCode := ""
	if runErr != nil {
		errorCode = providerRunErrorCode(runErr)
	}
	diagnosticCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	command, err := s.DB.Pool().Exec(diagnosticCtx, `UPDATE public.diagnostic_model_runs SET status=CASE WHEN status IN ('completed','failed','cancelled','timeout') AND $2 IN ('completed','failed','cancelled','timeout') THEN status ELSE $2 END,error_code=CASE WHEN $3='' OR COALESCE(error_code,'')<>'' THEN error_code ELSE $3 END,started_at=CASE WHEN $2='running' THEN COALESCE(started_at,now()) ELSE started_at END,completed_at=CASE WHEN $2 IN ('completed','failed','cancelled','timeout') THEN COALESCE(completed_at,now()) ELSE completed_at END WHERE id=$1 AND ((status='queued' AND $2 IN ('running','completed','failed','cancelled','timeout')) OR (status='running' AND $2 IN ('completed','failed','cancelled','timeout')) OR (status IN ('completed','failed','cancelled','timeout') AND $2 IN ('completed','failed','cancelled','timeout')) OR status=$2)`, id, status, nullableString(errorCode))
	if err != nil || command.RowsAffected() != 1 {
		if err == nil {
			err = errors.New("diagnostic_model_run_state_not_written")
		}
		recordDiagnosticPersistenceFailure("model_run", "state", id, err)
	}
}

func providerRunErrorCode(err error) string {
	if err == nil {
		return ""
	}
	// Tool-call normalization is a protocol failure, rather than a generic
	// transport/provider failure. Keep this classification on the queue callback
	// path as well as the immediate Provider response path so the terminal model
	// run cannot be relabelled as `provider_request_failed` by a late state update.
	if providerToolCallInvalidError(err) {
		return "tool_call_invalid"
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

// ProviderErrorInfo returns the bounded, operator-safe classification for a
// Provider failure. The second value is populated only for the closed set of
// tool-call normalization reasons; it never contains model-controlled IDs,
// names, argument text, or the original error string.
func ProviderErrorInfo(err error) (code, reason string) {
	code = providerRunErrorCode(err)
	if !providerToolCallInvalidError(err) {
		return code, ""
	}
	var normalizationErr *providerToolCallNormalizationError
	if !errors.As(err, &normalizationErr) || normalizationErr == nil {
		return code, ""
	}
	return code, providerToolCallNormalizationReason(normalizationErr.Reason)
}

// ProviderErrorCode is the code-only form used by callers that do not need a
// detailed normalization reason (for example public workflow logs).
func ProviderErrorCode(err error) string {
	code, _ := ProviderErrorInfo(err)
	return code
}

func providerToolCallInvalidError(err error) bool {
	if err == nil {
		return false
	}
	var normalizationErr *providerToolCallNormalizationError
	if errors.As(err, &normalizationErr) && normalizationErr != nil {
		return true
	}
	return strings.TrimSpace(err.Error()) == "tool_call_invalid"
}

func providerToolCallNormalizationReason(reason string) string {
	switch strings.TrimSpace(reason) {
	case "item_not_object", "unsupported_type", "id_required", "name_invalid", "duplicate_id",
		"arguments_required", "arguments_empty", "arguments_oversized", "arguments_invalid_json",
		"arguments_not_object":
		return strings.TrimSpace(reason)
	default:
		return ""
	}
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

func (s providerRuntimeSupport) PersistModelRunLifecycle(ctx context.Context, role, endpointID, modelID, correlationID, scenario string, priority int, prompt any, response any, status, errorCode string) (string, error) {
	if s.DB == nil || s.DB.Pool() == nil {
		return "", ErrDiagnosticsUnavailable
	}
	if strings.TrimSpace(correlationID) == "" {
		correlationID = "provider:" + stableDigest(role+":"+modelID+":"+time.Now().UTC().Format(time.RFC3339Nano))
	}
	promptJSON, err := json.Marshal(redactDiagnostic(prompt))
	if err != nil {
		return "", fmt.Errorf("marshal diagnostic model-run prompt: %w", err)
	}
	var responseJSON []byte
	if response != nil {
		responseJSON, err = json.Marshal(redactDiagnostic(response))
		if err != nil {
			return "", fmt.Errorf("marshal diagnostic model-run response: %w", err)
		}
	}
	bindingRole := providerBindingRole(role)
	if strings.TrimSpace(scenario) == "" {
		scenario = providerScenario(ctx, role, "")
	}
	if priority <= 0 {
		priority = providerPriority(scenario)
	}
	metrics := providerPromptDiagnostics(ctx)
	if strings.TrimSpace(stringValue(metrics["run_id"])) == "" {
		// A physical model run inherits the parent correlation when no durable
		// workflow run identity was supplied. This keeps run_id exportable without
		// manufacturing a second random correlation namespace.
		metrics["run_id"] = correlationID
	}
	attemptID := providerAttemptIdentity(ctx)
	if attemptID == "" {
		// Legacy/internal diagnostic callers that do not pass through Provider
		// retain deterministic identity. Real Provider invocations always set an
		// explicit attempt before their queued write.
		attemptID = "legacy"
	}
	metrics["provider_attempt_id"] = attemptID
	fluctlightID := strings.TrimSpace(stringValue(metrics["fluctlight_id"]))
	if fluctlightID == "" {
		fluctlightID = s.inferDiagnosticFluctlightID(ctx, correlationID)
	}
	estimatedInputTokens := intValue(mapValue(metrics["prompt_budget"])["estimated_input_tokens"])
	id, digest := providerModelRunIdentity(bindingRole, endpointID, modelID, scenario, correlationID, attemptID, promptJSON)
	startedAt := any(nil)
	completedAt := any(nil)
	if status == providerRunRunning {
		startedAt = time.Now().UTC()
	}
	if status == providerRunCompleted || status == providerRunFailed || status == providerRunCancelled || status == providerRunTimeout {
		completedAt = time.Now().UTC()
	}
	err = withTransaction(ctx, s.DB.Pool(), func(tx pgx.Tx) error {
		command, err := tx.Exec(ctx, `INSERT INTO public.diagnostic_model_runs(id,role,binding_role,scenario,priority,endpoint_id,model_id,prompt,response,status,error_code,correlation_id,fluctlight_id,metrics,estimated_input_tokens,queued_at,started_at,completed_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,now(),$16,$17) ON CONFLICT(id) DO UPDATE SET role=excluded.role,binding_role=excluded.binding_role,scenario=excluded.scenario,priority=excluded.priority,response=COALESCE(excluded.response,public.diagnostic_model_runs.response),status=CASE WHEN public.diagnostic_model_runs.status IN ('completed','failed','cancelled','timeout') THEN public.diagnostic_model_runs.status WHEN public.diagnostic_model_runs.status='running' AND excluded.status='queued' THEN public.diagnostic_model_runs.status ELSE excluded.status END,error_code=CASE WHEN public.diagnostic_model_runs.status IN ('completed','failed','cancelled','timeout') THEN public.diagnostic_model_runs.error_code ELSE excluded.error_code END,fluctlight_id=COALESCE(public.diagnostic_model_runs.fluctlight_id,excluded.fluctlight_id),metrics=CASE WHEN public.diagnostic_model_runs.status IN ('completed','failed','cancelled','timeout') OR excluded.metrics='{}'::jsonb THEN public.diagnostic_model_runs.metrics ELSE excluded.metrics END,estimated_input_tokens=COALESCE(public.diagnostic_model_runs.estimated_input_tokens,excluded.estimated_input_tokens),started_at=COALESCE(public.diagnostic_model_runs.started_at,excluded.started_at),completed_at=COALESCE(public.diagnostic_model_runs.completed_at,excluded.completed_at)`, id, role, bindingRole, scenario, priority, nullableString(endpointID), modelID, promptJSON, responseJSON, status, nullableString(errorCode), correlationID, nullableString(fluctlightID), jsonBytes(metrics), nullableInt(estimatedInputTokens, estimatedInputTokens > 0), startedAt, completedAt)
		if err != nil {
			return fmt.Errorf("persist diagnostic model run: %w", err)
		}
		if command.RowsAffected() != 1 {
			return errors.New("diagnostic_model_run_not_written")
		}
		command, err = tx.Exec(ctx, `INSERT INTO public.provider_provenance(id,role,endpoint_id,model_id,prompt_version,schema_version,correlation_id,token_budget) SELECT $1,$2::varchar(64),$3,$4,$5,$6,$7,COALESCE((SELECT token_budget FROM public.model_roles WHERE role=$2::varchar(64)),0) ON CONFLICT(id) DO UPDATE SET correlation_id=excluded.correlation_id`, "provider_provenance_"+hex.EncodeToString(digest[:])[:32], bindingRole, endpointID, modelID, providerPromptVersion(scenario), providerSchemaVersion(scenario), correlationID)
		if err != nil {
			return fmt.Errorf("persist diagnostic provider provenance: %w", err)
		}
		if command.RowsAffected() != 1 {
			return errors.New("diagnostic_provider_provenance_not_written")
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return id, nil
}

func providerModelRunIdentity(bindingRole, endpointID, modelID, scenario, correlationID, attemptID string, promptJSON []byte) (string, [sha256.Size]byte) {
	seed, _ := json.Marshal([]string{bindingRole, endpointID, modelID, scenario, correlationID, attemptID, string(promptJSON)})
	digest := sha256.Sum256(seed)
	return "model_run_" + hex.EncodeToString(digest[:])[:32], digest
}

func providerDiagnosticRequestID(role, correlationID string) string {
	return "provider:" + stableDigest(strings.TrimSpace(role)+":"+strings.TrimSpace(correlationID))
}

func (s providerRuntimeSupport) inferDiagnosticFluctlightID(ctx context.Context, correlationID string) string {
	if s.DB == nil {
		return ""
	}
	var fluctlightID string
	if intentID := strings.TrimPrefix(correlationID, "conversation-summary:"); intentID != correlationID {
		_ = s.DB.Pool().QueryRow(ctx, `SELECT COALESCE(payload->>'fluctlight_id','') FROM public.platform_workflow_intents WHERE intent_id=$1 AND intent_type='conversation.summary'`, intentID).Scan(&fluctlightID)
		return strings.TrimSpace(fluctlightID)
	}
	if frozenID := strings.TrimPrefix(correlationID, "query-continuation:"); frozenID != correlationID {
		_ = s.DB.Pool().QueryRow(ctx, `SELECT fluctlight_id FROM public.cognition_frozen_actions WHERE id=$1`, frozenID).Scan(&fluctlightID)
	}
	return strings.TrimSpace(fluctlightID)
}

func normalizeProviderUsage(envelope map[string]any) map[string]any {
	usage := mapValue(envelope["usage"])
	result := map[string]any{}
	for _, key := range []string{"prompt_tokens", "completion_tokens", "total_tokens"} {
		if value := intValue(usage[key]); value >= 0 && usage[key] != nil {
			result[key] = value
		}
	}
	return result
}

func (s providerRuntimeSupport) UpdateModelRunPromptMetrics(ctx context.Context, id string, usage map[string]any, latency time.Duration) {
	if s.DB == nil || strings.TrimSpace(id) == "" {
		return
	}
	promptTokens, promptPresent := usage["prompt_tokens"]
	completionTokens, completionPresent := usage["completion_tokens"]
	metrics := map[string]any{"provider_usage": usage, "latency_ms": latency.Milliseconds()}
	diagnosticCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	command, err := s.DB.Pool().Exec(diagnosticCtx, `UPDATE public.diagnostic_model_runs SET actual_prompt_tokens=$2,actual_completion_tokens=$3,latency_ms=$4,metrics=metrics || $5::jsonb || CASE WHEN $2::integer IS NULL OR estimated_input_tokens IS NULL THEN '{}'::jsonb ELSE jsonb_build_object('estimator_delta_tokens',$2-estimated_input_tokens) END WHERE id=$1`, id, nullableInt(intValue(promptTokens), promptPresent), nullableInt(intValue(completionTokens), completionPresent), maxInt64(0, latency.Milliseconds()), jsonBytes(metrics))
	if err != nil || command.RowsAffected() != 1 {
		if err == nil {
			err = errors.New("diagnostic_model_run_metrics_not_written")
		}
		recordDiagnosticPersistenceFailure("model_run", "metrics", id, err)
	}
}

func maxInt64(left, right int64) int64 {
	if left > right {
		return left
	}
	return right
}

func providerPromptVersion(scenario string) string {
	if scenario == "reflection" {
		return "reflection.v2"
	}
	return "go-provider-v1"
}

func providerSchemaVersion(scenario string) string {
	if scenario == "reflection" {
		return reflectionProposalV2SchemaVersion
	}
	return "fluctlight." + scenario + ".v1"
}

func (s providerRuntimeSupport) RecordDiagnosticEvent(ctx context.Context, eventType, severity, fluctlightID, causationID, correlationID string, payload any) {
	if _, err := s.persistDiagnosticEvent(ctx, eventType, severity, fluctlightID, causationID, correlationID, payload); err != nil {
		recordDiagnosticPersistenceFailure("event", "create", correlationID, err, "event_type", eventType)
	}
}

func (s providerRuntimeSupport) persistDiagnosticEvent(ctx context.Context, eventType, severity, fluctlightID, causationID, correlationID string, payload any) (string, error) {
	if s.DB == nil || s.DB.Pool() == nil {
		return "", ErrDiagnosticsUnavailable
	}
	if correlationID == "" {
		correlationID = eventType + ":" + stableDigest(time.Now().UTC().Format(time.RFC3339Nano))
	}
	encoded, err := json.Marshal(redactDiagnostic(payload))
	if err != nil {
		return "", fmt.Errorf("marshal diagnostic event: %w", err)
	}
	id := "diagnostic_" + stableDigest(eventType+":"+correlationID+":"+string(encoded))
	var storedID string
	if err := s.DB.Pool().QueryRow(ctx, `INSERT INTO public.diagnostic_events(id,event_type,severity,fluctlight_id,causation_id,correlation_id,payload) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(id) DO UPDATE SET id=excluded.id RETURNING id`, id, eventType, severity, nullableString(fluctlightID), nullableString(causationID), correlationID, encoded).Scan(&storedID); err != nil {
		return "", fmt.Errorf("persist diagnostic event: %w", err)
	}
	if storedID != id {
		return "", errors.New("diagnostic_event_identity_mismatch")
	}
	return id, nil
}
