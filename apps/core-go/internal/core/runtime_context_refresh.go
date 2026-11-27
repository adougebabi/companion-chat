package core

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/cloudwego/eino/schema"
	capabilitycontract "github.com/fluctlight/local-ai-companion/apps/core-go/internal/capability"
)

type runtimeContextRefreshPlanKey struct{}

type modelContextRefreshContent struct {
	System        string
	Runtime       string
	SystemOrigin  promptMessageOrigin
	RuntimeOrigin promptMessageOrigin
	Projection    ContextProjection
	BudgetTrace   PromptAssemblyTrace
}

// The plan is bound by the host surface. It rebuilds only Provider data, using
// the same authorization and memory scope as the first physical request.
func withRuntimeContextRefreshPlan(ctx context.Context, refresh func(context.Context) (modelContextRefreshContent, error)) context.Context {
	return context.WithValue(ctx, runtimeContextRefreshPlanKey{}, refresh)
}

func runtimeContextRefreshPlan(ctx context.Context) func(context.Context) (modelContextRefreshContent, error) {
	refresh, _ := ctx.Value(runtimeContextRefreshPlanKey{}).(func(context.Context) (modelContextRefreshContent, error))
	return refresh
}

type runtimeContextRefresh struct {
	kevVersion         int64
	mu                 sync.Mutex
	systemOriginIndex  int
	runtimeOriginIndex int
	target             *schema.Message
	system             *schema.Message
	dirty              bool
	latest             *modelContextRefreshContent
	refresh            func(context.Context) (modelContextRefreshContent, error)
	base               ContextProjection
	batches            map[uint64]map[string]string
	staleReplyCallID   string
}

// A Schedule mutation and a visible reply from one physical model response
// were both composed against the old context. The reply must wait for the
// next model request, whose runtime context is rebuilt after the mutation.
func (r *runtimeContextRefresh) noteModelToolCalls(sequence uint64, calls []schema.ToolCall) {
	if r == nil || sequence == 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.batches == nil {
		r.batches = make(map[uint64]map[string]string)
	}
	if r.batches[sequence] == nil {
		r.batches[sequence] = make(map[string]string)
	}
	for _, call := range calls {
		if id, name := strings.TrimSpace(call.ID), strings.TrimSpace(call.Function.Name); id != "" && name != "" {
			name = capabilitycontract.CanonicalCapabilityName(name)
			r.batches[sequence][id] = name
		}
	}
}

func (r *runtimeContextRefresh) replyNeedsFreshSchedule(sequence uint64, callID string) bool {
	code, _, blocked := replyBlockedBySameBatchSideEffect(r, sequence, callID, nil)
	return blocked && code == "schedule_context_refresh_required"
}

// Occurrences describe delivery attempts, independently of the first execution
// identity kept by ADKCapabilityTrace for durable receipt replay.
func (r *runtimeContextRefresh) modelToolCallSequences(callID, capabilityName string) []uint64 {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	var sequences []uint64
	for sequence, calls := range r.batches {
		if name, found := calls[callID]; found && capabilitycontract.CapabilityNamesMatch(name, capabilityName) {
			sequences = append(sequences, sequence)
		}
	}
	return sequences
}

