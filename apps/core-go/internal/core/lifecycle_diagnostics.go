package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
)

const lifecycleDiagnosticSchemaVersion = "lifecycle-diagnostic.v1"

var ErrDiagnosticsUnavailable = errors.New("diagnostics_unavailable")

type LifecycleTransition string

const (
	LifecycleTransitionScheduled           LifecycleTransition = "scheduled"
	LifecycleTransitionTriggerReleased     LifecycleTransition = "trigger_released"
	LifecycleTransitionIntentCreated       LifecycleTransition = "intent_created"
	LifecycleTransitionQueued              LifecycleTransition = "queued"
	LifecycleTransitionDispatched          LifecycleTransition = "dispatched"
	LifecycleTransitionAlreadyRunning      LifecycleTransition = "already_running"
	LifecycleTransitionActivityStarted     LifecycleTransition = "activity_started"
	LifecycleTransitionProviderQueued      LifecycleTransition = "provider_queued"
	LifecycleTransitionCompletedActionable LifecycleTransition = "completed_actionable"
	LifecycleTransitionCompletedNoop       LifecycleTransition = "completed_noop"
	LifecycleTransitionRetryScheduled      LifecycleTransition = "retry_scheduled"
	LifecycleTransitionFailed              LifecycleTransition = "failed"
	LifecycleTransitionCancelled           LifecycleTransition = "cancelled"
	LifecycleTransitionNextCycleScheduled  LifecycleTransition = "next_cycle_scheduled"
	LifecycleTransitionOverdue             LifecycleTransition = "overdue"
)

var lifecycleTransitions = map[LifecycleTransition]struct{}{
	LifecycleTransitionScheduled:           {},
	LifecycleTransitionTriggerReleased:     {},
	LifecycleTransitionIntentCreated:       {},
	LifecycleTransitionQueued:              {},
	LifecycleTransitionDispatched:          {},
	LifecycleTransitionAlreadyRunning:      {},
	LifecycleTransitionActivityStarted:     {},
	LifecycleTransitionProviderQueued:      {},
	LifecycleTransitionCompletedActionable: {},
	LifecycleTransitionCompletedNoop:       {},
	LifecycleTransitionRetryScheduled:      {},
	LifecycleTransitionFailed:              {},
	LifecycleTransitionCancelled:           {},
	LifecycleTransitionNextCycleScheduled:  {},
	LifecycleTransitionOverdue:             {},
}

// LifecycleDiagnostic is the bounded cross-runtime transition envelope. It is
// intentionally free of prompts, responses and arbitrary domain snapshots.
type LifecycleDiagnostic struct {
	Surface           string
	Transition        LifecycleTransition
	Severity          string
	FluctlightID      string
	CorrelationID     string
	CausationID       string
	IntentID          string
	WorkflowID        string
	RunID             string
	ActivityType      string
	ActivityID        string
	ProviderAttemptID string
	ProviderRequestID string
	ModelRunID        string
	Stage             string
	Status            string
	ReasonCode        string
	ErrorCategory     string
	ErrorCode         string
	SafeCause         string
	Retryable         bool
	Attempt           int
	MaxAttempts       int
	NextDueAt         time.Time
	OccurredAt        time.Time
	Metadata          map[string]any
}

