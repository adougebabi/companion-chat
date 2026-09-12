package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func (a *App) capabilityRegistry() *CapabilityRegistry {
	if a != nil && a.Capabilities != nil {
		return a.Capabilities
	}
	registry, _ := NewCapabilityRegistry(builtinCapabilities(a)...)
	return registry
}

func (a *App) capabilityRuntime() *CapabilityRuntime {
	if a != nil && a.Runtime != nil {
		return a.Runtime
	}
	registry := a.capabilityRegistry()
	if a == nil || a.ContextResolver == nil {
		return nil
	}
	resolver := a.ContextResolver
	runtime, err := NewCapabilityRuntime(registry, resolver)
	if err != nil {
		return nil
	}
	if a != nil {
		a.Runtime = runtime
	}
	return runtime
}

func capabilityCatalog(registry *CapabilityRegistry, surface CapabilitySurface) []CapabilityDefinition {
	if registry == nil {
		return nil
	}
	return registry.Catalog(surface)
}

// ExecuteCapabilities is the standalone planning/query entry point. It never
// self-commits a transactional Capability; callers that own an action use the
// frozen plan plus settleDeferredCapabilitiesTx in their Unit of Work.
func (a *App) ExecuteCapabilities(ctx context.Context, fluctlightID, conversationID, sourceFactID string, invocations []CapabilityInvocation) ([]CapabilityResult, error) {
	return a.executeCapabilities(ctx, fluctlightID, conversationID, sourceFactID, invocations, nil, true, true, true)
}

func (a *App) planCapabilitiesForTransaction(ctx context.Context, fluctlightID, conversationID, sourceFactID string, invocations []CapabilityInvocation, existing []CapabilityResult) ([]CapabilityResult, error) {
	// Transaction plans are built only after the caller has frozen and
	// persisted Capability Prepare output. Re-running the preparer here would
	// make planner I/O depend on the settlement attempt instead of the frozen
	// replay boundary.
	return a.executeCapabilities(ctx, fluctlightID, conversationID, sourceFactID, invocations, existing, true, false, false)
}