func (r *runtimeContextRefresh) sameBatch(sequence uint64, callID string) (map[string]string, bool) {
	if r == nil || sequence == 0 {
		return nil, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	batch := r.batches[sequence]
	if !capabilitycontract.CapabilityNamesMatch(batch[callID], conversationReplyCapabilityName) {
		return nil, false
	}
	copyBatch := make(map[string]string, len(batch))
	for id, name := range batch {
		copyBatch[id] = name
	}
	return copyBatch, true
}

// replyBlockedBySameBatchSideEffect defers a visible reply only when its words
// depend on the result of a same-batch schedule or wardrobe change. Recording
// tools (memory_event, actor.fact.record) and image requests do not change what
// the reply may say, so they run alongside it.
func replyBlockedBySameBatchSideEffect(refresh *runtimeContextRefresh, sequence uint64, callID string, _ *App) (string, string, bool) {
	batch, ok := refresh.sameBatch(sequence, callID)
	if !ok {
		return "", "", false
	}
	schedule := false
	wardrobe := false
	for id, name := range batch {
		if id == callID {
			continue
		}
		if name == scheduleActivityCapabilityName || name == "schedule.replan" || name == scheduleEditCapabilityName {
			schedule = true
			continue
		}
		if capabilitycontract.CapabilityNamesMatch(name, wardrobeWearCapabilityName) {
			wardrobe = true
		}
	}
	if schedule {
		return "schedule_context_refresh_required", "Wait for the Schedule Tool result and refreshed context, then compose the reply in the next model turn.", true
	}
	if wardrobe {
		return "wardrobe_context_refresh_required", "Wait for the actual wardrobe.wear receipt, then compose the reply from the refreshed current wearing in the next model turn. Do not claim a failed or uncommitted change.", true
	}
	return "", "", false
}

func (r *runtimeContextRefresh) latestProjection() *ContextProjection {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.latest == nil {
		return nil
	}
	projection := r.latest.Projection
	return &projection
}

func (r *runtimeContextRefresh) latestBudgetTrace() *PromptAssemblyTrace {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.latest == nil {
		return nil
	}
	trace := r.latest.BudgetTrace
	return &trace
}

func (r *runtimeContextRefresh) latestPromptOrigins() *modelContextRefreshContent {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.latest == nil {
		return nil
	}
	return &modelContextRefreshContent{SystemOrigin: r.latest.SystemOrigin, RuntimeOrigin: r.latest.RuntimeOrigin, BudgetTrace: r.latest.BudgetTrace}
}

func (r *runtimeContextRefresh) markDirty() {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.dirty = true
	r.mu.Unlock()
}

func (r *runtimeContextRefresh) markStaleReply(callID string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.dirty = true
	r.staleReplyCallID = callID
	r.mu.Unlock()
}

func isRuntimeContextMessage(message *schema.Message) bool {
	return message != nil && message.Role == schema.User && len(message.UserInputMultiContent) == 0 &&
		strings.HasPrefix(message.Content, "[RUNTIME CONTEXT]\n") && strings.HasSuffix(message.Content, "\n[/RUNTIME CONTEXT]")
}

func promptMessageOriginIndex(assembly PromptAssemblyResult, source string) int {
	if len(assembly.messageOrigins) != len(assembly.Messages) {
		return -1
	}
	index := -1
	for position, origin := range assembly.messageOrigins {
		if origin.Source == source {
			if index >= 0 {
				return -1
			}
			index = position
		}
	}
	return index
}

func modelContextContent(assembly PromptAssemblyResult) (modelContextRefreshContent, error) {
	content := modelContextRefreshContent{}
	systemIndex := promptMessageOriginIndex(assembly, "trusted_task_configuration")
	runtimeIndex := promptMessageOriginIndex(assembly, "scoped_runtime_projection")
	if systemIndex >= 0 && stringValue(assembly.Messages[systemIndex]["role"]) == "system" {
		content.System = stringValue(assembly.Messages[systemIndex]["content"])
		content.SystemOrigin = assembly.messageOrigins[systemIndex]
	}
	if runtimeIndex >= 0 && stringValue(assembly.Messages[runtimeIndex]["role"]) == "user" {
		content.Runtime = stringValue(assembly.Messages[runtimeIndex]["content"])
		content.RuntimeOrigin = assembly.messageOrigins[runtimeIndex]
	}
	if content.System == "" || !strings.HasPrefix(content.Runtime, "[RUNTIME CONTEXT]\n") || !strings.HasSuffix(content.Runtime, "\n[/RUNTIME CONTEXT]") {
		return modelContextRefreshContent{}, errors.New("model_context_refresh_content_missing")
	}
	return content, nil
}

// prepare changes only the outbound copy. Eino owns the original pointers and
// the assistant ToolCall/tool-result history; neither may be edited to refresh
// a runtime fact after a committed Tool mutation.
func (r *runtimeContextRefresh) prepare(ctx context.Context, original []*schema.Message) ([]*schema.Message, error) {
	copyMessages := normalizeEinoToolMessageNames(original)
	if r == nil {
		return copyMessages, nil
	}
	r.mu.Lock()
	if r.target == nil {
		if r.systemOriginIndex >= 0 && r.systemOriginIndex < len(original) {
			message := original[r.systemOriginIndex]
			if message != nil && message.Role == schema.System {
				r.system = message
			}
		}
		if r.runtimeOriginIndex >= 0 && r.runtimeOriginIndex < len(original) {
			message := original[r.runtimeOriginIndex]
			if isRuntimeContextMessage(message) {
				r.target = message
			}
		}
	}
	target := r.target
	system := r.system
	dirty := r.dirty
	latest := r.latest
	staleReplyCallID := r.staleReplyCallID
	refresh := r.refresh
	r.mu.Unlock()
	if !dirty && latest == nil {
		return copyMessages, nil
	}
	if target == nil || system == nil || (dirty && refresh == nil) {
		return nil, errors.New("runtime_context_refresh_unavailable")
	}
	index := -1
	systemIndex := -1
	for position, message := range original {
		if message == system {
			systemIndex = position
		}
		if message == target {
			index = position
		}
	}
	if index < 0 || systemIndex < 0 || !isRuntimeContextMessage(target) {
		return nil, errors.New("runtime_context_target_missing")
	}
	var content modelContextRefreshContent
	if dirty {
		refreshed, err := refresh(ctx)
		if err != nil {
			return nil, err
		}
		content = refreshed
		content.Projection.AuthorityAtRunStart = &CognitionAuthorityAtRunStart{
			Foundation: r.base.ContextRevision, CurrentState: r.base.CurrentStateRevision,
			CurrentFacts: r.base.CurrentFactsRevision, LifeContext: r.base.LifeContextRevision,
		}
		if latest != nil && latest.Projection.AuthorityAtRunStart != nil {
			anchor := *latest.Projection.AuthorityAtRunStart
			content.Projection.AuthorityAtRunStart = &anchor
		}
		if staleReplyCallID != "" {
			adkContext, ok := adkCapabilityContext(ctx)
			if !ok || adkContext.Trace == nil {
				return nil, errors.New("stale_reply_recovery_trace_missing")
			}
			invocations, _ := adkContext.Trace.Snapshot()
			if len(invocations) == 0 {
				return nil, errors.New("stale_reply_recovery_trace_missing")
			}
			content.Projection.AuthorityAtRunStart = &CognitionAuthorityAtRunStart{
				Foundation: content.Projection.ContextRevision, CurrentState: content.Projection.CurrentStateRevision,
				CurrentFacts: content.Projection.CurrentFactsRevision, LifeContext: content.Projection.LifeContextRevision,
				AfterRecoveryCallID: invocations[len(invocations)-1].CallID, StaleReplyCallID: staleReplyCallID,
			}
		}
		if content.Projection.FluctlightID != "" && r.base.ReferenceIndex.expectedScope() == content.Projection.ReferenceIndex.expectedScope() {
			for ref, entry := range r.base.ReferenceIndex.ByRef {
				if _, present := content.Projection.ReferenceIndex.ByRef[ref]; !present {
					content.Projection.ReferenceIndex.ByRef[ref] = entry
				}
			}
			if err := content.Projection.ReferenceIndex.Validate(); err != nil {
				return nil, err
			}
		}
		// Final validation must use the same scope/index as settlement, including
		// the original references preserved above. Alias decoding may retain old
		// mappings, but an old profile's ref is not automatically valid now.
		if adkContext, ok := adkCapabilityContext(ctx); ok {
			if err := adkContext.Refs.registerIndex(content.Projection.ReferenceIndex); err != nil {
				return nil, err
			}
		}
	} else {
		content = *latest
	}
	if content.System == "" || !strings.HasPrefix(content.Runtime, "[RUNTIME CONTEXT]\n") || !strings.HasSuffix(content.Runtime, "\n[/RUNTIME CONTEXT]") {
		return nil, errors.New("runtime_context_refresh_invalid")
	}
	copyMessages[systemIndex].Content = content.System
	copyMessages[index].Content = content.Runtime
	if dirty {
		r.mu.Lock()
		r.latest = &content
		r.dirty = false
		r.staleReplyCallID = ""
		r.mu.Unlock()
	}
	return copyMessages, nil
}