func (value LifecycleDiagnostic) Validate() error {
	if !validLifecycleToken(value.Surface, 64, true) {
		return errors.New("lifecycle_diagnostic_surface_invalid")
	}
	if _, ok := lifecycleTransitions[value.Transition]; !ok {
		return errors.New("lifecycle_diagnostic_transition_invalid")
	}
	if !validLifecycleIdentity(value.CorrelationID, 128, true) {
		return errors.New("lifecycle_diagnostic_correlation_invalid")
	}
	for _, field := range []struct {
		name     string
		value    string
		maxRunes int
	}{
		{name: "fluctlight", value: value.FluctlightID, maxRunes: 128},
		{name: "causation", value: value.CausationID, maxRunes: 128},
		{name: "intent", value: value.IntentID, maxRunes: 128},
		{name: "workflow", value: value.WorkflowID, maxRunes: 128},
		{name: "run", value: value.RunID, maxRunes: 128},
		{name: "activity_type", value: value.ActivityType, maxRunes: 128},
		{name: "activity", value: value.ActivityID, maxRunes: 128},
		{name: "provider_attempt", value: value.ProviderAttemptID, maxRunes: 128},
		{name: "provider_request", value: value.ProviderRequestID, maxRunes: 128},
		{name: "model_run", value: value.ModelRunID, maxRunes: 128},
	} {
		if !validLifecycleIdentity(field.value, field.maxRunes, false) {
			return fmt.Errorf("lifecycle_diagnostic_%s_invalid", field.name)
		}
	}
	for _, field := range []struct {
		name  string
		value string
	}{
		{name: "stage", value: value.Stage},
		{name: "status", value: value.Status},
		{name: "reason", value: value.ReasonCode},
		{name: "error_category", value: value.ErrorCategory},
		{name: "error_code", value: value.ErrorCode},
	} {
		if strings.TrimSpace(field.value) != "" && !validLifecycleToken(field.value, 128, true) {
			return fmt.Errorf("lifecycle_diagnostic_%s_invalid", field.name)
		}
	}
	if value.Severity != "" && value.Severity != "info" && value.Severity != "warn" && value.Severity != "error" {
		return errors.New("lifecycle_diagnostic_severity_invalid")
	}
	if value.Attempt < 0 || value.MaxAttempts < 0 || (value.MaxAttempts > 0 && value.Attempt > value.MaxAttempts) {
		return errors.New("lifecycle_diagnostic_attempt_invalid")
	}
	return nil
}

func lifecycleDiagnosticPayload(value LifecycleDiagnostic, recordedAt time.Time) (map[string]any, error) {
	if err := value.Validate(); err != nil {
		return nil, err
	}
	if recordedAt.IsZero() {
		recordedAt = time.Now().UTC()
	}
	occurredAt := value.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = recordedAt
	}
	payload := map[string]any{
		"schema_version":   lifecycleDiagnosticSchemaVersion,
		"surface":          value.Surface,
		"transition":       string(value.Transition),
		"status":           value.Status,
		"retryable":        value.Retryable,
		"occurred_at":      occurredAt.UTC().Format(time.RFC3339Nano),
		"first_seen_at":    recordedAt.UTC().Format(time.RFC3339Nano),
		"last_seen_at":     recordedAt.UTC().Format(time.RFC3339Nano),
		"occurrence_count": 1,
	}
	for key, item := range map[string]string{
		"intent_id":           value.IntentID,
		"workflow_id":         value.WorkflowID,
		"run_id":              value.RunID,
		"activity_type":       value.ActivityType,
		"activity_id":         value.ActivityID,
		"provider_attempt_id": value.ProviderAttemptID,
		"provider_request_id": value.ProviderRequestID,
		"model_run_id":        value.ModelRunID,
		"stage":               value.Stage,
		"reason_code":         value.ReasonCode,
		"error_category":      value.ErrorCategory,
		"error_code":          value.ErrorCode,
	} {
		if text := strings.TrimSpace(item); text != "" {
			payload[key] = text
		}
	}
	if cause := boundedLifecycleCause(value.SafeCause); cause != "" {
		payload["safe_cause"] = cause
		payload["first_safe_cause"] = cause
		payload["latest_safe_cause"] = cause
	}
	if code := strings.TrimSpace(value.ErrorCode); code != "" {
		payload["first_error_code"] = code
		payload["latest_error_code"] = code
	}
	if value.Attempt > 0 {
		payload["attempt"] = value.Attempt
	}
	if value.MaxAttempts > 0 {
		payload["max_attempts"] = value.MaxAttempts
	}
	if !value.NextDueAt.IsZero() {
		payload["next_due_at"] = value.NextDueAt.UTC().Format(time.RFC3339Nano)
	}
	if len(value.Metadata) > 0 {
		metadata, err := boundedLifecycleDiagnosticValue(reflect.ValueOf(value.Metadata), 0, map[lifecycleMetadataVisit]struct{}{}, "")
		if err != nil {
			return nil, err
		}
		payload["metadata"] = metadata
	}
	return payload, nil
}