func (a *App) executeCapabilities(ctx context.Context, fluctlightID, conversationID, sourceFactID string, invocations []CapabilityInvocation, existing []CapabilityResult, deferTransactional, persistStandaloneOutcomes, prepareInvocations bool) ([]CapabilityResult, error) {
	if len(invocations) == 0 {
		return []CapabilityResult{}, nil
	}
	runtime := a.capabilityRuntime()
	if runtime == nil {
		results := make([]CapabilityResult, 0, len(invocations))
		for _, invocation := range invocations {
			results = append(results, failedCapabilityResultDetail(invocation, "capability_not_found", false, "capability runtime is unavailable"))
		}
		return results, ErrCapabilityNotFound
	}
	registry := runtime.Registry
	results := append([]CapabilityResult(nil), existing...)
	var firstErr error
	for index := range invocations {
		invocation := normalizeCapabilityInvocationMetadata(invocations[index], fluctlightID, conversationID, sourceFactID, sourceFactID, index)
		invocations[index] = invocation
		if existingResult, found := capabilityResultForCall(results, invocation.CallID); found && existingResult.Status == "completed" {
			if existingResult.CapabilityName != invocation.CapabilityName {
				return results, fmt.Errorf("capability result identity mismatch for call %q", invocation.CallID)
			}
			continue
		}
		definition, ok := registry.Definition(invocation.CapabilityName)
		if !ok {
			result := failedCapabilityResultDetail(invocation, "capability_not_found", false, invocation.CapabilityName)
			results = replaceCapabilityResult(results, result)
			if firstErr == nil {
				firstErr = fmt.Errorf("%w: %s", ErrCapabilityNotFound, invocation.CapabilityName)
			}
			continue
		}
		capability, ok := registry.LookupCapability(invocation.CapabilityName)
		if !ok {
			result := failedCapabilityResultDetail(invocation, "capability_not_found", false, "implementation is not registered")
			results = replaceCapabilityResult(results, result)
			if firstErr == nil {
				firstErr = fmt.Errorf("%w: %s", ErrCapabilityNotFound, invocation.CapabilityName)
			}
			continue
		}
		executionClass, classErr := classifyCapabilityExecution(capability, definition)
		if classErr != nil {
			result := failedCapabilityResultDetail(invocation, "capability_execution_class_invalid", false, classErr.Error())
			results = replaceCapabilityResult(results, result)
			if firstErr == nil && definition.FailurePolicy == FailurePolicyRequiredForVisibleClaim {
				firstErr = classErr
			}
			continue
		}
		if prepareInvocations {
			if len(invocation.PreparedPayload) > 0 {
				if _, prepareErr := decodeCapabilityPreparedPayload(invocation.PreparedPayload); prepareErr != nil {
					result := failedCapabilityResultDetail(invocation, "capability_prepared_payload_invalid", false, prepareErr.Error())
					results = replaceCapabilityResult(results, result)
					if firstErr == nil && definition.FailurePolicy == FailurePolicyRequiredForVisibleClaim {
						firstErr = prepareErr
					}
					continue
				}
			} else {
				prepared, _, prepareErr := runtime.Prepare(ctx, invocation)
				if prepareErr != nil {
					code, retryable := capabilityErrorInfo(prepareErr, "capability_prepare_failed", true)
					result := failedCapabilityResultDetail(invocation, code, retryable, prepareErr.Error())
					results = replaceCapabilityResult(results, result)
					if firstErr == nil && definition.FailurePolicy == FailurePolicyRequiredForVisibleClaim {
						firstErr = prepareErr
					}
					continue
				}
				invocation = prepared
				invocations[index] = prepared
			}
		}
		_, transactional := capability.(TransactionalCapability)
		if deferTransactional && (executionClass == CapabilityExecutionTransactionalMutation || (executionClass == CapabilityExecutionExternalAsyncIntent && transactional)) {
			results = replaceCapabilityResult(results, CapabilityResult{CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: "deferred", Output: map[string]any{"reason": "transaction_pending"}, Retryable: true, ProviderRequestID: invocation.ProviderRequestID, CorrelationID: "capability:" + invocation.CallID, RequiredContext: append([]ContextSlot(nil), definition.RequiredContext...)})
			continue
		}
		if executionClass == CapabilityExecutionDeferredOutput || (executionClass == CapabilityExecutionExternalAsyncIntent && definition.IsDeferredOutput()) {
			results = replaceCapabilityResult(results, CapabilityResult{CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: "deferred", Output: map[string]any{"reason": "output_target_pending"}, Retryable: true, ProviderRequestID: invocation.ProviderRequestID, CorrelationID: "capability:" + invocation.CallID, RequiredContext: append([]ContextSlot(nil), definition.RequiredContext...)})
			continue
		}
		var result CapabilityResult
		var err error
		if definition.ConcurrencyClass == "exclusive" && a != nil && a.DB != nil {
			result, err = a.executeExclusiveCapabilityCanonical(ctx, fluctlightID, invocation, capability)
		} else {
			result, err = runtime.Execute(ctx, invocation)
		}
		results = replaceCapabilityResult(results, result)
		if err != nil && firstErr == nil && definition.FailurePolicy == FailurePolicyRequiredForVisibleClaim {
			firstErr = err
		}
	}
	if persistStandaloneOutcomes {
		if persistErr := a.persistStandaloneCapabilityOutcomes(ctx, fluctlightID, invocations, results, firstErr); persistErr != nil && firstErr == nil {
			firstErr = persistErr
		}
	}
	return results, firstErr
}

func (a *App) prepareCapabilityInvocations(ctx context.Context, fluctlightID, conversationID, sourceFactID string, invocations []CapabilityInvocation, existing ...[]CapabilityResult) ([]CapabilityInvocation, error) {
	prepared := append([]CapabilityInvocation(nil), invocations...)
	runtime := a.capabilityRuntime()
	if runtime == nil {
		return nil, ErrCapabilityNotFound
	}
	for index := range prepared {
		prepared[index] = normalizeCapabilityInvocationMetadata(prepared[index], fluctlightID, conversationID, sourceFactID, sourceFactID, index)
		if len(existing) > 0 {
			if result, found := capabilityResultForCall(existing[0], prepared[index].CallID); found && result.Status == "completed" {
				continue
			}
		}
		_, ok := runtime.Registry.Definition(prepared[index].CapabilityName)
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrCapabilityNotFound, prepared[index].CapabilityName)
		}
		if len(prepared[index].PreparedPayload) > 0 {
			if _, err := decodeCapabilityPreparedPayload(prepared[index].PreparedPayload); err != nil {
				return nil, fmt.Errorf("%w: %v", ErrInvalidArguments, err)
			}
			continue
		}
		invocation, _, err := runtime.Prepare(ctx, prepared[index])
		if err != nil {
			return nil, err
		}
		prepared[index] = invocation
	}
	return prepared, nil
}