func lifecycleDiagnosticID(value LifecycleDiagnostic) string {
	seed := strings.Join([]string{
		value.Surface,
		string(value.Transition),
		value.CorrelationID,
		value.IntentID,
		value.WorkflowID,
		value.RunID,
		value.ActivityID,
		value.ProviderAttemptID,
		value.Stage,
		value.Status,
		value.ReasonCode,
		fmt.Sprint(value.Attempt),
	}, "|")
	return "diagnostic_" + stableDigest(seed)
}

func (a *App) RecordLifecycleDiagnostic(ctx context.Context, value LifecycleDiagnostic) (string, error) {
	if a == nil || a.DB == nil || a.DB.Pool() == nil {
		return "", ErrDiagnosticsUnavailable
	}
	recordedAt := time.Now().UTC()
	payload, err := lifecycleDiagnosticPayload(value, recordedAt)
	if err != nil {
		return "", err
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal lifecycle diagnostic: %w", err)
	}
	eventID := lifecycleDiagnosticID(value)
	eventType := "lifecycle." + value.Surface + "." + string(value.Transition)
	severity := value.Severity
	if severity == "" {
		severity = "info"
	}
	err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var storedID string
		if err := tx.QueryRow(ctx, `
			INSERT INTO public.diagnostic_events(
				id,event_type,severity,fluctlight_id,causation_id,correlation_id,payload,created_at
			) VALUES($1,$2,$3,$4,$5,$6,$7,$8)
			ON CONFLICT(id) DO UPDATE SET
				severity=excluded.severity,
				payload=public.diagnostic_events.payload || excluded.payload ||
					jsonb_build_object(
						'first_seen_at',COALESCE(public.diagnostic_events.payload->'first_seen_at',to_jsonb(public.diagnostic_events.created_at)),
						'last_seen_at',excluded.payload->'last_seen_at',
						'occurrence_count',COALESCE((public.diagnostic_events.payload->>'occurrence_count')::integer,1)+1,
						'first_error_code',COALESCE(public.diagnostic_events.payload->'first_error_code',public.diagnostic_events.payload->'error_code',excluded.payload->'first_error_code'),
						'latest_error_code',COALESCE(excluded.payload->'latest_error_code',excluded.payload->'error_code',public.diagnostic_events.payload->'latest_error_code'),
						'first_safe_cause',COALESCE(public.diagnostic_events.payload->'first_safe_cause',public.diagnostic_events.payload->'safe_cause',excluded.payload->'first_safe_cause'),
						'latest_safe_cause',COALESCE(excluded.payload->'latest_safe_cause',excluded.payload->'safe_cause',public.diagnostic_events.payload->'latest_safe_cause')
					)
			RETURNING id`,
			eventID, eventType, severity, nullableString(value.FluctlightID),
			nullableString(value.CausationID), value.CorrelationID, encoded, recordedAt,
		).Scan(&storedID); err != nil {
			return fmt.Errorf("persist lifecycle diagnostic event: %w", err)
		}
		if storedID != eventID {
			return errors.New("lifecycle_diagnostic_event_identity_mismatch")
		}
		if strings.TrimSpace(value.WorkflowID) != "" {
			linkID := "workflow_link_" + stableDigest(value.CorrelationID+":"+value.WorkflowID+":"+eventID)
			command, err := tx.Exec(ctx, `
				INSERT INTO public.diagnostic_workflow_links(
					id,correlation_id,workflow_id,intent_id,event_id,created_at
				) VALUES($1,$2,$3,$4,$5,$6)
				ON CONFLICT(id) DO UPDATE SET
					intent_id=COALESCE(excluded.intent_id,public.diagnostic_workflow_links.intent_id),
					event_id=excluded.event_id`,
				linkID, value.CorrelationID, value.WorkflowID,
				nullableString(value.IntentID), eventID, recordedAt,
			)
			if err != nil {
				return fmt.Errorf("persist lifecycle diagnostic workflow link: %w", err)
			}
			if command.RowsAffected() != 1 {
				return errors.New("lifecycle_diagnostic_workflow_link_not_written")
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return eventID, nil
}

var lifecycleDiagnosticWarningState = struct {
	sync.Mutex
	last map[string]time.Time
}{last: map[string]time.Time{}}

func (a *App) RecordLifecycleDiagnosticBestEffort(ctx context.Context, value LifecycleDiagnostic) {
	if _, err := a.RecordLifecycleDiagnostic(ctx, value); err != nil {
		recordDiagnosticPersistenceFailure(
			value.Surface,
			value.Stage,
			value.CorrelationID,
			err,
			"transition", value.Transition,
		)
	}
}

func recordDiagnosticPersistenceFailure(component, stage, correlationID string, err error, attributes ...any) {
	key := component + ":" + stage + ":" + fmt.Sprintf("%T", err)
	now := time.Now().UTC()
	lifecycleDiagnosticWarningState.Lock()
	last := lifecycleDiagnosticWarningState.last[key]
	shouldLog := last.IsZero() || now.Sub(last) >= time.Minute
	if shouldLog {
		lifecycleDiagnosticWarningState.last[key] = now
	}
	lifecycleDiagnosticWarningState.Unlock()
	if !shouldLog {
		return
	}
	fields := []any{
		"component", component,
		"stage", stage,
		"correlation_id", correlationID,
		"error_type", fmt.Sprintf("%T", err),
		"safe_cause", boundedLifecycleCause(err.Error()),
	}
	fields = append(fields, attributes...)
	slog.Warn("Go Core diagnostic persistence failed", fields...)
}

func validLifecycleIdentity(value string, maxRunes int, required bool) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return !required
	}
	return len([]rune(value)) <= maxRunes
}

func validLifecycleToken(value string, maxRunes int, required bool) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return !required
	}
	if len([]rune(value)) > maxRunes {
		return false
	}
	for _, character := range value {
		if (character < 'a' || character > 'z') &&
			(character < '0' || character > '9') &&
			character != '_' && character != '.' && character != '-' {
			return false
		}
	}
	return true
}

func boundedLifecycleCause(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	if value == "" {
		return ""
	}
	lower := strings.ToLower(value)
	for _, secret := range []string{"authorization", "bearer ", "password", "secret", "api_key", "apikey", "cookie", "service_key"} {
		if strings.Contains(lower, secret) {
			return "[REDACTED]"
		}
	}
	return boundedLifecycleString(value)
}

func boundedLifecycleString(value string) string {
	runes := []rune(value)
	if len(runes) > 512 {
		runes = runes[:512]
	}
	return string(runes)
}

type lifecycleMetadataVisit struct {
	typeName reflect.Type
	pointer  uintptr
}

func boundedLifecycleDiagnosticValue(value reflect.Value, depth int, seen map[lifecycleMetadataVisit]struct{}, fieldName string) (any, error) {
	if depth >= 4 {
		return "[TRUNCATED]", nil
	}
	if !value.IsValid() {
		return nil, nil
	}
	for value.Kind() == reflect.Interface {
		if value.IsNil() {
			return nil, nil
		}
		value = value.Elem()
	}
	if lifecycleMetadataSecretField(fieldName) {
		return "[REDACTED]", nil
	}
	if value.Type() == reflect.TypeOf(json.RawMessage{}) || ((value.Kind() == reflect.Slice || value.Kind() == reflect.Array) && value.Type().Elem().Kind() == reflect.Uint8) {
		return "[REDACTED_BINARY]", nil
	}
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return nil, nil
		}
		visit := lifecycleMetadataVisit{typeName: value.Type(), pointer: value.Pointer()}
		if _, exists := seen[visit]; exists {
			return nil, errors.New("lifecycle_diagnostic_metadata_cycle")
		}
		seen[visit] = struct{}{}
		defer delete(seen, visit)
		return boundedLifecycleDiagnosticValue(value.Elem(), depth, seen, fieldName)
	}
	switch value.Kind() {
	case reflect.Map:
		if value.IsNil() {
			return map[string]any{}, nil
		}
		if value.Type().Key().Kind() != reflect.String {
			return "[UNSUPPORTED]", nil
		}
		visit := lifecycleMetadataVisit{typeName: value.Type(), pointer: value.Pointer()}
		if _, exists := seen[visit]; exists {
			return nil, errors.New("lifecycle_diagnostic_metadata_cycle")
		}
		seen[visit] = struct{}{}
		defer delete(seen, visit)
		keys := value.MapKeys()
		sort.Slice(keys, func(left, right int) bool { return keys[left].String() < keys[right].String() })
		if len(keys) > 64 {
			keys = keys[:64]
		}
		result := make(map[string]any, len(keys))
		for _, key := range keys {
			name := key.String()
			if lifecycleMetadataDroppedField(name) {
				continue
			}
			child, err := boundedLifecycleDiagnosticValue(value.MapIndex(key), depth+1, seen, name)
			if err != nil {
				return nil, err
			}
			result[name] = child
		}
		return result, nil
	case reflect.Struct:
		fields := make([]reflect.StructField, 0, value.NumField())
		for index := 0; index < value.NumField(); index++ {
			field := value.Type().Field(index)
			if field.PkgPath == "" {
				fields = append(fields, field)
			}
		}
		if len(fields) > 64 {
			fields = fields[:64]
		}
		result := make(map[string]any, len(fields))
		for _, field := range fields {
			name := field.Name
			if tag := strings.Split(field.Tag.Get("json"), ",")[0]; tag == "-" {
				continue
			} else if tag != "" {
				name = tag
			}
			if lifecycleMetadataDroppedField(name) {
				continue
			}
			child, err := boundedLifecycleDiagnosticValue(value.FieldByIndex(field.Index), depth+1, seen, name)
			if err != nil {
				return nil, err
			}
			result[name] = child
		}
		return result, nil
	case reflect.Slice, reflect.Array:
		if value.Kind() == reflect.Slice && value.IsNil() {
			return []any{}, nil
		}
		if value.Kind() == reflect.Slice {
			visit := lifecycleMetadataVisit{typeName: value.Type(), pointer: value.Pointer()}
			if _, exists := seen[visit]; exists {
				return nil, errors.New("lifecycle_diagnostic_metadata_cycle")
			}
			seen[visit] = struct{}{}
			defer delete(seen, visit)
		}
		length := min(value.Len(), 32)
		result := make([]any, length)
		for index := 0; index < length; index++ {
			child, err := boundedLifecycleDiagnosticValue(value.Index(index), depth+1, seen, "")
			if err != nil {
				return nil, err
			}
			result[index] = child
		}
		return result, nil
	case reflect.String:
		return boundedLifecycleString(value.String()), nil
	case reflect.Bool:
		return value.Bool(), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return value.Int(), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return value.Uint(), nil
	case reflect.Float32, reflect.Float64:
		return value.Float(), nil
	default:
		return "[UNSUPPORTED]", nil
	}
}

func lifecycleMetadataSecretField(fieldName string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(fieldName, "-", ""), "_", ""))
	_, secret := diagnosticSecretKeys[normalized]
	return secret
}

func lifecycleMetadataDroppedField(fieldName string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(fieldName, "-", ""), "_", ""))
	return normalized == "perception" || normalized == "appraisal"
}