// validateCapabilityInvocationsForPersistence performs the cheap, deterministic
// checks that are safe before an action is durable. Capability preflight and
// planner I/O intentionally stay on the action worker: a transient provider or
// renderer failure must not discard the WakeUp decision before its retryable
// action/failure record exists.
func (a *App) validateCapabilityInvocationsForPersistence(invocations []CapabilityInvocation) error {
	registry := a.capabilityRegistry()
	if registry == nil {
		return ErrCapabilityNotFound
	}
	for _, invocation := range invocations {
		definition, ok := registry.Definition(invocation.CapabilityName)
		if !ok {
			return fmt.Errorf("%w: %s", ErrCapabilityNotFound, invocation.CapabilityName)
		}
		if err := invocation.Validate(definition); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) bindCapabilityInvocationsToProjection(invocations []CapabilityInvocation, projection ContextProjection, actionID, sourceFactID string, surface CapabilitySurface) ([]CapabilityInvocation, error) {
	bound := append([]CapabilityInvocation(nil), invocations...)
	registry := a.capabilityRegistry()
	for index := range bound {
		bound[index] = normalizeCapabilityInvocationMetadata(bound[index], projection.FluctlightID, projection.ConversationID, sourceFactID, sourceFactID, index)
		bound[index].ActionID = actionID
		bound[index].Metadata.Surface = surface
		definition, ok := registry.Definition(bound[index].CapabilityName)
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrCapabilityNotFound, bound[index].CapabilityName)
		}
		bound[index].ContextSnapshot = capabilitySnapshotForProjection(projection, definition.RequiredContext, actionID)
	}
	return bound, nil
}

func normalizeCapabilityInvocationMetadata(invocation CapabilityInvocation, fluctlightID, conversationID, sourceFactID, identityScope string, sequence int) CapabilityInvocation {
	if invocation.CallID == "" {
		return invocation
	}
	if invocation.SourceFactID == "" {
		invocation.SourceFactID = sourceFactID
	}
	if invocation.ProviderRequestID == "" {
		invocation.ProviderRequestID = "provider:" + stableDigest(identityScope+":"+invocation.CallID)
	}
	if invocation.Sequence < 0 {
		invocation.Sequence = sequence
	}
	if invocation.Metadata.FluctlightID == "" {
		invocation.Metadata.FluctlightID = fluctlightID
	}
	if invocation.Metadata.ConversationID == "" {
		invocation.Metadata.ConversationID = conversationID
	}
	if invocation.Metadata.Surface == "" {
		invocation.Metadata.Surface = CapabilitySurfaceConversation
	}
	if invocation.Intent == "" {
		var args map[string]any
		if json.Unmarshal(invocation.Arguments, &args) == nil {
			invocation.Intent = stringValue(args["intent"])
		}
	}
	if invocation.SchemaVersion == "" {
		invocation.SchemaVersion = CapabilityInvocationSchemaVersion
	}
	return invocation
}

func (a *App) executeExclusiveCapabilityCanonical(ctx context.Context, fluctlightID string, invocation CapabilityInvocation, capability Capability) (CapabilityResult, error) {
	conn, err := a.DB.Pool().Acquire(ctx)
	if err != nil {
		return failedCapabilityResultDetail(invocation, "capability_busy", true, err.Error()), err
	}
	defer conn.Release()
	var acquired bool
	lockKey := "capability:" + fluctlightID + ":" + invocation.CapabilityName
	if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock(hashtext($1))`, lockKey).Scan(&acquired); err != nil {
		return failedCapabilityResultDetail(invocation, "capability_busy", true, err.Error()), err
	}
	if !acquired {
		return failedCapabilityResultDetail(invocation, "capability_busy", true, "capability is already running"), errors.New("capability_busy")
	}
	defer func() {
		unlockCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
		defer cancel()
		_, _ = conn.Exec(unlockCtx, `SELECT pg_advisory_unlock(hashtext($1))`, lockKey)
	}()
	return a.capabilityRuntime().Execute(ctx, invocation)
}

func requiredCapabilityFailureCanonical(results []CapabilityResult, invocations []CapabilityInvocation, registry *CapabilityRegistry, settled bool) error {
	if registry == nil {
		return newCapabilityError("capability_not_found", false, fmt.Errorf("%w: registry is unavailable", ErrCapabilityNotFound))
	}
	for _, invocation := range invocations {
		definition, ok := registry.Definition(invocation.CapabilityName)
		if !ok {
			return newCapabilityError("capability_not_found", false, fmt.Errorf("required capability %q is unavailable", invocation.CapabilityName))
		}
		if definition.FailurePolicy != FailurePolicyRequiredForVisibleClaim {
			continue
		}
		result, found := capabilityResultForCall(results, invocation.CallID)
		if !found {
			return newCapabilityError("capability_result_missing", false, fmt.Errorf("required capability %q has no result", invocation.CapabilityName))
		}
		if result.CallID != invocation.CallID || result.CapabilityName != invocation.CapabilityName {
			return newCapabilityError("capability_result_identity_mismatch", false, fmt.Errorf("required capability %q result identity mismatch", invocation.CapabilityName))
		}
		if result.Status == "failed" || result.Status == "rejected" || (settled && result.Status != "completed") {
			code := result.ErrorCode
			if code == "" {
				code = "capability_execution_failed"
			}
			return newCapabilityError(code, result.Retryable, fmt.Errorf("required capability %q failed", invocation.CapabilityName))
		}
		if !settled && result.Status != "completed" && result.Status != "deferred" {
			return newCapabilityError("capability_result_status_invalid", false, fmt.Errorf("required capability %q returned invalid status %q", invocation.CapabilityName, result.Status))
		}
	}
	return nil
}

func capabilityFailureInfo(err error, results []CapabilityResult, invocations []CapabilityInvocation, registry *CapabilityRegistry, fallbackCode string) (string, bool) {
	code, retryable := capabilityErrorInfo(err, fallbackCode, true)
	if errors.Is(err, ErrCapabilityNotFound) || errors.Is(err, ErrInvalidArguments) || errors.Is(err, ErrConflict) {
		retryable = false
	}
	resultByCall := make(map[string]CapabilityResult, len(results))
	for _, result := range results {
		resultByCall[result.CallID] = result
	}
	for _, invocation := range invocations {
		definition, found := registry.Definition(invocation.CapabilityName)
		if !found || definition.FailurePolicy != FailurePolicyRequiredForVisibleClaim {
			continue
		}
		result, found := resultByCall[invocation.CallID]
		if !found || (result.Status != "failed" && result.Status != "rejected") {
			continue
		}
		if strings.TrimSpace(result.ErrorCode) != "" {
			code = strings.TrimSpace(result.ErrorCode)
		}
		if !result.Retryable {
			return code, false
		}
	}
	return code, retryable
}

func stringSliceAny(values []string) []any {
	result := make([]any, len(values))
	for index, value := range values {
		result[index] = value
	}
	return result
}

// settleDeferredCapabilitiesTx performs the output-target phase for canonical
// invocations. It keeps external/provider work out of the transaction and
// returns a result for every deferred invocation, including bounded failures.
func (a *App) settleDeferredCapabilitiesTx(ctx context.Context, tx pgx.Tx, fluctlightID, sourceFactID, identityScope string, invocations []CapabilityInvocation, existing []CapabilityResult, binding OutputBindingV1) ([]CapabilityResult, error) {
	results := append([]CapabilityResult(nil), existing...)
	if len(invocations) == 0 {
		return results, nil
	}
	runtime := a.capabilityRuntime()
	if runtime == nil {
		for _, invocation := range invocations {
			results = replaceCapabilityResult(results, failedCapabilityResultDetail(invocation, "capability_not_found", false, "capability runtime is unavailable"))
		}
		return results, ErrCapabilityNotFound
	}
	for index := range invocations {
		invocation := normalizeCapabilityInvocationMetadata(invocations[index], fluctlightID, "", sourceFactID, identityScope, index)
		invocations[index] = invocation
		definition, ok := runtime.Registry.Definition(invocation.CapabilityName)
		if !ok {
			results = replaceCapabilityResult(results, failedCapabilityResultDetail(invocation, "capability_not_found", false, invocation.CapabilityName))
			continue
		}
		capability, found := runtime.Registry.LookupCapability(invocation.CapabilityName)
		if !found {
			results = replaceCapabilityResult(results, failedCapabilityResultDetail(invocation, "capability_not_found", false, "implementation is not registered"))
			continue
		}
		executionClass, classErr := classifyCapabilityExecution(capability, definition)
		if classErr != nil {
			results = replaceCapabilityResult(results, failedCapabilityResultDetail(invocation, "capability_execution_class_invalid", false, classErr.Error()))
			continue
		}
		_, transactional := capability.(TransactionalCapability)
		if executionClass != CapabilityExecutionTransactionalMutation && executionClass != CapabilityExecutionDeferredOutput && !(executionClass == CapabilityExecutionExternalAsyncIntent && (definition.IsDeferredOutput() || transactional)) {
			continue
		}
		if existingResult, found := capabilityResultForCall(results, invocation.CallID); found && existingResult.Status == "completed" {
			continue
		}
		if definition.IsDeferredOutput() && binding.TargetKind == "" {
			continue
		}
		var result CapabilityResult
		var err error
		if transactional {
			applyTx := tx
			var savepoint pgx.Tx
			if tx != nil {
				savepoint, err = tx.Begin(ctx)
				if err == nil {
					applyTx = savepoint
				}
			}
			if err == nil {
				result, err = runtime.ExecuteTransactional(ctx, applyTx, invocation)
			}
			if savepoint != nil {
				if err != nil {
					_ = savepoint.Rollback(ctx)
				} else if commitErr := savepoint.Commit(ctx); commitErr != nil {
					err = commitErr
				}
			}
		} else {
			callBinding := binding
			callBinding.ToolCallID = invocation.CallID
			invocation.Metadata.OutputBinding = &callBinding
			invocations[index] = invocation
			result, err = runtime.ExecuteDeferred(ctx, tx, invocation, callBinding)
		}
		if err != nil {
			// ExecuteDeferred already normalizes identity/status, but make the
			// transaction boundary fail-closed even for a faulty implementation.
			result.CallID = invocation.CallID
			result.CapabilityName = invocation.CapabilityName
			result.Status = "failed"
			if result.ErrorCode == "" {
				result.ErrorCode, result.Retryable = capabilityErrorInfo(err, "capability_execution_failed", true)
			}
		}
		if result.ProviderRequestID == "" {
			result.ProviderRequestID = invocation.ProviderRequestID
		}
		if result.CorrelationID == "" {
			result.CorrelationID = "capability:" + invocation.CallID
		}
		if result.Status == "completed" {
			if validationErr := definition.ValidateOutput(result.Output); validationErr != nil {
				result = failedCapabilityResultDetail(invocation, "capability_output_invalid", false, validationErr.Error())
			}
		}
		results = replaceCapabilityResult(results, result)
	}
	return results, nil
}

func capabilityResultForCall(results []CapabilityResult, callID string) (CapabilityResult, bool) {
	for _, result := range results {
		if result.CallID == callID {
			return result, true
		}
	}
	return CapabilityResult{}, false
}

func replaceCapabilityResult(results []CapabilityResult, replacement CapabilityResult) []CapabilityResult {
	for index := range results {
		if results[index].CallID == replacement.CallID {
			results[index] = replacement
			return results
		}
	}
	return append(results, replacement)
}

// capabilityResultsAfterSettlementFailure converts any output-target result
// that could not be committed into an explicit failed result. A transaction
// rollback means that even a capability which returned completed did not leave
// its durable target behind; replay must therefore retry/quarantine it instead
// of treating the provisional result as success.
func capabilityResultsAfterSettlementFailure(results []CapabilityResult, invocations []CapabilityInvocation, registry *CapabilityRegistry, code string) []CapabilityResult {
	if code == "" {
		code = "capability_settlement_failed"
	}
	settled := append([]CapabilityResult(nil), results...)
	if registry == nil {
		for _, invocation := range invocations {
			settled = replaceCapabilityResult(settled, failedCapabilityResult(invocation, code, true))
		}
		return settled
	}
	for _, invocation := range invocations {
		definition, ok := registry.Definition(invocation.CapabilityName)
		capability, capabilityFound := registry.LookupCapability(invocation.CapabilityName)
		_, transactional := capability.(TransactionalCapability)
		if !ok || (!definition.IsDeferredOutput() && (!capabilityFound || !transactional)) {
			continue
		}
		result, found := capabilityResultForCall(settled, invocation.CallID)
		if found && result.Status == "completed" {
			result.Status = "failed"
			result.ErrorCode = code
			result.Retryable = true
			result.CallID = invocation.CallID
			result.CapabilityName = invocation.CapabilityName
			if result.ProviderRequestID == "" {
				result.ProviderRequestID = invocation.ProviderRequestID
			}
			settled = replaceCapabilityResult(settled, result)
			continue
		}
		if !found || result.Status == "deferred" {
			settled = replaceCapabilityResult(settled, failedCapabilityResult(invocation, code, true))
		}
	}
	return settled
}

func capabilityResultsFromValue(value any) ([]CapabilityResult, error) {
	if value == nil {
		return []CapabilityResult{}, nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("capability result payload invalid: %w", err)
	}
	if string(data) == "null" {
		return []CapabilityResult{}, nil
	}
	var results []CapabilityResult
	if err := json.Unmarshal(data, &results); err != nil {
		return nil, fmt.Errorf("capability result payload invalid: %w", err)
	}
	seen := make(map[string]struct{}, len(results))
	for index := range results {
		if strings.TrimSpace(results[index].CallID) == "" || strings.TrimSpace(results[index].CapabilityName) == "" {
			return nil, fmt.Errorf("capability result %d identity is missing", index)
		}
		if _, duplicate := seen[results[index].CallID]; duplicate {
			return nil, fmt.Errorf("capability result %d call id is duplicated", index)
		}
		seen[results[index].CallID] = struct{}{}
		switch results[index].Status {
		case "completed", "failed", "rejected", "deferred":
		default:
			return nil, fmt.Errorf("capability result %d status is invalid", index)
		}
	}
	return results, nil
}

func mediaIntentIDFromCapabilityResults(results []CapabilityResult) string {
	for _, result := range results {
		if result.Status != "completed" {
			continue
		}
		if output := mapValue(result.Output); len(output) > 0 {
			if id := stringValue(output["media_intent_id"]); id != "" {
				return id
			}
		}
	}
	return ""
}

func hasConversationReplyCapability(invocations []CapabilityInvocation, registry *CapabilityRegistry) bool {
	if registry == nil {
		return false
	}
	for _, invocation := range invocations {
		if definition, ok := registry.Definition(invocation.CapabilityName); ok && definition.OutputRole == "conversation_message" {
			return true
		}
	}
	return false
}

func replyTextFromCapabilityInvocations(invocations []CapabilityInvocation, registry *CapabilityRegistry) string {
	if registry == nil {
		return ""
	}
	for _, invocation := range invocations {
		definition, ok := registry.Definition(invocation.CapabilityName)
		if !ok || definition.OutputRole != "conversation_message" {
			continue
		}
		var args map[string]any
		if json.Unmarshal(invocation.Arguments, &args) != nil {
			continue
		}
		if text := normalizeVisibleReply(stringValue(args["text"])); text != "" {
			return text
		}
	}
	return ""
}

func resolveCapabilityAction(invocations []CapabilityInvocation, definitions map[string]CapabilityDefinition) (string, error) {
	if len(invocations) == 0 {
		return "", errors.New("at least one capability invocation is required")
	}
	for _, invocation := range invocations {
		definition, ok := definitions[invocation.CapabilityName]
		if !ok {
			continue
		}
		if invocation.Metadata.OutputBinding != nil && invocation.Metadata.OutputBinding.TargetKind == "conversation_message" {
			return "reply", nil
		}
		if definition.OutputRole == "conversation_message" {
			return "reply", nil
		}
	}
	return "no_op", nil
}
